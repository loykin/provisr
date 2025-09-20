package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAPIClient(t *testing.T) {
	// Test default values
	client := NewAPIClient("", 0)
	if client.baseURL != "http://localhost:8080/api" {
		t.Errorf("Expected default baseURL http://localhost:8080/api, got %s", client.baseURL)
	}
	if client.client.Timeout != 10*time.Second {
		t.Errorf("Expected default timeout 10s, got %v", client.client.Timeout)
	}

	// Test custom values
	client = NewAPIClient("http://example.com/api", 5*time.Second)
	if client.baseURL != "http://example.com/api" {
		t.Errorf("Expected baseURL http://example.com/api, got %s", client.baseURL)
	}
	if client.client.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", client.client.Timeout)
	}
}

func TestAPIClientIsReachable(t *testing.T) {
	// Test reachable server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, time.Second)
	if !client.IsReachable() {
		t.Error("Expected server to be reachable")
	}

	// Test unreachable server
	client = NewAPIClient("http://localhost:99999", 100*time.Millisecond)
	if client.IsReachable() {
		t.Error("Expected server to be unreachable")
	}

	// Test 404 response
	server404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server404.Close()

	client = NewAPIClient(server404.URL, time.Second)
	if client.IsReachable() {
		t.Error("Expected server returning 404 to be unreachable")
	}
}

func TestAPIClientStartProcess(t *testing.T) {
	// Test successful start
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":"success"}`))
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, time.Second)
	spec := map[string]interface{}{
		"name":    "test-process",
		"command": "echo hello",
	}

	err := client.StartProcess(spec)
	if err != nil {
		t.Errorf("Expected successful start, got error: %v", err)
	}

	// Test API error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" && r.Method == "POST" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"process already running"}`))
		}
	}))
	defer errorServer.Close()

	client = NewAPIClient(errorServer.URL, time.Second)
	err = client.StartProcess(spec)
	if err == nil {
		t.Fatal("Expected error for API error response, but got nil")
	} else {
		expectedMsg := "API error: process already running"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got: %q", expectedMsg, err.Error())
		}
	}
}

func TestAPIClientGetStatus(t *testing.T) {
	// Test successful status call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			if r.URL.Query().Get("name") == "test-process" {
				_, _ = w.Write([]byte(`{"name":"test-process","running":true}`))
			} else {
				_, _ = w.Write([]byte(`[{"name":"process1","running":true}]`))
			}
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, time.Second)

	// Test with specific process name
	result, err := client.GetStatus("test-process")
	if err != nil {
		t.Errorf("Expected successful status call, got error: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}

	// Test with empty name (all processes)
	result, err = client.GetStatus("")
	if err != nil {
		t.Errorf("Expected successful status call for all processes, got error: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result for all processes")
	}

	// Test API error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		}
	}))
	defer errorServer.Close()

	client = NewAPIClient(errorServer.URL, time.Second)
	_, err = client.GetStatus("test")
	if err == nil {
		t.Fatal("Expected error for API error response, but got nil")
	}
}

func TestAPIClientStopProcess(t *testing.T) {
	// Test successful stop
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stop" && r.Method == "POST" {
			name := r.URL.Query().Get("name")

			if name == "test-process" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result":"success"}`))
			} else {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"process not found"}`))
			}
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, time.Second)

	// Test successful stop
	err := client.StopProcess("test-process", 5*time.Second)
	if err != nil {
		t.Errorf("Expected successful stop, got error: %v", err)
	}

	// Test process not found
	err = client.StopProcess("non-existent", 5*time.Second)
	if err == nil {
		t.Error("Expected error for non-existent process, but got nil")
	} else {
		expectedMsg := "API error: process not found"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got: %q", expectedMsg, err.Error())
		}
	}
}

func TestAPIClientNetworkErrors(t *testing.T) {
	// Test network error during start
	client := NewAPIClient("http://localhost:99999", 100*time.Millisecond)

	err := client.StartProcess(map[string]interface{}{"name": "test", "command": "echo test"})
	if err == nil {
		t.Error("Expected network error for start")
	}

	// Test network error during status
	_, err = client.GetStatus("test")
	if err == nil {
		t.Error("Expected network error for status")
	}

	// Test network error during stop
	err = client.StopProcess("test", time.Second)
	if err == nil {
		t.Error("Expected network error for stop")
	}
}
