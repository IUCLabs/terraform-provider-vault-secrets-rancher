package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secretBackendServiceAccountResource{}
	_ resource.ResourceWithImportState = &secretBackendServiceAccountResource{}
)

type secretBackendServiceAccountResource struct {
	client *vaultClient
}

type secretBackendServiceAccountModel struct {
	Backend        types.String `tfsdk:"backend"`
	Name           types.String `tfsdk:"name"`
	Token          types.String `tfsdk:"token"`
	TokenName      types.String `tfsdk:"token_name"`
	TokenTTL       types.Int64  `tfsdk:"token_ttl"`
	RotationPeriod types.Int64  `tfsdk:"rotation_period"`
	LastRotated    types.String `tfsdk:"last_rotated"`
}

func NewSecretBackendServiceAccountResource() resource.Resource {
	return &secretBackendServiceAccountResource{}
}

func (r *secretBackendServiceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_backend_service_account"
}

func (r *secretBackendServiceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Seeds a Rancher service-account token that the engine auto-rotates before Rancher's TTL cap.",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description: "Mount path of the Rancher secrets engine.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the service account.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token": schema.StringAttribute{
				Description: "Rancher API token to seed with. Write-only; the engine rotates it from here on and never returns it.",
				Required:    true,
				Sensitive:   true,
			},
			"token_name": schema.StringAttribute{
				Description: "ext.cattle.io Token resource name (metadata.name) of the seed token, so the engine can delete it after the first rotation.",
				Optional:    true,
				Computed:    true,
			},
			"token_ttl": schema.Int64Attribute{
				Description: "Lifetime in seconds requested for each rotated replacement token (default 90d). Must not exceed Rancher's auth-token-max-ttl-minutes.",
				Optional:    true,
				Computed:    true,
			},
			"rotation_period": schema.Int64Attribute{
				Description: "Seconds a token is used before rotation (default 45d). Must be less than token_ttl.",
				Optional:    true,
				Computed:    true,
			},
			"last_rotated": schema.StringAttribute{
				Description: "RFC3339 timestamp of the current token's issuance (computed).",
				Computed:    true,
			},
		},
	}
}

func (r *secretBackendServiceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *secretBackendServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretBackendServiceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.writeAndRead(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan secretBackendServiceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.writeAndRead(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// writeAndRead writes the service account then refreshes computed fields from
// the backend.
func (r *secretBackendServiceAccountResource) writeAndRead(ctx context.Context, plan *secretBackendServiceAccountModel) diag.Diagnostics {
	var diags diag.Diagnostics
	data := map[string]interface{}{"token": plan.Token.ValueString()}
	if !plan.TokenName.IsNull() && !plan.TokenName.IsUnknown() {
		data["token_name"] = plan.TokenName.ValueString()
	}
	if !plan.TokenTTL.IsNull() && !plan.TokenTTL.IsUnknown() {
		data["token_ttl"] = plan.TokenTTL.ValueInt64()
	}
	if !plan.RotationPeriod.IsNull() && !plan.RotationPeriod.IsUnknown() {
		data["rotation_period"] = plan.RotationPeriod.ValueInt64()
	}

	if err := r.client.write(ctx, serviceAccountPath(plan.Backend.ValueString(), plan.Name.ValueString()), data); err != nil {
		diags.AddError("failed to write rancher service account", err.Error())
		return diags
	}
	sa, err := r.client.read(ctx, serviceAccountPath(plan.Backend.ValueString(), plan.Name.ValueString()))
	if err != nil {
		diags.AddError("failed to read back rancher service account", err.Error())
		return diags
	}
	if sa == nil {
		diags.AddError("service account missing after write", "not found immediately after being written")
		return diags
	}
	applyServiceAccount(plan, sa)
	return diags
}

func (r *secretBackendServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretBackendServiceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := r.client.read(ctx, serviceAccountPath(state.Backend.ValueString(), state.Name.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("failed to read rancher service account", err.Error())
		return
	}
	if sa == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	applyServiceAccount(&state, sa)
	// token is never returned by the backend; preserve the state value.
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *secretBackendServiceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretBackendServiceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, serviceAccountPath(state.Backend.ValueString(), state.Name.ValueString())); err != nil {
		resp.Diagnostics.AddError("failed to delete rancher service account", err.Error())
		return
	}
}

func (r *secretBackendServiceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	backend, name, ok := splitBackendName(req.ID, "service-accounts")
	if !ok {
		resp.Diagnostics.AddError(
			"invalid import ID",
			fmt.Sprintf("expected \"<backend>/service-accounts/<name>\", got %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("backend"), backend)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

func applyServiceAccount(m *secretBackendServiceAccountModel, sa map[string]interface{}) {
	if v, ok := sa["token_name"].(string); ok {
		m.TokenName = types.StringValue(v)
	}
	if n, ok := toInt64(sa["token_ttl"]); ok {
		m.TokenTTL = types.Int64Value(n)
	}
	if n, ok := toInt64(sa["rotation_period"]); ok {
		m.RotationPeriod = types.Int64Value(n)
	}
	if v, ok := sa["last_rotated"].(string); ok {
		m.LastRotated = types.StringValue(v)
	}
}
