package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	// testAccProtoV6ProviderFactories are used to instantiate a provider during
	// acceptance testing. The factory function will be invoked for every Terraform
	// CLI command executed to create a provider server to which the CLI can
	// reattach.
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"onelogin": providerserver.NewProtocol6WithError(New("test")()),
	}
)

type PlanCheckImpl struct {
	checkPlanFunc CheckPlanFunc
}

func (p *PlanCheckImpl) CheckPlan(ctx context.Context, req plancheck.CheckPlanRequest, res *plancheck.CheckPlanResponse) {
	p.checkPlanFunc(ctx, req, res)
}

type CheckPlanFunc func(context.Context, plancheck.CheckPlanRequest, *plancheck.CheckPlanResponse)

func plancheckFunc(checkPlanFunc CheckPlanFunc) plancheck.PlanCheck {
	return &PlanCheckImpl{
		checkPlanFunc: checkPlanFunc,
	}
}

type providerTestSuite struct {
	suite.Suite

	client         *client
	providerConfig string
}

func TestProvider(t *testing.T) {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	subdomain := os.Getenv("SUBDOMAIN")

	client, err := newClient(&clientConfig{
		clientID:     clientID,
		clientSecret: clientSecret,
		subdomain:    subdomain,
	})
	require.NoError(t, err)

	testSuite := &providerTestSuite{
		client: client,
		providerConfig: fmt.Sprintf(`
		provider "onelogin" {
			client_id = "%s"
			client_secret = "%s"
			subdomain = "%s"
		}
		`, clientID, clientSecret, subdomain),
	}

	suite.Run(t, testSuite)
}

func (s *providerTestSuite) SetupTest() {
}

func (s *providerTestSuite) randString() string {
	return acctest.RandStringFromCharSet(5, acctest.CharSetAlphaNum)
}
