package manager

import (
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

func TestManagedProcess_ExecuteLifecycleHooks(t *testing.T) {
	envMerger := func(spec process.Spec) []string {
		return spec.Env
	}

	spec := process.Spec{
		Name:    "test-process",
		Command: "echo 'test process'",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "pre-hook",
					Command:     "echo 'pre-start hook executed'",
					FailureMode: process.FailureModeFail,
					RunMode:     process.RunModeBlocking,
				},
			},
			PostStart: []process.Hook{
				{
					Name:        "post-hook",
					Command:     "echo 'post-start hook executed'",
					FailureMode: process.FailureModeIgnore,
					RunMode:     process.RunModeBlocking,
				},
			},
		},
	}

	mp := NewManagedProcess(spec, envMerger)

	// Test pre-start hooks
	err := mp.executeLifecycleHooks(spec, process.PhasePreStart)
	if err != nil {
		t.Errorf("executeLifecycleHooks(PreStart) failed: %v", err)
	}

	// Test post-start hooks
	err = mp.executeLifecycleHooks(spec, process.PhasePostStart)
	if err != nil {
		t.Errorf("executeLifecycleHooks(PostStart) failed: %v", err)
	}

	// Test with no hooks
	emptySpec := process.Spec{
		Name:    "empty-process",
		Command: "echo test",
	}
	err = mp.executeLifecycleHooks(emptySpec, process.PhasePreStart)
	if err != nil {
		t.Errorf("executeLifecycleHooks with no hooks failed: %v", err)
	}

	_ = mp.Shutdown()
}

func TestManagedProcess_ExecuteHookFailureModes(t *testing.T) {
	envMerger := func(spec process.Spec) []string {
		return spec.Env
	}

	tests := []struct {
		name        string
		hook        process.Hook
		expectError bool
	}{
		{
			name: "success hook",
			hook: process.Hook{
				Name:        "success",
				Command:     "echo 'success'",
				FailureMode: process.FailureModeFail,
			},
			expectError: false,
		},
		{
			name: "failing hook with ignore mode",
			hook: process.Hook{
				Name:        "fail-ignore",
				Command:     "exit 1",
				FailureMode: process.FailureModeIgnore,
			},
			expectError: false,
		},
		{
			name: "failing hook with fail mode",
			hook: process.Hook{
				Name:        "fail-fail",
				Command:     "exit 1",
				FailureMode: process.FailureModeFail,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := process.Spec{
				Name:    "test-process",
				Command: "echo test",
				Lifecycle: process.LifecycleHooks{
					PreStart: []process.Hook{tt.hook},
				},
			}

			mp := NewManagedProcess(spec, envMerger)
			err := mp.executeLifecycleHooks(spec, process.PhasePreStart)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			_ = mp.Shutdown()
		})
	}
}

func TestManagedProcess_ExecuteHookWithTimeout(t *testing.T) {
	envMerger := func(spec process.Spec) []string {
		return spec.Env
	}

	// Test hook that should timeout
	spec := process.Spec{
		Name:    "test-process",
		Command: "echo test",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "timeout-hook",
					Command:     "sleep 2",
					Timeout:     100 * time.Millisecond,
					FailureMode: process.FailureModeFail,
				},
			},
		},
	}

	mp := NewManagedProcess(spec, envMerger)
	start := time.Now()
	err := mp.executeLifecycleHooks(spec, process.PhasePreStart)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error but got none")
	}

	// Should complete in around timeout duration, not the full sleep duration
	if duration > 1*time.Second {
		t.Errorf("Hook took too long to timeout: %v", duration)
	}

	_ = mp.Shutdown()
}

