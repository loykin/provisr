package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr/internal/store"
)

func main() {
	ctx := context.Background()

	// Example 1: Using generic store factory
	fmt.Println("=== Generic Store Factory Example ===")

	// Create SQLite store
	sqliteConfig := store.Config{
		Type: "sqlite",
		Path: "example_store.db",
	}

	sqliteStore, err := store.CreateStore(sqliteConfig)
	if err != nil {
		log.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	fmt.Printf("Created SQLite store successfully\n")

	// Test connection
	if err := sqliteStore.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping SQLite store: %v", err)
	}
	fmt.Printf("SQLite store connection verified\n")

	// Example 2: Using auth-specific store
	fmt.Println("\n=== Auth Store Example ===")

	authConfig := store.Config{
		Type: "sqlite",
		Path: "example_auth.db",
	}

	authStore, err := store.NewAuthStore(authConfig)
	if err != nil {
		log.Fatalf("Failed to create auth store: %v", err)
	}
	defer func() { _ = authStore.Close() }()

	// Create a test user
	user := &store.User{
		ID:           "user-123",
		Username:     "testuser",
		PasswordHash: "hashed-password",
		Email:        "test@example.com",
		Roles:        []string{"user"},
		Metadata:     map[string]string{"department": "engineering"},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Active:       true,
	}

	if err := authStore.CreateUser(ctx, user); err != nil {
		log.Printf("Note: User might already exist: %v", err)
	} else {
		fmt.Printf("Created user: %s\n", user.Username)
	}

	// Retrieve the user
	retrievedUser, err := authStore.GetUserByUsername(ctx, "testuser")
	if err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}
	fmt.Printf("Retrieved user: %s (ID: %s)\n", retrievedUser.Username, retrievedUser.ID)

	// Create a test client
	client := &store.ClientCredential{
		ID:           "client-456",
		ClientID:     "test-client",
		ClientSecret: "secret-123",
		Name:         "Test Client",
		Scopes:       []string{"read", "write"},
		Metadata:     map[string]string{"app": "test-app"},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Active:       true,
	}

	if err := authStore.CreateClient(ctx, client); err != nil {
		log.Printf("Note: Client might already exist: %v", err)
	} else {
		fmt.Printf("Created client: %s\n", client.Name)
	}

	// List users and clients
	users, userCount, err := authStore.ListUsers(ctx, 0, 10)
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}
	fmt.Printf("Total users: %d\n", userCount)
	for _, u := range users {
		fmt.Printf("  - %s (%s)\n", u.Username, u.Email)
	}

	clients, clientCount, err := authStore.ListClients(ctx, 0, 10)
	if err != nil {
		log.Fatalf("Failed to list clients: %v", err)
	}
	fmt.Printf("Total clients: %d\n", clientCount)
	for _, c := range clients {
		fmt.Printf("  - %s (%s)\n", c.Name, c.ClientID)
	}

	// Example 3: Update user
	fmt.Println("\n=== Update User Example ===")

	// Update user email
	retrievedUser.Email = "updated@example.com"
	if err := authStore.UpdateUser(ctx, retrievedUser); err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}

	fmt.Printf("Successfully updated user email\n")

	// Example 4: Show supported store types
	fmt.Println("\n=== Supported Store Types ===")
	supportedTypes := store.SupportedTypes()
	fmt.Printf("Available store types: %v\n", supportedTypes)

	fmt.Println("\n✓ Store examples completed successfully!")
}
