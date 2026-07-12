package main

import (
	"context"
	"fmt"
	"time"

	"github.com/loykin/provisr/internal/auth"
	"github.com/loykin/provisr/internal/auth/store"
	"github.com/loykin/provisr/internal/config"
)

// createAuthenticatedAPIClient creates an API client with session authentication
func (c *command) createAuthenticatedAPIClient(apiUrl string, timeout time.Duration) (*APIClient, error) {
	// Try to load session first
	sessionManager := NewSessionManager()
	session, err := sessionManager.LoadSession()
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// If no session found, return regular client
	if session == nil {
		return NewAPIClient(apiUrl, timeout), nil
	}

	// Use session's server URL if apiUrl is empty
	if apiUrl == "" {
		apiUrl = session.ServerURL
	}

	// Create authenticated API client
	client := NewAPIClient(apiUrl, timeout)
	client.SetAuthToken(session.Token)

	return client, nil
}

// createAuthStore creates an auth store from config file
func (c *command) createAuthStore(configPath string) (store.AuthStore, error) {
	// Default auth store config
	authConfig := store.Config{
		Type: "sqlite",
		Path: "auth.db",
	}

	if configPath != "" {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		if cfg.Server != nil && cfg.Server.Auth != nil {
			authConfig = cfg.Server.Auth.Store
		}
	}

	return store.NewAuthStore(authConfig)
}

// AuthUserCreate creates a new user
func (c *command) AuthUserCreate(f AuthUserCreateFlags, configPath string) error {
	ctx := context.Background()
	validRoles := map[string]bool{"admin": true, "operator": true, "viewer": true}
	if len(f.Roles) == 0 {
		return fmt.Errorf("at least one role is required")
	}
	for _, role := range f.Roles {
		if !validRoles[role] {
			return fmt.Errorf("invalid role %q (allowed: admin, operator, viewer)", role)
		}
	}

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService, err := auth.NewAuthServiceWithStore(authStore)
	if err != nil {
		return fmt.Errorf("failed to create auth service: %w", err)
	}

	// Create user
	_, err = authService.CreateUser(ctx, f.Username, f.Password, f.Email, f.Roles, nil)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Printf("User '%s' created successfully\n", f.Username)
	return nil
}

// AuthUserList lists all users
func (c *command) AuthUserList(configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService, err := auth.NewAuthServiceWithStore(authStore)
	if err != nil {
		return fmt.Errorf("failed to create auth service: %w", err)
	}
	cliHelper := auth.NewCLIHelper(authService)

	return cliHelper.ListUsers(ctx)
}

// AuthUserDelete deletes a user
func (c *command) AuthUserDelete(f AuthUserDeleteFlags, configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService, err := auth.NewAuthServiceWithStore(authStore)
	if err != nil {
		return fmt.Errorf("failed to create auth service: %w", err)
	}
	cliHelper := auth.NewCLIHelper(authService)

	return cliHelper.DeleteUser(ctx, f.Username)
}

// AuthUserPassword resets a user's password
func (c *command) AuthUserPassword(f AuthUserPasswordFlags, configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService, err := auth.NewAuthServiceWithStore(authStore)
	if err != nil {
		return fmt.Errorf("failed to create auth service: %w", err)
	}
	cliHelper := auth.NewCLIHelper(authService)

	return cliHelper.ResetUserPassword(ctx, f.Username, f.NewPassword)
}

// AuthTest tests authentication with given credentials
func (c *command) AuthTest(f AuthTestFlags, configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService, err := auth.NewAuthServiceWithStore(authStore)
	if err != nil {
		return fmt.Errorf("failed to create auth service: %w", err)
	}
	cliHelper := auth.NewCLIHelper(authService)

	// Convert method string to AuthMethod
	var method auth.AuthMethod
	switch f.Method {
	case "basic":
		method = auth.AuthMethodBasic
	case "jwt":
		method = auth.AuthMethodJWT
	default:
		return fmt.Errorf("unsupported auth method: %s (supported: basic, jwt)", f.Method)
	}

	credentials := map[string]string{
		"username": f.Username,
		"password": f.Password,
		"token":    f.Token,
	}

	return cliHelper.TestAuthentication(ctx, method, credentials)
}

// Login performs login and saves session
func (c *command) Login(f LoginFlags) error {

	// Validate input parameters first, before trying to connect
	switch f.Method {
	case "basic":
		if f.Username == "" || f.Password == "" {
			return fmt.Errorf("username and password are required for basic auth")
		}
	default:
		return fmt.Errorf("unsupported auth method: %s (supported: basic)", f.Method)
	}

	// Default server URL if not specified
	serverURL := f.ServerURL
	if serverURL == "" {
		serverURL = "http://localhost:8080/api"
	}

	// Create API client
	apiClient := NewAPIClient(serverURL, 30*time.Second)
	if !apiClient.IsReachable() {
		return fmt.Errorf("server not reachable at %s - please start daemon first with 'provisr serve'", serverURL)
	}

	// Prepare login request
	loginRequest := map[string]interface{}{
		"method": f.Method,
	}

	if f.Method == "basic" {
		loginRequest["username"] = f.Username
		loginRequest["password"] = f.Password
	}

	// Make login request
	result, err := apiClient.Login(loginRequest)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Save session
	sessionManager := NewSessionManager()
	session := &Session{
		Token:     result.Token.Value,
		TokenType: result.Token.Type,
		ExpiresAt: result.Token.ExpiresAt,
		Username:  result.Username,
		UserID:    result.UserID,
		Roles:     result.Roles,
		ServerURL: serverURL,
	}

	if err := sessionManager.SaveSession(session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	fmt.Printf("Login successful! Logged in as %s\n", result.Username)
	fmt.Printf("Session saved to %s\n", sessionManager.GetSessionPath())
	fmt.Printf("Token expires at: %s\n", result.Token.ExpiresAt.Format(time.RFC3339))

	return nil
}

// Logout clears the saved session
func (c *command) Logout() error {
	sessionManager := NewSessionManager()

	if !sessionManager.IsLoggedIn() {
		fmt.Println("No active session found")
		return nil
	}

	if err := sessionManager.ClearSession(); err != nil {
		return fmt.Errorf("failed to clear session: %w", err)
	}

	fmt.Println("Logged out successfully")
	return nil
}
