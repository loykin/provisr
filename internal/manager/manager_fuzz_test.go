package manager

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// FuzzManagerConcurrentOperations tests Manager under high-load concurrent scenarios
// with random operations to catch race conditions and state inconsistencies
func FuzzManagerConcurrentOperations(f *testing.F) {
	// Seed with interesting test cases
	f.Add([]byte("proc-1 sleep 0.1"))
	f.Add([]byte("test-proc /bin/sh -c 'echo hello'"))
	f.Add([]byte("multi-instance-proc echo test"))
	f.Add([]byte("long-name-process-with-special-chars_123 true"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 || len(data) > 1000 {
			t.Skip("data too short or too long")
		}

		// Parse fuzzing input into command specs
		specs := parseSpecs(string(data))
		if len(specs) == 0 {
			t.Skip("no valid specs generated")
		}

		mgr := NewManager()
		defer func() {
			// Cleanup all processes
			_ = mgr.StopAll("fuzz-cleanup", 200*time.Millisecond)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Run concurrent operations
		var wg sync.WaitGroup
		errChan := make(chan error, 100)

		// Start multiple goroutines performing random operations
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

				for j := 0; j < 5; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}

					spec := specs[r.Intn(len(specs))]

					// Random operations
					switch r.Intn(4) {
					case 0: // Start
						if err := mgr.Start(spec); err != nil && !isAcceptableError(err) {
							errChan <- fmt.Errorf("start %s: %w", spec.Name, err)
						}
					case 1: // Stop
						if err := mgr.Stop(spec.Name, 100*time.Millisecond); err != nil && !isAcceptableError(err) {
							errChan <- fmt.Errorf("stop %s: %w", spec.Name, err)
						}
					case 2: // Status
						if _, err := mgr.Status(spec.Name); err != nil && !isAcceptableError(err) {
							errChan <- fmt.Errorf("status %s: %w", spec.Name, err)
						}
					case 3: // StatusAll
						_, _ = mgr.StatusAll("*")
					}

					time.Sleep(time.Duration(r.Intn(10)) * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// Check for critical errors
		for err := range errChan {
			if isCriticalError(err) {
				t.Fatal(err)
			}
		}

		// Final cleanup
		_ = mgr.StopAll("fuzz-cleanup", 200*time.Millisecond)
	})
}

// FuzzProcessSpecValidation tests process specification validation
func FuzzProcessSpecValidation(f *testing.F) {
	// Seed with edge cases
	f.Add("", "", "", int64(0), int64(0), int64(1))
	f.Add("proc", "echo test", "/tmp", int64(1000), int64(5000), int64(2))
	f.Add("../..", "/bin/sh", ".", int64(-1), int64(0), int64(0))

	f.Fuzz(func(t *testing.T, name, command, workdir string,
		startDur, stopWait int64, instances int64) {

		// Limit resource consumption
		if len(name) > 200 || len(command) > 1000 || len(workdir) > 500 {
			t.Skip("input too long")
		}

		if instances < 0 || instances > 10 {
			instances = 1
		}

		spec := process.Spec{
			Name:          name,
			Command:       command,
			WorkDir:       workdir,
			StartDuration: time.Duration(startDur%10000) * time.Millisecond,
			Instances:     int(instances),
		}

		mgr := NewManager()
		defer func() {
			_ = mgr.StopAll("fuzz-cleanup", 200*time.Millisecond)
		}()

		// Test start - should handle invalid specs gracefully
		err := mgr.Start(spec)

		// For fuzzing, we mainly care that the system doesn't crash
		// Some invalid specs might be accepted by the manager but fail later
		// which is acceptable behavior
		if name == "" {
			// Empty name should be handled somehow, but not crash
			if err == nil {
				// This might be acceptable - the underlying process might handle it
				t.Logf("empty name accepted, err=%v", err)
			}
		}

		// Test status operations regardless of start result
		if name != "" {
			status, statusErr := mgr.Status(name)
			if statusErr != nil && !strings.Contains(statusErr.Error(), "not found") {
				if isCriticalError(statusErr) {
					t.Errorf("critical status error: %v", statusErr)
				}
			}

			// Verify status consistency
			if statusErr == nil && status.Name != name {
				t.Errorf("status name mismatch: got %s, want %s", status.Name, name)
			}
		}

		// Cleanup
		if name != "" {
			_ = mgr.Stop(name, 100*time.Millisecond)
		}
	})
}

// FuzzPatternMatching tests wildcard and pattern matching functionality
func FuzzPatternMatching(f *testing.F) {
	f.Add("test-*", "test-1")
	f.Add("proc", "proc")
	f.Add("*", "anything")
	f.Add("", "empty")
	f.Add("special-chars_123.*", "special-chars_123.instance")

	f.Fuzz(func(t *testing.T, pattern, processName string) {
		if len(pattern) > 100 || len(processName) > 100 {
			t.Skip("input too long")
		}

		mgr := NewManager()
		defer func() {
			_ = mgr.StopAll("fuzz-cleanup", 100*time.Millisecond)
		}()

		// Create a test process if name is valid
		if processName != "" && isValidName(processName) {
			spec := process.Spec{
				Name:      processName,
				Command:   "sleep 0.1",
				Instances: 1,
			}
			_ = mgr.Start(spec) // Ignore errors for fuzzing
		}

		// Test pattern matching operations - should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in StatusMatch: %v", r)
				}
			}()
			_, _ = mgr.StatusMatch(pattern)
		}()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in StopMatch: %v", r)
				}
			}()
			_ = mgr.StopMatch(pattern, 50*time.Millisecond)
		}()
	})
}

// Helper functions

func parseSpecs(data string) []process.Spec {
	var specs []process.Spec
	lines := strings.Split(data, "\n")

	for i, line := range lines {
		if len(line) < 3 {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		command := strings.Join(parts[1:], " ")

		// Basic validation to avoid completely invalid specs
		if !isValidName(name) || len(command) > 500 {
			continue
		}

		spec := process.Spec{
			Name:      name,
			Command:   command,
			Instances: 1 + (i % 3), // Vary instances
		}

		specs = append(specs, spec)

		if len(specs) >= 5 { // Limit number of specs
			break
		}
	}

	return specs
}

func isValidName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}

	// Check for dangerous patterns
	if strings.Contains(name, "..") ||
		strings.ContainsAny(name, "/\\") ||
		strings.HasPrefix(name, ".") {
		return false
	}

	return true
}

func isAcceptableError(err error) bool {
	if err == nil {
		return true
	}

	errStr := err.Error()

	// These are expected/acceptable errors during fuzzing
	acceptable := []string{
		"not found",
		"already running",
		"already starting",
		"currently stopping",
		"invalid state",
		"failed to start",
		"failed to stop",
		"process manager shutting down",
		"no such file or directory",
		"permission denied",
		"executable file not found",
	}

	for _, pattern := range acceptable {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

func isCriticalError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// These indicate serious problems that should fail the test
	critical := []string{
		"panic",
		"race",
		"deadlock",
		"concurrent map",
		"nil pointer",
		"index out of range",
	}

	for _, pattern := range critical {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
