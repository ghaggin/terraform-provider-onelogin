package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource = &oneloginMappingOrderResource{}
)

func NewOneLoginMappingOrderResource(client *onelogin.Client) newResourceFunc {
	return func() resource.Resource {
		return &oneloginMappingOrderResource{
			client: client,
		}
	}
}

type oneloginMappingOrderResource struct {
	client *onelogin.Client
}

type oneloginMappingOrder struct {
	Enabled  []int64 `tfsdk:"enabled"`
	Disabled []int64 `tfsdk:"disabled"`
}

func (r *oneloginMappingOrderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mapping_order"
}

func (r *oneloginMappingOrderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"enabled": schema.ListAttribute{
				ElementType: types.Int64Type,
				Required:    true,
			},
			"disabled": schema.ListAttribute{
				ElementType: types.Int64Type,
				Required:    true,
			},
		},
	}
}

// Create reconciles the terraform config vs the state of Onelogin.
// No Onelogin resources are created or destroyed during this operation
// and config writers should create the config to match the state of Onelogin.
// Modifications to resources in OneLogin should only be made through the update operation.
func (r *oneloginMappingOrderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state oneloginMappingOrder
	diags := req.Plan.Get(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Reconcile enabled and disabled against Onelogin
	enabled, diags := r.getEnabled(ctx)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	disabled, diags := r.getDisabled(ctx)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	if len(enabled) != len(state.Enabled) {
		inOneloginIDs := make([]int64, len(enabled))
		for i, m := range enabled {
			inOneloginIDs[i] = m.ID
		}
		inState, inOnelogin := findDifference(state.Enabled, inOneloginIDs)
		resp.Diagnostics.AddError("enabled length different in config and onelogin", fmt.Sprintf("state not in onelogin: %v\nonelogin not in state: %v", inState, inOnelogin))
		return
	}

	disabledIDs := make([]int64, len(disabled))
	for i, m := range disabled {
		disabledIDs[i] = m.ID
	}
	if len(disabled) != len(state.Disabled) {
		inState, inOnelogin := findDifference(state.Disabled, disabledIDs)
		resp.Diagnostics.AddError("disabled length different in config and onelogin", fmt.Sprintf("state not in onelogin: %v\nonelogin not in state: %v", inState, inOnelogin))
	}

	if diags.HasError() {
		return
	}

	// Reconcile enabled
	for i, id := range state.Enabled {
		if enabled[i].ID != id {
			resp.Diagnostics.AddError("enabled mappings do not match onelogin", fmt.Sprintf("position: %d\nconfig: %d\nonelogin: %d", i, id, enabled[i].ID))
			return
		}
	}

	// Reconcile disabled
	if inState, inOnelogin := findDifference(state.Disabled, disabledIDs); len(inState) != 0 || len(inOnelogin) != 0 {
		resp.Diagnostics.AddError(
			"disabled different in config and onelogin",
			fmt.Sprintf("state not in onelogin: %v\nonelogin not in state: %v", inState, inOnelogin),
		)
		return
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
}

func (r *oneloginMappingOrderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oneloginMappingOrder
	diags := req.State.Get(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Get enabled
	enabled, diags := r.getEnabled(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	enabledIDs := make([]int64, len(enabled))
	for i, m := range enabled {
		enabledIDs[i] = m.ID
	}

	// Get disabled
	disabled, diags := r.getDisabled(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	disabledIDs := make([]int64, len(disabled))
	for i, m := range disabled {
		disabledIDs[i] = m.ID
	}

	disabledInState, disabledInOnelogin := findDifference(state.Disabled, disabledIDs)
	if len(disabledInState) != 0 || len(disabledInOnelogin) != 0 {
		resp.Diagnostics.AddError("found difference in disabled mappings between onelogin and config state", "")
		return
	}

	// Convert to state
	var newState oneloginMappingOrder
	newState.Enabled = enabledIDs
	newState.Disabled = state.Disabled

	diags = resp.State.Set(ctx, &newState)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *oneloginMappingOrderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state oneloginMappingOrder
	diags := req.Plan.Get(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	enabled, diags := r.getEnabled(ctx)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	disabled, diags := r.getDisabled(ctx)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Build maps of enabled and disabled.
	// Validate that ids are not duplicated across both lists
	allInPlan := map[int64]bool{}
	enabledInPlan := map[int64]bool{}
	disabledInPlan := map[int64]bool{}

	for _, id := range state.Enabled {
		enabledInPlan[id] = true
		_, ok := allInPlan[id]
		if ok {
			resp.Diagnostics.AddError("duplicate id in enabled", "Please ensure all IDs are only added to the mapping_order once")
			return
		}
		allInPlan[id] = true
	}

	for _, id := range state.Disabled {
		disabledInPlan[id] = true
		_, ok := allInPlan[id]
		if ok {
			resp.Diagnostics.AddError("duplicate id in enabled", "Please ensure all IDs are only added to the mapping_order once")
			return
		}
		allInPlan[id] = true
	}

	// Find any currently enabled mappings that should be disabled
	// and disable them
	for _, m := range enabled {
		_, ok := disabledInPlan[m.ID]
		if ok {
			targetID := m.ID
			m.ID = 0
			m.Position = nil
			m.Enabled = false

			var updateResp struct {
				ID int64 `json:"id"`
			}
			err := r.client.ExecRequest(&onelogin.Request{
				Context:   ctx,
				Method:    onelogin.MethodPut,
				Path:      fmt.Sprintf("%s/%d", onelogin.PathMappings, targetID),
				Body:      m,
				RespModel: &updateResp,
			})
			if err != nil || updateResp.ID != targetID {
				resp.Diagnostics.AddError("failed to disable mapping", fmt.Sprintf("err: %v\nresp id: %v\ntarget id: %v", err.Error(), updateResp.ID, targetID))
				return
			}
		}
	}

	// Find any currently disabled mappings that should be enabled
	// Enable them with null position and sort them in the next step
	for _, m := range disabled {
		_, ok := enabledInPlan[m.ID]
		if ok {
			targetID := m.ID
			m.ID = 0
			m.Position = nil
			m.Enabled = true

			var updateResp struct {
				ID int64 `json:"id"`
			}
			err := r.client.ExecRequest(&onelogin.Request{
				Context:   ctx,
				Method:    onelogin.MethodPut,
				Path:      fmt.Sprintf("%s/%d", onelogin.PathMappings, targetID),
				Body:      m,
				RespModel: &updateResp,
			})
			if err != nil || updateResp.ID != targetID {
				resp.Diagnostics.AddError("failed to enable mapping", fmt.Sprintf("err: %v\nresp id: %v\ntarget id: %v", err.Error(), updateResp.ID, targetID))
				return
			}
		}
	}

	// Sort enabled mappings
	var sortResp []int64
	err := r.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodPut,
		Path:      onelogin.PathMappingsSort,
		Body:      state.Enabled,
		RespModel: &sortResp,
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to sort mappings", err.Error())
		return
	}

	// Reconcile response vs state
	if len(sortResp) != len(state.Enabled) {
		resp.Diagnostics.AddError("enexpected failed to sort mappings", "sort response is a different length from the state enabled list")
		return
	}

	for i, id := range sortResp {
		if id != state.Enabled[i] {
			resp.Diagnostics.AddError("failed to sort mappings", "sort response does not match state enabled list")
			return
		}
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
}

func (r *oneloginMappingOrderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Noop, nothing to delete in onelogin
}

func (r *oneloginMappingOrderResource) getEnabled(ctx context.Context) ([]onelogin.Mapping, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	// Get enabled
	var enabled []onelogin.Mapping
	err := r.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathMappings,
		RespModel: &enabled,
	})
	if err != nil {
		diags.AddError("failed to get enabled mappings: ", err.Error())
		return nil, diags
	}

	// Ensure no enabled mappings have a null position
	for _, m := range enabled {
		if m.Position == nil {
			diags.AddError("enabled mappings cannot have a null position", "Please update the enabled mappings to include a position")
			return nil, diags
		}
	}

	// Ensure enabled is sorted by position
	sort.Slice(enabled, func(i, j int) bool {
		return *enabled[i].Position < *enabled[j].Position
	})

	// Ensure position value is as expected
	// Assumptions made in this provider rely on this numbering
	for i, m := range enabled {
		expectedPosition := i + 1
		if *m.Position != int64(expectedPosition) {
			diags.AddError(
				"mapping positions are not linearly increasing starting at 1",
				fmt.Sprintf("Assumptions made in this provider rely on this numbering\nMapping id %v found at position %v, expected position %v", m.ID, *m.Position, expectedPosition),
			)
			return nil, diags
		}
	}

	return enabled, nil
}

func (r *oneloginMappingOrderResource) getDisabled(ctx context.Context) ([]onelogin.Mapping, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	// Get disabled
	var disabled []onelogin.Mapping
	err := r.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathMappings,
		RespModel: &disabled,
		QueryParams: onelogin.QueryParams{
			"enabled": "false",
		},
	})
	if err != nil {
		diags.AddError("failed to get disabled mappings: ", err.Error())
		return nil, diags
	}

	return disabled, nil
}

func findDifference(a, b []int64) ([]int64, []int64) {
	aNotInB := []int64{}
	bNotInA := []int64{}

	aElements := make(map[int64]bool)
	bElements := make(map[int64]bool)

	// Add all elements of a to aElements
	for _, e := range a {
		aElements[e] = true
	}

	// Check if b elements are in a
	// -and-
	// Add all elements of b to bElements
	for _, e := range b {
		_, ok := aElements[e]
		if !ok {
			bNotInA = append(bNotInA, e)
		}

		bElements[e] = true
	}

	// Check if a elements are in b
	for _, e := range a {
		_, ok := bElements[e]
		if !ok {
			aNotInB = append(aNotInB, e)
		}
	}

	return aNotInB, bNotInA
}
