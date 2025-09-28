package main

import (
	"fmt"
	"log"
	"time"

	"github.com/loykin/provisr"
	"github.com/loykin/provisr/internal/process"
)

// Example demonstrating different failure modes in lifecycle hooks
func main() {
	fmt.Println("=== Provisr Lifecycle Failure Modes Example ===")

	mgr := provisr.New()

	// Example 1: Process with failing pre-start hook (fail mode)
	fmt.Println("\n=== Example 1: Fail Mode Hook ===")
	failProcessSpec := provisr.Spec{
		Name:    "fail-process",
		Command: "echo 'This process should not start'",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "failing-setup",
					Command:     "echo 'Setup failing...' && exit 1", // This will fail
					FailureMode: process.FailureModeFail,             // Stop process start on failure
					RunMode:     process.RunModeBlocking,
				},
			},
		},
	}

	if err := mgr.Register(failProcessSpec); err != nil {
		log.Fatalf("Failed to register fail-process: %v", err)
	}

	fmt.Println("Attempting to start process with failing pre-start hook...")
	if err := mgr.Start("fail-process"); err != nil {
		fmt.Printf("✓ Expected failure: %v\n", err)
	} else {
		fmt.Println("✗ Process started unexpectedly!")
	}

	// Example 2: Process with failing hook but ignore mode
	fmt.Println("\n=== Example 2: Ignore Mode Hook ===")
	ignoreProcessSpec := provisr.Spec{
		Name:    "ignore-process",
		Command: "sh -c 'echo \"Process started despite hook failure\"; sleep 2'",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "optional-setup",
					Command:     "echo 'Optional setup failing...' && exit 1", // This will fail
					FailureMode: process.FailureModeIgnore,                    // Continue despite failure
					RunMode:     process.RunModeBlocking,
				},
			},
			PostStop: []process.Hook{
				{
					Name:        "optional-cleanup",
					Command:     "echo 'Optional cleanup failing...' && exit 1", // This will also fail
					FailureMode: process.FailureModeIgnore,                      // Continue despite failure
					RunMode:     process.RunModeBlocking,
				},
			},
		},
	}

	if err := mgr.Register(ignoreProcessSpec); err != nil {
		log.Fatalf("Failed to register ignore-process: %v", err)
	}

	fmt.Println("Starting process with failing pre-start hook (ignore mode)...")
	if err := mgr.Start("ignore-process"); err != nil {
		fmt.Printf("✗ Unexpected failure: %v\n", err)
	} else {
		fmt.Println("✓ Process started successfully despite hook failure")
	}

	// Let it run briefly
	time.Sleep(3 * time.Second)

	fmt.Println("Stopping process (will have failing post-stop hook with ignore mode)...")
	if err := mgr.Stop("ignore-process", 5*time.Second); err != nil {
		fmt.Printf("✗ Unexpected stop failure: %v\n", err)
	} else {
		fmt.Println("✓ Process stopped successfully despite hook failure")
	}

	// Example 3: Process with retry mode hook
	fmt.Println("\n=== Example 3: Retry Mode Hook ===")
	retryProcessSpec := provisr.Spec{
		Name:    "retry-process",
		Command: "sh -c 'echo \"Process with retry hooks\"; sleep 3'",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "flaky-setup",
					Command:     "echo 'Attempting flaky setup...' && if [ $RANDOM -gt 16384 ]; then exit 1; else echo 'Setup succeeded'; fi",
					FailureMode: process.FailureModeRetry, // Retry once on failure
					RunMode:     process.RunModeBlocking,
					Timeout:     5 * time.Second,
				},
			},
		},
	}

	if err := mgr.Register(retryProcessSpec); err != nil {
		log.Fatalf("Failed to register retry-process: %v", err)
	}

	fmt.Println("Starting process with retry-mode hook (may retry on failure)...")
	if err := mgr.Start("retry-process"); err != nil {
		fmt.Printf("Process failed to start after retry: %v\n", err)
	} else {
		fmt.Println("✓ Process started (setup succeeded or succeeded after retry)")
	}

	// Let it run briefly
	time.Sleep(4 * time.Second)

	fmt.Println("Stopping retry process...")
	if err := mgr.Stop("retry-process", 5*time.Second); err != nil {
		fmt.Printf("Error stopping retry-process: %v\n", err)
	}

	// Example 4: Async hooks that don't block
	fmt.Println("\n=== Example 4: Async vs Blocking Hooks ===")
	asyncProcessSpec := provisr.Spec{
		Name:    "async-process",
		Command: "sh -c 'echo \"Process with async hooks\"; sleep 2'",
		Lifecycle: process.LifecycleHooks{
			PostStart: []process.Hook{
				{
					Name:        "slow-notification",
					Command:     "sh -c 'echo \"Starting slow notification...\"; sleep 3; echo \"Notification sent\"'",
					FailureMode: process.FailureModeIgnore,
					RunMode:     process.RunModeAsync, // Don't block process start
					Timeout:     10 * time.Second,
				},
				{
					Name:        "quick-log",
					Command:     "echo 'Quick log entry created'",
					FailureMode: process.FailureModeIgnore,
					RunMode:     process.RunModeBlocking, // Block briefly for logging
				},
			},
		},
	}

	if err := mgr.Register(asyncProcessSpec); err != nil {
		log.Fatalf("Failed to register async-process: %v", err)
	}

	fmt.Println("Starting process with async post-start hook...")
	start := time.Now()
	if err := mgr.Start("async-process"); err != nil {
		fmt.Printf("Failed to start async-process: %v\n", err)
	} else {
		duration := time.Since(start)
		fmt.Printf("✓ Process started in %v (async hook didn't block)\n", duration)
	}

	// Let everything finish
	time.Sleep(5 * time.Second)

	fmt.Println("Stopping async process...")
	if err := mgr.Stop("async-process", 5*time.Second); err != nil {
		fmt.Printf("Error stopping async-process: %v\n", err)
	}

	fmt.Println("\n=== Failure Modes Example completed ===")
	fmt.Println("\nSummary of failure modes:")
	fmt.Println("- fail: Stop the operation if hook fails")
	fmt.Println("- ignore: Continue despite hook failure")
	fmt.Println("- retry: Retry the hook once, then fail if still failing")
	fmt.Println("\nSummary of run modes:")
	fmt.Println("- blocking: Wait for hook to complete before continuing")
	fmt.Println("- async: Start hook and continue immediately")
}
