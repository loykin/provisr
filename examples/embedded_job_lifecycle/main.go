package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
)

// Example demonstrating Job and CronJob with lifecycle hooks
func main() {
	fmt.Println("=== Provisr Job Lifecycle Hooks Example ===")

	mgr := provisr.New()

	// Example 1: Data processing job with setup and cleanup hooks
	dataJobSpec := provisr.JobSpec{
		Name:    "data-processor",
		Command: "sh -c 'echo \"Processing data...\"; sleep 3; echo \"Data processing completed\"'",
		Lifecycle: provisr.LifecycleHooks{
			PreStart: []provisr.Hook{
				{
					Name:        "download-data",
					Command:     "echo 'Downloading input data...' && sleep 1",
					FailureMode: provisr.FailureModeFail,
					RunMode:     provisr.RunModeBlocking,
				},
				{
					Name:        "validate-input",
					Command:     "echo 'Validating input data...' && sleep 1",
					FailureMode: provisr.FailureModeFail,
					RunMode:     provisr.RunModeBlocking,
				},
			},
			PostStart: []provisr.Hook{
				{
					Name:        "notify-start",
					Command:     "echo 'Notifying team that job started...'",
					FailureMode: provisr.FailureModeIgnore,
					RunMode:     provisr.RunModeAsync,
				},
			},
			PostStop: []provisr.Hook{
				{
					Name:        "upload-results",
					Command:     "echo 'Uploading results...' && sleep 1",
					FailureMode: provisr.FailureModeRetry,
					RunMode:     provisr.RunModeBlocking,
				},
				{
					Name:        "cleanup-workspace",
					Command:     "echo 'Cleaning up workspace...'",
					FailureMode: provisr.FailureModeIgnore,
					RunMode:     provisr.RunModeBlocking,
				},
				{
					Name:        "notify-completion",
					Command:     "echo 'Notifying team that job completed...'",
					FailureMode: provisr.FailureModeIgnore,
					RunMode:     provisr.RunModeAsync,
				},
			},
		},
	}

	// Create a job manager
	jobMgr := provisr.NewJobManager(mgr)

	fmt.Println("\n--- Starting data processing job ---")
	if err := jobMgr.CreateJob(dataJobSpec); err != nil {
		log.Fatalf("Failed to create job: %v", err)
	}

	// Monitor job status
	fmt.Println("\n--- Monitoring job progress ---")
	for i := 0; i < 10; i++ {
		status, exists := jobMgr.GetJob("data-processor")
		if !exists {
			fmt.Println("Job not found")
			break
		}

		fmt.Printf("Job status: %s (Active: %d, Succeeded: %d, Failed: %d)\n",
			status.Phase, status.Active, status.Succeeded, status.Failed)

		if status.Phase == "Succeeded" || status.Phase == "Failed" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Wait a bit more for cleanup hooks to complete
	time.Sleep(2 * time.Second)

	fmt.Println("\n=== Job Example completed ===")
}
