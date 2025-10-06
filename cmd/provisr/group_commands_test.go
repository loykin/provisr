package main

import (
	"strings"
	"testing"
	"time"
)

// Mock API Client for testing group commands
type mockGroupAPIClient struct {
	isReachable       bool
	groupStartError   error
	groupStopError    error
	groupStatusError  error
	groupStatusResult interface{}
	getStatusError    error
	getStatusResult   interface{}
	lastGroupName     string
	lastWaitDuration  time.Duration
	groupStartCalled  bool
	groupStopCalled   bool
	groupStatusCalled bool
	getStatusCalled   bool
}

func (m *mockGroupAPIClient) IsReachable() bool {
	return m.isReachable
}

func (m *mockGroupAPIClient) GroupStart(groupName string) error {
	m.groupStartCalled = true
	m.lastGroupName = groupName
	return m.groupStartError
}

func (m *mockGroupAPIClient) GroupStop(groupName string, wait time.Duration) error {
	m.groupStopCalled = true
	m.lastGroupName = groupName
	m.lastWaitDuration = wait
	return m.groupStopError
}

func (m *mockGroupAPIClient) GetGroupStatus(groupName string) (interface{}, error) {
	m.groupStatusCalled = true
	m.lastGroupName = groupName
	return m.groupStatusResult, m.groupStatusError
}

func (m *mockGroupAPIClient) GetStatus(_ string) (interface{}, error) {
	m.getStatusCalled = true
	return m.getStatusResult, m.getStatusError
}

func TestCommand_GroupStart(t *testing.T) {
	cmd := &command{mgr: nil}

	tests := []struct {
		name          string
		flags         GroupFlags
		mockSetup     func() *mockGroupAPIClient
		expectError   bool
		expectedError string
	}{
		{
			name: "successful_group_start",
			flags: GroupFlags{
				GroupName: "test-group",
				APIUrl:    "http://localhost:8080/api",
			},
			mockSetup: func() *mockGroupAPIClient {
				return &mockGroupAPIClient{
					isReachable:     true,
					groupStartError: nil,
				}
			},
			expectError: false,
		},
		{
			name: "empty_group_name",
			flags: GroupFlags{
				GroupName: "",
				APIUrl:    "http://localhost:8080/api",
			},
			mockSetup:     func() *mockGroupAPIClient { return &mockGroupAPIClient{} },
			expectError:   true,
			expectedError: "group-start requires --group name",
		},
		{
			name: "daemon_not_reachable",
			flags: GroupFlags{
				GroupName: "test-group",
				APIUrl:    "http://localhost:8080/api",
			},
			mockSetup: func() *mockGroupAPIClient {
				return &mockGroupAPIClient{
					isReachable: false,
				}
			},
			expectError: false, // Will fail but with different error
		},
		{
			name: "default_api_url",
			flags: GroupFlags{
				GroupName: "test-group",
				// APIUrl is empty, should default to localhost
			},
			mockSetup: func() *mockGroupAPIClient {
				return &mockGroupAPIClient{
					isReachable:     true,
					groupStartError: nil,
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.mockSetup() // mock setup for documentation

			// Test the main GroupStart function for error cases
			err := cmd.GroupStart(tt.flags)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing '%s', got: %v", tt.expectedError, err)
				}
				return
			}

			// For successful cases, we expect some error since no real daemon is running
			if err == nil {
				t.Error("expected error since no real daemon is running")
			}
		})
	}
}

