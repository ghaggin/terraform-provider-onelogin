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

	enabledStr := "[" + strings.Join(enabledIDs, ",") + "]"
	enabledStrWithNewResource := "[" + strings.Join(enabledIDs, ",") + ",onelogin_mapping.test.id" + "]"
	disabledStr := "[" + strings.Join(disabledIDs, ",") + "]"
	disabledStrWithNewResource := "[" + strings.Join(disabledIDs, ",") + ",onelogin_mapping.test.id" + "]"

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
				`, enabledStr, disabledStrWithNewResource),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled))),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "enabled", "false"),
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
				`, enabledStrWithNewResource, disabledStr),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "enabled.#", fmt.Sprintf("%v", len(enabled)+1)),
					resource.TestCheckResourceAttr("onelogin_mapping_order.test", "disabled.#", fmt.Sprintf("%v", len(disabled))),
					resource.TestCheckResourceAttr("onelogin_mapping.test", "enabled", "true"),
				),
			},
		},
	})
}
