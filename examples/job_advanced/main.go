package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
)

func main() {
	fmt.Println("=== Job Advanced Example ===")

	// Create process manager
	mgr := provisr.New()

	// Create job manager
	jobMgr := provisr.NewJobManager(mgr)

	// Example 1: Batch processing job
	fmt.Println("\n--- Example 1: Batch Processing Job ---")
	batchJob := provisr.JobSpec{
		Name:                    "batch-process",
		Command:                 "sh -c 'echo Processing batch $((RANDOM % 100)); sleep 2; exit $((RANDOM % 2))'",
		Parallelism:             int32Ptr(3),  // Process 3 items in parallel
		Completions:             int32Ptr(5),  // Need 5 successful completions
		BackoffLimit:            int32Ptr(10), // Allow up to 10 retries
		ActiveDeadlineSeconds:   int64Ptr(60), // Job timeout of 60 seconds
		TTLSecondsAfterFinished: int32Ptr(30), // Auto-cleanup after 30 seconds
		RestartPolicy:           "OnFailure",
	}

	err := jobMgr.CreateJob(batchJob)
	if err != nil {
		log.Fatalf("Failed to create batch job: %v", err)
	}

	// Monitor job progress
	go monitorJob(jobMgr, "batch-process")

	// Example 2: One-time migration job
	fmt.Println("\n--- Example 2: Migration Job ---")
	migrationJob := provisr.JobSpec{
		Name:          "db-migration",
		Command:       "sh -c 'echo Running migration...; sleep 3; echo Migration completed'",
		Parallelism:   int32Ptr(1), // Only one instance
		Completions:   int32Ptr(1), // Only need one success
		BackoffLimit:  int32Ptr(0), // No retries for migrations
		RestartPolicy: "Never",
	}

	err = jobMgr.CreateJob(migrationJob)
	if err != nil {
		log.Fatalf("Failed to create migration job: %v", err)
	}

	// Wait for migration to complete
	fmt.Println("Waiting for migration job to complete...")
	waitForJobCompletion(jobMgr, "db-migration", 15*time.Second)

	// Example 3: Job with indexed completion mode
	fmt.Println("\n--- Example 3: Indexed Completion Job ---")
	indexedJob := provisr.JobSpec{
		Name:           "indexed-process",
		Command:        "sh -c 'echo Processing item $INDEX; sleep 1'",
		Parallelism:    int32Ptr(2),
		Completions:    int32Ptr(4),
		CompletionMode: "Indexed", // Each completion represents a specific index
		RestartPolicy:  "Never",
	}

	err = jobMgr.CreateJob(indexedJob)
	if err != nil {
		log.Fatalf("Failed to create indexed job: %v", err)
	}

	// Wait for jobs to complete
	fmt.Println("\nWaiting for all jobs to complete...")

	waitForJobCompletion(jobMgr, "batch-process", 70*time.Second)
	waitForJobCompletion(jobMgr, "indexed-process", 20*time.Second)

	// Show final status of all jobs
	fmt.Println("\n--- Final Job Status ---")
	jobs := jobMgr.ListJobs()
	for name, status := range jobs {
		fmt.Printf("Job: %s\n", name)
		fmt.Printf("  Phase: %s\n", status.Phase)
		fmt.Printf("  Active: %d, Succeeded: %d, Failed: %d\n",
			status.Active, status.Succeeded, status.Failed)
		if status.StartTime != nil && status.CompletionTime != nil {
			duration := status.CompletionTime.Sub(*status.StartTime)
			fmt.Printf("  Duration: %v\n", duration)
		}
		fmt.Println()
	}

	// Cleanup (TTL jobs will auto-cleanup)
	fmt.Println("Cleaning up...")
	for name := range jobs {
		_ = jobMgr.DeleteJob(name)
	}

	fmt.Println("Advanced example completed")
}

// waitForJobCompletion waits for a job to complete or timeout
func waitForJobCompletion(jobMgr *provisr.JobManager, jobName string, timeout time.Duration) {
	start := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		status, exists := jobMgr.GetJob(jobName)
		if !exists {
			fmt.Printf("Job %s not found\n", jobName)
			return
		}

		if status.Phase == "Succeeded" {
			fmt.Printf("Job %s completed successfully\n", jobName)
			return
		}

		if status.Phase == "Failed" {
			fmt.Printf("Job %s failed\n", jobName)
			return
		}

		if time.Since(start) > timeout {
			fmt.Printf("Job %s timed out after %v\n", jobName, timeout)
			return
		}

		<-ticker.C
	}
}

// monitorJob demonstrates real-time job monitoring
func monitorJob(jobMgr *provisr.JobManager, name string) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		status, exists := jobMgr.GetJob(name)
		if !exists {
			fmt.Printf("Job %s no longer exists, stopping monitoring\n", name)
			return
		}

		if status.Phase == "Succeeded" || status.Phase == "Failed" {
			fmt.Printf("Job %s finished monitoring (Phase: %s)\n", name, status.Phase)
			return
		}

		fmt.Printf("Job %s - Active: %d, Succeeded: %d, Failed: %d\n",
			name, status.Active, status.Succeeded, status.Failed)

		<-ticker.C
	}
}

// Helper functions
func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }
