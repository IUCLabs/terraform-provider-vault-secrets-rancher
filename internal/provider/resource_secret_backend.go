package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secretBackendResource{}
	_ resource.ResourceWithImportState = &secretBackendResource{}
)

const defaultPluginType = "vault-plugin-secrets-rancher"

type secretBackendResource struct {
	client *vaultClient
}

type secretBackendModel struct {
	Path                  types.String `tfsdk:"path"`
	Plugin                types.String `tfsdk:"plugin"`
	Description           types.String `tfsdk:"description"`
	RancherURL            types.String `tfsdk:"rancher_url"`
	CACert                types.String `tfsdk:"ca_cert"`
	TLSSkipVerify         types.Bool   `tfsdk:"tls_skip_verify"`
	RequestTimeoutSeconds types.Int64  `tfsdk:"request_timeout_seconds"`
}

func NewSecretBackendResource() resource.Resource {
	return &secretBackendResource{}
}

func (r *secretBackendResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_backend"
}

func (r *secretBackendResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Mounts the Rancher secrets engine and writes its connection config.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Description: "Mount path for the Rancher secrets engine (e.g. \"rancher\").",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plugin": schema.StringAttribute{
				Description: "Registered plugin name/type to mount.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(defaultPluginType),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Human-readable description of the mount.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"rancher_url": schema.StringAttribute{
				Description: "Base URL of the Rancher server (e.g. https://rancher.example.com).",
				Required:    true,
			},
			"ca_cert": schema.StringAttribute{
				Description: "PEM CA certificate that signed the Rancher server's TLS certificate.",
				Optional:    true,
			},
			"tls_skip_verify": schema.BoolAttribute{
				Description: "Skip TLS verification of the Rancher server (not recommended).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"request_timeout_seconds": schema.Int64Attribute{
				Description: "HTTP timeout in seconds for calls from the plugin to Rancher.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(30),
			},
		},
	}
}

func (r *secretBackendResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*vaultClient)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *secretBackendResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretBackendModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(plan.Path.ValueString(), "/")

	err := r.client.enableMount(ctx, mountPath, plan.Plugin.ValueString(), plan.Description.ValueString(), mountConfig{})
	if err != nil {
		if isMountAlreadyExists(err) {
			resp.Diagnostics.AddError(
				"mount path already in use",
				fmt.Sprintf("A secrets engine is already mounted at %q. Import it or choose another path.", mountPath),
			)
			return
		}
		resp.Diagnostics.AddError("failed to enable rancher secrets engine", err.Error())
		return
	}

	if err := r.client.write(ctx, configPath(mountPath), r.configData(plan)); err != nil {
		// Roll back the mount so we don't leave a half-configured engine.
		_ = r.client.disableMount(ctx, mountPath)
		resp.Diagnostics.AddError("failed to write rancher config", err.Error())
		return
	}

	plan.Path = types.StringValue(mountPath)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretBackendModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(state.Path.ValueString(), "/")

	mount, err := r.client.mountInfo(ctx, mountPath)
	if err != nil {
		resp.Diagnostics.AddError("failed to read mount", err.Error())
		return
	}
	if mount == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Description = types.StringValue(mount.Description)
	if mount.Type != "" {
		state.Plugin = types.StringValue(mount.Type)
	}

	cfg, err := r.client.read(ctx, configPath(mountPath))
	if err != nil {
		resp.Diagnostics.AddError("failed to read rancher config", err.Error())
		return
	}
	if cfg != nil {
		if v, ok := cfg["rancher_url"].(string); ok {
			state.RancherURL = types.StringValue(v)
		}
		if v, ok := cfg["tls_skip_verify"].(bool); ok {
			state.TLSSkipVerify = types.BoolValue(v)
		}
		if n, ok := toInt64(cfg["request_timeout_seconds"]); ok {
			state.RequestTimeoutSeconds = types.Int64Value(n)
		}
	}
	// ca_cert is not returned by the backend; preserve the state value.

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *secretBackendResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state secretBackendModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(state.Path.ValueString(), "/")

	if !plan.Description.Equal(state.Description) {
		if err := r.client.tuneMount(ctx, mountPath, plan.Description.ValueString(), mountConfig{}); err != nil {
			resp.Diagnostics.AddError("failed to tune mount description", err.Error())
			return
		}
	}

	if err := r.client.write(ctx, configPath(mountPath), r.configData(plan)); err != nil {
		resp.Diagnostics.AddError("failed to update rancher config", err.Error())
		return
	}

	plan.Path = types.StringValue(mountPath)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretBackendModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.disableMount(ctx, strings.Trim(state.Path.ValueString(), "/")); err != nil {
		resp.Diagnostics.AddError("failed to disable rancher secrets engine", err.Error())
		return
	}
}

func (r *secretBackendResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("path"), req, resp)
}

func (r *secretBackendResource) configData(m secretBackendModel) map[string]interface{} {
	data := map[string]interface{}{
		"rancher_url":             m.RancherURL.ValueString(),
		"tls_skip_verify":         m.TLSSkipVerify.ValueBool(),
		"request_timeout_seconds": m.RequestTimeoutSeconds.ValueInt64(),
	}
	if !m.CACert.IsNull() && !m.CACert.IsUnknown() {
		data["ca_cert"] = m.CACert.ValueString()
	}
	return data
}
