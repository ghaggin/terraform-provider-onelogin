package onelogin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRunClientTestSuite(t *testing.T) {
	s := &clientTestSuite{
		clientID:     "test_client_id",
		clientSecret: "test_client_secret",
		subdomain:    "test_subdomain",
	}
	suite.Run(t, s)
}

func (s *clientTestSuite) SetupTest() {
	authResponder, err := httpmock.NewJsonResponder(200, &authResponse{
		AccessToken:  "test_access_token",
		CreatedAt:    time.Now().UTC(),
		ExpiresIn:    int((time.Hour * 10).Seconds()),
		RefreshToken: "test_refresh_token",
		TokenType:    "bearer",
		AccountID:    0,
	})
	s.Require().NoError(err)

	httpmock.Activate()
	httpmock.RegisterResponder(http.MethodPost, "https://test_subdomain.onelogin.com/auth/oauth2/v2/token", authResponder)

	c, err := NewClient(&ClientConfig{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		Subdomain:    s.subdomain,
	})
	s.Require().NoError(err)
	s.client = c

}

func (s *clientTestSuite) TearDownTest() {
	httpmock.Deactivate()
}

func (s *clientTestSuite) Test_IsRetriable() {
	s.True(s.client.isRetriable(500, []int{504, 503, 500, 501}))
	s.False(s.client.isRetriable(401, []int{504, 503, 500, 501}))
}

func (s *clientTestSuite) Test_Retries() {
	request := &Request{
		Method: MethodGet,
		Path:   "/test",

		Retry:                3,
		RetriableStatusCodes: []int{500},
		RetryBackoffFactor:   1,
		RetryWait:            time.Second * 0,
	}

	// Test that all retries are exhausted and the last failure causes an error to be returned
	timesCalled := 0
	httpmock.RegisterResponder(string(MethodGet), "https://test_subdomain.onelogin.com/test", func(req *http.Request) (*http.Response, error) {
		timesCalled++
		return &http.Response{
			StatusCode: 500,
		}, nil
	})
	err := s.client.ExecRequest(request)
	s.Require().Error(err)
	s.Contains(err.Error(), "request failed with status code 500")
	s.Equal(4, timesCalled)

	// Test that it retries after one failure and then returns successfully after success.
	// Create a test response struct that will be returns as a json and processed.
	timesCalled = 0
	type testRespModel struct {
		Test string `json:"test"`
	}
	var resp testRespModel
	request.RespModel = &resp
	testRespContent := "test_resp_content"
	httpmock.RegisterResponder(string(MethodGet), "https://test_subdomain.onelogin.com/test", func(req *http.Request) (*http.Response, error) {
		timesCalled++
		if timesCalled == 2 {
			b, err := json.Marshal(testRespModel{Test: testRespContent})
			s.Require().NoError(err)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}
		return &http.Response{
			StatusCode: 500,
		}, nil
	})
	err = s.client.ExecRequest(request)
	s.Require().NoError(err)
	s.Equal(testRespContent, resp.Test)
	s.Equal(2, timesCalled)

	// Test context cancelation while waiting while waiting to retry
	timesCalled = 0
	called := make(chan bool, 1)
	defer close(called)
	httpmock.RegisterResponder(string(MethodGet), "https://test_subdomain.onelogin.com/test", func(req *http.Request) (*http.Response, error) {
		timesCalled++
		called <- true
		return &http.Response{
			StatusCode: 500,
		}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request.Context = ctx
	request.RetryWait = time.Hour
	go func() {
		<-called
		time.Sleep(time.Millisecond * 100)
		cancel()
	}()
	err = s.client.ExecRequest(request)
	s.Require().Error(err)
	s.Equal("context canceled", err.Error())
	s.Equal(1, timesCalled)

	// Test retry backoff set to 0
	// This means that there will be the same wait period for each retry
	timesCalled = 0
	timeCalled := make([]time.Time, 4)
	httpmock.RegisterResponder(string(MethodGet), "https://test_subdomain.onelogin.com/test", func(req *http.Request) (*http.Response, error) {
		timeCalled[timesCalled] = time.Now()
		timesCalled++
		return &http.Response{
			StatusCode: 500,
		}, nil
	})
	request.Context = context.Background()
	request.RetryWait = time.Millisecond * 100
	request.RetryBackoffFactor = 0
	s.client.ExecRequest(request)
	s.Require().Equal(4, timesCalled)
	timeBetween := []time.Duration{
		timeCalled[1].Sub(timeCalled[0]),
		timeCalled[2].Sub(timeCalled[1]),
		timeCalled[3].Sub(timeCalled[2]),
	}
	for _, d := range timeBetween {
		s.Greater(d, request.RetryWait)
		s.Greater(request.RetryWait*2, d)
	}

	// Test retry backoff set to 1
	// This means that there will be an exponentially increasing backoff by powers of 2.
	timesCalled = 0
	timeCalled = make([]time.Time, 4)
	httpmock.RegisterResponder(string(MethodGet), "https://test_subdomain.onelogin.com/test", func(req *http.Request) (*http.Response, error) {
		timeCalled[timesCalled] = time.Now()
		timesCalled++
		return &http.Response{
			StatusCode: 500,
		}, nil
	})
	request.Context = context.Background()
	request.RetryWait = time.Millisecond * 100
	request.RetryBackoffFactor = 1
	s.client.ExecRequest(request)
	s.Require().Equal(4, timesCalled)
	timeBetween = []time.Duration{
		timeCalled[1].Sub(timeCalled[0]),
		timeCalled[2].Sub(timeCalled[1]),
		timeCalled[3].Sub(timeCalled[2]),
	}
	for i, d := range timeBetween {
		s.Greater(d, request.RetryWait*time.Duration(pow(2, i)))
		s.Greater((request.RetryWait*time.Duration(pow(2, i)))+request.RetryWait, d)
	}
}

func TestAuth(t *testing.T) {
	ctx := context.Background()

	c := &Client{
		config: &ClientConfig{
			Subdomain:    "test_subdomain",
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
		},
		log:        NewNoopLogger(),
		httpClient: &http.Client{},
	}

	authResponse := &authResponse{
		AccessToken:  "test_access_token",
		CreatedAt:    time.Now().UTC(),
		ExpiresIn:    int((time.Hour * 10).Seconds()),
		RefreshToken: "test_refresh_token",
		TokenType:    "bearer",
		AccountID:    0,
	}

	authResponder, err := httpmock.NewJsonResponder(200, authResponse)
	require.NoError(t, err)
	httpmock.Activate()
	defer httpmock.Deactivate()
	httpmock.RegisterResponder(http.MethodPost, "https://test_subdomain.onelogin.com/auth/oauth2/v2/token", authResponder)
	resp, err := c.authRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, authResponse, resp)

	testErr := errors.New("test_error")
	httpmock.Deactivate()
	httpmock.Activate()
	httpmock.RegisterResponder(http.MethodPost, "https://test_subdomain.onelogin.com/auth/oauth2/v2/token", httpmock.NewErrorResponder(testErr))
	_, err = c.authRequest(ctx)
	assert.Equal(t, testErr, err.(*url.Error).Err)

	authResponder, err = httpmock.NewJsonResponder(401, authResponse)
	require.NoError(t, err)
	httpmock.Deactivate()
	httpmock.Activate()
	httpmock.RegisterResponder(http.MethodPost, "https://test_subdomain.onelogin.com/auth/oauth2/v2/token", authResponder)
	_, err = c.authRequest(ctx)
	assert.Error(t, err)
}
