package provider

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// DELETE TEST MAPPINGS
// This is useful for cleaning up tests that leave the instance in a
// corrupted state.
//
// Only run ad hoc when other load tests require cleanup.
func (s *providerTestSuite) Test_DeleteMappings() {
	if os.Getenv("LOAD_TEST") != "1" {
		return
	}

	mo := oneloginMappingOrderResource{s.client}
	_, diags := mo.getEnabled(context.Background())
	s.False(diags.HasError())

	var enabled []onelogin.Mapping
	err := s.client.ExecRequest(&onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathMappings,
		RespModel: &enabled,
	})
	s.Require().NoError(err)

	sort.Slice(enabled, func(i, j int) bool {
		return *enabled[i].Position > *enabled[j].Position
	})

	for _, m := range enabled {
		if strings.Contains(m.Name, "terraform_test_") {
			fmt.Printf("Deleting mapping %s [%d]\n", m.Name, m.ID)
			err = s.client.ExecRequest(&onelogin.Request{
				Method: onelogin.MethodDelete,
				Path:   fmt.Sprintf("%s/%d", onelogin.PathMappings, m.ID),
			})

			if err != nil {
				fmt.Printf("Error deleting mapping %s [%d]: %v\n", m.Name, m.ID, err)
			}
		}
	}
}

// This test demonstrates how to delete a large number of mappings in
// multiple threads.
//
// Only run when LOAD_TEST is 1.  This test runs for >10 min.
func (s *providerTestSuite) Test_CreateAndDeleteLotsOfMappings() {
	if os.Getenv("LOAD_TEST") != "1" {
		return
	}

	ctx := context.Background()

	numMappings := 1000
	for i := 0; i < numMappings; i++ {
		mapping := onelogin.Mapping{
			Name:  fmt.Sprintf("terraform_test_%d", i),
			Match: "all",
			Conditions: []onelogin.MappingCondition{
				{
					Source:   "last_login",
					Operator: ">",
					Value:    "90",
				},
			},
			Actions: []onelogin.MappingAction{
				{
					Action: "set_status",
					Value:  []string{"2"},
				},
			},
			Enabled:  true,
			Position: nil,
		}

		fmt.Printf("creating mapping %d\n", i)
		err := s.client.ExecRequest(&onelogin.Request{
			Method: onelogin.MethodPost,
			Path:   onelogin.PathMappings,
			Body:   &mapping,
		})
		s.Require().NoError(err)
	}

	mo := oneloginMappingOrderResource{s.client}
	enabled, diags := mo.getEnabled(context.Background())
	if diags.HasError() {
		s.False(diags.HasError())
		s.T().Log(diags.Errors())
	}

	numWorkers := 10
	wg := &sync.WaitGroup{}
	wg.Add(10)
	type deleteRequest struct {
		ID  int64
		Num int
	}
	queue := make(chan deleteRequest, numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for r := range queue {
				fmt.Printf("deleting mapping %d\n", r.Num)
				err := s.client.ExecRequest(&onelogin.Request{
					Method: onelogin.MethodDelete,
					Path:   fmt.Sprintf("%s/%v", onelogin.PathMappings, r.ID),

					Retry:                10,
					RetryWait:            time.Second,
					RetryBackoffFactor:   1,
					RetriableStatusCodes: []int{404, 429, 500, 502, 504},
				})
				s.NoError(err)
			}
		}()
	}

	for i, mapping := range enabled {
		queue <- deleteRequest{
			Num: i,
			ID:  mapping.ID,
		}
	}
	close(queue)
	wg.Wait()

	enabled, diags = mo.getEnabled(ctx)
	s.Require().False(diags.HasError())
	s.Equal(0, len(enabled))
}

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

