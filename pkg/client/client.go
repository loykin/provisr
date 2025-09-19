package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client provides HTTP client functionality to communicate with provisr daemon
type Client struct {
	baseURL string
	client  *http.Client
}

// Config holds client configuration
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns default client configuration
func DefaultConfig() Config {
	return Config{
		BaseURL: "http://localhost:8080/api",
		Timeout: 10 * time.Second,
	}
}

// New creates a new provisr API client
func New(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080/api"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	return &Client{
		baseURL: config.BaseURL,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// IsReachable checks if the daemon is running and reachable
func (c *Client) IsReachable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/status", nil)
	if err != nil {
		return false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode != http.StatusNotFound
}

// StartProcess starts a process with the given specification
func (c *Client) StartProcess(ctx context.Context, req StartRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/start", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}

	return nil
}
