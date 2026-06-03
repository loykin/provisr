package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
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
		Lifecycle: provisr.LifecycleHooks{
			PreStart: []provisr.Hook{
				{
					Name:        "check-dependencies",
					Command:     "echo 'Checking dependencies...' && sleep 1",
					FailureMode: provisr.FailureModeFail,
					RunMode:     provisr.RunModeBlocking,
				},
				{
					Name:        "setup-database",
					Command:     "echo 'Setting up database connection...' && sleep 1",
					FailureMode: provisr.FailureModeFail,
					RunMode:     provisr.RunModeBlocking,
				},
				{
					Name:        "migrate-schema",
					Command:     "echo 'Running database migrations...' && sleep 1",
					FailureMode: provisr.FailureModeFail,
					RunMode:     provisr.RunModeBlocking,
				},
			},
			PostStart: []provisr.Hook{
				{
					Name:        "health-check",
					Command:     "echo 'Performing health check...' && sleep 1 && echo 'Health check passed'",
					FailureMode: provisr.FailureModeIgnore, // Don't fail if health check fails
					RunMode:     provisr.RunModeBlocking,
					Timeout:     10 * time.Second,
				},
				{
					Name:        "register-service",
					Command:     "echo 'Registering service in service discovery...'",
					FailureMode: provisr.FailureModeIgnore,
					RunMode:     provisr.RunModeAsync, // Don't block on service registration
				},
			},
			PreStop: []provisr.Hook{
				{
					Name:        "drain-connections",
					Command:     "echo 'Draining active connections...' && sleep 2",
					FailureMode: provisr.FailureModeIgnore, // Continue shutdown even if drain fails
					RunMode:     provisr.RunModeBlocking,
					Timeout:     30 * time.Second,
				},
			},
			PostStop: []provisr.Hook{
				{
					Name:        "cleanup-temp-files",
					Command:     "echo 'Cleaning up temporary files...' && sleep 1",
					FailureMode: provisr.FailureModeIgnore,
					RunMode:     provisr.RunModeBlocking,
				},
				{
					Name:        "deregister-service",
					Command:     "echo 'Deregistering service from service discovery...'",
					FailureMode: provisr.FailureModeIgnore,
					RunMode:     provisr.RunModeBlocking,
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