func TestManagedProcess_ExecuteHookEnvironmentVariables(t *testing.T) {
	envMerger := func(spec process.Spec) []string {
		return append(spec.Env, "GLOBAL_VAR=global_value")
	}

	// Create a hook that prints environment variables
	spec := process.Spec{
		Name:    "test-process",
		Command: "echo test",
		Env:     []string{"PROCESS_VAR=process_value"},
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:    "env-test",
					Command: "env | grep -E '(PROVISR_|PROCESS_|HOOK_)'",
					Env:     []string{"HOOK_VAR=hook_value"},
				},
			},
		},
	}

	mp := NewManagedProcess(spec, envMerger)

	// This should not fail - environment variables should be properly set
	err := mp.executeLifecycleHooks(spec, process.PhasePreStart)
	if err != nil {
		t.Errorf("executeLifecycleHooks failed: %v", err)
	}

	_ = mp.Shutdown()
}

func TestManagedProcess_HookIntegrationWithStartStop(t *testing.T) {
	envMerger := func(spec process.Spec) []string {
		return spec.Env
	}

	// Create a spec with hooks that create/remove a test file
	testFile := "/tmp/provisr_hook_test"

	spec := process.Spec{
		Name:    "hook-integration-test",
		Command: "sleep 0.5", // Short-lived process
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "create-file",
					Command:     "touch " + testFile,
					FailureMode: process.FailureModeFail,
				},
			},
			PostStart: []process.Hook{
				{
					Name:        "verify-file",
					Command:     "test -f " + testFile,
					FailureMode: process.FailureModeFail,
				},
			},
			PreStop: []process.Hook{
				{
					Name:        "pre-cleanup",
					Command:     "echo 'cleaning up'",
					FailureMode: process.FailureModeIgnore,
				},
			},
			PostStop: []process.Hook{
				{
					Name:        "remove-file",
					Command:     "rm -f " + testFile,
					FailureMode: process.FailureModeIgnore,
				},
			},
		},
	}

	mp := NewManagedProcess(spec, envMerger)

	// Start the process (this should trigger pre and post start hooks)
	err := mp.Start(spec)
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait a bit for the process to run
	time.Sleep(100 * time.Millisecond)

	// Stop the process (this should trigger pre and post stop hooks)
	err = mp.Stop(2 * time.Second)
	if err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}

	// Cleanup in case hooks failed
	_ = mp.Shutdown()
}

func TestManagedProcess_HookFailureInPreStart(t *testing.T) {
	envMerger := func(spec process.Spec) []string {
		return spec.Env
	}

	spec := process.Spec{
		Name:    "failing-prestart-test",
		Command: "echo 'should not run'",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "failing-hook",
					Command:     "exit 1",
					FailureMode: process.FailureModeFail,
				},
			},
		},
	}

	mp := NewManagedProcess(spec, envMerger)

	// Start should fail due to pre-start hook failure
	err := mp.Start(spec)
	if err == nil {
		t.Error("Expected start to fail due to pre-start hook failure")
	}

	if err != nil && !strings.Contains(err.Error(), "pre_start hooks failed") {
		t.Errorf("Error should mention pre_start hooks failure, got: %v", err)
	}

	// Process should be stopped
	status := mp.Status()
	if status.Running {
		t.Error("Process should not be running after pre-start hook failure")
	}

	_ = mp.Shutdown()
}

func TestManagedProcess_AsyncHookExecution(t *testing.T) {
	envMerger := func(spec process.Spec) []string {
		return spec.Env
	}

	spec := process.Spec{
		Name:    "async-hook-test",
		Command: "echo test",
		Lifecycle: process.LifecycleHooks{
			PostStart: []process.Hook{
				{
					Name:    "async-hook",
					Command: "sleep 0.1 && echo 'async completed'",
					RunMode: process.RunModeAsync,
				},
			},
		},
	}

	mp := NewManagedProcess(spec, envMerger)

	// Async hooks should not block execution
	start := time.Now()
	err := mp.executeLifecycleHooks(spec, process.PhasePostStart)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Async hook execution failed: %v", err)
	}

	// Should complete quickly since it's async
	if duration > 50*time.Millisecond {
		t.Errorf("Async hook took too long: %v", duration)
	}

	_ = mp.Shutdown()
}
