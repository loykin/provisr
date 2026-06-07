package job

import (
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/core/internal/process"
)

func TestSpec_BasicCreation(t *testing.T) {
	parallelism := int32(3)
	completions := int32(5)
	backoffLimit := int32(2)
	activeDeadline := int64(300)
	ttl := int32(600)

	spec := Spec{
		Name:                    "test-job",
		Command:                 "echo hello world",
		WorkDir:                 "/tmp",
		Env:                     []string{"ENV1=value1", "ENV2=value2"},
		TTLSecondsAfterFinished: &ttl,
		ActiveDeadlineSeconds:   &activeDeadline,
		BackoffLimit:            &backoffLimit,
		Parallelism:             &parallelism,
		Completions:             &completions,
		CompletionMode:          "NonIndexed",
		RestartPolicy:           "OnFailure",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:    "pre-start-hook",
					Command: "echo pre-start",
				},
			},
		},
	}

	if spec.Name != "test-job" {
		t.Errorf("Expected name test-job, got %s", spec.Name)
	}
	if spec.Command != "echo hello world" {
		t.Errorf("Expected command 'echo hello world', got %s", spec.Command)
	}
	if spec.WorkDir != "/tmp" {
		t.Errorf("Expected work dir /tmp, got %s", spec.WorkDir)
	}
	if len(spec.Env) != 2 {
		t.Errorf("Expected 2 env vars, got %d", len(spec.Env))
	}
	if *spec.Parallelism != 3 {
		t.Errorf("Expected parallelism 3, got %d", *spec.Parallelism)
	}
	if *spec.Completions != 5 {
		t.Errorf("Expected completions 5, got %d", *spec.Completions)
	}
	if *spec.BackoffLimit != 2 {
		t.Errorf("Expected backoff limit 2, got %d", *spec.BackoffLimit)
	}
	if spec.CompletionMode != "NonIndexed" {
		t.Errorf("Expected completion mode NonIndexed, got %s", spec.CompletionMode)
	}
	if spec.RestartPolicy != "OnFailure" {
		t.Errorf("Expected restart policy OnFailure, got %s", spec.RestartPolicy)
	}
}

func TestSpec_MinimalBasicCreation(t *testing.T) {
	spec := Spec{
		Name:    "minimal-job",
		Command: "echo minimal",
	}

	if spec.Name != "minimal-job" {
		t.Errorf("Expected name minimal-job, got %s", spec.Name)
	}
	if spec.Command != "echo minimal" {
		t.Errorf("Expected command 'echo minimal', got %s", spec.Command)
	}
	if spec.WorkDir != "" {
		t.Errorf("Expected empty work dir, got %s", spec.WorkDir)
	}
	if len(spec.Env) != 0 {
		t.Errorf("Expected 0 env vars, got %d", len(spec.Env))
	}
}

func TestJobStatus_BasicCreation(t *testing.T) {
	startTime := time.Now()
	completionTime := startTime.Add(5 * time.Minute)

	status := JobStatus{
		Phase:          JobPhaseRunning,
		StartTime:      &startTime,
		CompletionTime: nil, // Not completed yet
		Active:         2,
		Succeeded:      1,
		Failed:         0,
		Conditions: []Condition{
			{
				Type:   "Ready",
				Status: "True",
			},
		},
	}

	if status.Phase != JobPhaseRunning {
		t.Errorf("Expected phase Running, got %s", status.Phase)
	}
	if status.StartTime == nil {
		t.Error("Expected start time to be set")
	}
	if status.CompletionTime != nil {
		t.Error("Expected completion time to be nil for running job")
	}
	if status.Active != 2 {
		t.Errorf("Expected 2 active, got %d", status.Active)
	}
	if status.Succeeded != 1 {
		t.Errorf("Expected 1 succeeded, got %d", status.Succeeded)
	}
	if status.Failed != 0 {
		t.Errorf("Expected 0 failed, got %d", status.Failed)
	}
	if len(status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(status.Conditions))
	}

	// Test completed job
	status.Phase = JobPhaseSucceeded
	status.CompletionTime = &completionTime
	status.Active = 0
	status.Succeeded = 3

	if status.Phase != JobPhaseSucceeded {
		t.Errorf("Expected phase Succeeded, got %s", status.Phase)
	}
	if status.CompletionTime == nil {
		t.Error("Expected completion time to be set for completed job")
	}
	if status.Active != 0 {
		t.Errorf("Expected 0 active for completed job, got %d", status.Active)
	}
}

