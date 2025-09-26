package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr"
)

// Mock API server for testing
func createMockAPIServer(responses map[string]string, statusCodes map[string]int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method
		key := fmt.Sprintf("%s:%s", method, path)

		// Set the status code first
		if statusCode, exists := statusCodes[key]; exists {
			w.WriteHeader(statusCode)
		} else {
			w.WriteHeader(http.StatusOK) // Default to 200 if not specified
		}

		// Then write the response
		if response, exists := responses[key]; exists {
			_, _ = w.Write([]byte(response))
		} else {
			// Only write "not found" if we explicitly set a 404 status
			if statusCode, exists := statusCodes[key]; exists && statusCode == 404 {
				_, _ = w.Write([]byte(`{"error": "not found"}`))
			} else {
				_, _ = w.Write([]byte(`{"message": "success"}`))
			}
		}
	}))
}

func TestCommand_StartViaAPI(t *testing.T) {
	tests := []struct {
		name        string
		flags       StartFlags
		mockResp    map[string]string
		statusCodes map[string]int
		expectErr   bool
		errContains string
	}{
		{
			name: "successful start",
			flags: StartFlags{
				Name: "test-proc",
			},
			mockResp: map[string]string{
				"POST:/api/processes/test-proc/start": `{"status": "started"}`,
			},
			statusCodes: map[string]int{
				"POST:/api/processes/test-proc/start": 200,
			},
			expectErr: false,
		},
		{
			name: "empty name should fail",
			flags: StartFlags{
				Name: "",
			},
			expectErr:   true,
			errContains: "process name is required",
		},
		{
			name: "API error",
			flags: StartFlags{
				Name: "test-proc",
			},
			mockResp: map[string]string{
				"POST:/api/processes/test-proc/start": `{"error": "process not found"}`,
			},
			statusCodes: map[string]int{
				"POST:/api/processes/test-proc/start": 404,
			},
			expectErr:   false,
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockServer *httptest.Server
			var apiClient *APIClient

			if tt.flags.Name != "" {
				mockServer = createMockAPIServer(tt.mockResp, tt.statusCodes)
				defer mockServer.Close()
				apiClient = NewAPIClient(mockServer.URL+"/api", 5*time.Second)
			}

			cmd := &command{mgr: &provisr.Manager{}}

			var err error
			if tt.flags.Name == "" {
				err = cmd.startViaAPI(tt.flags, nil)
			} else {
				err = cmd.startViaAPI(tt.flags, apiClient)
			}

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCommand_StatusViaAPI(t *testing.T) {
	tests := []struct {
		name        string
		flags       StatusFlags
		mockResp    map[string]string
		statusCodes map[string]int
		expectErr   bool
	}{
		{
			name: "successful status",
			flags: StatusFlags{
				Name: "test-proc",
			},
			mockResp: map[string]string{
				"GET:/api/processes/test-proc": `{"name": "test-proc", "status": "running"}`,
			},
			statusCodes: map[string]int{
				"GET:/api/processes/test-proc": 200,
			},
			expectErr: false,
		},
		{
			name: "API error",
			flags: StatusFlags{
				Name: "test-proc",
			},
			statusCodes: map[string]int{
				"GET:/api/processes/test-proc": 500,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := createMockAPIServer(tt.mockResp, tt.statusCodes)
			defer mockServer.Close()

			apiClient := NewAPIClient(mockServer.URL+"/api", 5*time.Second)
			cmd := &command{mgr: &provisr.Manager{}}

			err := cmd.statusViaAPI(tt.flags, apiClient)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCommand_StopViaAPI(t *testing.T) {
	tests := []struct {
		name        string
		flags       StopFlags
		mockResp    map[string]string
		statusCodes map[string]int
		expectErr   bool
		errContains string
	}{
		{
			name: "successful stop",
			flags: StopFlags{
				Name: "test-proc",
				Wait: 3 * time.Second,
			},
			mockResp: map[string]string{
				"POST:/api/processes/test-proc/stop": `{"status": "stopped"}`,
				"GET:/api/processes/test-proc":       `{"name": "test-proc", "status": "stopped"}`,
			},
			statusCodes: map[string]int{
				"POST:/api/processes/test-proc/stop": 200,
				"GET:/api/processes/test-proc":       200,
			},
			expectErr: false,
		},
		{
			name: "empty name should fail",
			flags: StopFlags{
				Name: "",
			},
			expectErr:   true,
			errContains: "process name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.flags.Name == "" {
				cmd := &command{mgr: &provisr.Manager{}}
				err := cmd.stopViaAPI(tt.flags, nil)
				if !tt.expectErr || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			mockServer := createMockAPIServer(tt.mockResp, tt.statusCodes)
			defer mockServer.Close()

			apiClient := NewAPIClient(mockServer.URL+"/api", 5*time.Second)
			cmd := &command{mgr: &provisr.Manager{}}

			err := cmd.stopViaAPI(tt.flags, apiClient)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsExpectedShutdownError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"signal terminated", errors.New("signal: terminated"), true},
		{"signal killed", errors.New("signal: killed"), true},
		{"signal interrupt", errors.New("signal: interrupt"), true},
		{"exit status 1", errors.New("exit status 1"), true},
		{"exit status 130", errors.New("exit status 130"), true},
		{"exit status 143", errors.New("exit status 143"), true},
		{"wrapped signal terminated", errors.New("failed to stop process: signal: terminated"), true},
		{"API error signal", errors.New("API error: signal: terminated"), true},
		{"contains signal terminated", errors.New("some prefix signal: terminated suffix"), true},
		{"random error", errors.New("random error message"), false},
		{"empty error", errors.New(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExpectedShutdownError(tt.err)
			if result != tt.expected {
				t.Errorf("isExpectedShutdownError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestCommand_GroupStartViaAPI(t *testing.T) {
	tests := []struct {
		name        string
		flags       GroupFlags
		mockResp    map[string]string
		statusCodes map[string]int
		expectErr   bool
	}{
		{
			name: "successful group start",
			flags: GroupFlags{
				GroupName: "test-group",
			},
			mockResp: map[string]string{
				"POST:/api/groups/test-group/start": `{"status": "started"}`,
			},
			statusCodes: map[string]int{
				"POST:/api/groups/test-group/start": 200,
			},
			expectErr: false,
		},
		{
			name: "API error",
			flags: GroupFlags{
				GroupName: "test-group",
			},
			statusCodes: map[string]int{
				"POST:/api/groups/test-group/start": 500,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := createMockAPIServer(tt.mockResp, tt.statusCodes)
			defer mockServer.Close()

			apiClient := NewAPIClient(mockServer.URL+"/api", 5*time.Second)
			cmd := &command{mgr: &provisr.Manager{}}

			err := cmd.groupStartViaAPI(tt.flags, apiClient)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCommand_GroupStopViaAPI(t *testing.T) {
	mockServer := createMockAPIServer(
		map[string]string{
			"POST:/api/groups/test-group/stop": `{"status": "stopped"}`,
		},
		map[string]int{
			"POST:/api/groups/test-group/stop": 200,
		},
	)
	defer mockServer.Close()

	apiClient := NewAPIClient(mockServer.URL+"/api", 5*time.Second)
	cmd := &command{mgr: &provisr.Manager{}}

	flags := GroupFlags{
		GroupName: "test-group",
		Wait:      3 * time.Second,
	}

	err := cmd.groupStopViaAPI(flags, apiClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommand_GroupStatusViaAPI(t *testing.T) {
	mockServer := createMockAPIServer(
		map[string]string{
			"GET:/api/groups/test-group": `{"name": "test-group", "members": ["proc1", "proc2"]}`,
		},
		map[string]int{
			"GET:/api/groups/test-group": 200,
		},
	)
	defer mockServer.Close()

	apiClient := NewAPIClient(mockServer.URL+"/api", 5*time.Second)
	cmd := &command{mgr: &provisr.Manager{}}

	flags := GroupFlags{
		GroupName: "test-group",
	}

	err := cmd.groupStatusViaAPI(flags, apiClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommand_Start_DaemonNotReachable(t *testing.T) {
	cmd := &command{mgr: &provisr.Manager{}}

	flags := StartFlags{
		Name:       "test-proc",
		APIUrl:     "http://localhost:99999/api", // unreachable
		APITimeout: 1 * time.Second,
	}

	err := cmd.Start(flags)
	if err == nil {
		t.Fatal("expected error for unreachable daemon")
	}

	if !strings.Contains(err.Error(), "daemon not reachable") {
		t.Errorf("expected daemon not reachable error, got: %v", err)
	}
}

func TestCommand_Status_DaemonNotReachable(t *testing.T) {
	cmd := &command{mgr: &provisr.Manager{}}

	flags := StatusFlags{
		Name:       "test-proc",
		APIUrl:     "http://localhost:99999/api", // unreachable
		APITimeout: 1 * time.Second,
	}

	err := cmd.Status(flags)
	if err == nil {
		t.Fatal("expected error for unreachable daemon")
	}

	if !strings.Contains(err.Error(), "daemon not reachable") {
		t.Errorf("expected daemon not reachable error, got: %v", err)
	}
}

func TestCommand_Stop_DaemonNotReachable(t *testing.T) {
	cmd := &command{mgr: &provisr.Manager{}}

	flags := StopFlags{
		Name:       "test-proc",
		APIUrl:     "http://localhost:99999/api", // unreachable
		APITimeout: 1 * time.Second,
	}

	err := cmd.Stop(flags)
	if err == nil {
		t.Fatal("expected error for unreachable daemon")
	}

	if !strings.Contains(err.Error(), "daemon not reachable") {
		t.Errorf("expected daemon not reachable error, got: %v", err)
	}
}

func TestCommand_Cron_DaemonNotReachable(t *testing.T) {
	cmd := &command{mgr: &provisr.Manager{}}

	flags := CronFlags{
		APIUrl:     "http://localhost:99999/api", // unreachable
		APITimeout: 1 * time.Second,
	}

	err := cmd.Cron(flags)
	if err == nil {
		t.Fatal("expected error for unreachable daemon")
	}

	if !strings.Contains(err.Error(), "daemon not reachable") {
		t.Errorf("expected daemon not reachable error, got: %v", err)
	}
}

func TestCommand_GroupStart_EmptyGroupName(t *testing.T) {
	cmd := &command{mgr: &provisr.Manager{}}

	flags := GroupFlags{
		GroupName: "",
	}

	err := cmd.GroupStart(flags)
	if err == nil {
		t.Fatal("expected error for empty group name")
	}

	if !strings.Contains(err.Error(), "group-start requires --group name") {
		t.Errorf("expected group name required error, got: %v", err)
	}
}

func TestCommand_GroupStop_EmptyGroupName(t *testing.T) {
	cmd := &command{mgr: &provisr.Manager{}}

	flags := GroupFlags{
		GroupName: "",
	}

	err := cmd.GroupStop(flags)
	if err == nil {
		t.Fatal("expected error for empty group name")
	}

	if !strings.Contains(err.Error(), "group-stop requires --group name") {
		t.Errorf("expected group name required error, got: %v", err)
	}
}

func TestCommand_GroupStatus_EmptyGroupName(t *testing.T) {
	cmd := &command{mgr: &provisr.Manager{}}

	flags := GroupFlags{
		GroupName: "",
	}

	err := cmd.GroupStatus(flags)
	if err == nil {
		t.Fatal("expected error for empty group name")
	}

	if !strings.Contains(err.Error(), "group-status requires --group name") {
		t.Errorf("expected group name required error, got: %v", err)
	}
}
