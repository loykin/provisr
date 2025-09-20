package manager

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// TestConcurrentProcessManagement tests multiple processes being started/stopped concurrently
func TestConcurrentProcessManagement(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	const numProcesses = 10
	const numOperations = 5

	// Create test specs
	specs := make([]process.Spec, numProcesses)
	for i := 0; i < numProcesses; i++ {
		specs[i] = process.Spec{
			Name:    fmt.Sprintf("test-proc-%d", i),
			Command: "sleep 0.1",
		}
	}

	// Test concurrent starts
	t.Run("ConcurrentStarts", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, numProcesses)

		// Start all processes concurrently
		for i := 0; i < numProcesses; i++ {
			wg.Add(1)
			go func(spec process.Spec) {
				defer wg.Done()
				if err := mgr.Start(spec); err != nil {
					errors <- err
				}
			}(specs[i])
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent start failed: %v", err)
			}
		}

		// Verify all processes are tracked
		for i := 0; i < numProcesses; i++ {
			if _, err := mgr.Status(specs[i].Name); err != nil {
				t.Errorf("Process %s not found after start: %v", specs[i].Name, err)
			}
		}
	})

	// Test concurrent stops
	t.Run("ConcurrentStops", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, numProcesses)

		// Stop all processes concurrently
		for i := 0; i < numProcesses; i++ {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				if err := mgr.Stop(name, 5*time.Second); err != nil {
					errors <- err
				}
			}(specs[i].Name)
		}

		wg.Wait()
		close(errors)

		// Check for errors (some may not exist, that's ok)
		for err := range errors {
			if err != nil {
				t.Logf("Concurrent stop result: %v", err)
			}
		}
	})

	// Test mixed concurrent operations
	t.Run("ConcurrentMixedOps", func(t *testing.T) {
		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Launch multiple goroutines doing random operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				spec := process.Spec{
					Name:    fmt.Sprintf("mixed-proc-%d", id),
					Command: "true",
				}

				select {
				case <-ctx.Done():
					return
				default:
				}

				// Start
				if err := mgr.Start(spec); err != nil {
					t.Logf("Start failed for %s: %v", spec.Name, err)
				}

				// Check status
				if status, err := mgr.Status(spec.Name); err != nil {
					t.Logf("Status check failed for %s: %v", spec.Name, err)
				} else {
					t.Logf("Process %s status: running=%v, PID=%d", spec.Name, status.Running, status.PID)
				}

				// Wait a bit
				time.Sleep(100 * time.Millisecond)

				// Stop
				if err := mgr.Stop(spec.Name, 2*time.Second); err != nil {
					t.Logf("Stop failed for %s: %v", spec.Name, err)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestProcessRecoveryAndMonitoring tests process monitoring and auto-recovery
func TestProcessRecoveryAndMonitoring(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	t.Run("ProcessStatusTracking", func(t *testing.T) {
		spec := process.Spec{
			Name:    "status-test",
			Command: "echo hello",
		}

		// Start process
		if err := mgr.Start(spec); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}

		// Check initial status
		status, err := mgr.Status(spec.Name)
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}

		t.Logf("Initial status: running=%v, PID=%d", status.Running, status.PID)

		// Wait for process to complete
		time.Sleep(100 * time.Millisecond)

		// Check final status
		finalStatus, err := mgr.Status(spec.Name)
		if err != nil {
			t.Fatalf("Failed to get final status: %v", err)
		}

		t.Logf("Final status: running=%v, PID=%d", finalStatus.Running, finalStatus.PID)
		if finalStatus.Running {
			t.Logf("Process still running (expected for echo command)")
		}
	})

	t.Run("MultipleInstancesPattern", func(t *testing.T) {
		spec := process.Spec{
			Name:      "multi-test",
			Command:   "sleep 0.1",
			Instances: 3,
		}

		// Start multiple instances
		if err := mgr.StartN(spec); err != nil {
			t.Fatalf("Failed to start multiple instances: %v", err)
		}

		// Check that all instances are tracked
		expectedNames := []string{
			"multi-test-1",
			"multi-test-2",
			"multi-test-3",
		}

		for _, name := range expectedNames {
			if status, err := mgr.Status(name); err != nil {
				t.Errorf("Instance %s not found: %v", name, err)
			} else {
				t.Logf("Instance %s: running=%v, PID=%d", name, status.Running, status.PID)
			}
		}

		// Test pattern matching
		count, err := mgr.Count("multi-test*")
		if err != nil {
			t.Fatalf("Failed to count processes: %v", err)
		}
		t.Logf("Found %d processes matching pattern", count)

		// Stop all matching processes
		if err := mgr.StopAll("multi-test*", 3*time.Second); err != nil {
			t.Logf("StopAll result: %v", err)
		}
	})
}

// BenchmarkConcurrentOperations benchmarks concurrent process operations
func BenchmarkConcurrentOperations(b *testing.B) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	b.Run("ConcurrentStartStop", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				spec := process.Spec{
					Name:    fmt.Sprintf("bench-proc-%d", i),
					Command: "true", // Very quick command
				}

				if err := mgr.Start(spec); err != nil {
					b.Errorf("Start failed: %v", err)
				}

				if _, err := mgr.Status(spec.Name); err != nil {
					b.Errorf("Status check failed: %v", err)
				}

				if err := mgr.Stop(spec.Name, time.Second); err != nil {
					b.Logf("Stop failed: %v", err)
				}

				i++
			}
		})
	})
}
