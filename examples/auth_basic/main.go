package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr/internal/auth"
)

func main() {
	// Configure auth service
	config := auth.AuthConfig{
		Store: auth.StoreConfig{
			Type: "sqlite",
			Path: "example_auth.db",
		},
		JWTSecret:  "example-secret-key-change-in-production",
		TokenTTL:   24 * time.Hour,
		BcryptCost: 10,
	}

	// Create auth service
	authService, err := auth.NewAuthService(config)
	if err != nil {
		log.Fatalf("Failed to create auth service: %v", err)
	}
	defer func() { _ = authService.Close() }()

	ctx := context.Background()

	// Create a user
	fmt.Println("Creating user...")
	user, err := authService.CreateUser(ctx, "admin", "password123", "admin@example.com", []string{"admin"}, nil)
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}
	fmt.Printf("Created user: %s\n", user.Username)

	// Create a client credential
	fmt.Println("\nCreating client credential...")
	client, err := authService.CreateClient(ctx, "API Client", []string{"operator"}, nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	fmt.Printf("Created client: %s (ID: %s)\n", client.Name, client.ClientID)
	fmt.Printf("Client Secret: %s\n", client.ClientSecret)

	// Test basic authentication
	fmt.Println("\nTesting basic authentication...")
	basicReq := auth.LoginRequest{
		Method:   auth.AuthMethodBasic,
		Username: "admin",
		Password: "password123",
	}

	result, err := authService.Authenticate(ctx, basicReq)
	if err != nil {
		log.Fatalf("Basic auth failed: %v", err)
	}

	if result.Success {
		fmt.Printf("✓ Basic auth successful: %s\n", result.Username)
		fmt.Printf("  Token expires: %s\n", result.Token.ExpiresAt.Format(time.RFC3339))
	}

	// Test client credentials authentication
	fmt.Println("\nTesting client credentials authentication...")
	clientReq := auth.LoginRequest{
		Method:       auth.AuthMethodClientSecret,
		ClientID:     client.ClientID,
		ClientSecret: client.ClientSecret,
	}

	result, err = authService.Authenticate(ctx, clientReq)
	if err != nil {
		log.Fatalf("Client auth failed: %v", err)
	}

	if result.Success {
		fmt.Printf("✓ Client auth successful: %s\n", result.Username)
		fmt.Printf("  Scopes: %v\n", result.Roles)
	}

	// Test JWT authentication
	fmt.Println("\nTesting JWT authentication...")
	jwtReq := auth.LoginRequest{
		Method: auth.AuthMethodJWT,
		Token:  result.Token.Value,
	}

	result, err = authService.Authenticate(ctx, jwtReq)
	if err != nil {
		log.Fatalf("JWT auth failed: %v", err)
	}

	if result.Success {
		fmt.Printf("✓ JWT auth successful: %s\n", result.Username)
	}

	// List users
	fmt.Println("\nListing users...")
	users, total, err := authService.ListUsers(ctx, 0, 10)
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}

	fmt.Printf("Found %d users:\n", total)
	for _, u := range users {
		fmt.Printf("  - %s (%s)\n", u.Username, u.Email)
	}

	// List clients
	fmt.Println("\nListing clients...")
	clients, total, err := authService.ListClients(ctx, 0, 10)
	if err != nil {
		log.Fatalf("Failed to list clients: %v", err)
	}

	fmt.Printf("Found %d clients:\n", total)
	for _, c := range clients {
		fmt.Printf("  - %s (%s)\n", c.Name, c.ClientID)
	}

	fmt.Println("\n✓ Auth example completed successfully!")
}
