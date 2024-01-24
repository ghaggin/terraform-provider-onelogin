package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ghaggin/terraform-provider-onelogin/internal/util"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &oneloginUserDataSource{}
var _ datasource.DataSourceWithConfigure = &oneloginUserDataSource{}

type oneloginUserDataSource struct {
	client *client
}

type oneloginUserModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Username    types.String `tfsdk:"username"`
	LastUpdated types.String `tfsdk:"last_updated"`
}

type oneloginNativeUserModel struct {
	ID       int64  `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
}

// OneLogin User Datasource

func NewOneLoginUserDataSource(client *client) newDataSourceFunc {
	return func() datasource.DataSource {
		return &oneloginUserDataSource{
			client: client,
		}
	}
}

func (d *oneloginUserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *oneloginUserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "OneLogin User data source",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.Int64Attribute{
				Computed: true,
			},
			"username": dschema.StringAttribute{
				Required: true,
			},
			"last_updated": dschema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *oneloginUserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
}

func (d *oneloginUserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data oneloginUserModel

	// Read Terraform configuration data into the model
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query OneLogin API for user
	username := data.Username.ValueString()
	// users, err := d.client.ListUsers(&onelogin.UserQuery{Username: username})
	var users []oneloginNativeUserModel

	err := d.client.execRequest(&oneloginRequest{
		method: methodGet,
		path:   pathUsers,
		queryParams: queryParams{
			"username": username,
		},
		respModel: &users,
	})
	if err != nil || len(users) == 0 {
		resp.Diagnostics.AddError(
			"client error",
			fmt.Sprintf("Unable to read user %s, got error: %s", username, err),
		)
		return
	} else if len(users) > 1 {
		resp.Diagnostics.AddError(
			"client error",
			fmt.Sprintf("Found multiple users with username %s", username),
		)
		return
	}

	// Set data values from client response
	data.ID = types.Int64Value(users[0].ID)
	data.Username = types.StringValue(users[0].Username)

	// Update state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// OneLogin User Resource

type oneloginUserResource struct {
	client *client
}

func NewOneLoginUserResource(client *client) newResourceFunc {
	return func() resource.Resource {
		return &oneloginUserResource{
			client: client,
		}
	}
}

func (r *oneloginUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *oneloginUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		MarkdownDescription: "OneLogin User resource",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"username": rschema.StringAttribute{
				Required: true,
			},
			"last_updated": rschema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *oneloginUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data oneloginUserModel

	// Read Terraform configuration data into the model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var userResp oneloginNativeUserModel
	err := r.client.execRequestCtx(ctx, &oneloginRequest{
		method:    methodPost,
		path:      pathUsers,
		respModel: &userResp,
		body: &oneloginNativeUserModel{
			Username: data.Username.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"client error",
			fmt.Sprintf("Unable to create user %s, got error: %s", data.Username.ValueString(), err),
		)
		return
	}

	// Set data values from client response
	data.ID = types.Int64Value(userResp.ID)
	data.Username = types.StringValue(userResp.Username)
	data.LastUpdated = types.StringValue(util.GetTimestampString())

	// Update state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *oneloginUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oneloginUserModel

	// Read Terraform configuration data into the model
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.read(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *oneloginUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data oneloginUserModel

	// Retrieve values from plan
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.ValueInt64() == 0 {
		resp.Diagnostics.AddError(
			"client error",
			"Unable to update user, no ID provided",
		)
		return
	}

	// Update user
	var userResp oneloginNativeUserModel
	err := r.client.execRequestCtx(ctx, &oneloginRequest{
		method:    methodPut,
		path:      fmt.Sprintf("%s/%v", pathUsers, data.ID.ValueInt64()),
		respModel: &userResp,
		body: &oneloginNativeUserModel{
			Username: data.Username.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"client error",
			fmt.Sprintf("Unable to update user %v, got error: %s", data.ID.ValueInt64(), err),
		)
		return
	}

	// Set data values from client response
	data.Username = types.StringValue(userResp.Username)
	data.LastUpdated = types.StringValue(util.GetTimestampString())

	// Update state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *oneloginUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data oneloginUserModel

	// Retrieve values from plan
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete user
	err := r.client.execRequestCtx(ctx, &oneloginRequest{
		method: methodDelete,
		path:   fmt.Sprintf("%s/%v", pathUsers, data.ID.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"client error",
			fmt.Sprintf("Unable to delete user %v, got error: %s", data.ID.ValueInt64(), err),
		)
		return
	}
}

func (r *oneloginUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing ID for import user",
			"Could not parse ID "+req.ID+": "+err.Error(),
		)
		return
	}

	state := oneloginUserModel{
		ID: types.Int64Value(int64(id)),
	}

	r.read(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *oneloginUserResource) read(ctx context.Context, state *oneloginUserModel, respState *tfsdk.State, d *diag.Diagnostics) {
	var user oneloginNativeUserModel

	id := state.ID.ValueInt64()

	err := r.client.execRequest(&oneloginRequest{
		method:    methodGet,
		path:      fmt.Sprintf("%s/%v", pathUsers, id),
		respModel: &user,
	})
	if err != nil {
		d.AddError(
			"client error",
			fmt.Sprintf("Unable to read user %v, got error: %s", id, err),
		)
		return
	}

	// Set data values from client response
	state.ID = types.Int64Value(user.ID)
	state.Username = types.StringValue(user.Username)

	// Update state
	diags := respState.Set(ctx, &state)
	d.Append(diags...)
}
