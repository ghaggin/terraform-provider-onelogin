package onelogin

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
	DefaultTimeout = 10 * time.Second
)

type Client struct {
	config         *ClientConfig
	httpClient     *http.Client
	authToken      string
	authExpiration time.Time
}

type ClientConfig struct {
	ClientID     string
	ClientSecret string
	Subdomain    string
	Timeout      time.Duration
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
	MethodGet    method = http.MethodGet
	MethodPost   method = http.MethodPost
	MethodPut    method = http.MethodPut
	MethodDelete method = http.MethodDelete

	PathApps     = "/api/2/apps"
	PathRoles    = "/api/2/roles"
	PathUsers    = "/api/2/users"
	PathMappings = "/api/2/mappings"
)

type Request struct {
	Method      method
	Path        string
	QueryParams QueryParamInterface
	Body        interface{} // error returned if this can't be marshalled to json
	RespModel   interface{} // error returned if this can't be unmarshalled from json
}

type QueryParamInterface interface {
	// to query string returns a query string from the queryParams instance
	toQueryString() string
}

type QueryParams map[string]interface{}

func (q QueryParams) toQueryString() string {
	if len(q) == 0 {
		return ""
	}

	values := url.Values{}
	for key, value := range q {
		values.Add(key, fmt.Sprintf("%v", value))
	}

	return "?" + values.Encode()
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	c := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}

	// Attempt to authenticate
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	_, err := c.getToken(ctx)

	return c, err
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	if time.Now().After(c.authExpiration) {
		return c.getTokenForce(ctx)
	}

	return c.authToken, nil
}

func (c *Client) getTokenForce(ctx context.Context) (string, error) {
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

func (c *Client) authRequest(ctx context.Context) (*authResponse, error) {
	authURL := fmt.Sprintf("https://%s.onelogin.com/auth/oauth2/v2/token", c.config.Subdomain)

	// Convert payload to JSON
	jsonBody, _ := json.Marshal(map[string]string{
		"grant_type": "client_credentials",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.config.ClientID, c.config.ClientSecret)
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

func (c *Client) ExecRequest(req *Request) error {
	return c.ExecRequestCtx(context.Background(), req)
}

func (c *Client) ExecRequestCtx(ctx context.Context, req *Request) error {
	// add configured timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	url := fmt.Sprintf("https://%s.onelogin.com%s", c.config.Subdomain, req.Path)
	if req.QueryParams != nil {
		url += req.QueryParams.toQueryString()
	}

	var body io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return err
		}
		body = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, string(req.Method), url, body)
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

	if req.RespModel != nil {
		return json.NewDecoder(resp.Body).Decode(req.RespModel)
	}

	return nil
}