// Test enabled mappings.  Requires mapping order resources
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

	maybeComma := func(s []string) string {
		if len(s) > 0 {
			return ","
		}
		return ""
	}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = [%v]
						disabled = [%v]
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
				`, strings.Join(enabledIDs, ","), strings.Join(disabledIDs, ",")+maybeComma(disabledIDs)+"onelogin_mapping.test.id"),
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
						enabled = [%v]
						disabled = [%v]
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
				`, "onelogin_mapping.test.id"+maybeComma(enabledIDs)+strings.Join(enabledIDs, ","), strings.Join(disabledIDs, ",")),
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
						enabled = [%v]
						disabled = [%v]
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
				`, "onelogin_mapping.test.id"+maybeComma(enabledIDs)+strings.Join(enabledIDs, ","), strings.Join(disabledIDs, ",")),
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
				// delete all resources
				Config: s.providerConfig,
			},
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = [%v]
						disabled = [%v]
					}

					resource "onelogin_mapping" "test_2" {
						name = "test_mapping_enabled"
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
				`, "onelogin_mapping.test_2.id"+maybeComma(enabledIDs)+strings.Join(enabledIDs, ","), strings.Join(disabledIDs, ",")),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled))),
					resource.TestCheckResourceAttr("onelogin_mapping.test_2", "name", "test_mapping_enabled"),
					resource.TestCheckResourceAttr("onelogin_mapping.test_2", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test_2", "id"),
					resource.TestCheckResourceAttr("onelogin_mapping.test_2", "conditions.0.source", "last_login"),
					resource.TestCheckResourceAttr("onelogin_mapping.test_2", "conditions.0.operator", ">"),
					resource.TestCheckResourceAttr("onelogin_mapping.test_2", "conditions.0.value", "90"),
					resource.TestCheckResourceAttr("onelogin_mapping.test_2", "actions.0.action", "set_status"),
					resource.TestCheckResourceAttr("onelogin_mapping.test_2", "actions.0.value.0", "2"),
				),
			},
		},
	})
}

// Only run the mapping load test if the environment variable
// LOAD_TEST is set to 1.  This test takes >10 min to run
// so should only be explicitly tested as needed.
func (s *providerTestSuite) TestAccResourceMappingLoad() {
	if os.Getenv("LOAD_TEST") != "1" {
		return
	}

	// get all the enabled and disabled mappings
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

	mapping := func(i int) string {
		return fmt.Sprintf(`
			resource "onelogin_mapping" "terraform_test_%d" {
				name = "terraform_test_%d"
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
		`, i, i)
	}

	totalMappings := 1000

	enabledStart := enabledIDs
	configStart := &strings.Builder{}
	configStart.WriteString(s.providerConfig)
	for i := 0; i < totalMappings; i++ {
		configStart.WriteString(mapping(i))
		enabledStart = append(enabledStart, fmt.Sprintf("onelogin_mapping.terraform_test_%d.id", i))
	}
	configStart.WriteString(fmt.Sprintf(`
		resource "onelogin_mapping_order" "test" {
			enabled = [%s]
			disabled = [%s]
		}
	`, strings.Join(enabledStart, ","), strings.Join(disabledIDs, ",")))

	enabledUpdated := enabledIDs
	configUpdated := &strings.Builder{}
	configUpdated.WriteString(s.providerConfig)
	for i := 0; i < totalMappings; i++ {
		// delete every fifth mapping
		if i%5 == 0 {
			continue
		}
		configUpdated.WriteString(mapping(i))
		enabledUpdated = append(enabledUpdated, fmt.Sprintf("onelogin_mapping.terraform_test_%d.id", i))
	}
	rand.Shuffle(len(enabledUpdated), func(i, j int) { enabledUpdated[i], enabledUpdated[j] = enabledUpdated[j], enabledUpdated[i] })
	configUpdated.WriteString(fmt.Sprintf(`
		resource "onelogin_mapping_order" "test" {
			enabled = [%s]
			disabled = [%s]
		}
	`, strings.Join(enabledUpdated, ","), strings.Join(disabledIDs, ",")))

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: configStart.String(),
			},
			{
				Config: configUpdated.String(),
			},
			{
				Config: s.providerConfig,
			},
		},
	})
}
