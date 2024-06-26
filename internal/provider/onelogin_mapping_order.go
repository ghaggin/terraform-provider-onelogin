package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

	diags = r.updateOrCreate(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics = diags
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

	diags = r.updateOrCreate(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
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

func (r *oneloginMappingOrderResource) updateOrCreate(ctx context.Context, state *oneloginMappingOrder) diag.Diagnostics {
	// get all enabled mappings from OneLogin
	enabled, diags := r.getEnabled(ctx)
	if diags.HasError() {
		return diags
	}

	// get all disabled mappings from OneLogin
	disabled, diags := r.getDisabled(ctx)
	if diags.HasError() {
		return diags
	}

	// Build maps of enabled and disabled.
	// Ensure that ids are not duplicated across both lists
	allInPlan := map[int64]bool{}
	enabledInPlan := map[int64]bool{}
	disabledInPlan := map[int64]bool{}

	for _, id := range state.Enabled {
		enabledInPlan[id] = true
		_, ok := allInPlan[id]
		if ok {
			diags.AddError(
				"duplicate id in enabled",
				fmt.Sprintf("id: %d", id),
			)
		}
		allInPlan[id] = true
	}

	for _, id := range state.Disabled {
		disabledInPlan[id] = true
		_, ok := allInPlan[id]
		if ok {
			diags.AddError(
				"duplicate id in disabled",
				fmt.Sprintf("id: %d", id),
			)
		}
		allInPlan[id] = true
	}

	if diags.HasError() {
		return diags
	}

	// Find any currently enabled mappings that should be disabled
	// and disable them
	for _, m := range enabled {
		if _, ok := disabledInPlan[m.ID]; ok {
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

				Retry:                10,
				RetryWait:            time.Second,
				RetryBackoffFactor:   1,
				RetriableStatusCodes: []int{404, 429, 500, 502, 504},
			})
			if err != nil || updateResp.ID != targetID {
				diags.AddError("failed to disable mapping", fmt.Sprintf("err: %v\nresp id: %v\ntarget id: %v", err.Error(), updateResp.ID, targetID))
			}
		}
	}

	if diags.HasError() {
		return diags
	}

	// Find any currently disabled mappings that should be enabled
	// Enable them with null position and sort them in the next step
	for _, m := range disabled {
		if _, ok := enabledInPlan[m.ID]; ok {
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

				Retry:                10,
				RetryWait:            time.Second,
				RetryBackoffFactor:   1,
				RetriableStatusCodes: []int{404, 429, 500, 502, 504},
			})
			if err != nil || updateResp.ID != targetID {
				diags.AddError("failed to enable mapping", fmt.Sprintf("err: %v\nresp id: %v\ntarget id: %v", err.Error(), updateResp.ID, targetID))
			}
		}
	}

	if diags.HasError() {
		return diags
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
		diags.AddError("failed to sort mappings", err.Error())
		return diags
	}

	// Reconcile response vs state
	if len(sortResp) != len(state.Enabled) {
		diags.AddError("enexpected failed to sort mappings", "sort response is a different length from the state enabled list")
		return diags
	}

	for i, id := range sortResp {
		if id != state.Enabled[i] {
			diags.AddError("failed to sort mappings", "sort response does not match state enabled list")
			return diags
		}
	}

	return nil
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
			diags.AddError("enabled mappings cannot have a null position", fmt.Sprintf("id: %d\tname: %s\t", m.ID, m.Name))
		}
	}
	if diags.HasError() {
		return nil, diags
	}

	// Ensure enabled is sorted by position
	sort.Slice(enabled, func(i, j int) bool {
		return *enabled[i].Position < *enabled[j].Position
	})

	// Ensure position value is as expected
	// Assumptions made in this provider rely on this numbering

	type outOfPositionMapping struct {
		id               int64
		position         int64
		expectedPosition int
	}
	outOfPosition := []*outOfPositionMapping{}
	for i, m := range enabled {
		expectedPosition := i + 1
		if *m.Position != int64(expectedPosition) {
			outOfPosition = append(outOfPosition, &outOfPositionMapping{
				id:               m.ID,
				position:         *m.Position,
				expectedPosition: expectedPosition,
			})
		}
	}

	if len(outOfPosition) != 0 {
		details := &strings.Builder{}

		details.WriteString("Assumptions made in this provider rely on this numbering\n\n")
		details.WriteString("id, actual_pos, expected_pos\n")
		details.WriteString("--------------------------\n")
		for _, oop := range outOfPosition {
			details.WriteString(fmt.Sprintf("%d, %d, %d\n", oop.id, oop.position, oop.expectedPosition))
		}

		diags.AddWarning(
			"mapping positions are not linearly increasing starting at 1",
			details.String(),
		)

		return enabled, diags
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
