package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/loykin/provisr"
	"github.com/loykin/provisr/internal/config"
)

// Example demonstrating lifecycle hooks from configuration file
func main() {
	fmt.Println("=== Provisr Lifecycle Configuration Example ===")

	// Get the current directory to find the config file
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	configPath := filepath.Join(pwd, "provisr.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try relative to the example directory
		configPath = filepath.Join(filepath.Dir(pwd), "embedded_lifecycle_config", "provisr.yaml")
	}

	fmt.Printf("Loading configuration from: %s\n", configPath)

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create manager
	mgr := provisr.New()

	// Apply configuration
	if err := mgr.ApplyConfig(cfg.Specs); err != nil {
		log.Fatalf("Failed to apply config: %v", err)
	}

	// Set instance groups
	if len(cfg.GroupSpecs) > 0 {
		// Convert ServiceGroup to ManagerInstanceGroup (this is a simplified conversion)
		var instanceGroups []provisr.ManagerInstanceGroup
		for _, sg := range cfg.GroupSpecs {
			instanceGroups = append(instanceGroups, provisr.ManagerInstanceGroup{
				Name:    sg.Name,
				Members: sg.Members,
			})
		}
		mgr.SetInstanceGroups(instanceGroups)
	}

	fmt.Println("\n--- Starting web-app with lifecycle hooks ---")
	if err := mgr.Start("web-app"); err != nil {
		log.Printf("Failed to start web-app: %v", err)
	} else {
		fmt.Println("✓ web-app started successfully")
	}

	// Wait a moment
	time.Sleep(3 * time.Second)

	fmt.Println("\n--- Starting background-worker ---")
	if err := mgr.Start("background-worker"); err != nil {
		log.Printf("Failed to start background-worker: %v", err)
	} else {
		fmt.Println("✓ background-worker started successfully")
	}

	// Show status
	fmt.Println("\n--- Process Status ---")
	if status, err := mgr.Status("web-app"); err == nil {
		fmt.Printf("web-app: Running=%v, PID=%d\n", status.Running, status.PID)
	}
	if status, err := mgr.Status("background-worker"); err == nil {
		fmt.Printf("background-worker: Running=%v, PID=%d\n", status.Running, status.PID)
	}

	// Let processes run for a while
	fmt.Println("\n--- Letting processes run for 5 seconds ---")
	time.Sleep(5 * time.Second)

	// Stop background worker first
	fmt.Println("\n--- Stopping background-worker ---")
	if err := mgr.Stop("background-worker", 10*time.Second); err != nil {
		log.Printf("Error stopping background-worker: %v", err)
	} else {
		fmt.Println("✓ background-worker stopped successfully")
	}

	// Stop web app
	fmt.Println("\n--- Stopping web-app ---")
	if err := mgr.Stop("web-app", 10*time.Second); err != nil {
		log.Printf("Error stopping web-app: %v", err)
	} else {
		fmt.Println("✓ web-app stopped successfully")
	}

	fmt.Println("\n=== Configuration Example completed ===")

	// Show information about the cronjob that was configured
	fmt.Println("\nNote: The daily-backup cronjob has been configured but not started.")
	fmt.Println("It would run daily at 2 AM with its own set of lifecycle hooks.")
	fmt.Println("CronJob-level hooks are merged with JobTemplate hooks when executed.")
}