func TestJobPhase_BasicValues(t *testing.T) {
	phases := []JobPhase{
		JobPhasePending,
		JobPhaseRunning,
		JobPhaseSucceeded,
		JobPhaseFailed,
	}

	for _, phase := range phases {
		if phase == "" {
			t.Error("Expected phase to have a value")
		}
	}
}

func TestSpec_Args(t *testing.T) {
	tests := []struct {
		name    string
		spec    Spec
		wantErr string
	}{
		{
			name:    "args only is valid",
			spec:    Spec{Name: "job", Args: []string{"go", "run", "."}},
			wantErr: "",
		},
		{
			name:    "command only is valid",
			spec:    Spec{Name: "job", Command: "echo hello"},
			wantErr: "",
		},
		{
			name:    "neither command nor args",
			spec:    Spec{Name: "job"},
			wantErr: "requires command or args",
		},
		{
			name:    "command and args together",
			spec:    Spec{Name: "job", Command: "echo hello", Args: []string{"echo", "hello"}},
			wantErr: "mutually exclusive",
		},
		{
			name:    "args[0] empty",
			spec:    Spec{Name: "job", Args: []string{"", "arg"}},
			wantErr: "args[0] must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSpec_ArgsToProcessSpec(t *testing.T) {
	spec := Spec{
		Name: "args-job",
		Args: []string{"go", "test", "-v", "./..."},
	}

	ps := spec.ToProcessSpec()

	if len(ps.Args) != 4 {
		t.Fatalf("ToProcessSpec() Args len = %d, want 4", len(ps.Args))
	}
	if ps.Command != "" {
		t.Errorf("ToProcessSpec() Command = %q, want empty when Args is set", ps.Command)
	}

	// verify deep copy
	spec.Args[0] = "mutated"
	if ps.Args[0] != "go" {
		t.Error("ToProcessSpec() did not deep copy Args slice")
	}
}

func TestSpec_WithBasicLifecycleHooks(t *testing.T) {
	spec := Spec{
		Name:    "job-with-hooks",
		Command: "echo hello",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:    "pre-start-hook",
					Command: "echo pre-start",
				},
			},
			PostStart: []process.Hook{
				{
					Name:    "post-start-hook",
					Command: "echo post-start",
				},
			},
			PreStop: []process.Hook{
				{
					Name:    "pre-stop-hook",
					Command: "echo pre-stop",
				},
			},
			PostStop: []process.Hook{
				{
					Name:    "post-stop-hook",
					Command: "echo post-stop",
				},
			},
		},
	}

	if len(spec.Lifecycle.PreStart) == 0 {
		t.Error("Expected pre-start hook to be set")
	}
	if len(spec.Lifecycle.PostStart) == 0 {
		t.Error("Expected post-start hook to be set")
	}
	if len(spec.Lifecycle.PreStop) == 0 {
		t.Error("Expected pre-stop hook to be set")
	}
	if len(spec.Lifecycle.PostStop) == 0 {
		t.Error("Expected post-stop hook to be set")
	}

	if len(spec.Lifecycle.PreStart) > 0 && spec.Lifecycle.PreStart[0].Command != "echo pre-start" {
		t.Errorf("Expected pre-start command 'echo pre-start', got %s", spec.Lifecycle.PreStart[0].Command)
	}
}
