package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &oneloginMappingResource{}
	_ resource.ResourceWithConfigure   = &oneloginMappingResource{}
	_ resource.ResourceWithImportState = &oneloginMappingResource{}
)

func NewOneLoginMappingResource(client *client) newResourceFunc {
	return func() resource.Resource {
		return &oneloginMappingResource{
			client: client,
		}
	}
}

type oneloginMappingResource struct {
	client *client
}

type oneloginMapping struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Match      types.String `tfsdk:"match"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Position   types.Int64  `tfsdk:"position"`
	Conditions types.List   `tfsdk:"conditions"`
	Actions    types.List   `tfsdk:"actions"`
}

type oneloginNativeMapping struct {
	ID         int64                            `json:"id,omitempty"`
	Name       string                           `json:"name"`
	Match      string                           `json:"match"`
	Enabled    bool                             `json:"enabled"`
	Position   *int64                           `json:"position"`
	Conditions []oneloginNativeMappingCondition `json:"conditions"`
	Actions    []oneloginNativeMappingAction    `json:"actions"`
}

type oneloginNativeMappingCondition struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type oneloginMappingCondition struct {
	Source   types.String `tfsdk:"source"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

func oneloginMappingConditionTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source":   types.StringType,
		"operator": types.StringType,
		"value":    types.StringType,
	}
}

type oneloginNativeMappingAction struct {
	Action string   `json:"action"`
	Value  []string `json:"value"`
}

type oneloginMappingAction struct {
	Action types.String `tfsdk:"action"`
	Value  types.List   `tfsdk:"value"`
}

func oneloginMappingActionTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"action": types.StringType,
		"value":  types.ListType{ElemType: types.StringType},
	}
}

func (d *oneloginMappingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mapping"
}

func (d *oneloginMappingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

func (d *oneloginMappingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"match": schema.StringAttribute{
				Required: true,
			},
			"enabled": schema.BoolAttribute{
				Required: true,
			},

			// TODO: make this work..
			// Issues:
			//  - update does not work.  use bulk sort: https://developers.onelogin.com/api-docs/2/user-mappings/bulk-sort
			//  - handle position for disabled mapping
			//  - create needs to sort the mapping into the existing mappings
			"position": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{
					// int64planmodifier.UseStateForUnknown(),
				},
			},

			// Condition  sources, operators and values can be discovered with these endpoints
			// https://developers.onelogin.com/api-docs/2/user-mappings/list-conditions
			// https://developers.onelogin.com/api-docs/2/user-mappings/list-condition-operators
			// https://developers.onelogin.com/api-docs/2/user-mappings/list-condition-values
			"conditions": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"source": schema.StringAttribute{
							Required: true,
						},
						"operator": schema.StringAttribute{
							Required: true,
						},
						"value": schema.StringAttribute{
							Required: true,
						},
					},
				},
				Required: true,
			},

			// Actions and values can be discovered with these endpoints
			// https://developers.onelogin.com/api-docs/2/user-mappings/list-actions
			// https://developers.onelogin.com/api-docs/2/user-mappings/list-action-values
			"actions": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"action": schema.StringAttribute{
							Required: true,
						},
						"value": schema.ListAttribute{
							ElementType: types.StringType,
							Required:    true,
						},
					},
				},
				Required: true,
			},
		},
	}
}

