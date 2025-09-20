package process

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// TestDetectAlive_ProcessLifecycle tests the complete lifecycle of process detection
func TestDetectAlive_ProcessLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name    string
		command string
		timeout time.Duration
	}{
		{
			name:    "simple_echo_process",
			command: "sh -c 'echo test process; sleep 5'",
			timeout: 10 * time.Second,
		},
		{
			name:    "long_running_process",
			command: "sh -c 'while true; do echo running; sleep 1; done'",
			timeout: 15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal process spec
			spec := Spec{
				Name:    tt.name,
				Command: tt.command,
			}

			// Create process instance
			proc := New(spec)

			// Phase 1: Start process and verify it's alive
			t.Log("Phase 1: Starting process and verifying alive detection")

			// Build and start command
			cmd := spec.BuildCommand()
			err := proc.TryStart(cmd)
			if err != nil {
				t.Fatalf("Failed to start process: %v", err)
			}
			proc.SetStarted(cmd)

			defer func() {
				if proc.cmd != nil && proc.cmd.Process != nil {
					_ = proc.cmd.Process.Kill()
				}
			}()

			// Wait a moment for process to fully start
			time.Sleep(100 * time.Millisecond)

			// Verify process is detected as alive
			alive, source := proc.DetectAlive()
			if !alive {
				t.Fatalf("Process should be alive after start, got alive=%v, source=%s", alive, source)
			}
			t.Logf("✓ Process correctly detected as alive: source=%s", source)

			// Get the PID for verification
			if proc.cmd == nil || proc.cmd.Process == nil {
				t.Fatal("Process command or process is nil")
			}
			pid := proc.cmd.Process.Pid
			t.Logf("Process PID: %d", pid)

			// Phase 2: Kill the process and verify it's detected as dead
			t.Log("Phase 2: Killing process and verifying dead detection")

			// Kill the process forcefully
			err = proc.cmd.Process.Kill()
			if err != nil {
				t.Fatalf("Failed to kill process: %v", err)
			}

			// Wait for process to die
			err = proc.cmd.Wait()
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					t.Logf("Process exited as expected: %v", exitErr)
				}
			}
			// Give some time for the system to clean up
			time.Sleep(200 * time.Millisecond)

			// Phase 3: Verify DetectAlive correctly identifies dead process
			t.Log("Phase 3: Verifying dead process detection")

			maxAttempts := 5
			var lastResult bool
			var lastSource string

			for i := 0; i < maxAttempts; i++ {
				alive, source = proc.DetectAlive()
				lastResult = alive
				lastSource = source

				t.Logf("Attempt %d: DetectAlive returned alive=%v, source=%s", i+1, alive, source)

				if !alive {
					t.Logf("✓ Process correctly detected as dead after %d attempts", i+1)
					break
				}

				// Wait a bit before retrying
				time.Sleep(100 * time.Millisecond)
			}

			if lastResult {
				// This is the critical failure - false positive detection
				t.Errorf("CRITICAL: DetectAlive shows false positive! Process PID %d is dead but DetectAlive returned alive=true, source=%s", pid, lastSource)

				// Additional verification using system tools
				t.Logf("Additional verification for PID %d:", pid)

				// Test with direct kill signal
				err := syscall.Kill(pid, 0)
				t.Logf("syscall.Kill(pid, 0) error: %v", err)

				// Test with ps command
				cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid))
				output, err := cmd.CombinedOutput()
				t.Logf("ps command output: %s, error: %v", string(output), err)
			}
		})
	}
}

// TestDetectAlive_FalsePositiveScenarios tests specific scenarios known to cause false positives
func TestDetectAlive_FalsePositiveScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Test the specific api-server command that's causing issues
	spec := Spec{
		Name:    "test-api-server",
		Command: "sh -c 'while true; do echo api-server running; sleep 2; done'",
	}

	proc := New(spec)

	// Start the process
	cmd := spec.BuildCommand()
	err := proc.TryStart(cmd)
	if err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	proc.SetStarted(cmd)

	defer func() {
		if proc.cmd != nil && proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
	}()

	// Wait for process to start
	time.Sleep(200 * time.Millisecond)

	// Verify alive
	alive, source := proc.DetectAlive()
	if !alive {
		t.Fatalf("Process should be alive, got alive=%v, source=%s", alive, source)
	}

	pid := proc.cmd.Process.Pid
	t.Logf("Started test process with PID: %d", pid)

	// Kill with SIGKILL
	t.Logf("Killing process PID %d with SIGKILL", pid)
	err = syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		t.Fatalf("Failed to send SIGKILL: %v", err)
	}

	// Wait for process to die
	err = proc.cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Logf("Process exited as expected: %v", exitErr)
		}
	}

	time.Sleep(2 * time.Second)

	// This is the critical test - should detect as dead
	alive, source = proc.DetectAlive()
	if alive {
		t.Errorf("FALSE POSITIVE DETECTED: PID %d is dead but DetectAlive returned alive=true, source=%s", pid, source)

		// Debug information
		t.Logf("OS: %s", runtime.GOOS)

		// Test raw syscall
		killErr := syscall.Kill(pid, 0)
		t.Logf("Raw syscall.Kill(%d, 0) error: %v", pid, killErr)

		// Test ps command
		psCmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid))
		psOutput, psErr := psCmd.CombinedOutput()
		t.Logf("ps -p %d output: %s, error: %v", pid, string(psOutput), psErr)
	} else {
		t.Logf("✓ Correctly detected dead process: alive=%v, source=%s", alive, source)
	}
}

// BenchmarkDetectAlive benchmarks the performance of DetectAlive
func BenchmarkDetectAlive(b *testing.B) {
	spec := Spec{
		Name:    "benchmark-process",
		Command: "sleep 10",
	}

	proc := New(spec)

	cmd := spec.BuildCommand()
	err := proc.TryStart(cmd)
	if err != nil {
		b.Fatalf("Failed to start process: %v", err)
	}
	proc.SetStarted(cmd)

	defer func() {
		if proc.cmd != nil && proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proc.DetectAlive()
	}
}
