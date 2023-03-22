package gqlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
)

const (
	// DefaultRetryCount is the default number of retries.
	DefaultRetryCount = 3
)

// Client is a GraphQL HTTP client.
type Client struct {
	endpoint      string
	http          *http.Client
	token         string
	retryCount    int
	tokenProvider TokenProvider
	mu            sync.Mutex
}

// TokenProvider is an interface for providing a token.
type TokenProvider interface {
	Token() (string, error)
}

// Request is a GraphQL request.
type Request struct {
	Operation string      `json:"-"`
	Query     string      `json:"query"`
	Variables interface{} `json:"variables"`
}

// Response is a GraphQL response.
type Response struct {
	Data   interface{} `json:"data"`
	Errors []Error     `json:"errors"`
}

// Options are options for creating a GraphQL client.
type Options struct {
	Endpoint      string
	HTTPClient    *http.Client
	TokenProvider TokenProvider
}

// New creates a new GraphQL client with the specified endpoint.
func New(opts Options) *Client {
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	return &Client{
		endpoint:      opts.Endpoint,
		http:          opts.HTTPClient,
		tokenProvider: opts.TokenProvider,
	}
}

// Query executes a GraphQL query.
func (c *Client) Query(ctx context.Context, q string, v interface{}, resp interface{}) error {
	req := &Request{
		Operation: "query",
		Query:     q,
		Variables: v,
	}
	return c.do(ctx, req, resp)
}

// Mutation executes a GraphQL mutation.
func (c *Client) Mutation(ctx context.Context, q string, v interface{}, resp interface{}) error {
	req := &Request{
		Operation: "mutation",
		Query:     q,
		Variables: v,
	}
	return c.do(ctx, req, resp)
}

// do executes a GraphQL request.
func (c *Client) do(ctx context.Context, req *Request, data interface{}) error {
	err := validateOperationVariables(req.Variables)
	if err != nil {
		return fmt.Errorf("failed to validate operation variables: %w", err)
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(jsonReq))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusUnauthorized {
		c.token = ""
		// perform at most one retry
		return c.retry(ctx, req, data)
	}

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}

	resp := &Response{
		Data: data,
	}

	if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return &resp.Errors[0]
	}

	return nil
}

// retry retries a GraphQL request.
func (c *Client) retry(ctx context.Context, req *Request, data interface{}) error {
	c.mu.Lock()
	c.retryCount++
	c.mu.Unlock()

	if c.retryCount > DefaultRetryCount {
		c.mu.Lock()
		c.retryCount = 0
		c.mu.Unlock()
		return fmt.Errorf("failed to retry, max retry count reached")
	}

	return c.do(ctx, req, data)
}

func (c *Client) refreshToken() error {
	var err error

	if c.token == "" && c.tokenProvider != nil {
		c.token, err = c.tokenProvider.Token()
		fmt.Println("token", c.token, "err", err, "retryCount", c.retryCount)
		if err != nil {
			return fmt.Errorf("failed to get token: %w", err)
		}
	}

	return nil
}

func validateOperationVariables(v interface{}) error {
	reflectType := reflect.TypeOf(v)
	if reflectType.Kind() == reflect.Map {
		if reflectType.Key().Kind() != reflect.String {
			return fmt.Errorf("expected map key to be string, got %s", reflectType.Key().Kind())
		}
		if reflectType.Elem().Kind() != reflect.Interface {
			return fmt.Errorf("expected map value to be interface{}, got %s", reflectType.Elem().Kind())
		}
	} else if reflectType.Kind() != reflect.Struct {
		return fmt.Errorf("expected variables to be map[string]interface{} or struct, got %s", reflectType.Kind())
	}

	return nil
}
