package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
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

func NewOneLoginMappingResource(client *onelogin.Client) newResourceFunc {
	return func() resource.Resource {
		return &oneloginMappingResource{
			client: client,
		}
	}
}

type oneloginMappingResource struct {
	client *onelogin.Client
}

type oneloginMapping struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Match      types.String `tfsdk:"match"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Conditions types.List   `tfsdk:"conditions"`
	Actions    types.List   `tfsdk:"actions"`
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
				Computed: true,
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

	// Always create in the disabled state
	// Position and enable are set via the mapping_order resource
	native.Position = nil
	native.Enabled = false

	var mapping onelogin.Mapping
	err := d.client.ExecRequestCtx(ctx, &onelogin.Request{
		Method:    onelogin.MethodPost,
		Path:      onelogin.PathMappings,
		Body:      native,
		RespModel: &mapping,
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

	// Get the current enabled/disabled state from OneLogin and use that
	var mappingResp onelogin.Mapping
	err := d.client.ExecRequestCtx(ctx, &onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathMappings, id),
		RespModel: &mappingResp,
	})
	if err != nil || mappingResp.ID != id {
		resp.Diagnostics.AddError(
			"Error updating mapping",
			fmt.Sprintf("Could not get mapping with id:%v  error:%v ", id, err.Error()),
		)
		return
	}

	mappingBody := state.toNativeMapping(ctx)
	mappingBody.ID = 0
	mappingBody.Position = nil
	mappingBody.Enabled = mappingResp.Enabled

	err = d.client.ExecRequestCtx(ctx, &onelogin.Request{
		Method:    onelogin.MethodPut,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathMappings, id),
		Body:      mappingBody,
		RespModel: &mappingResp,
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
	err := d.client.ExecRequestCtx(ctx, &onelogin.Request{
		Method: onelogin.MethodDelete,
		Path:   fmt.Sprintf("%s/%v", onelogin.PathMappings, id),
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
	var mapping onelogin.Mapping
	id := state.ID.ValueInt64()

	err := d.client.ExecRequestCtx(ctx, &onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathMappings, id),
		RespModel: &mapping,
	})

	if err != nil {
		diags.AddError(
			"client error",
			fmt.Sprintf("Unable to read user %v, got error: %s", id, err),
		)
		return
	}

	newState, newDiags := mappingToState(ctx, &mapping)
	if newDiags.HasError() {
		diags.Append(newDiags...)
		return
	}

	// Update state
	newDiags = respState.Set(ctx, newState)
	diags.Append(newDiags...)
}

func (state *oneloginMapping) toNativeMapping(ctx context.Context) *onelogin.Mapping {
	native := &onelogin.Mapping{
		ID:      state.ID.ValueInt64(),
		Name:    state.Name.ValueString(),
		Match:   state.Match.ValueString(),
		Enabled: state.Enabled.ValueBool(),
	}

	conditions := []oneloginMappingCondition{}
	state.Conditions.ElementsAs(ctx, &conditions, false)
	for _, condition := range conditions {
		native.Conditions = append(native.Conditions, onelogin.MappingCondition{
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
		native.Actions = append(native.Actions, onelogin.MappingAction{
			Action: action.Action.ValueString(),
			Value:  values,
		})
	}

	return native
}

func mappingToState(ctx context.Context, mapping *onelogin.Mapping) (*oneloginMapping, diag.Diagnostics) {
	state := &oneloginMapping{
		ID:      types.Int64Value(mapping.ID),
		Name:    types.StringValue(mapping.Name),
		Match:   types.StringValue(mapping.Match),
		Enabled: types.BoolValue(mapping.Enabled),
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
