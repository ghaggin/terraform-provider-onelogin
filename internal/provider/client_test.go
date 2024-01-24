package provider

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

	ctx context.Context
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

	c := &client{
		config: &clientConfig{
			clientID:     "",
			clientSecret: "",
			subdomain:    "",
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

	c.config.clientID = s.clientID
	c.config.clientSecret = s.clientSecret
	c.config.subdomain = s.subdomain

	token, err = c.getToken(s.ctx)
	s.Require().NoError(err)
	s.NotEqual("", token)
	s.NotEqual(testToken, c.authToken)
	s.Equal(token, c.authToken)
	s.NotEqual(testExpiration, c.authExpiration)
	s.NotEqual(time.Time{}, c.authExpiration)
	s.True(time.Now().Before(c.authExpiration))
}