func (d *oneloginMappingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state oneloginMapping
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	native := state.toNativeMapping(ctx)

	// TODO: Implement the position logic
	native.Position = nil

	var mapping oneloginNativeMapping
	err := d.client.execRequestCtx(ctx, &oneloginRequest{
		method:    methodPost,
		path:      pathMappings,
		body:      native,
		respModel: &mapping,
	})
	if err != nil || mapping.ID == 0 {
		resp.Diagnostics.AddError(
			"Error creating mapping",
			"Could not create mapping: "+err.Error(),
		)
		return
	}

	state.ID = types.Int64Value(mapping.ID)
	d.readToState(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (d *oneloginMappingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oneloginMapping
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	d.readToState(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (d *oneloginMappingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state oneloginMapping
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueInt64()

	mappingBody := state.toNativeMapping(ctx)
	mappingBody.ID = 0
	mappingBody.Position = nil

	var mappingResp oneloginNativeMapping
	err := d.client.execRequestCtx(ctx, &oneloginRequest{
		method:    methodPut,
		path:      fmt.Sprintf("%s/%v", pathMappings, id),
		body:      mappingBody,
		respModel: &mappingResp,
	})
	if err != nil || mappingResp.ID != id {
		resp.Diagnostics.AddError(
			"Error updating mapping",
			fmt.Sprintf("Could not update mapping with id:%v  error:%v ", id, err.Error()),
		)
		return
	}

	d.readToState(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (d *oneloginMappingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oneloginMapping
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueInt64()
	err := d.client.execRequestCtx(ctx, &oneloginRequest{
		method: methodDelete,
		path:   fmt.Sprintf("%s/%v", pathMappings, id),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting mapping",
			fmt.Sprintf("Could not delete mapping with id:%v, err:%v", id, err),
		)
		return
	}
}

func (d *oneloginMappingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing ID for import",
			"Could not parse ID "+req.ID+": "+err.Error(),
		)
		return
	}

	state := &oneloginMapping{
		ID: types.Int64Value(int64(id)),
	}

	d.readToState(ctx, state, &resp.State, &resp.Diagnostics)
}

func (d *oneloginMappingResource) readToState(ctx context.Context, state *oneloginMapping, respState *tfsdk.State, diags *diag.Diagnostics) {
	var mapping oneloginNativeMapping
	id := state.ID.ValueInt64()

	err := d.client.execRequestCtx(ctx, &oneloginRequest{
		method:    methodGet,
		path:      fmt.Sprintf("%s/%v", pathMappings, id),
		respModel: &mapping,
	})

	if err != nil {
		diags.AddError(
			"client error",
			fmt.Sprintf("Unable to read user %v, got error: %s", id, err),
		)
		return
	}

	newState, newDiags := mapping.toState(ctx)
	if newDiags.HasError() {
		diags.Append(newDiags...)
		return
	}

	// Update state
	newDiags = respState.Set(ctx, newState)
	diags.Append(newDiags...)
}

func (state *oneloginMapping) toNativeMapping(ctx context.Context) *oneloginNativeMapping {
	native := &oneloginNativeMapping{
		ID:       state.ID.ValueInt64(),
		Name:     state.Name.ValueString(),
		Match:    state.Match.ValueString(),
		Enabled:  state.Enabled.ValueBool(),
		Position: state.Position.ValueInt64Pointer(),
	}

	conditions := []oneloginMappingCondition{}
	state.Conditions.ElementsAs(ctx, &conditions, false)
	for _, condition := range conditions {
		native.Conditions = append(native.Conditions, oneloginNativeMappingCondition{
			Source:   condition.Source.ValueString(),
			Operator: condition.Operator.ValueString(),
			Value:    condition.Value.ValueString(),
		})
	}

	actions := []oneloginMappingAction{}
	state.Actions.ElementsAs(ctx, &actions, false)
	for _, action := range actions {
		values := []string{}
		action.Value.ElementsAs(ctx, &values, false)
		native.Actions = append(native.Actions, oneloginNativeMappingAction{
			Action: action.Action.ValueString(),
			Value:  values,
		})
	}

	return native
}

func (mapping *oneloginNativeMapping) toState(ctx context.Context) (*oneloginMapping, diag.Diagnostics) {
	state := &oneloginMapping{
		ID:      types.Int64Value(mapping.ID),
		Name:    types.StringValue(mapping.Name),
		Match:   types.StringValue(mapping.Match),
		Enabled: types.BoolValue(mapping.Enabled),
	}

	if mapping.Position != nil {
		state.Position = types.Int64Value(*mapping.Position)
	}

	diags := diag.Diagnostics{}
	var newDiags diag.Diagnostics
	conditions := []oneloginMappingCondition{}
	for _, condition := range mapping.Conditions {
		conditions = append(conditions, oneloginMappingCondition{
			Source:   types.StringValue(condition.Source),
			Operator: types.StringValue(condition.Operator),
			Value:    types.StringValue(condition.Value),
		})
	}
	state.Conditions, newDiags = types.ListValueFrom(ctx, types.ObjectNull(oneloginMappingConditionTypes()).Type(ctx), conditions)
	diags.Append(newDiags...)
	if newDiags.HasError() {
		return state, diags
	}

	actions := []oneloginMappingAction{}
	for _, action := range mapping.Actions {
		value, newDiags := types.ListValueFrom(ctx, types.StringType, action.Value)
		actions = append(actions, oneloginMappingAction{
			Action: types.StringValue(action.Action),
			Value:  value,
		})
		diags.Append(newDiags...)
		if diags.HasError() {
			return state, diags
		}
	}
	state.Actions, newDiags = types.ListValueFrom(ctx, types.ObjectNull(oneloginMappingActionTypes()).Type(ctx), actions)
	diags.Append(newDiags...)
	if newDiags.HasError() {
		return state, diags
	}

	return state, diags
}
