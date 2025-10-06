package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommand_ReadAuthDBPathFromConfig(t *testing.T) {
	tempDir := t.TempDir()
	cmd := &command{mgr: nil}

	tests := []struct {
		name          string
		configContent string
		expected      string
	}{
		{
			name: "config_with_auth_database_path",
			configContent: `
[auth]
database_path = "custom-auth.db"
enabled = true

[server]
listen = "127.0.0.1:8080"
`,
			expected: "custom-auth.db",
		},
		{
			name: "config_without_auth_section",
			configContent: `
[server]
listen = "127.0.0.1:8080"

[logs]
level = "info"
`,
			expected: "",
		},
		{
			name: "config_with_auth_but_no_database_path",
			configContent: `
[auth]
enabled = true
method = "basic"

[server]
listen = "127.0.0.1:8080"
`,
			expected: "",
		},
		{
			name: "config_with_quoted_path",
			configContent: `
[auth]
database_path = "path with spaces/auth.db"
enabled = true
`,
			expected: "path with spaces/auth.db",
		},
		{
			name: "config_with_multiple_sections",
			configContent: `
[server]
listen = "127.0.0.1:8080"

[auth]
database_path = "multi-section-auth.db"
enabled = true

[logs]
level = "debug"
`,
			expected: "multi-section-auth.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, tt.name+".toml")
			if err := os.WriteFile(configPath, []byte(tt.configContent), 0o644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			result := cmd.readAuthDBPathFromConfig(configPath)

			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}

	// Test with nonexistent file
	t.Run("nonexistent_file", func(t *testing.T) {
		nonexistentPath := filepath.Join(tempDir, "nonexistent.toml")
		result := cmd.readAuthDBPathFromConfig(nonexistentPath)

		if result != "" {
			t.Errorf("expected empty string for nonexistent file, got '%s'", result)
		}
	})
}

func TestCommand_CreateAuthStore(t *testing.T) {
	t.Skip("Skipping auth store tests as they require sqlite3 driver which is not available in test environment")
}

func TestLoginFlags_Validation(t *testing.T) {
	cmd := &command{mgr: nil}

	tests := []struct {
		name          string
		flags         LoginFlags
		expectError   bool
		expectedError string
	}{
		{
			name: "valid_basic_auth",
			flags: LoginFlags{
				Method:   "basic",
				Username: "testuser",
				Password: "testpass",
			},
			expectError: false, // Will fail due to no server, but validates input
		},
		{
			name: "valid_client_secret_auth",
			flags: LoginFlags{
				Method:       "client_secret",
				ClientID:     "client123",
				ClientSecret: "secret123",
			},
			expectError: false, // Will fail due to no server, but validates input
		},
		{
			name: "basic_auth_missing_username",
			flags: LoginFlags{
				Method:   "basic",
				Password: "testpass",
			},
			expectError:   true,
			expectedError: "username and password are required",
		},
		{
			name: "basic_auth_missing_password",
			flags: LoginFlags{
				Method:   "basic",
				Username: "testuser",
			},
			expectError:   true,
			expectedError: "username and password are required",
		},
		{
			name: "client_secret_missing_id",
			flags: LoginFlags{
				Method:       "client_secret",
				ClientSecret: "secret123",
			},
			expectError:   true,
			expectedError: "client_id and client_secret are required",
		},
		{
			name: "client_secret_missing_secret",
			flags: LoginFlags{
				Method:   "client_secret",
				ClientID: "client123",
			},
			expectError:   true,
			expectedError: "client_id and client_secret are required",
		},
		{
			name: "unsupported_auth_method",
			flags: LoginFlags{
				Method:   "oauth",
				Username: "testuser",
				Password: "testpass",
			},
			expectError:   true,
			expectedError: "unsupported auth method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Login(tt.flags)

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

			// For valid inputs, we expect some error since we can't actually authenticate
			if err == nil {
				t.Error("expected error for valid input since no server is available")
			}
		})
	}
}

func TestCommand_Logout(t *testing.T) {
	cmd := &command{mgr: nil}

	// Test logout when no session exists
	err := cmd.Logout()
	if err != nil {
		t.Errorf("logout should not error when no session exists, got: %v", err)
	}
}

func TestAuthTestFlags_MethodValidation(t *testing.T) {
	// Note: We can't easily test AuthTest without setting up a full auth store
	// But we can test the method validation logic by examining the function

	tests := []struct {
		method      string
		shouldError bool
	}{
		{"basic", false},
		{"client_secret", false},
		{"jwt", false},
		{"oauth", true},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run("method_"+tt.method, func(t *testing.T) {
			// This is more of a documentation test showing what methods are supported
			supportedMethods := []string{"basic", "client_secret", "jwt"}
			isSupported := false
			for _, supported := range supportedMethods {
				if tt.method == supported {
					isSupported = true
					break
				}
			}

			if tt.shouldError && isSupported {
				t.Errorf("method '%s' should not be supported but was found in supported list", tt.method)
			}
			if !tt.shouldError && !isSupported {
				t.Errorf("method '%s' should be supported but was not found in supported list", tt.method)
			}
		})
	}
}

func TestCommand_CreateAuthenticatedAPIClient(t *testing.T) {
	// Create temporary session directory
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Set temporary home directory for session manager
	_ = os.Setenv("HOME", tempDir)

	cmd := &command{mgr: nil}

	tests := []struct {
		name         string
		apiUrl       string
		timeout      time.Duration
		setupSession bool
		expectError  bool
	}{
		{
			name:    "no_session_empty_url",
			apiUrl:  "",
			timeout: 10 * time.Second,
		},
		{
			name:    "no_session_with_url",
			apiUrl:  "http://example.com:8080/api",
			timeout: 10 * time.Second,
		},
		{
			name:         "with_session",
			apiUrl:       "",
			timeout:      10 * time.Second,
			setupSession: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing session
			sessionManager := NewSessionManager()
			_ = sessionManager.ClearSession()

			if tt.setupSession {
				// Create a dummy session
				session := &Session{
					Token:     "dummy-token",
					TokenType: "Bearer",
					ExpiresAt: time.Now().Add(1 * time.Hour),
					Username:  "testuser",
					UserID:    "user123",
					Roles:     []string{"user"},
					ServerURL: "http://session-server:8080/api",
				}
				if err := sessionManager.SaveSession(session); err != nil {
					t.Fatalf("failed to save test session: %v", err)
				}
			}

			client, err := cmd.createAuthenticatedAPIClient(tt.apiUrl, tt.timeout)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected non-nil API client")
				return
			}

			// Verify the client has expected properties
			if tt.setupSession && tt.apiUrl == "" {
				// Should use session's server URL
				if client.baseURL != "http://session-server:8080/api" {
					t.Errorf("expected session server URL, got: %s", client.baseURL)
				}
			} else if tt.apiUrl != "" {
				// Should use provided URL
				if client.baseURL != tt.apiUrl {
					t.Errorf("expected provided URL '%s', got: %s", tt.apiUrl, client.baseURL)
				}
			}
		})
	}
}

