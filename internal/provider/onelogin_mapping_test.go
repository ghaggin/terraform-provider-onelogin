package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func (s *providerTestSuite) Test_mappingToState() {
	id := int64(1234)
	name := "test_name"
	match := "test_match"
	source := "test_source"
	operator := "test_operator"
	value := "test_value"
	action := "test_action"
	actionValue1 := "test_value_1"
	actionValue2 := "test_value_2"

	nativeMapping := &onelogin.Mapping{
		ID:       id,
		Name:     name,
		Match:    match,
		Position: nil,
		Conditions: []onelogin.MappingCondition{
			{
				Source:   source,
				Operator: operator,
				Value:    value,
			},
		},
		Actions: []onelogin.MappingAction{
			{
				Action: action,
				Value:  []string{actionValue1, actionValue2},
			},
		},
	}

	ctx := context.Background()

	state, diags := mappingToState(ctx, nativeMapping)
	s.Require().False(diags.HasError(), diags.Errors())

	s.Equal(id, state.ID.ValueInt64())
	s.Equal(name, state.Name.ValueString())
	s.Equal(match, state.Match.ValueString())

	conditions := []oneloginMappingCondition{}
	state.Conditions.ElementsAs(ctx, &conditions, false)
	s.Require().Len(conditions, 1)
	s.Equal(source, conditions[0].Source.ValueString())
	s.Equal(operator, conditions[0].Operator.ValueString())
	s.Equal(value, conditions[0].Value.ValueString())

	actions := []oneloginMappingAction{}
	state.Actions.ElementsAs(ctx, &actions, false)
	s.Require().Len(actions, 1)
	s.Equal(action, actions[0].Action.ValueString())
	values := []string{}
	actions[0].Value.ElementsAs(ctx, &values, false)
	s.Require().Len(values, 2)
	s.Equal(actionValue1, values[0])
	s.Equal(actionValue2, values[1])

	newNativeMapping := state.toNativeMapping(ctx)
	s.Equal(nativeMapping, newNativeMapping)
}

// Test mappings without position
func (s *providerTestSuite) TestAccResourceMapping() {
	name := "test_mapping"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					# required in next step
					resource "onelogin_role" "test" {
						name = "test_role_1234"
					}

					resource "onelogin_mapping" "test" {
						name = "%v"
						match = "all"
						conditions = [
							{
								source = "last_login"
								operator = ">"
								value = "90"
							}
						]
						actions = [
							{
								action = "set_status"
								value = ["2"]
							}
						]
					}
				`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", name),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "all"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.source", "last_login"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.operator", ">"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.value", "90"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.action", "set_status"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.value.0", "2"),
				),
			},
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_role" "test" {
						name = "test_role_1234"
					}

					resource "onelogin_mapping" "test" {
						name = "%v_1234"
						match = "any"
						conditions = [
							{
								source = "has_role"
								operator = "ri"
								value = tostring(onelogin_role.test.id)
							}
						]
						actions = [
							{
								action = "add_role"
								value = [tostring(onelogin_role.test.id)]
							}
						]
					}
				`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", name+"_1234"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "any"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.source", "has_role"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.operator", "ri"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "conditions.0.value"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.action", "add_role"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "actions.0.value.0"),
				),
			},
			{
				ResourceName:      "onelogin_mapping.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Test enabled mappings.  Requires mapping order resourse
func (s *providerTestSuite) TestAccResourceMappingEnabled() {
	ctx := context.Background()

	mappingOrderResource := oneloginMappingOrderResource{
		client: s.client,
	}

	enabled, diags := mappingOrderResource.getEnabled(ctx)
	s.Require().Nil(diags, diags.Errors())

	disabled, diags := mappingOrderResource.getDisabled(ctx)
	s.Require().Nil(diags, diags.Errors())

	enabledIDs := make([]string, len(enabled))
	for i, m := range enabled {
		enabledIDs[i] = strconv.Itoa(int(m.ID))
	}

	disabledIDs := make([]string, len(disabled))
	for i, m := range disabled {
		disabledIDs[i] = strconv.Itoa(int(m.ID))
	}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = %v
						disabled = %v
					}

					resource "onelogin_mapping" "test" {
						name = "test_mapping"
						match = "all"
						conditions = [
							{
								source = "last_login"
								operator = ">"
								value = "90"
							}
						]
						actions = [
							{
								action = "set_status"
								value = ["2"]
							}
						]
					}
				`, "["+strings.Join(enabledIDs, ",")+"]", "["+strings.Join(disabledIDs, ",")+",onelogin_mapping.test.id]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled))),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", "test_mapping"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "id"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.source", "last_login"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.operator", ">"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.value", "90"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.action", "set_status"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.value.0", "2"),
				),
			},
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = %v
						disabled = %v
					}

					resource "onelogin_mapping" "test" {
						name = "test_mapping"
						match = "all"
						conditions = [
							{
								source = "last_login"
								operator = ">"
								value = "90"
							}
						]
						actions = [
							{
								action = "set_status"
								value = ["2"]
							}
						]
					}
				`, "[onelogin_mapping.test.id,"+strings.Join(enabledIDs, ",")+"]", "["+strings.Join(disabledIDs, ",")+"]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled))),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", "test_mapping"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "id"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.source", "last_login"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.operator", ">"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.value", "90"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.action", "set_status"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.value.0", "2"),
				),
			},
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = %v
						disabled = %v
					}

					resource "onelogin_mapping" "test" {
						name = "test_mapping"
						match = "all"
						conditions = [
							{
								source = "last_login"
								operator = ">"
								value = "90"
							},
							{
								operator = "!~"
								source   = "member_of"
								value    = "cn=test_group,"
							}
						]
						actions = [
							{
								action = "set_status"
								value = ["2"]
							}
						]
					}
				`, "[onelogin_mapping.test.id,"+strings.Join(enabledIDs, ",")+"]", "["+strings.Join(disabledIDs, ",")+"]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled))),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", "test_mapping"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "id"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.source", "last_login"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.operator", ">"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "conditions.0.value", "90"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.action", "set_status"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "actions.0.value.0", "2"),
				),
			},
		},
	})
}
