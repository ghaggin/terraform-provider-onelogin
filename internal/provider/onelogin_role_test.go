package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Isolated Role Test
// This tests functionality that is not dependent on other resources
func (s *providerTestSuite) TestAccOneloginRoleIsolated() {
	roleName := "test_role_" + acctest.RandStringFromCharSet(5, acctest.CharSetAlphaNum)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_role" "test_role" {
						name = "%v"
					}
					`, roleName),

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_role.test_role", "name", roleName),
				),
			},
		},
	})
}

// Integrated Role Test
// This tests functionality that depends on other resources
func (s *providerTestSuite) TestAccOneloginRoleIntegrated() {
	roleName := "test_role_" + s.randString()
	userName1 := "test_user_" + s.randString()
	userName2 := "test_user_" + s.randString()
	appName1 := "test_app_" + s.randString()
	appName2 := "test_app_" + s.randString()
	appConnector := "110016"

	// Map of ids is filled in in the first step ConfigPlanChecks
	ids := map[string]string{}

	// Given a resource name, return a function that compares the id of the resource
	compareID := func(name string) func(v string) error {
		return func(v string) error {
			id, ok := ids[name]
			if !ok {
				return fmt.Errorf("no id found for %v", name)
			}
			if v != id {
				return fmt.Errorf("expected %v, got %v", id, v)
			}
			return nil
		}
	}

	// Given a resource name, return a function that checks the id of the resource
	checkID := func(name string) resource.TestCheckFunc {
		return resource.TestCheckResourceAttrWith(name, "id", compareID(name))
	}

	// User and Apps resource config common to all checks
	usersAndAppsConfig := fmt.Sprintf(`
			resource "onelogin_user" "test_user_1" {
				username = "%v"
			}

			resource "onelogin_user" "test_user_2" {
				username = "%v"
			}
		`, userName1, userName2) +
		fmt.Sprintf(`
			resource "onelogin_app" "test_app_1" {
				name = "%v"
				connector_id = %v
			}

			resource "onelogin_app" "test_app_2" {
				name = "%v"
				connector_id = %v
			}
		`, appName1, appConnector, appName2, appConnector)

	// Config used to create initially create the role
	startingConfig := s.providerConfig +
		fmt.Sprintf(`
			resource "onelogin_role" "test_role" {
				name = "%v"
				users = [onelogin_user.test_user_1.id]
				admins = [onelogin_user.test_user_1.id]
				apps = [onelogin_app.test_app_1.id]
			}
		`, roleName) + usersAndAppsConfig

	// Config used to update the role
	updatedConfig := s.providerConfig +
		fmt.Sprintf(`
			resource "onelogin_role" "test_role" {
				name = "%v"
				users = [onelogin_user.test_user_2.id]
				admins = [onelogin_user.test_user_2.id]
				apps = [onelogin_app.test_app_2.id]
			}
		`, roleName) + usersAndAppsConfig

	// Run the tests
	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: startingConfig,

				// Fill in IDs
				// The values in the id map cannot be used in this steps Check function
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheckFunc(func(ctx context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
							var p struct {
								Resource_changes []struct {
									Address string
									Change  struct {
										After struct {
											Id int64
										}
									}
								}
							}

							j, err := json.Marshal(req.Plan)
							s.Require().NoError(err)
							err = json.Unmarshal(j, &p)
							s.Require().NoError(err)

							for _, resourceChanges := range p.Resource_changes {
								ids[resourceChanges.Address] = fmt.Sprintf("%v", resourceChanges.Change.After.Id)
							}
						}),
					},
				},

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_role.test_role", "name", roleName),
					resource.TestCheckResourceAttr("onelogin_user.test_user_1", "username", userName1),
					resource.TestCheckResourceAttr("onelogin_user.test_user_2", "username", userName2),
					resource.TestCheckResourceAttr("onelogin_app.test_app_1", "name", appName1),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "name", appName2),

					resource.TestCheckResourceAttrSet("onelogin_user.test_user_1", "id"),
					resource.TestCheckResourceAttrSet("onelogin_user.test_user_2", "id"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_1", "id"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "id"),

					resource.TestCheckResourceAttr("onelogin_role.test_role", "users.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "admins.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "apps.#", "1"),
				),
			},
			{
				// Maintain the state to perform id dependent tests
				// Putting these in the above step causes the tests to fail because
				// check functions are run before the ids are saved
				Config: startingConfig,

				Check: resource.ComposeAggregateTestCheckFunc(
					checkID("onelogin_user.test_user_1"),
					checkID("onelogin_user.test_user_2"),
					checkID("onelogin_app.test_app_1"),
					checkID("onelogin_app.test_app_2"),

					resource.TestCheckResourceAttr("onelogin_role.test_role", "users.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "admins.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "apps.#", "1"),

					resource.TestCheckResourceAttrWith("onelogin_role.test_role", "users.0", compareID("onelogin_user.test_user_1")),
					resource.TestCheckResourceAttrWith("onelogin_role.test_role", "admins.0", compareID("onelogin_user.test_user_1")),
					resource.TestCheckResourceAttrWith("onelogin_role.test_role", "apps.0", compareID("onelogin_app.test_app_1")),
				),
			},

			// Update the role and check the new values
			{
				Config: updatedConfig,

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_role.test_role", "name", roleName),
					resource.TestCheckResourceAttr("onelogin_user.test_user_1", "username", userName1),
					resource.TestCheckResourceAttr("onelogin_user.test_user_2", "username", userName2),
					resource.TestCheckResourceAttr("onelogin_app.test_app_1", "name", appName1),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "name", appName2),

					checkID("onelogin_role.test_role"),
					checkID("onelogin_user.test_user_1"),
					checkID("onelogin_user.test_user_2"),
					checkID("onelogin_app.test_app_1"),
					checkID("onelogin_app.test_app_2"),

					resource.TestCheckResourceAttr("onelogin_role.test_role", "users.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "admins.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "apps.#", "1"),

					resource.TestCheckResourceAttrWith("onelogin_role.test_role", "users.0", compareID("onelogin_user.test_user_2")),
					resource.TestCheckResourceAttrWith("onelogin_role.test_role", "admins.0", compareID("onelogin_user.test_user_2")),
					resource.TestCheckResourceAttrWith("onelogin_role.test_role", "apps.0", compareID("onelogin_app.test_app_2")),
				),
			},
		},
	})
}

func (s *providerTestSuite) TestAccOneloginRoleImported() {
	var users []onelogin.User
	err := s.client.ExecRequestPaged(&onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathUsers,
		RespModel: &users,
	}, &onelogin.Page{
		Limit: 2,
		Page:  1,
	})
	s.Require().Nil(err)
	s.Require().Len(users, 2)

	var apps []onelogin.Application
	err = s.client.ExecRequestPaged(&onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathApps,
		RespModel: &apps,
	}, &onelogin.Page{
		Limit: 2,
		Page:  1,
	})
	s.Require().Nil(err)
	s.Require().Len(apps, 2)

	roleName := "test_role_" + s.randString()

	var role struct {
		Id int64
	}
	err = s.client.ExecRequest(&onelogin.Request{
		Method: onelogin.MethodPost,
		Path:   onelogin.PathRoles,
		Body: onelogin.Role{
			Name:  roleName,
			Users: []int64{users[0].ID},
			Apps:  []int64{apps[0].ID},
		},
		RespModel: &role,
	})
	s.Require().Nil(err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					import {
						id = %v
						to = onelogin_role.test_role
					}
					resource "onelogin_role" "test_role" {
						name = "%v"
						apps = [%v]
						users = [data.onelogin_user.test.id]
					}

					data "onelogin_user" "test" {
						username = "%v"
					}
				`, role.Id, roleName, apps[0].ID, users[1].Username),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_role.test_role", "name", roleName),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "apps.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "apps.0", fmt.Sprintf("%v", apps[0].ID)),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "users.#", "1"),
					resource.TestCheckResourceAttr("onelogin_role.test_role", "users.0", fmt.Sprintf("%v", users[1].ID)),
					resource.TestCheckResourceAttrSet("data.onelogin_user.test", "id"),
				),
			},
		},
	})
}
