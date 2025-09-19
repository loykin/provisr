package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/loykin/provisr/pkg/client"
)

func main() {
	// Create a provisr client
	cfg := client.DefaultConfig()
	provisrClient := client.New(cfg)

	ctx := context.Background()

	// In CI environment, be more tolerant of daemon not being available
	if os.Getenv("CI") == "true" {
		fmt.Println("🔧 CI environment detected - checking daemon connectivity with timeout...")

		// Use a shorter context timeout in CI
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		if !provisrClient.IsReachable(timeoutCtx) {
			fmt.Println("⚠️  Provisr daemon not reachable in CI environment")
			fmt.Println("   This is expected if running without daemon setup")
			fmt.Println("   In production, ensure daemon is running with:")
			fmt.Println("   provisr serve daemon-config.toml")
			return
		}
	} else {
		// Check if provisr daemon is reachable
		if !provisrClient.IsReachable(ctx) {
			log.Fatal("❌ Provisr daemon not reachable. Start it with: provisr serve examples/embedded_client/daemon-config.toml")
		}
	}

	fmt.Println("✅ Connected to provisr daemon")

	// Start a process
	startReq := client.StartRequest{
		Name:      "my-worker",
		Command:   "sleep 5", // Shorter duration for CI
		Instances: 1,         // Fewer instances for CI
	}

	fmt.Println("Starting process...")
	if err := provisrClient.StartProcess(ctx, startReq); err != nil {
		log.Fatalf("Start failed: %v", err)
	}
	fmt.Println("✅ Process started successfully")

	// In CI, give some time for process to run then exit
	if os.Getenv("CI") == "true" {
		fmt.Println("🔄 CI mode - waiting briefly for process to run...")
		time.Sleep(2 * time.Second)
		fmt.Println("✅ Example completed successfully in CI environment")
	}
}
