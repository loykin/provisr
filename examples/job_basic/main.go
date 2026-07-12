package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
)

func main() {
	fmt.Println("=== Job Basic Example ===")
	mgr := provisr.New()
	defer func() { _ = mgr.Shutdown() }()
	jobs := provisr.NewJobManager(mgr)

	parallelism := int32(2)
	completions := int32(2)
	spec := provisr.JobSpec{
		Name:          "hello-job",
		Command:       "sh -c 'echo Hello-from-job; sleep 2'",
		Parallelism:   &parallelism,
		Completions:   &completions,
		RestartPolicy: "Never",
	}
	if err := jobs.CreateJob(spec); err != nil {
		log.Fatalf("Failed to create job: %v", err)
	}

	fmt.Println("Job created successfully!")
	fmt.Println("Monitoring job status...")
	for i := 0; i < 10; i++ {
		status, exists := jobs.GetJob(spec.Name)
		if !exists {
			log.Fatalf("Job disappeared before completion")
		}
		fmt.Printf("Iteration %d: phase=%s active=%d succeeded=%d failed=%d\n",
			i+1, status.Phase, status.Active, status.Succeeded, status.Failed)
		if status.Phase == "Succeeded" || status.Phase == "Failed" {
			break
		}
		time.Sleep(time.Second)
	}

	fmt.Println("Example completed")
}
