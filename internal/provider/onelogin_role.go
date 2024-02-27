package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ghaggin/terraform-provider-onelogin/internal/util"
	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &oneloginRoleResource{}
	_ resource.ResourceWithConfigure   = &oneloginRoleResource{}
	_ resource.ResourceWithImportState = &oneloginRoleResource{}
)

func NewOneLoginRoleResource(client *onelogin.Client) newResourceFunc {
	return func() resource.Resource {
		return &oneloginRoleResource{
			client: client,
		}
	}
}

type oneloginRoleResource struct {
	client *onelogin.Client
}

type oneloginRole struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`

	Admins types.List `tfsdk:"admins"`
	Apps   types.List `tfsdk:"apps"`
	Users  types.List `tfsdk:"users"`

	// LastUpdated attribute local to terraform object
	LastUpdated types.String `tfsdk:"last_updated"`
}

func (d *oneloginRoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *oneloginRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

func (d *oneloginRoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"admins": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
			},
			"apps": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
			},
			"users": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Computed:    true,
			},

			// Note: attribute local to terraform objects
			"last_updated": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the last time this role was updated",
				Computed:            true,
			},
		},
	}
}

func (d *oneloginRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var state oneloginRole
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var role onelogin.Role
	err := d.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodPost,
		Path:      onelogin.PathRoles,
		Body:      state.toNative(ctx),
		RespModel: &role,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			"Could not create role: "+err.Error(),
		)
		return
	}

	newState, diags := d.read(ctx, role.ID)
	if diags.HasError() {
		return
	}

	newState.LastUpdated = types.StringValue(util.GetTimestampString())

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneloginRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oneloginRole
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

	newState.LastUpdated = state.LastUpdated

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneloginRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state oneloginRole
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := state.toNative(ctx)
	body.ID = 0 // zero out id to omit from the json body

	var role onelogin.Role
	err := d.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodPut,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathRoles, state.ID.ValueInt64()),
		Body:      body,
		RespModel: &role,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating role",
			"Could not update role: "+err.Error(),
		)
		return
	}

	newState, diags := d.read(ctx, role.ID)
	if diags.HasError() {
		return
	}

	newState.LastUpdated = types.StringValue(util.GetTimestampString())

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneloginRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oneloginRole
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// queryParams is inexplicably unused
	err := d.client.ExecRequest(&onelogin.Request{
		Context: ctx,
		Method:  onelogin.MethodDelete,
		Path:    fmt.Sprintf("%s/%v", onelogin.PathRoles, state.ID.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting role",
			"Could not delete role with ID "+state.ID.String()+": "+err.Error(),
		)
		return
	}
}

func (d *oneloginRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing ID for import role", "Could not parse ID "+req.ID+": "+err.Error())
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

func (d *oneloginRoleResource) read(ctx context.Context, id int64) (*oneloginRole, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	// This function doesn't even make any sense.  Query params need to be included
	// but it is impossible to use query params when getting role by ID.
	var role onelogin.Role
	err := d.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodGet,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathRoles, id),
		RespModel: &role,
	})
	if err != nil || role.ID == 0 {
		diags.AddError("Error reading role", "Could not read role with ID "+strconv.Itoa(int(id))+": "+err.Error())
		return nil, diags
	}

	return roleToState(ctx, &role)
}

func (state *oneloginRole) toNative(ctx context.Context) *onelogin.Role {
	admins := []int64{}
	if !state.Admins.IsNull() {
		state.Admins.ElementsAs(ctx, &admins, false)
	}

	apps := []int64{}
	if !state.Apps.IsNull() {
		state.Apps.ElementsAs(ctx, &apps, false)
	}

	users := []int64{}
	if !state.Users.IsNull() {
		state.Users.ElementsAs(ctx, &users, false)
	}

	return &onelogin.Role{
		ID:     state.ID.ValueInt64(),
		Name:   state.Name.ValueString(),
		Admins: admins,
		Apps:   apps,
		Users:  users,
	}
}

func roleToState(ctx context.Context, role *onelogin.Role) (*oneloginRole, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	state := &oneloginRole{
		ID:     types.Int64Value(role.ID),
		Name:   types.StringValue(role.Name),
		Admins: types.ListNull(types.Int64Type),
		Apps:   types.ListNull(types.Int64Type),
		Users:  types.ListNull(types.Int64Type),
	}

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

	return state, diags
}
