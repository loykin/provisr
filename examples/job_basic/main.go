package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
)

func main() {
	fmt.Println("=== Job Basic Example ===")

	// Load configuration with job definitions
	config, err := provisr.LoadConfig("job_example.toml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Loaded configuration with %d process definitions\n", len(config.Specs))

	// Create manager
	mgr := provisr.New()

	// Apply configuration (this will start the job)
	if err := mgr.ApplyConfig(config.Specs); err != nil {
		log.Fatalf("Failed to apply config: %v", err)
	}

	fmt.Println("Job started successfully!")

	// Monitor job status
	fmt.Println("Monitoring job status...")
	for i := 0; i < 10; i++ {
		statuses, err := mgr.StatusAll("hello-job")
		if err != nil {
			log.Printf("Failed to get status: %v", err)
			continue
		}

		fmt.Printf("Iteration %d: Found %d job instances\n", i+1, len(statuses))
		activeCount := 0
		for _, status := range statuses {
			if status.Running {
				activeCount++
			}
			fmt.Printf("  - %s: Running=%v, PID=%d\n", status.Name, status.Running, status.PID)
		}

		if activeCount == 0 {
			fmt.Println("All job instances completed!")
			break
		}

		time.Sleep(2 * time.Second)
	}

	fmt.Println("Example completed")
}
