package job

import (
	"strings"
	"testing"

	"github.com/loykin/provisr/internal/process"
)

func TestJobSpec_WithLifecycleHooks(t *testing.T) {
	spec := Spec{
		Name:    "test-job",
		Command: "echo 'job running'",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "setup",
					Command:     "echo 'job setup'",
					FailureMode: process.FailureModeFail,
				},
			},
			PostStop: []process.Hook{
				{
					Name:        "cleanup",
					Command:     "echo 'job cleanup'",
					FailureMode: process.FailureModeIgnore,
				},
			},
		},
	}

	// Test validation
	err := spec.Validate()
	if err != nil {
		t.Errorf("JobSpec.Validate() failed: %v", err)
	}

	// Test conversion to ProcessSpec
	processSpec := spec.ToProcessSpec()
	if processSpec.Name != spec.Name {
		t.Errorf("ToProcessSpec() Name = %v, want %v", processSpec.Name, spec.Name)
	}
	if processSpec.Command != spec.Command {
		t.Errorf("ToProcessSpec() Command = %v, want %v", processSpec.Command, spec.Command)
	}

	// Test lifecycle hooks are copied
	if len(processSpec.Lifecycle.PreStart) != 1 {
		t.Errorf("ToProcessSpec() PreStart hooks count = %d, want 1", len(processSpec.Lifecycle.PreStart))
	}
	if len(processSpec.Lifecycle.PostStop) != 1 {
		t.Errorf("ToProcessSpec() PostStop hooks count = %d, want 1", len(processSpec.Lifecycle.PostStop))
	}

	if processSpec.Lifecycle.PreStart[0].Name != "setup" {
		t.Errorf("ToProcessSpec() PreStart hook name = %v, want setup", processSpec.Lifecycle.PreStart[0].Name)
	}
}

func TestJobSpec_LifecycleValidation(t *testing.T) {
	tests := []struct {
		name      string
		lifecycle process.LifecycleHooks
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid lifecycle",
			lifecycle: process.LifecycleHooks{
				PreStart: []process.Hook{
					{Name: "setup", Command: "echo setup"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid hook in lifecycle",
			lifecycle: process.LifecycleHooks{
				PreStart: []process.Hook{
					{Name: "", Command: "echo test"}, // Invalid: empty name
				},
			},
			wantErr: true,
			errMsg:  "lifecycle validation failed",
		},
		{
			name: "duplicate hook names",
			lifecycle: process.LifecycleHooks{
				PreStart: []process.Hook{
					{Name: "hook1", Command: "echo test1"},
				},
				PostStart: []process.Hook{
					{Name: "hook1", Command: "echo test2"}, // Duplicate name
				},
			},
			wantErr: true,
			errMsg:  "duplicate hook name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := Spec{
				Name:      "test-job",
				Command:   "echo test",
				Lifecycle: tt.lifecycle,
			}

			err := spec.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("JobSpec.Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("JobSpec.Validate() error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("JobSpec.Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestJobSpec_LifecycleWithJobSettings(t *testing.T) {
	parallelism := int32(2)
	backoffLimit := int32(3)

	spec := Spec{
		Name:          "parallel-job",
		Command:       "echo 'parallel job'",
		Parallelism:   &parallelism,
		BackoffLimit:  &backoffLimit,
		RestartPolicy: string(RestartPolicyOnFailure),
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "parallel-setup",
					Command:     "echo 'setting up parallel job'",
					FailureMode: process.FailureModeFail,
				},
			},
		},
	}

	err := spec.Validate()
	if err != nil {
		t.Errorf("JobSpec.Validate() failed: %v", err)
	}

	processSpec := spec.ToProcessSpec()

	// Test that job settings are preserved
	if processSpec.Instances != int(parallelism) {
		t.Errorf("ToProcessSpec() Instances = %d, want %d", processSpec.Instances, parallelism)
	}
	if processSpec.RetryCount != uint32(backoffLimit) {
		t.Errorf("ToProcessSpec() RetryCount = %d, want %d", processSpec.RetryCount, backoffLimit)
	}
	if !processSpec.AutoRestart {
		t.Error("ToProcessSpec() AutoRestart = false, want true for OnFailure restart policy")
	}

	// Test that lifecycle hooks are preserved
	if len(processSpec.Lifecycle.PreStart) != 1 {
		t.Errorf("ToProcessSpec() PreStart hooks count = %d, want 1", len(processSpec.Lifecycle.PreStart))
	}
}

func TestJobSpec_LifecycleDeepCopy(t *testing.T) {
	original := Spec{
		Name:    "original-job",
		Command: "echo original",
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:    "setup",
					Command: "echo setup",
					Env:     []string{"TEST=original"},
				},
			},
		},
	}

	processSpec := original.ToProcessSpec()

	// Modify the original
	original.Lifecycle.PreStart[0].Env[0] = "TEST=modified"

	// Verify the process spec is not affected (deep copy worked)
	if processSpec.Lifecycle.PreStart[0].Env[0] == "TEST=modified" {
		t.Error("ToProcessSpec() did not create deep copy of lifecycle hooks")
	}
}
