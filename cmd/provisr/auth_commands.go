package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/auth"
	"github.com/loykin/provisr/internal/store"
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

	// Try to read auth config from config file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			// Simple TOML parsing to get auth database configuration
			if dbPath := c.readAuthDBPathFromConfig(configPath); dbPath != "" {
				authConfig.Path = dbPath
			}
		}
	}

	return store.NewAuthStore(authConfig)
}

// readAuthDBPathFromConfig reads auth database path from config.toml
func (c *command) readAuthDBPathFromConfig(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	// Simple TOML parsing to find auth.database_path
	lines := strings.Split(string(data), "\n")
	inAuthSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Start of auth section
		if strings.Contains(line, "[auth]") {
			inAuthSection = true
			continue
		}

		// Start of another section
		if strings.HasPrefix(line, "[") && !strings.Contains(line, "[auth]") {
			inAuthSection = false
			continue
		}

		// Check for database_path in auth section
		if inAuthSection && strings.HasPrefix(line, "database_path") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, `"`)
				return value
			}
		}
	}

	return ""
}

// AuthUserCreate creates a new user
func (c *command) AuthUserCreate(f AuthUserCreateFlags, configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService := auth.NewAuthServiceWithStore(authStore)

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

	authService := auth.NewAuthServiceWithStore(authStore)
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

	authService := auth.NewAuthServiceWithStore(authStore)
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

	authService := auth.NewAuthServiceWithStore(authStore)
	cliHelper := auth.NewCLIHelper(authService)

	return cliHelper.ResetUserPassword(ctx, f.Username, f.NewPassword)
}

// AuthClientCreate creates a new client credential
func (c *command) AuthClientCreate(f AuthClientCreateFlags, configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService := auth.NewAuthServiceWithStore(authStore)
	cliHelper := auth.NewCLIHelper(authService)

	_, err = cliHelper.CreateAPIClient(ctx, f.Name, f.Scopes)
	return err
}

// AuthClientList lists all client credentials
func (c *command) AuthClientList(configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService := auth.NewAuthServiceWithStore(authStore)
	cliHelper := auth.NewCLIHelper(authService)

	return cliHelper.ListClients(ctx)
}

// AuthClientDelete deletes a client credential
func (c *command) AuthClientDelete(f AuthClientDeleteFlags, configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService := auth.NewAuthServiceWithStore(authStore)
	cliHelper := auth.NewCLIHelper(authService)

	return cliHelper.DeleteClient(ctx, f.ClientID)
}

// AuthTest tests authentication with given credentials
func (c *command) AuthTest(f AuthTestFlags, configPath string) error {
	ctx := context.Background()

	authStore, err := c.createAuthStore(configPath)
	if err != nil {
		return fmt.Errorf("failed to create auth store: %w", err)
	}
	defer func() { _ = authStore.Close() }()

	authService := auth.NewAuthServiceWithStore(authStore)
	cliHelper := auth.NewCLIHelper(authService)

	// Convert method string to AuthMethod
	var method auth.AuthMethod
	switch f.Method {
	case "basic":
		method = auth.AuthMethodBasic
	case "client_secret":
		method = auth.AuthMethodClientSecret
	case "jwt":
		method = auth.AuthMethodJWT
	default:
		return fmt.Errorf("unsupported auth method: %s (supported: basic, client_secret, jwt)", f.Method)
	}

	credentials := map[string]string{
		"username":      f.Username,
		"password":      f.Password,
		"client_id":     f.ClientID,
		"client_secret": f.ClientSecret,
		"token":         f.Token,
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
	case "client_secret":
		if f.ClientID == "" || f.ClientSecret == "" {
			return fmt.Errorf("client_id and client_secret are required for client_secret auth")
		}
	default:
		return fmt.Errorf("unsupported auth method: %s (supported: basic, client_secret)", f.Method)
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

	switch f.Method {
	case "basic":
		loginRequest["username"] = f.Username
		loginRequest["password"] = f.Password
	case "client_secret":
		loginRequest["client_id"] = f.ClientID
		loginRequest["client_secret"] = f.ClientSecret
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
