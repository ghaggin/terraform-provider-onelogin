package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &oneloginProvider{}

type newResourceFunc func() resource.Resource
type newDataSourceFunc func() datasource.DataSource

// ScaffoldingProvider defines the provider implementation.
type oneloginProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string

	client client
}

// ScaffoldingProviderModel describes the provider data model.
type oneLoginProviderModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	CLientSecret types.String `tfsdk:"client_secret"`
	Subdomain    types.String `tfsdk:"subdomain"`
	Region       types.String `tfsdk:"region"`
}

func (p *oneloginProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "onelogin"
	resp.Version = p.version
}

func (p *oneloginProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Admin oauth client id",
				Required:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Admin oauth client secret",
				Required:            true,
				Sensitive:           true,
			},
			"subdomain": schema.StringAttribute{
				MarkdownDescription: "Instance subdomain",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Region",
				Optional:            true,
			},
		},
	}
}

func (p *oneloginProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data oneLoginProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	// if data.Endpoint.IsNull() { /* ... */ }
	if data.ClientID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_id"), "client_id is required", "")
	}

	if data.CLientSecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret"), "client_secret is required", "")
	}

	if data.Subdomain.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("url"), "url is required", "")
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := newClient(&clientConfig{
		clientID:     data.ClientID.ValueString(),
		clientSecret: data.CLientSecret.ValueString(),
		subdomain:    data.Subdomain.ValueString(),

		// This needs to be high because some operations are very slow,
		// but still complete after context cancellation, which leaves
		// the state inconsistent.
		timeout: 60 * time.Second,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create client", err.Error())
		return
	}

	p.client = *client
}

func (p *oneloginProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewOneLoginRoleResource(&p.client),
		NewOneLoginAppResource(&p.client),
		NewOneLoginUserResource(&p.client),
	}
}

func (p *oneloginProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOneLoginUserDataSource(&p.client),
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &oneloginProvider{
			version: version,
		}
	}
}