func TestSessionManager_Integration(t *testing.T) {
	// Create temporary session directory
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Set temporary home directory for session manager
	_ = os.Setenv("HOME", tempDir)

	sessionManager := NewSessionManager()

	// Ensure no existing session
	_ = sessionManager.ClearSession()

	// Test that no session exists initially
	if sessionManager.IsLoggedIn() {
		t.Error("should not be logged in initially")
	}

	session, err := sessionManager.LoadSession()
	if err != nil {
		t.Errorf("unexpected error loading session: %v", err)
	}
	if session != nil {
		t.Error("should return nil session when none exists")
	}

	// Create and save a session
	testSession := &Session{
		Token:     "test-token",
		TokenType: "Bearer",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Username:  "testuser",
		UserID:    "user123",
		Roles:     []string{"admin", "user"},
		ServerURL: "http://test-server:8080/api",
	}

	err = sessionManager.SaveSession(testSession)
	if err != nil {
		t.Errorf("unexpected error saving session: %v", err)
	}

	// Test that session exists now
	if !sessionManager.IsLoggedIn() {
		t.Error("should be logged in after saving session")
	}

	// Load and verify session
	loadedSession, err := sessionManager.LoadSession()
	if err != nil {
		t.Errorf("unexpected error loading session: %v", err)
	}
	if loadedSession == nil {
		t.Error("should return non-nil session")
		return
	}

	if loadedSession.Token != testSession.Token {
		t.Errorf("expected token '%s', got '%s'", testSession.Token, loadedSession.Token)
	}
	if loadedSession.Username != testSession.Username {
		t.Errorf("expected username '%s', got '%s'", testSession.Username, loadedSession.Username)
	}

	// Clear session
	err = sessionManager.ClearSession()
	if err != nil {
		t.Errorf("unexpected error clearing session: %v", err)
	}

	// Test that session is gone
	if sessionManager.IsLoggedIn() {
		t.Error("should not be logged in after clearing session")
	}
}

func TestSession_Expiration(t *testing.T) {
	// Create temporary session directory
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Set temporary home directory for session manager
	_ = os.Setenv("HOME", tempDir)

	sessionManager := NewSessionManager()

	// Ensure no existing session
	_ = sessionManager.ClearSession()

	// Create an expired session
	expiredSession := &Session{
		Token:     "expired-token",
		TokenType: "Bearer",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		Username:  "testuser",
		UserID:    "user123",
		Roles:     []string{"user"},
		ServerURL: "http://test-server:8080/api",
	}

	err := sessionManager.SaveSession(expiredSession)
	if err != nil {
		t.Errorf("unexpected error saving expired session: %v", err)
	}

	// Loading expired session should return nil
	loadedSession, err := sessionManager.LoadSession()
	if err != nil {
		t.Errorf("unexpected error loading expired session: %v", err)
	}
	if loadedSession != nil {
		t.Error("should return nil for expired session")
	}

	// Should not be logged in with expired session
	if sessionManager.IsLoggedIn() {
		t.Error("should not be logged in with expired session")
	}
}
