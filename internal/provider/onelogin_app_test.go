package provider

import (
	"context"
	"fmt"
	"regexp"

	fres "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func (s *providerTestSuite) TestAccResourceAppMisc() {
	var appi interface{} = NewOneLoginAppResource(&client{})
	s.NotNil(appi)

	app := oneloginAppResource{}

	ctx := context.Background()
	resp := &fres.MetadataResponse{}
	app.Metadata(ctx, fres.MetadataRequest{
		ProviderTypeName: "onelogin",
	}, resp)
	s.Equal("onelogin_app", resp.TypeName)

	// no-op
	app.Configure(ctx, fres.ConfigureRequest{}, &fres.ConfigureResponse{})

	sresp := &fres.SchemaResponse{}
	app.Schema(ctx, fres.SchemaRequest{}, sresp)
	s.Contains(sresp.Schema.Attributes, "id")
}

func (s *providerTestSuite) TestAccResourceApp() {
	name := "test_app_" + acctest.RandStringFromCharSet(5, acctest.CharSetAlphaNum)
	connectorID := "110016"

	checkRegex := func(r string) func(string) error {
		return func(v string) error {
			s.True(regexp.MustCompile(r).MatchString(v))
			return nil
		}
	}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_app" "test_app" {
						name = "%v"
						connector_id = %v
					}
				`, name, connectorID),
				Check: resource.ComposeAggregateTestCheckFunc(
					// explicitly set
					resource.TestCheckResourceAttr("onelogin_app.test_app", "name", name),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "connector_id", connectorID),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "provisioning_enabled", "false"),

					// unknown until apply
					resource.TestCheckResourceAttrSet("onelogin_app.test_app", "id"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app", "created_at"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app", "updated_at"),

					// don't care about value
					resource.TestCheckResourceAttrSet("onelogin_app.test_app", "icon_url"),

					// default attributes
					// these were determined from creating an app with this connector_id and
					// checking the defaults.  These may be subject to change
					resource.TestCheckResourceAttr("onelogin_app.test_app", "visible", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "auth_method", "2"),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "auth_method_description", "SAML2.0"),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "allow_assumed_signin", "false"),

					resource.TestCheckNoResourceAttr("onelogin_app.test_app", "description"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app", "tab_id"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app", "notes"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app", "policy_id"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app", "brand_id"),

					// sso
					resource.TestCheckResourceAttrWith("onelogin_app.test_app", "sso.acs_url", checkRegex(`^https://[^\.]+\.onelogin\.com/trust/saml2/http-post/sso/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app", "sso.issuer", checkRegex(`^https://app\.onelogin\.com/saml/metadata/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app", "sso.metadata_url", checkRegex(`^https://app\.onelogin\.com/saml/metadata/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app", "sso.sls_url", checkRegex(`^https://[^\.]+\.onelogin\.com/trust/saml2/http-redirect/slo/[0-9]+$`)),

					// parameters
					// connector_id 110016 has 1 default parameter with attributes:
					//  label                    = "NameID value"
					//  provisioned_entitlements = false
					//  skip_if_blank            = false
					resource.TestCheckResourceAttr("onelogin_app.test_app", "parameters.%", "1"),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "parameters.saml_username.label", "NameID value"),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "parameters.saml_username.provisioned_entitlements", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app", "parameters.saml_username.skip_if_blank", "false"),

					// TODO:
					// provisioning
					// configuration
				),
			},
			{
				// This will cause test_app to be deleted and test_app_2 to be created
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_app" "test_app_2" {
						name = "%v"
						connector_id = %v

						visible = true
						allow_assumed_signin = true

						parameters = {
							"saml_username" = {
							  label                    = "NameID value"
							  provisioned_entitlements = false
							  skip_if_blank            = false
							}
							"test" = {
							  label                     = "test_label"
							  provisioned_entitlements  = true
							  skip_if_blank             = true
							  include_in_saml_assertion = true
							}
						  }
					}
				`, name, connectorID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "name", name),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "connector_id", connectorID),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "visible", "true"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "allow_assumed_signin", "true"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "provisioning_enabled", "false"),

					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "id"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "created_at"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "updated_at"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "icon_url"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "auth_method"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "auth_method_description"),

					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "description"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "tab_id"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "notes"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "policy_id"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "brand_id"),

					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.acs_url", checkRegex(`^https://[^\.]+\.onelogin\.com/trust/saml2/http-post/sso/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.issuer", checkRegex(`^https://app\.onelogin\.com/saml/metadata/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.metadata_url", checkRegex(`^https://app\.onelogin\.com/saml/metadata/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.sls_url", checkRegex(`^https://[^\.]+\.onelogin\.com/trust/saml2/http-redirect/slo/[0-9]+$`)),

					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.%", "2"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.saml_username.label", "NameID value"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.saml_username.provisioned_entitlements", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.saml_username.skip_if_blank", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.label", "test_label"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.provisioned_entitlements", "true"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.skip_if_blank", "true"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.include_in_saml_assertion", "true"),
				),
			},

			{
				// This will update test_app_2
				//
				// TODO: update parameters, waiting on onelogin to fix api
				Config: s.providerConfig + fmt.Sprintf(`
					resource "onelogin_app" "test_app_2" {
						name = "%v"
						connector_id = %v

						visible = false
						allow_assumed_signin = false

						parameters = {
							"saml_username" = {
							  label                    = "NameID value"
							  provisioned_entitlements = false
							  skip_if_blank            = false
							}
							"test" = {
							  label                     = "test_label"
							  provisioned_entitlements  = true
							  skip_if_blank             = true
							  include_in_saml_assertion = true
							}
						  }
					}
				`, name, connectorID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "name", name),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "connector_id", connectorID),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "visible", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "allow_assumed_signin", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "provisioning_enabled", "false"),

					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "id"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "created_at"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "updated_at"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "icon_url"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "auth_method"),
					resource.TestCheckResourceAttrSet("onelogin_app.test_app_2", "auth_method_description"),

					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "description"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "tab_id"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "notes"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "policy_id"),
					resource.TestCheckNoResourceAttr("onelogin_app.test_app_2", "brand_id"),

					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.acs_url", checkRegex(`^https://[^\.]+\.onelogin\.com/trust/saml2/http-post/sso/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.issuer", checkRegex(`^https://app\.onelogin\.com/saml/metadata/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.metadata_url", checkRegex(`^https://app\.onelogin\.com/saml/metadata/[a-z0-9-]+$`)),
					resource.TestCheckResourceAttrWith("onelogin_app.test_app_2", "sso.sls_url", checkRegex(`^https://[^\.]+\.onelogin\.com/trust/saml2/http-redirect/slo/[0-9]+$`)),

					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.%", "2"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.saml_username.label", "NameID value"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.saml_username.provisioned_entitlements", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.saml_username.skip_if_blank", "false"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.label", "test_label"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.provisioned_entitlements", "true"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.skip_if_blank", "true"),
					resource.TestCheckResourceAttr("onelogin_app.test_app_2", "parameters.test.include_in_saml_assertion", "true"),
				),
			},

			{
				ResourceName:      "onelogin_app.test_app_2",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func (s *providerTestSuite) Test_appToState() {
	ClientID := "test_ClientID"
	ClientSecret := "test_ClientSecret"
	MetadataURL := "https://example.com/metadata"
	ACSURL := "https://example.com/acs"
	SLSURL := "https://example.com/sls"
	Issuer := "https://example.com/issuer"
	WSFED_SSO_URL := "https://example.com/wsfed_sso"

	CertificateID := int64(1)
	CertificateValue := "abcd"
	CertificateName := "test"

	app := &oneloginNativeApp{
		ID:                    1,
		Name:                  "test",
		ConnectorID:           2,
		Visible:               true,
		AuthMethod:            3,
		AuthMethodDescription: "test",
		AllowAssumedSignin:    true,
		CreatedAt:             "2020-01-01T00:00:00.000Z",
		UpdatedAt:             "2020-01-01T00:00:00.000Z",
		IconURL:               "https://example.com",

		SSO: &oneloginNativeAppSSO{
			ClientID:      &ClientID,
			ClientSecret:  &ClientSecret,
			MetadataURL:   &MetadataURL,
			ACSURL:        &ACSURL,
			SLSURL:        &SLSURL,
			Issuer:        &Issuer,
			WSFED_SSO_URL: &WSFED_SSO_URL,
			Certificate: oneloginNativeAppCertificate{
				CertificateID:    &CertificateID,
				CertificateValue: &CertificateValue,
				CertificateName:  &CertificateName,
			},
		},

		Parameters: map[string]oneloginNativeAppParameter{
			"test": {
				ID:                      1,
				Label:                   "test_label",
				ProvisionedEntitlements: false,
				SkipIfBlank:             false,
			},
			"test_2": {
				ID:                      2,
				Label:                   "test_2_label",
				ProvisionedEntitlements: true,
				SkipIfBlank:             true,
			},
		},
	}

	state, diags := app.toState(context.Background())
	if diags.HasError() {
		s.T().Fatalf("unexpected error: %v", diags.Errors())
	}

	if state.SSO.IsNull() {
		s.T().Fatal("expected SSO to be non-null")
	}
	if state.SSO.IsUnknown() {
		s.T().Fatal("expected SSO to be non-unknown")
	}

	if state.Parameters.IsNull() {
		s.T().Fatal("expected Parameters to be non-null")
	}
	if state.Parameters.IsUnknown() {
		s.T().Fatal("expected Parameters to be non-unknown")
	}

	newApp := state.toNativApp(context.Background())

	s.Equal(app.ID, newApp.ID)
	s.Equal(app.SSO.ClientID, newApp.SSO.ClientID)
	s.Equal(app.SSO.ClientSecret, newApp.SSO.ClientSecret)
	s.Equal(app.SSO.MetadataURL, newApp.SSO.MetadataURL)
	s.Equal(app.SSO.ACSURL, newApp.SSO.ACSURL)
	s.Equal(app.SSO.SLSURL, newApp.SSO.SLSURL)
	s.Equal(app.SSO.Issuer, newApp.SSO.Issuer)
	s.Equal(app.SSO.WSFED_SSO_URL, newApp.SSO.WSFED_SSO_URL)
	s.Equal(app.SSO.Certificate.CertificateID, newApp.SSO.Certificate.CertificateID)
	s.Equal(app.SSO.Certificate.CertificateValue, newApp.SSO.Certificate.CertificateValue)
	s.Equal(app.SSO.Certificate.CertificateName, newApp.SSO.Certificate.CertificateName)

	s.Require().Contains(newApp.Parameters, "test")
	s.Equal(app.Parameters["test"].ID, newApp.Parameters["test"].ID)
	s.Equal(app.Parameters["test"].Label, newApp.Parameters["test"].Label)
	s.Equal(app.Parameters["test"].ProvisionedEntitlements, newApp.Parameters["test"].ProvisionedEntitlements)
	s.Equal(app.Parameters["test"].SkipIfBlank, newApp.Parameters["test"].SkipIfBlank)
	s.Require().Contains(newApp.Parameters, "test_2")
	s.Equal(app.Parameters["test_2"].ID, newApp.Parameters["test_2"].ID)
	s.Equal(app.Parameters["test_2"].Label, newApp.Parameters["test_2"].Label)
	s.Equal(app.Parameters["test_2"].ProvisionedEntitlements, newApp.Parameters["test_2"].ProvisionedEntitlements)
	s.Equal(app.Parameters["test_2"].SkipIfBlank, newApp.Parameters["test_2"].SkipIfBlank)
}
