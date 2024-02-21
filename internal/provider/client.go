package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
)

type client struct {
	config         *clientConfig
	httpClient     *http.Client
	authToken      string
	authExpiration time.Time
}

type clientConfig struct {
	clientID     string
	clientSecret string
	subdomain    string
	timeout      time.Duration
}

type authResponse struct {
	AccessToken  string    `json:"access_token,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	AccountID    int       `json:"account_id,omitempty"`
}

type method string

const (
	methodGet    method = http.MethodGet
	methodPost   method = http.MethodPost
	methodPut    method = http.MethodPut
	methodDelete method = http.MethodDelete

	pathApps     = "/api/2/apps"
	pathRoles    = "/api/2/roles"
	pathUsers    = "/api/2/users"
	pathMappings = "/api/2/mappings"
)

type oneloginRequest struct {
	method      method
	path        string
	queryParams queryParamInterface
	body        interface{} // error returned if this can't be marshalled to json
	respModel   interface{} // error returned if this can't be unmarshalled from json
}

type queryParamInterface interface {
	// to query string returns a query string from the queryParams instance
	toQueryString() string
}

type queryParams map[string]interface{}

func (q queryParams) toQueryString() string {
	if len(q) == 0 {
		return ""
	}

	values := url.Values{}
	for key, value := range q {
		values.Add(key, fmt.Sprintf("%v", value))
	}

	return "?" + values.Encode()
}

func newClient(config *clientConfig) (*client, error) {
	if config.timeout == 0 {
		config.timeout = defaultTimeout
	}

	c := &client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.timeout,
		},
	}

	// Attempt to authenticate
	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()
	_, err := c.getToken(ctx)

	return c, err
}

func (c *client) getToken(ctx context.Context) (string, error) {
	if time.Now().After(c.authExpiration) {
		return c.getTokenForce(ctx)
	}

	return c.authToken, nil
}

func (c *client) getTokenForce(ctx context.Context) (string, error) {
	resp, err := c.authRequest(ctx)
	if err != nil {
		c.authToken = ""
		c.authExpiration = time.Time{}
	} else {
		c.authToken = resp.AccessToken
		c.authExpiration = resp.CreatedAt.Add(time.Duration(resp.ExpiresIn) * time.Second)
	}

	return c.authToken, err
}

func (c *client) authRequest(ctx context.Context) (*authResponse, error) {
	authURL := fmt.Sprintf("https://%s.onelogin.com/auth/oauth2/v2/token", c.config.subdomain)

	// Convert payload to JSON
	jsonBody, _ := json.Marshal(map[string]string{
		"grant_type": "client_credentials",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.config.clientID, c.config.clientSecret)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed with status code %d", resp.StatusCode)
	}

	var authResponse authResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	if err != nil {
		return nil, err
	}

	return &authResponse, err
}

func (c *client) execRequest(req *oneloginRequest) error {
	return c.execRequestCtx(context.Background(), req)
}

func (c *client) execRequestCtx(ctx context.Context, req *oneloginRequest) error {
	// add configured timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()

	url := fmt.Sprintf("https://%s.onelogin.com%s", c.config.subdomain, req.path)
	if req.queryParams != nil {
		url += req.queryParams.toQueryString()
	}

	var body io.Reader
	if req.body != nil {
		bodyBytes, err := json.Marshal(req.body)
		if err != nil {
			return err
		}
		body = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, string(req.method), url, body)
	if err != nil {
		return err
	}

	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	if body != nil {
		httpReq.Header.Add("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
	} else if resp.StatusCode/100 != 2 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status code %d\n%s", resp.StatusCode, string(bodyBytes))
	}

	if req.respModel != nil {
		return json.NewDecoder(resp.Body).Decode(req.respModel)
	}

	return nil
}
