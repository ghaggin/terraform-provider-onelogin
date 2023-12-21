package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ghaggin/terraform-provider-onelogin/internal/util"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/onelogin/onelogin-go-sdk/pkg/client"
	"github.com/onelogin/onelogin-go-sdk/pkg/services/roles"
)

var (
	_ resource.Resource                = &oneLoginRole{}
	_ resource.ResourceWithConfigure   = &oneLoginRole{}
	_ resource.ResourceWithImportState = &oneLoginRole{}
)

func NewOneLoginRole() resource.Resource {
	return &oneLoginRole{}
}

type oneLoginRole struct {
	client *client.APIClient
}

type oneLoginRoleModel struct {
	ID          types.Int64  `tfsdk:"id"`
	LastUpdated types.String `tfsdk:"last_updated"`
	Name        types.String `tfsdk:"name"`
	Admins      types.List   `tfsdk:"admins"`
	Apps        types.List   `tfsdk:"apps"`
	Users       types.List   `tfsdk:"users"`
}

func (d *oneLoginRole) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *oneLoginRole) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (d *oneLoginRole) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				MarkdownDescription: "OneLogin object ID",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"last_updated": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the last time this role was updated",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the role",
				Required:            true,
			},
			"admins": schema.ListAttribute{
				ElementType:         types.Int64Type,
				MarkdownDescription: "List of admin IDs",
				Optional:            true,
			},
			"apps": schema.ListAttribute{
				ElementType:         types.Int64Type,
				MarkdownDescription: "List of app IDs",
				Optional:            true,
			},
			"users": schema.ListAttribute{
				ElementType:         types.Int64Type,
				MarkdownDescription: "List of user IDs",
				Optional:            true,
			},
		},
	}
}

func (d *oneLoginRole) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var state oneLoginRoleModel
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := d.roleFromState(&state)
	err := d.client.Services.RolesV1.Create(role)

	if err != nil || role.ID == nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			"Could not create role: "+err.Error(),
		)
		return
	}

	newState, diags := d.read(ctx, int64(*role.ID))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState.LastUpdated = types.StringValue(util.GetTimestampString())

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginRole) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oneLoginRoleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := d.read(ctx, state.ID.ValueInt64())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginRole) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state oneLoginRoleModel
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := d.roleFromState(&state)
	err := d.client.Services.RolesV1.Update(role)

	if err != nil || role.ID == nil {
		resp.Diagnostics.AddError(
			"Error updating role",
			"Could not update role: "+err.Error(),
		)
		return
	}

	newState, diags := d.read(ctx, int64(*role.ID))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState.LastUpdated = types.StringValue(util.GetTimestampString())

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginRole) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oneLoginRoleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := d.client.Services.RolesV1.Destroy(int32(state.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting role",
			"Could not delete role with ID "+state.ID.String()+": "+err.Error(),
		)
		return
	}
}

func (d *oneLoginRole) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing role", "Could not convert provided id to int: "+err.Error())
		return
	}

	state, diags := d.read(ctx, int64(id))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginRole) roleFromState(state *oneLoginRoleModel) *roles.Role {
	admins := []int32{}
	if !state.Admins.IsNull() {
		state.Admins.ElementsAs(context.Background(), &admins, false)
	}

	apps := []int32{}
	if !state.Apps.IsNull() {
		state.Apps.ElementsAs(context.Background(), &apps, false)
	}

	users := []int32{}
	if !state.Users.IsNull() {
		state.Users.ElementsAs(context.Background(), &users, false)
	}

	var id *int32 = nil
	if !state.ID.IsNull() && !state.ID.IsUnknown() {
		tmp := int32(state.ID.ValueInt64())
		id = &tmp
	}

	return &roles.Role{
		ID:     id,
		Name:   state.Name.ValueStringPointer(),
		Admins: admins,
		Apps:   apps,
		Users:  users,
	}
}

func (d *oneLoginRole) read(ctx context.Context, id int64) (*oneLoginRoleModel, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	role, err := d.client.Services.RolesV1.GetOne(int32(id))
	if err != nil {
		diags.AddError("Error reading role", "Could not read role with ID "+strconv.Itoa(int(id))+": "+err.Error())
		return nil, diags
	}

	if role.ID == nil {
		diags.AddError("Error reading role", "Could not read role with ID "+strconv.Itoa(int(id))+": role not found")
		return nil, diags
	}

	state := &oneLoginRoleModel{
		ID:     types.Int64Value(int64(*role.ID)),
		Admins: types.ListNull(types.Int64Type),
		Apps:   types.ListNull(types.Int64Type),
		Users:  types.ListNull(types.Int64Type),
	}

	var newDiags diag.Diagnostics
	state.Name = types.StringValue(*role.Name)

	admins, newDiags := types.ListValueFrom(ctx, types.Int64Type, role.Admins)
	diags.Append(newDiags...)
	if len(admins.Elements()) > 0 {
		state.Admins = admins
	}

	apps, newDiags := types.ListValueFrom(ctx, types.Int64Type, role.Apps)
	diags.Append(newDiags...)
	if len(apps.Elements()) > 0 {
		state.Apps = apps
	}

	users, newDiags := types.ListValueFrom(ctx, types.Int64Type, role.Users)
	diags.Append(newDiags...)
	if len(users.Elements()) > 0 {
		state.Users = users
	}

	return state, nil
}
