// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/onelogin/onelogin-go-sdk/pkg/client"
	"github.com/onelogin/onelogin-go-sdk/pkg/services/users"
)

var _ datasource.DataSource = &oneLoginUser{}
var _ datasource.DataSourceWithConfigure = &oneLoginUser{}

func NewOneLoginUser() datasource.DataSource {
	return &oneLoginUser{}
}

type oneLoginUser struct {
	client *client.APIClient
}

type oneLoginUserModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Username types.String `tfsdk:"username"`
}

func (d *oneLoginUser) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *oneLoginUser) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "OneLogin user data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				MarkdownDescription: "OneLogin object ID",
				Computed:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Username",
				Required:            true,
			},
		},
	}
}

func (d *oneLoginUser) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *oneLoginUser) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data oneLoginUserModel

	// Read Terraform configuration data into the model
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query OneLogin API for user
	username := data.Username.ValueString()
	users, err := d.client.Services.UsersV2.Query(&users.UserQuery{
		Limit:    "2",
		Username: &username,
	})
	if err != nil {
		resp.Diagnostics.AddError("client error", fmt.Sprintf("Unable to read user %s, got error: %s", username, err))
		return
	}

	// Datasource should only return one user
	if len(users) == 2 {
		resp.Diagnostics.AddError("client error", fmt.Sprintf("Found multiple users with username %s", username))
		return
	}

	// Log no user found and return
	if len(users) == 0 {
		tflog.Info(ctx, fmt.Sprintf("No user found with username %s", username))
		return
	}

	// Set data values from client response
	data.ID = types.Int64Value(int64(*users[0].ID))
	data.Username = types.StringValue(*users[0].Username)

	// Update state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
