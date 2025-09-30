package auth

import (
	"testing"
	"time"
)

func TestAuthMethod_Constants(t *testing.T) {
	testCases := []struct {
		name     string
		method   AuthMethod
		expected string
	}{
		{"basic auth", AuthMethodBasic, "basic"},
		{"client secret", AuthMethodClientSecret, "client_secret"},
		{"jwt auth", AuthMethodJWT, "jwt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.method) != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, string(tc.method))
			}
		})
	}
}

func TestAuthResult_Creation(t *testing.T) {
	result := AuthResult{
		Success:  true,
		UserID:   "user123",
		Username: "testuser",
		Roles:    []string{"admin", "user"},
		Metadata: map[string]string{
			"department": "engineering",
			"level":      "senior",
		},
		Token: &Token{
			Type:      "Bearer",
			Value:     "jwt.token.here",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
	if result.UserID != "user123" {
		t.Errorf("Expected user ID user123, got %s", result.UserID)
	}
	if result.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", result.Username)
	}
	if len(result.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(result.Roles))
	}
	if result.Token == nil {
		t.Error("Expected token to be set")
	}
	if result.Token.Type != "Bearer" {
		t.Errorf("Expected token type Bearer, got %s", result.Token.Type)
	}
}

func TestToken_Creation(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	token := Token{
		Type:      "Bearer",
		Value:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
		ExpiresAt: expiresAt,
	}

	if token.Type != "Bearer" {
		t.Errorf("Expected type Bearer, got %s", token.Type)
	}
	if token.Value == "" {
		t.Error("Expected value to be set")
	}
	if token.ExpiresAt.IsZero() {
		t.Error("Expected expires at to be set")
	}
	if !token.ExpiresAt.Equal(expiresAt) {
		t.Error("Expected expires at to match set time")
	}
}

func TestLoginRequest_Creation(t *testing.T) {
	testCases := []struct {
		name    string
		request LoginRequest
	}{
		{
			name: "basic_auth_request",
			request: LoginRequest{
				Method:   AuthMethodBasic,
				Username: "testuser",
				Password: "testpass",
			},
		},
		{
			name: "client_secret_request",
			request: LoginRequest{
				Method:       AuthMethodClientSecret,
				ClientID:     "client123",
				ClientSecret: "secret456",
			},
		},
		{
			name: "jwt_request",
			request: LoginRequest{
				Method: AuthMethodJWT,
				Token:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.request.Method == "" {
				t.Error("Expected method to be set")
			}

			switch tc.request.Method {
			case AuthMethodBasic:
				if tc.request.Username == "" {
					t.Error("Expected username for basic auth")
				}
				if tc.request.Password == "" {
					t.Error("Expected password for basic auth")
				}
			case AuthMethodClientSecret:
				if tc.request.ClientID == "" {
					t.Error("Expected client ID for client secret auth")
				}
				if tc.request.ClientSecret == "" {
					t.Error("Expected client secret for client secret auth")
				}
			case AuthMethodJWT:
				if tc.request.Token == "" {
					t.Error("Expected token for JWT auth")
				}
			}
		})
	}
}

func TestAuthResult_FailureCase(t *testing.T) {
	result := AuthResult{
		Success: false,
	}

	if result.Success {
		t.Error("Expected success to be false")
	}
	if result.UserID != "" {
		t.Error("Expected user ID to be empty for failed auth")
	}
	if result.Token != nil {
		t.Error("Expected token to be nil for failed auth")
	}
}
