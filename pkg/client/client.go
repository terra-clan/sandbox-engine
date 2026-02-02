package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// Client is a Go SDK for sandbox-engine API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option configures the client
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithTimeout sets the client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// NewClient creates a new sandbox-engine client
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Sandbox represents a sandbox response
type Sandbox struct {
	ID          string                 `json:"id"`
	TemplateID  string                 `json:"template_id"`
	UserID      string                 `json:"user_id"`
	Status      string                 `json:"status"`
	StatusMsg   string                 `json:"status_message,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	ExpiresAt   time.Time              `json:"expires_at"`
	ContainerID string                 `json:"container_id,omitempty"`
	Services    map[string]interface{} `json:"services,omitempty"`
	Endpoints   map[string]string      `json:"endpoints,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// CreateSandboxRequest represents a sandbox creation request
type CreateSandboxRequest struct {
	TemplateID string            `json:"template_id"`
	UserID     string            `json:"user_id"`
	TTL        *time.Duration    `json:"ttl,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ExtendTTLRequest represents a TTL extension request
type ExtendTTLRequest struct {
	Duration time.Duration `json:"duration"`
}

// ListOptions contains options for listing sandboxes
type ListOptions struct {
	UserID     string
	TemplateID string
	Status     string
	Limit      int
	Offset     int
}

// CreateSandbox creates a new sandbox
func (c *Client) CreateSandbox(ctx context.Context, req CreateSandboxRequest) (*Sandbox, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/sandboxes", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool     `json:"success"`
		Data    *Sandbox `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return result.Data, nil
}

// GetSandbox retrieves a sandbox by ID
func (c *Client) GetSandbox(ctx context.Context, id string) (*Sandbox, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/sandboxes/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool     `json:"success"`
		Data    *Sandbox `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return result.Data, nil
}

// DeleteSandbox removes a sandbox
func (c *Client) DeleteSandbox(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/sandboxes/%s", id), nil)
	if err != nil {
		return err
	}

	var result struct {
		Success bool `json:"success"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return nil
}

// StopSandbox stops a running sandbox
func (c *Client) StopSandbox(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/sandboxes/%s/stop", id), nil)
	if err != nil {
		return err
	}

	var result struct {
		Success bool `json:"success"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return nil
}

// ListSandboxes retrieves a list of sandboxes
func (c *Client) ListSandboxes(ctx context.Context, opts ListOptions) ([]*Sandbox, error) {
	path := "/api/v1/sandboxes?"
	if opts.UserID != "" {
		path += fmt.Sprintf("user_id=%s&", opts.UserID)
	}
	if opts.TemplateID != "" {
		path += fmt.Sprintf("template_id=%s&", opts.TemplateID)
	}
	if opts.Status != "" {
		path += fmt.Sprintf("status=%s&", opts.Status)
	}
	if opts.Limit > 0 {
		path += fmt.Sprintf("limit=%d&", opts.Limit)
	}
	if opts.Offset > 0 {
		path += fmt.Sprintf("offset=%d&", opts.Offset)
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Sandboxes []*Sandbox `json:"sandboxes"`
			Total     int        `json:"total"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return result.Data.Sandboxes, nil
}

// ExtendTTL extends the expiration time of a sandbox
func (c *Client) ExtendTTL(ctx context.Context, id string, duration time.Duration) (*Sandbox, error) {
	req := ExtendTTLRequest{Duration: duration}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/sandboxes/%s/extend", id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool     `json:"success"`
		Data    *Sandbox `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return result.Data, nil
}

// GetLogs retrieves logs from a sandbox
func (c *Client) GetLogs(ctx context.Context, id string, tail int) (string, error) {
	path := fmt.Sprintf("/api/v1/sandboxes/%s/logs", id)
	if tail > 0 {
		path += fmt.Sprintf("?tail=%d", tail)
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Logs string `json:"logs"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return "", fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return result.Data.Logs, nil
}

// ListTemplates retrieves all available templates
func (c *Client) ListTemplates(ctx context.Context) ([]*models.Template, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/templates", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Templates []*models.Template `json:"templates"`
			Total     int                `json:"total"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return result.Data.Templates, nil
}

// Health checks if the service is healthy
func (c *Client) Health(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/health", nil)
	return err
}

// doRequest performs an HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
