package auth

import (
	"context"
	"fmt"
	"time"
)

// CLIHelper provides utility functions for CLI commands
type CLIHelper struct {
	authService *AuthService
}

// NewCLIHelper creates a new CLI helper
func NewCLIHelper(authService *AuthService) *CLIHelper {
	return &CLIHelper{
		authService: authService,
	}
}

// ListUsers lists all users
func (cli *CLIHelper) ListUsers(ctx context.Context) error {
	users, total, err := cli.authService.store.ListUsers(ctx, 0, 100)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	fmt.Printf("Users (%d total):\n", total)
	fmt.Printf("%-20s %-30s %-20s %-10s %s\n", "ID", "Username", "Email", "Active", "Roles")
	fmt.Printf("%s\n", "─────────────────────────────────────────────────────────────────────────────────")

	for _, user := range users {
		email := user.Email
		if email == "" {
			email = "-"
		}

		active := "Yes"
		if !user.Active {
			active = "No"
		}

		roles := fmt.Sprintf("%v", user.Roles)
		if len(user.Roles) == 0 {
			roles = "-"
		}

		fmt.Printf("%-20s %-30s %-20s %-10s %s\n",
			user.ID, user.Username, email, active, roles)
	}

	return nil
}

// DeleteUser deletes a user by username or ID
func (cli *CLIHelper) DeleteUser(ctx context.Context, identifier string) error {
	// Try to get user by username first
	user, err := cli.authService.store.GetUserByUsername(ctx, identifier)
	if err != nil {
		// Try by ID
		user, err = cli.authService.store.GetUser(ctx, identifier)
		if err != nil {
			return fmt.Errorf("user not found: %s", identifier)
		}
	}

	if err := cli.authService.store.DeleteUser(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	fmt.Printf("User '%s' deleted successfully\n", user.Username)
	return nil
}

// ResetUserPassword resets a user's password
func (cli *CLIHelper) ResetUserPassword(ctx context.Context, identifier, newPassword string) error {
	// Try to get user by username first
	user, err := cli.authService.store.GetUserByUsername(ctx, identifier)
	if err != nil {
		// Try by ID
		user, err = cli.authService.store.GetUser(ctx, identifier)
		if err != nil {
			return fmt.Errorf("user not found: %s", identifier)
		}
	}

	if err := cli.authService.UpdateUserPassword(ctx, user.ID, newPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	fmt.Printf("Password updated successfully for user '%s'\n", user.Username)
	return nil
}

// TestAuthentication tests authentication with the given credentials
func (cli *CLIHelper) TestAuthentication(ctx context.Context, method AuthMethod, credentials map[string]string) error {
	var req LoginRequest
	req.Method = method

	switch method {
	case AuthMethodBasic:
		req.Username = credentials["username"]
		req.Password = credentials["password"]
	case AuthMethodJWT:
		req.Token = credentials["token"]
	default:
		return fmt.Errorf("unsupported auth method: %s", method)
	}

	result, err := cli.authService.Authenticate(ctx, req)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("authentication failed: invalid credentials")
	}

	fmt.Printf("Authentication successful:\n")
	fmt.Printf("  User ID: %s\n", result.UserID)
	fmt.Printf("  Username: %s\n", result.Username)
	fmt.Printf("  Roles: %v\n", result.Roles)

	if result.Token != nil {
		fmt.Printf("  Token Type: %s\n", result.Token.Type)
		fmt.Printf("  Token Expires: %s\n", result.Token.ExpiresAt.Format(time.RFC3339))
	}

	return nil
}
