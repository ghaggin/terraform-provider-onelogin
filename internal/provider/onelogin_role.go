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

	newRole, diags := state.toNative(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var role onelogin.Role
	err := d.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodPost,
		Path:      onelogin.PathRoles,
		Body:      newRole,
		RespModel: &role,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			"Could not create role: "+err.Error(),
		)
		return
	}

	newState, diags := d.read(ctx, role.ID, state.Users)
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

	newState, diags := d.read(ctx, state.ID.ValueInt64(), state.Users)
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
	var plan oneloginRole
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := plan.toNative(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body.ID = 0 // zero out id to omit from the json body

	// Omit users from the full update.
	// Update users individually based on add/remove from plan
	body.Users = nil

	var role onelogin.Role
	err := d.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodPut,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathRoles, plan.ID.ValueInt64()),
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

	// Calculate the added and removed users
	var state oneloginRole
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	addUsers, removeUsers, diags := calculateAddRemoveUsers(ctx, plan.Users, state.Users)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add users
	if len(addUsers) > 0 {
		var addUserResp []struct {
			ID int64 `json:"id"`
		}
		err = d.client.ExecRequest(&onelogin.Request{
			Context:   ctx,
			Method:    onelogin.MethodPost,
			Path:      fmt.Sprintf("%s/%d/users", onelogin.PathRoles, plan.ID.ValueInt64()),
			Body:      addUsers,
			RespModel: &addUserResp,
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error adding users to role",
				"Could not add users to role: "+err.Error(),
			)
			return
		}
		if len(addUserResp) != len(addUsers) {
			resp.Diagnostics.AddError(
				"Error adding users to role",
				fmt.Sprintf("Could not add all users to role\nadddUserResp: %v\naddUsers: %v", addUserResp, addUsers),
			)
			return
		}
	}

	// Delete users
	if len(removeUsers) > 0 {
		err = d.client.ExecRequest(&onelogin.Request{
			Context: ctx,
			Method:  onelogin.MethodDelete,
			Path:    fmt.Sprintf("%s/%d/users", onelogin.PathRoles, plan.ID.ValueInt64()),
			Body:    removeUsers,
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error removing users from role",
				"Could not remove users from role: "+err.Error(),
			)
			return
		}
	}

	// I think that add and delete may return prior to the transaction being fully committed.
	// In the case the transaction is not fully committed, the read will produce inconsistent results.
	// We will assume that user updates are eventually consistent and update the state to their expected value.
	newState, diags := d.read(ctx, role.ID, plan.Users)
	if diags.HasError() {
		return
	}
	newState.Users = plan.Users

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

	state, diags := d.read(ctx, int64(id), types.ListNull(types.Int64Type))
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

func (d *oneloginRoleResource) read(ctx context.Context, id int64, trackedUsers types.List) (*oneloginRole, diag.Diagnostics) {
	diags := diag.Diagnostics{}

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

	return roleToState(ctx, &role, trackedUsers)
}

func (state *oneloginRole) toNative(ctx context.Context) (*onelogin.Role, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	newDiags := diag.Diagnostics{}

	admins := []int64{}
	if !state.Admins.IsNull() && !state.Admins.IsUnknown() {
		newDiags = state.Admins.ElementsAs(ctx, &admins, false)
	}
	diags.Append(newDiags...)

	apps := []int64{}
	if !state.Apps.IsNull() && !state.Apps.IsUnknown() {
		newDiags = state.Apps.ElementsAs(ctx, &apps, false)
	}
	diags.Append(newDiags...)

	users := []int64{}
	if !state.Users.IsNull() && !state.Users.IsUnknown() {
		newDiags = state.Users.ElementsAs(ctx, &users, false)
	}
	diags.Append(newDiags...)

	return &onelogin.Role{
		ID:     state.ID.ValueInt64(),
		Name:   state.Name.ValueString(),
		Admins: admins,
		Apps:   apps,
		Users:  users,
	}, diags
}

func roleToState(ctx context.Context, role *onelogin.Role, trackedUsers types.List) (*oneloginRole, diag.Diagnostics) {
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

	if !trackedUsers.IsNull() && !trackedUsers.IsUnknown() {
		trackedUsersIDs := []int64{}
		newDiags = trackedUsers.ElementsAs(ctx, &trackedUsersIDs, false)
		diags.Append(newDiags...)
		if diags.HasError() {
			return state, diags
		}
		trackedUsersIDsMap := map[int64]bool{}
		for _, id := range trackedUsersIDs {
			trackedUsersIDsMap[id] = true
		}
		roleUsersTracked := []int64{}
		for _, id := range role.Users {
			if _, ok := trackedUsersIDsMap[id]; ok {
				roleUsersTracked = append(roleUsersTracked, id)
			}
		}
		users, newDiags := types.ListValueFrom(ctx, types.Int64Type, roleUsersTracked)
		diags.Append(newDiags...)
		if len(users.Elements()) > 0 {
			state.Users = users
		}
	}

	return state, diags
}

// state = old state
// plan = future state
func calculateAddRemoveUsers(ctx context.Context, plan, state types.List) ([]int64, []int64, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	planUsers := []int64{}
	newDiags := plan.ElementsAs(ctx, &planUsers, false)
	diags.Append(newDiags...)

	stateUsers := []int64{}
	newDiags = state.ElementsAs(ctx, &stateUsers, false)
	diags.Append(newDiags...)

	if diags.HasError() {
		return nil, nil, diags
	}

	add := []int64{}
	remove := []int64{}

	planUsersMap := map[int64]bool{}
	for _, planUser := range planUsers {
		planUsersMap[planUser] = true
	}

	stateUsersMap := map[int64]bool{}
	for _, stateUser := range stateUsers {
		stateUsersMap[stateUser] = true

		if _, inPlan := planUsersMap[stateUser]; !inPlan {
			remove = append(remove, stateUser)
		}
	}

	for _, planUser := range planUsers {
		if _, inState := stateUsersMap[planUser]; !inState {
			add = append(add, planUser)
		}
	}

	return add, remove, diags
}
