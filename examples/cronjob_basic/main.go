package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
)

func main() {
	fmt.Println("=== CronJob Basic Example ===")

	// Create process manager
	mgr := provisr.New()

	// Create cronjob scheduler
	scheduler := provisr.NewCronScheduler(mgr)

	// Define a cronjob spec
	cronJobSpec := provisr.CronJob{
		Name:              "hello-cronjob",
		Schedule:          "@every 5s", // Run every 5 seconds
		ConcurrencyPolicy: "Forbid",    // Don't allow concurrent executions
		JobTemplate: provisr.JobSpec{
			Name:          "hello-job-instance",
			Command:       "echo 'Hello from cronjob at $(date)!'",
			RestartPolicy: "Never",
		},
	}

	fmt.Printf("Creating cronjob: %s\n", cronJobSpec.Name)

	// Create and start the cronjob
	err := scheduler.Add(cronJobSpec)
	if err != nil {
		log.Fatalf("Failed to create cronjob: %v", err)
	}

	fmt.Printf("CronJob created successfully: %s\n", cronJobSpec.Name)
	fmt.Printf("Schedule: %s\n", cronJobSpec.Schedule)

	// Let it run for a while
	fmt.Println("Letting cronjob run for 20 seconds...")
	time.Sleep(20 * time.Second)

	fmt.Println("CronJob has been running - check the logs for executions")

	// Suspend and resume operations would need to be done via the internal manager
	// but for this basic example, we'll keep it simple

	fmt.Println("Example completed - cronjob will continue running until process ends")

	// Note: In a real application, you would call scheduler.Stop() when shutting down
	// scheduler.Stop()
}
