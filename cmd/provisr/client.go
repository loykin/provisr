package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// APIClient provides HTTP client functionality to communicate with provisr daemon
type APIClient struct {
	baseURL   string
	client    *http.Client
	authToken string
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string, timeout time.Duration) *APIClient {
	if baseURL == "" {
		baseURL = "http://localhost:8080/api"
	}
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &APIClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// IsReachable checks if the daemon is running and reachable
func (c *APIClient) IsReachable() bool {
	resp, err := c.client.Get(c.baseURL + "/status")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode != http.StatusNotFound // Accept any response except 404
}

// RegisterProcess registers and starts a process via API
func (c *APIClient) RegisterProcess(spec interface{}) error {
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}

	resp, err := c.client.Post(c.baseURL+"/register", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		if err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}

	return nil
}

// GetStatus gets process status via API
func (c *APIClient) GetStatus(name string) (interface{}, error) {
	url := c.baseURL + "/status"
	if name != "" {
		url += "?name=" + name
	} else {
		// When no name is provided, fetch all statuses using wildcard match
		url += "?wildcard=*"
	}

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("API error: %s", errorResp.Error)
	}

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// StopProcess stops a single process instance by exact name via API
func (c *APIClient) StopProcess(name string, wait ...time.Duration) error {
	url := c.baseURL + "/stop?name=" + name
	if len(wait) > 0 {
		url += "&wait=" + wait[0].String()
	}
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}
	return nil
}

// StopAll stops all instances with the same base name via API
func (c *APIClient) StopAll(base string, wait ...time.Duration) error {
	url := c.baseURL + "/stop?base=" + base
	if len(wait) > 0 {
		url += "&wait=" + wait[0].String()
	}
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}
	return nil
}

// StartProcess starts an already registered process via API
func (c *APIClient) StartProcess(name string) error {
	url := c.baseURL + "/start?name=" + name

	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}

	return nil
}

// UnregisterProcess stops and unregisters a process via API
func (c *APIClient) UnregisterProcess(name string, wait ...time.Duration) error {
	url := c.baseURL + "/unregister?name=" + name
	if len(wait) > 0 {
		url += "&wait=" + wait[0].String()
	}
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}
	return nil
}

// UnregisterAllProcesses stops and unregisters all processes with the same base name via API
func (c *APIClient) UnregisterAllProcesses(base string, wait ...time.Duration) error {
	url := c.baseURL + "/unregister?base=" + base
	if len(wait) > 0 {
		url += "&wait=" + wait[0].String()
	}
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}
	return nil
}

// GetGroupStatus gets the status of all processes in a group
func (c *APIClient) GetGroupStatus(groupName string) (interface{}, error) {
	url := c.baseURL + "/group/status?group=" + groupName
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if errorMap, ok := result.(map[string]interface{}); ok {
			if errorMsg, exists := errorMap["error"]; exists {
				return nil, fmt.Errorf("API error: %v", errorMsg)
			}
		}
		return nil, fmt.Errorf("API error: HTTP %d", resp.StatusCode)
	}

	return result, nil
}

// GroupStart starts all processes in a group
func (c *APIClient) GroupStart(groupName string) error {
	url := c.baseURL + "/group/start?group=" + groupName
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}
	return nil
}

// GroupStop stops all processes in a group
func (c *APIClient) GroupStop(groupName string, wait ...time.Duration) error {
	url := c.baseURL + "/group/stop?group=" + groupName
	if len(wait) > 0 {
		url += "&wait=" + wait[0].String()
	}
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return err
		}
		return fmt.Errorf("API error: %s", errorResp.Error)
	}
	return nil
}

// LoginResponse represents the response from login endpoint
type LoginResponse struct {
	Success  bool       `json:"success"`
	UserID   string     `json:"user_id"`
	Username string     `json:"username"`
	Roles    []string   `json:"roles"`
	Token    *TokenInfo `json:"token"`
}

// TokenInfo represents JWT token information
type TokenInfo struct {
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Login authenticates with the server and returns login response
func (c *APIClient) Login(loginRequest map[string]interface{}) (*LoginResponse, error) {
	data, err := json.Marshal(loginRequest)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Post(c.baseURL+"/auth/login", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		// Try to decode error response again
		if jsonErr := json.NewDecoder(resp.Body).Decode(&errorResp); jsonErr == nil {
			if errorResp.Message != "" {
				return nil, fmt.Errorf("login failed: %s", errorResp.Message)
			}
		}
		return nil, fmt.Errorf("login failed: HTTP %d", resp.StatusCode)
	}

	return &result, nil
}

// SetAuthToken sets the authentication token for API requests
func (c *APIClient) SetAuthToken(token string) {
	c.authToken = token
}

// addAuthHeaders adds authentication headers to the request
func (c *APIClient) addAuthHeaders(req *http.Request) {
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
}