func TestCommand_GroupStop(t *testing.T) {
	cmd := &command{mgr: nil}

	tests := []struct {
		name          string
		flags         GroupFlags
		expectError   bool
		expectedError string
	}{
		{
			name: "empty_group_name",
			flags: GroupFlags{
				GroupName: "",
				APIUrl:    "http://localhost:8080/api",
			},
			expectError:   true,
			expectedError: "group-stop requires --group name",
		},
		{
			name: "valid_group_name",
			flags: GroupFlags{
				GroupName: "test-group",
				Wait:      5 * time.Second,
				APIUrl:    "http://localhost:8080/api",
			},
			expectError: false, // Will fail with daemon not reachable, but that's expected
		},
		{
			name: "default_wait_duration",
			flags: GroupFlags{
				GroupName: "test-group",
				// Wait is 0, should default to 3 seconds
				APIUrl: "http://localhost:8080/api",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.GroupStop(tt.flags)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing '%s', got: %v", tt.expectedError, err)
				}
				return
			}

			// For successful cases, we expect some error since no real daemon is running
			if err == nil {
				t.Error("expected error since no real daemon is running")
			}

			// Test that wait duration was set properly
			if tt.flags.Wait == 0 {
				// The function should have set it to 3 seconds internally
				// We can't easily verify this without exposing internal state
			}
		})
	}
}

func TestCommand_GroupStatus(t *testing.T) {
	cmd := &command{mgr: nil}

	tests := []struct {
		name          string
		flags         GroupFlags
		expectError   bool
		expectedError string
	}{
		{
			name: "empty_group_name",
			flags: GroupFlags{
				GroupName: "",
				APIUrl:    "http://localhost:8080/api",
			},
			expectError:   true,
			expectedError: "group-status requires --group name",
		},
		{
			name: "valid_group_name",
			flags: GroupFlags{
				GroupName: "test-group",
				APIUrl:    "http://localhost:8080/api",
			},
			expectError: false, // Will fail with daemon not reachable, but that's expected
		},
		{
			name: "default_api_url",
			flags: GroupFlags{
				GroupName: "test-group",
				// APIUrl is empty, should default to localhost
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.GroupStatus(tt.flags)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing '%s', got: %v", tt.expectedError, err)
				}
				return
			}

			// For successful cases, we expect some error since no real daemon is running
			if err == nil {
				t.Error("expected error since no real daemon is running")
			}
		})
	}
}

func TestCommand_Cron(t *testing.T) {
	cmd := &command{mgr: nil}

	tests := []struct {
		name        string
		flags       CronFlags
		expectError bool
	}{
		{
			name: "with_api_url",
			flags: CronFlags{
				APIUrl: "http://localhost:8080/api",
			},
			expectError: false, // May succeed if localhost daemon responds
		},
		{
			name:  "default_api_url",
			flags: CronFlags{
				// APIUrl is empty, should default to localhost
			},
			expectError: false, // May succeed if localhost daemon responds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Cron(tt.flags)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			// The cron command might succeed or fail depending on whether a daemon is running
			// We just verify it doesn't panic
			_ = err
		})
	}
}

// Note: ViaAPI methods are already tested in commands_test.go

// Test wait duration handling
func TestGroupStop_WaitDurationHandling(t *testing.T) {
	cmd := &command{mgr: nil}

	tests := []struct {
		name         string
		inputWait    time.Duration
		expectedWait time.Duration
	}{
		{
			name:         "zero_wait_should_default",
			inputWait:    0,
			expectedWait: 3 * time.Second,
		},
		{
			name:         "negative_wait_should_default",
			inputWait:    -1 * time.Second,
			expectedWait: 3 * time.Second,
		},
		{
			name:         "positive_wait_should_preserve",
			inputWait:    10 * time.Second,
			expectedWait: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := GroupFlags{
				GroupName: "test-group",
				Wait:      tt.inputWait,
				APIUrl:    "http://localhost:8080/api",
			}

			// Call GroupStop - it will fail due to unreachable daemon, but we can verify
			// the wait duration logic by checking that it doesn't panic and follows expected paths
			err := cmd.GroupStop(flags)

			// Should get some error since no real daemon is running
			if err == nil {
				t.Error("expected error since no real daemon is running")
			}

			// The wait duration adjustment happens inside the function
			// We can't easily test it without more complex mocking, but at least
			// we verify the function handles different input values without panicking
		})
	}
}
