package provider

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func (s *providerTestSuite) TestMappingOrder() {
	var resp []struct {
		ID       int64 `json:"id"`
		Enabled  bool  `json:"enabled"`
		Position int64 `json:"position"`
	}

	err := s.client.ExecRequest(&onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathMappings,
		RespModel: &resp,
		// QueryParams: onelogin.QueryParams{
		// 	"enabled": "false",
		// },
	})
	s.Require().NoError(err)

	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Position < resp[j].Position
	})

	for i, m := range resp {
		s.Equal(int64(i+1), m.Position)
	}
}

func (s *providerTestSuite) TestAccResourceMappingOrderIndependent() {
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

	enabledStr := "[" + strings.Join(enabledIDs, ",") + "]"
	disabledStr := "[" + strings.Join(disabledIDs, ",") + "]"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = %v
						disabled = %v
					}
				`, enabledStr, disabledStr),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled))),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled))),
				),
			},
		},
	})
}

func (s *providerTestSuite) TestAccResourceMappingOrderIntegrated() {
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

	enabledStr := "[" + strings.Join(enabledIDs, ",") + "]"
	enabledStrWithNewResource1 := "[" + strings.Join(enabledIDs, ",") + maybeComma(enabledIDs) + "onelogin_mapping.test.id]"
	enabledStrWithNewResource2 := "[onelogin_mapping.test.id" + maybeComma(enabledIDs) + strings.Join(enabledIDs, ",") + "]"
	disabledStr := "[" + strings.Join(disabledIDs, ",") + "]"
	disabledStrWithNewResource := "[" + strings.Join(disabledIDs, ",") + maybeComma(disabledIDs) + "onelogin_mapping.test.id]"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create new mapping in the disabled state
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
				`, enabledStr, disabledStrWithNewResource),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled))),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", "test_mapping"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "id"),
				),
			},
			// Update the mapping_order so that onelogin_mapping.test is enabled
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
				`, enabledStrWithNewResource1, disabledStr),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled))),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", "test_mapping"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "id"),
				),
			},
			// Update enabled mapping order so onelogin_mapping.test is the first in the order
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
				`, enabledStrWithNewResource2, disabledStr),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled))),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "name", "test_mapping"),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test", "id"),
				),
			},
		},
	})

	// TODO: following the test, verify that the mapping order is the same as before the test

}

func (s *providerTestSuite) TestAccResourceMappingOrderIntegratedDisabledOrdering() {
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

	enabledStr := "[" + strings.Join(enabledIDs, ",") + "]"
	disabledStrWithNewResource1 := "[onelogin_mapping.test2.id" + maybeComma(disabledIDs) + strings.Join(disabledIDs, ",") + ",onelogin_mapping.test1.id]"
	disabledStrWithNewResource2 := "[" + strings.Join(disabledIDs, ",") + maybeComma(disabledIDs) + "onelogin_mapping.test1.id,onelogin_mapping.test2.id]"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = %v
						disabled = %v
					}

					resource "onelogin_mapping" "test1" {
						name = "test_mapping1"
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

					resource "onelogin_mapping" "test2" {
						name = "test_mapping2"
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
				`, enabledStr, disabledStrWithNewResource1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled))),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled)+2)),
					resource.TestCheckResourceAttr("onelogin_mapping.test1", "name", "test_mapping1"),
					resource.TestCheckResourceAttr("onelogin_mapping.test1", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test1", "id"),
					resource.TestCheckResourceAttr("onelogin_mapping.test2", "name", "test_mapping2"),
					resource.TestCheckResourceAttr("onelogin_mapping.test2", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test2", "id"),
				),
			},
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_mapping_order" "test" {
						enabled = %v
						disabled = %v
					}

					resource "onelogin_mapping" "test1" {
						name = "test_mapping1"
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

					resource "onelogin_mapping" "test2" {
						name = "test_mapping2"
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
				`, enabledStr, disabledStrWithNewResource2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled))),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled)+2)),
					resource.TestCheckResourceAttr("onelogin_mapping.test1", "name", "test_mapping1"),
					resource.TestCheckResourceAttr("onelogin_mapping.test1", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test1", "id"),
					resource.TestCheckResourceAttr("onelogin_mapping.test2", "name", "test_mapping2"),
					resource.TestCheckResourceAttr("onelogin_mapping.test2", "match", "all"),
					resource.TestCheckResourceAttrSet("onelogin_mapping.test2", "id"),
				),
			},
		},
	})
}
