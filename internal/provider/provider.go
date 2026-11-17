package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &rancherProvider{}

type rancherProvider struct {
	version string
}

type rancherProviderModel struct {
	Address types.String `tfsdk:"address"`
	Token   types.String `tfsdk:"token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &rancherProvider{version: version}
	}
}

func (p *rancherProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	// The provider's source address is IUCLabs/vault-secrets-rancher,
	// but its resources are prefixed "rancher_" (declare it in required_providers
	// under the local name "rancher"), mirroring how google-beta ships google_*.
	resp.TypeName = "rancher"
	resp.Version = p.version
}

func (p *rancherProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage the Rancher token secrets engine (config, service accounts, and roles) on HashiCorp Vault or OpenBao.",
		MarkdownDescription: "Manage the [Rancher token secrets engine](https://github.com/IUCLabs/vault-plugin-secrets-rancher) " +
			"(config, service accounts, and roles) on HashiCorp Vault or OpenBao. The plugin must already be registered in the " +
			"server's plugin catalog.\n\nResources are prefixed `rancher_`, so declare the provider under the local name `rancher`.",
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{
				Description: "Address of the Vault/OpenBao server. Falls back to the VAULT_ADDR environment variable.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "Token used to authenticate to Vault/OpenBao. Falls back to the VAULT_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *rancherProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config rancherProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	address := os.Getenv("VAULT_ADDR")
	if !config.Address.IsNull() && config.Address.ValueString() != "" {
		address = config.Address.ValueString()
	}

	token := os.Getenv("VAULT_TOKEN")
	if !config.Token.IsNull() && config.Token.ValueString() != "" {
		token = config.Token.ValueString()
	}

	if address == "" {
		resp.Diagnostics.AddError(
			"missing Vault address",
			"Set the provider \"address\" attribute or the VAULT_ADDR environment variable.",
		)
		return
	}

	client, err := newVaultClient(address, token)
	if err != nil {
		resp.Diagnostics.AddError("failed to create Vault client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *rancherProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSecretBackendResource,
		NewSecretBackendServiceAccountResource,
		NewSecretBackendRoleResource,
	}
}

func (p *rancherProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
