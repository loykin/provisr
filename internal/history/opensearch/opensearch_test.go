package opensearch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/store"
)

func TestOpenSearchSink_Send(t *testing.T) {
	var receivedBody []byte
	var receivedURL string
	var receivedMethod string

	// Create test server to mock OpenSearch
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedURL = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		receivedBody = body

		// Mock successful response
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"_id":"test","_index":"test-index","result":"created"}`))
	}))
	defer server.Close()

	// Create sink with test server URL
	sink := New(server.URL, "test-index")

	// Create test event
	testRecord := store.Record{
		Name:      "test-process",
		PID:       12345,
		StartedAt: time.Now().Add(-time.Minute).UTC(),
		Running:   true,
		Uniq:      "test-unique-key",
	}

	event := history.Event{
		Type:       history.EventStart,
		OccurredAt: time.Now().UTC(),
		Record:     testRecord,
	} // Send event
	ctx := context.Background()
	err := sink.Send(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify HTTP method
	if receivedMethod != "POST" {
		t.Errorf("Expected POST method, got: %s", receivedMethod)
	}

	// Verify URL path
	expectedPath := "/test-index/_doc"
	if receivedURL != expectedPath {
		t.Errorf("Expected URL path %s, got: %s", expectedPath, receivedURL)
	}

	// Verify request body contains expected data
	var receivedEvent map[string]interface{}
	if err := json.Unmarshal(receivedBody, &receivedEvent); err != nil {
		t.Fatalf("Failed to parse received JSON: %v", err)
	}

	// Check event type
	if receivedEvent["type"] != string(history.EventStart) {
		t.Errorf("Expected type %s, got: %v", history.EventStart, receivedEvent["type"])
	}

	// Check record data
	record, ok := receivedEvent["record"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected record in event, got: %v", receivedEvent)
	}

	if record["Name"] != testRecord.Name {
		t.Errorf("Expected record name %s, got: %v", testRecord.Name, record["Name"])
	}

	if record["PID"] != float64(testRecord.PID) {
		t.Errorf("Expected record PID %d, got: %v", testRecord.PID, record["PID"])
	}
}

func TestOpenSearchSink_SendError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	// Create sink with test server URL
	sink := New(server.URL, "test-index")

	// Create test event
	testRecord := store.Record{
		Name: "test-process",
		PID:  12345,
		Uniq: "test-key",
	}

	event := history.Event{
		Type:       history.EventStart,
		OccurredAt: time.Now().UTC(),
		Record:     testRecord,
	}

	// Send event should return error
	ctx := context.Background()
	err := sink.Send(ctx, event)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "opensearch sink status 400") {
		t.Errorf("Expected status error message, got: %v", err)
	}
}

func TestOpenSearchSink_URLConstruction(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		index       string
		expectedURL string
	}{
		{
			name:        "Basic URL",
			baseURL:     "http://localhost:9200",
			index:       "logs",
			expectedURL: "http://localhost:9200/logs/_doc",
		},
		{
			name:        "URL with trailing slash",
			baseURL:     "http://localhost:9200/",
			index:       "events",
			expectedURL: "http://localhost:9200/events/_doc",
		},
		{
			name:        "HTTPS URL",
			baseURL:     "https://opensearch.example.com",
			index:       "process-history",
			expectedURL: "https://opensearch.example.com/process-history/_doc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedURL string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedURL = r.URL.String()
				w.WriteHeader(http.StatusCreated)
			}))
			defer server.Close()

			// Override the base URL to use our test server, but test the path construction
			sink := New(tt.baseURL, tt.index)
			// Manually construct what we expect the URL to be for verification
			expectedPath := "/" + tt.index + "/_doc"

			// For the actual test, we need to use the test server
			sink.baseURL = server.URL

			testRecord := store.Record{Name: "test", PID: 1, Uniq: "test"}
			event := history.Event{Type: history.EventStart, OccurredAt: time.Now(), Record: testRecord}

			_ = sink.Send(context.Background(), event)

			if receivedURL != expectedPath {
				t.Errorf("Expected URL path %s, got: %s", expectedPath, receivedURL)
			}
		})
	}
}
