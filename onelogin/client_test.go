package onelogin

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type clientTestSuite struct {
	suite.Suite

	clientID     string
	clientSecret string
	subdomain    string

	client *Client

	ctx context.Context
}

func (s *clientTestSuite) SetupTest() {
	c, err := NewClient(&ClientConfig{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		Subdomain:    s.subdomain,
	})
	s.Require().NoError(err)
	s.client = c
}

func TestClient(t *testing.T) {
	clientTestSuite := &clientTestSuite{
		clientID:     os.Getenv("CLIENT_ID"),
		clientSecret: os.Getenv("CLIENT_SECRET"),
		subdomain:    os.Getenv("SUBDOMAIN"),

		ctx: context.Background(),
	}

	suite.Run(t, clientTestSuite)
}

func (s *clientTestSuite) Test_getToken() {
	testToken := "test_token"
	testExpiration := time.Now().Add(time.Hour)

	c := &Client{
		config: &ClientConfig{
			ClientID:     "",
			ClientSecret: "",
			Subdomain:    "",
		},
		authToken:      testToken,
		authExpiration: testExpiration,
		httpClient:     &http.Client{},
	}
	token, err := c.getToken(s.ctx)
	s.Require().NoError(err)
	s.Equal(testToken, token)
	s.Equal(token, c.authToken)
	s.Equal(testExpiration, c.authExpiration)

	c.authExpiration = time.Time{}
	token, err = c.getToken(s.ctx)
	s.Error(err)
	s.Equal("", token)
	s.Equal(token, c.authToken)
	s.Equal(time.Time{}, c.authExpiration)

	c.config.ClientID = s.clientID
	c.config.ClientSecret = s.clientSecret
	c.config.Subdomain = s.subdomain

	token, err = c.getToken(s.ctx)
	s.Require().NoError(err)
	s.NotEqual("", token)
	s.NotEqual(testToken, c.authToken)
	s.Equal(token, c.authToken)
	s.NotEqual(testExpiration, c.authExpiration)
	s.NotEqual(time.Time{}, c.authExpiration)
	s.True(time.Now().Before(c.authExpiration))
}

func (s *clientTestSuite) Test_ExecRequestPaged() {
	var resp1 []Role
	err := s.client.ExecRequestPaged(&Request{
		Method:    MethodGet,
		Path:      PathRoles,
		RespModel: &resp1,
	}, &Page{
		Limit: 2,
		Page:  1,
	})
	s.Require().NoError(err)

	var resp2 []Role
	err = s.client.ExecRequestPaged(&Request{
		Method:    MethodGet,
		Path:      PathRoles,
		RespModel: &resp2,
	}, &Page{
		Limit: 1,
		Page:  2,
	})
	s.Require().NoError(err)

	s.Require().Equal(2, len(resp1))
	s.Require().Equal(1, len(resp2))

	s.Equal(resp1[1].ID, resp2[0].ID)
}
