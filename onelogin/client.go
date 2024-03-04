package onelogin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	DefaultTimeout = 60 * time.Second
)

var (
	ErrRateLimitExceeded = fmt.Errorf("rate limit exceeded")
	ErrBadGateway        = fmt.Errorf("bad gateway")
	ErrNoMorePages       = fmt.Errorf("no more pages")
)

type Client struct {
	config         *ClientConfig
	httpClient     *http.Client
	authToken      string
	authExpiration time.Time

	maxPageSize map[string]int
	log         Logger
}

type ClientConfig struct {
	ClientID     string
	ClientSecret string
	Subdomain    string
	Timeout      time.Duration
	Logger       Logger
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

	PathApps         = "/api/2/apps"
	PathRoles        = "/api/2/roles"
	PathUsers        = "/api/2/users"
	PathMappings     = "/api/2/mappings"
	PathMappingsSort = "/api/2/mappings/sort"
)

type Request struct {
	Context     context.Context
	Method      method
	Path        string
	QueryParams QueryParamInterface
	Body        interface{} // error returned if this can't be marshalled to json
	RespModel   interface{} // error returned if this can't be unmarshalled from json
}

type QueryParamInterface interface {
	// to query string returns a query string from the queryParams instance
	toQueryString() string
	add(key string, value interface{})
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

func (q QueryParams) add(key string, value interface{}) {
	q[key] = value
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	if config.Logger == nil {
		config.Logger = &noopLogger{}
	}

	c := &Client{
		config: config,
		log:    config.Logger,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},

		maxPageSize: map[string]int{
			PathRoles: 650,
			PathApps:  1000,
			PathUsers: 50,
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

func (c *Client) ExecRequest(req *Request) (err error) {
	c.log.Info(req.Context, "executing request", map[string]interface{}{
		"method": req.Method,
		"path":   req.Path,
	})
	defer func() {
		if err != nil {
			c.log.Error(req.Context, "request failed", map[string]interface{}{
				"method": req.Method,
				"path":   req.Path,
				"error":  err.Error(),
			})
		}
	}()

	httpReq, err := c.requestToHTTP(req)
	if err != nil {
		return err
	}

	// Add default timeout
	ctx, cancel := context.WithTimeout(httpReq.Context(), c.config.Timeout)
	defer cancel()

	resp, err := c.httpClient.Do(httpReq.WithContext(ctx))
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

func (c *Client) requestToHTTP(req *Request) (*http.Request, error) {
	if req.Context == nil {
		req.Context = context.Background()
	}

	url := fmt.Sprintf("https://%s.onelogin.com%s", c.config.Subdomain, req.Path)
	if req.QueryParams != nil {
		url += req.QueryParams.toQueryString()
	}

	var body io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(req.Context, string(req.Method), url, body)
	if err != nil {
		return nil, err
	}

	token, err := c.getToken(req.Context)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	if body != nil {
		httpReq.Header.Add("Content-Type", "application/json")
	}

	return httpReq, nil
}

type Page struct {
	Limit int
	Page  int
}

// Pagination reference: https://developers.onelogin.com/api-docs/2/getting-started/using-query-parameters#pagination
func (c *Client) ExecRequestPaged(req *Request, page *Page) (err error) {
	var errInfo string
	c.log.Info(req.Context, "executing paged request", map[string]interface{}{
		"method": req.Method,
		"path":   req.Path,
	})
	defer func() {
		if err != nil {
			c.log.Error(req.Context, "paged request failed", map[string]interface{}{
				"method":     req.Method,
				"path":       req.Path,
				"error":      err.Error(),
				"error_info": errInfo,
			})
		}
	}()

	if req.QueryParams == nil {
		req.QueryParams = QueryParams{}
	}

	maxLimit, ok := c.maxPageSize[req.Path]
	if !ok {
		return fmt.Errorf("max page size not configured for path %s", req.Path)
	}

	if page.Limit > maxLimit {
		page.Limit = maxLimit
	}

	req.QueryParams.add("limit", page.Limit)
	req.QueryParams.add("page", page.Page)

	httpReq, err := c.requestToHTTP(req)
	if err != nil {
		return err
	}

	// Add default timeout
	ctx, cancel := context.WithTimeout(httpReq.Context(), c.config.Timeout)
	defer cancel()

	resp, err := c.httpClient.Do(httpReq.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway {
		b, _ := io.ReadAll(resp.Body)
		errInfo = string(b)
		return ErrBadGateway
	} else if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimitExceeded
	} else if resp.StatusCode/100 != 2 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status code %d\n%s", resp.StatusCode, string(bodyBytes))
	}

	if req.RespModel != nil {
		err = json.NewDecoder(resp.Body).Decode(req.RespModel)
		if err != nil {
			return err
		}
	}

	totalPagesString := resp.Header.Get("Total-Pages")
	if totalPagesString == "" {
		return fmt.Errorf("missing Total-Pages header")
	}

	totalPages, err := strconv.Atoi(totalPagesString)
	if err != nil {
		return err
	}

	if page.Page == totalPages {
		return ErrNoMorePages
	}

	return nil
}
