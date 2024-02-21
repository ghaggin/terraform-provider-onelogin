package provider

import (
	"fmt"
	"regexp"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func (s *providerTestSuite) TestAccDatasourceUser() {
	username := "test_user_" + acctest.RandStringFromCharSet(5, acctest.CharSetAlphaNum)
	var id int64

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create the user that will be queried by the data source.
				PreConfig: func() {
					var respModel struct {
						ID int64 `json:"id"`
					}
					err := s.client.ExecRequest(&onelogin.Request{
						Method: onelogin.MethodPost,
						Path:   onelogin.PathUsers,
						Body: &onelogin.User{
							Username: username,
						},
						RespModel: &respModel,
					})
					s.Require().NoError(err)
					s.Require().NotZero(respModel.ID)
					id = respModel.ID
				},

				// Query the user created in the PreConfig step
				Config: s.providerConfig + fmt.Sprintf(`data "onelogin_user" "test_user" {
					username = "%v"
				}`, username),

				// Check that the user returned by the data source matches the user created in the PreConfig step
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.onelogin_user.test_user", "username", username),
					resource.TestCheckResourceAttrWith("data.onelogin_user.test_user", "id", func(v string) error {
						expected := fmt.Sprintf("%v", id)
						if v != expected {
							return fmt.Errorf("%s: Attribute '%s' expected %s, got %s", "data.onelogin_user.test_user", "id", expected, v)
						}
						return nil
					}),
				),
			},
			{
				// Clean up the user created in the PreConfig step in the previous TestStep
				// Config should be the provider config and should not add any resources.
				PreConfig: func() {
					err := s.client.ExecRequest(&onelogin.Request{
						Method: onelogin.MethodDelete,
						Path:   fmt.Sprintf("%v/%v", onelogin.PathUsers, id),
					})
					s.NoError(err)
				},
				Config: s.providerConfig,
			},
		},
	})
}

func (s *providerTestSuite) TestAccResourceUser() {
	name := "test_user_" + s.randString()
	nameUpdated := "test_user_" + s.randString()

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{

			// Create and read testing
			{
				Config: s.providerConfig + fmt.Sprintf(`
						resource "onelogin_user" "test_user" {
							username = "%v"
						}
					`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_user.test_user", "username", name),
					resource.TestCheckResourceAttrSet("onelogin_user.test_user", "id"),
				),
			},

			// Try to create a duplicate resource with the same username
			{
				Config: s.providerConfig + fmt.Sprintf(`
						resource "onelogin_user" "test_user" {
							username = "%v"
						}

						resource "onelogin_user" "test_user_2" {
							username = "%v"
						}
					`, name, name),
				ExpectError: regexp.MustCompile("client error"),
			},

			// ImportState testing
			{
				ResourceName:            "onelogin_user.test_user",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"last_updated"},
			},

			// Update and read testing
			{
				Config: s.providerConfig + fmt.Sprintf(`
						resource "onelogin_user" "test_user" {
							username = "%v"
						}
					`, nameUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_user.test_user", "username", nameUpdated),
					resource.TestCheckResourceAttrSet("onelogin_user.test_user", "id"),
				),
			},
		},
	})
}
