package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// Client provides HTTP client functionality to communicate with provisr daemon
type Client struct {
	baseURL string
	client  *http.Client
	logger  *slog.Logger
}

// Config holds client configuration
type Config struct {
	BaseURL  string
	Timeout  time.Duration
	Logger   *slog.Logger // Optional logger for client operations
	TLS      *TLSClientConfig
	Insecure bool // Skip TLS verification
}

// TLSClientConfig holds TLS configuration for client
type TLSClientConfig struct {
	Enabled    bool   // Enable TLS
	CACert     string // CA certificate file path
	ClientCert string // Client certificate file
	ClientKey  string // Client private key file
	ServerName string // Server name for verification
	SkipVerify bool   // Skip certificate verification
}

// DefaultConfig returns default client configuration
func DefaultConfig() Config {
	return Config{
		BaseURL: "http://localhost:8080/api",
		Timeout: 10 * time.Second,
	}
}

// DefaultTLSConfig returns default TLS client configuration
func DefaultTLSConfig() Config {
	return Config{
		BaseURL: "https://localhost:8080/api",
		Timeout: 10 * time.Second,
		TLS: &TLSClientConfig{
			Enabled: true,
		},
	}
}

// InsecureConfig returns insecure client configuration (skip TLS verification)
func InsecureConfig() Config {
	return Config{
		BaseURL:  "https://localhost:8080/api",
		Timeout:  10 * time.Second,
		Insecure: true,
	}
}

// New creates a new provisr API client with TLS support
func New(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080/api"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	// Setup HTTP transport with TLS configuration
	transport := &http.Transport{}

	// Configure TLS if needed
	if config.TLS != nil && config.TLS.Enabled || config.Insecure {
		tlsConfig, err := setupClientTLS(config)
		if err != nil {
			config.Logger.Error("TLS setup failed", "error", err)
		} else {
			transport.TLSClientConfig = tlsConfig
		}
	}

	return &Client{
		baseURL: config.BaseURL,
		logger:  config.Logger,
		client: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
		},
	}
}

// IsReachable checks if the daemon is running and reachable
func (c *Client) IsReachable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/status", nil)
	if err != nil {
		c.logger.Debug("Failed to create request for reachability check", "error", err)
		return false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Debug("Daemon unreachable", "error", err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	isReachable := resp.StatusCode != http.StatusNotFound
	c.logger.Debug("Daemon reachability check", "reachable", isReachable, "status", resp.StatusCode)
	return isReachable
}

// RegisterProcess registers a new process with the given specification
func (c *Client) RegisterProcess(ctx context.Context, req RegisterRequest) error {
	c.logger.Debug("Registering process", "name", req.Name, "command", req.Command, "instances", req.Instances)

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/register"
	if err := c.doJSONRequest(ctx, "POST", url, data); err != nil {
		return err
	}

	c.logger.Debug("Process registration completed", "name", req.Name)
	return nil
}

// StartProcess starts an already registered process
func (c *Client) StartProcess(ctx context.Context, req StartRequest) error {
	c.logger.Debug("Starting registered process", "name", req.Name)

	// Use URL query parameter for start (no body needed)
	url := fmt.Sprintf("%s/start?name=%s", c.baseURL, req.Name)
	if err := c.doRequest(ctx, "POST", url, nil); err != nil {
		return err
	}

	c.logger.Debug("Process start completed", "name", req.Name)
	return nil
}

// UnregisterProcess unregisters processes based on the request parameters
func (c *Client) UnregisterProcess(ctx context.Context, req UnregisterRequest) error {
	c.logger.Debug("Unregistering processes", "name", req.Name, "base", req.Base, "wildcard", req.Wildcard)

	// Build query parameters
	url := c.buildUnregisterURL(req)
	if err := c.doRequest(ctx, "POST", url, nil); err != nil {
		return err
	}

	c.logger.Debug("Process unregistration completed")
	return nil
}

// setupClientTLS configures TLS settings for HTTP client
func setupClientTLS(config Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// Handle insecure mode (skip verification)
	if config.Insecure {
		tlsConfig.InsecureSkipVerify = true
		return tlsConfig, nil
	}

	// Configure TLS settings
	if config.TLS != nil {
		// Skip verification if requested
		if config.TLS.SkipVerify {
			tlsConfig.InsecureSkipVerify = true
		}

		// Set server name for verification
		if config.TLS.ServerName != "" {
			tlsConfig.ServerName = config.TLS.ServerName
		}

		// Load CA certificate if provided
		if config.TLS.CACert != "" {
			if err := loadCACert(tlsConfig, config.TLS.CACert); err != nil {
				return nil, fmt.Errorf("failed to load CA certificate: %w", err)
			}
		}

		// Load client certificate if provided
		if config.TLS.ClientCert != "" && config.TLS.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(config.TLS.ClientCert, config.TLS.ClientKey)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return tlsConfig, nil
}

// loadCACert loads CA certificate from file and adds it to TLS config
func loadCACert(tlsConfig *tls.Config, caCertPath string) error {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to parse CA certificate")
	}

	tlsConfig.RootCAs = caCertPool
	return nil
}

// doRequest performs HTTP request with common error handling
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) error {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("HTTP request failed", "error", err, "url", url)
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return c.handleErrorResponse(resp)
}

// doJSONRequest performs JSON request with marshaling
func (c *Client) doJSONRequest(ctx context.Context, method, url string, data []byte) error {
	return c.doRequest(ctx, method, url, data)
}

// handleErrorResponse handles HTTP error responses
func (c *Client) handleErrorResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	var errorResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
		c.logger.Error("Failed to decode error response", "status", resp.StatusCode)
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	c.logger.Error("API request failed", "error", errorResp.Error, "status", resp.StatusCode)
	return fmt.Errorf("API error: %s", errorResp.Error)
}

// buildUnregisterURL builds URL for unregister request
func (c *Client) buildUnregisterURL(req UnregisterRequest) string {
	url := c.baseURL + "/unregister?"
	var params []string

	if req.Name != "" {
		params = append(params, "name="+req.Name)
	}
	if req.Base != "" {
		params = append(params, "base="+req.Base)
	}
	if req.Wildcard != "" {
		params = append(params, "wildcard="+req.Wildcard)
	}
	if req.Wait > 0 {
		params = append(params, "wait="+req.Wait.String())
	}

	if len(params) > 0 {
		url += params[0]
		for _, param := range params[1:] {
			url += "&" + param
		}
	}

	return url
}
