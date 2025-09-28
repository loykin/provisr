package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
	"github.com/loykin/provisr/internal/process"
)

// Example demonstrating lifecycle hooks functionality
// This shows how to use pre_start, post_start, pre_stop, and post_stop hooks
func main() {
	fmt.Println("=== Provisr Lifecycle Hooks Example ===")

	mgr := provisr.New()

	// Example 1: Web server with database setup hooks
	webServerSpec := provisr.Spec{
		Name:    "web-server",
		Command: "sh -c 'echo \"Web server starting...\"; sleep 3; echo \"Web server running on port 8080\"; sleep 5; echo \"Web server shutting down\"'",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "check-dependencies",
					Command:     "echo 'Checking dependencies...' && sleep 1",
					FailureMode: process.FailureModeFail,
					RunMode:     process.RunModeBlocking,
				},
				{
					Name:        "setup-database",
					Command:     "echo 'Setting up database connection...' && sleep 1",
					FailureMode: process.FailureModeFail,
					RunMode:     process.RunModeBlocking,
				},
				{
					Name:        "migrate-schema",
					Command:     "echo 'Running database migrations...' && sleep 1",
					FailureMode: process.FailureModeFail,
					RunMode:     process.RunModeBlocking,
				},
			},
			PostStart: []process.Hook{
				{
					Name:        "health-check",
					Command:     "echo 'Performing health check...' && sleep 1 && echo 'Health check passed'",
					FailureMode: process.FailureModeIgnore, // Don't fail if health check fails
					RunMode:     process.RunModeBlocking,
					Timeout:     10 * time.Second,
				},
				{
					Name:        "register-service",
					Command:     "echo 'Registering service in service discovery...'",
					FailureMode: process.FailureModeIgnore,
					RunMode:     process.RunModeAsync, // Don't block on service registration
				},
			},
			PreStop: []process.Hook{
				{
					Name:        "drain-connections",
					Command:     "echo 'Draining active connections...' && sleep 2",
					FailureMode: process.FailureModeIgnore, // Continue shutdown even if drain fails
					RunMode:     process.RunModeBlocking,
					Timeout:     30 * time.Second,
				},
			},
			PostStop: []process.Hook{
				{
					Name:        "cleanup-temp-files",
					Command:     "echo 'Cleaning up temporary files...' && sleep 1",
					FailureMode: process.FailureModeIgnore,
					RunMode:     process.RunModeBlocking,
				},
				{
					Name:        "deregister-service",
					Command:     "echo 'Deregistering service from service discovery...'",
					FailureMode: process.FailureModeIgnore,
					RunMode:     process.RunModeBlocking,
				},
			},
		},
	}

	// Register the process
	if err := mgr.Register(webServerSpec); err != nil {
		log.Fatalf("Failed to register web server: %v", err)
	}

	fmt.Println("\n--- Starting web server with lifecycle hooks ---")
	if err := mgr.Start("web-server"); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}

	// Let it run for a while
	fmt.Println("\n--- Web server is running, waiting 8 seconds ---")
	time.Sleep(8 * time.Second)

	// Stop the process
	fmt.Println("\n--- Stopping web server with lifecycle hooks ---")
	if err := mgr.Stop("web-server", 10*time.Second); err != nil {
		log.Printf("Error stopping web server: %v", err)
	}

	fmt.Println("\n=== Example completed ===")
}
