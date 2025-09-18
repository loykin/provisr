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
	baseURL string
	client  *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	if baseURL == "" {
		baseURL = "http://localhost:8080/api"
	}
	return &APIClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
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

// StartProcess starts a process via API
func (c *APIClient) StartProcess(spec interface{}) error {
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}

	resp, err := c.client.Post(c.baseURL+"/start", "application/json", bytes.NewReader(data))
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
	}

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		if err != nil {
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

// StopProcess stops a process via API
func (c *APIClient) StopProcess(name string) error {
	data, err := json.Marshal(map[string]interface{}{
		"name": name,
	})
	if err != nil {
		return err
	}

	resp, err := c.client.Post(c.baseURL+"/stop", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	func() { _ = resp.Body.Close() }()

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
