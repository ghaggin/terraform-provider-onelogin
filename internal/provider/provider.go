// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/ghaggin/terraform-provider-onelogin/internal/datasources"
	"github.com/ghaggin/terraform-provider-onelogin/internal/resources"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/onelogin/onelogin-go-sdk/pkg/client"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &oneloginProvider{}

// ScaffoldingProvider defines the provider implementation.
type oneloginProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ScaffoldingProviderModel describes the provider data model.
type oneLoginProviderModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	CLientSecret types.String `tfsdk:"client_secret"`
	URL          types.String `tfsdk:"url"`
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
			"url": schema.StringAttribute{
				MarkdownDescription: "Instance url",
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

	if data.URL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("url"), "url is required", "")
	}

	region := client.USRegion
	if !data.Region.IsUnknown() {
		region = data.Region.ValueString()
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := client.NewClient(&client.APIClientConfig{
		Timeout:      client.DefaultTimeout,
		ClientID:     data.ClientID.ValueString(),
		ClientSecret: data.CLientSecret.ValueString(),
		Url:          data.URL.ValueString(),
		Region:       region,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create OneLogin client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *oneloginProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewOneLoginRole,
	}
}

func (p *oneloginProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewOneLoginUser,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &oneloginProvider{
			version: version,
		}
	}
}
