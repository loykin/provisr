package cronjob

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/core/internal/job"
	"github.com/loykin/provisr/core/internal/process"
	"github.com/loykin/provisr/core/observability"
)

// fakeJobRunner captures the job.Spec passed to CreateJob so tests can
// inspect exactly what a scheduled run would have submitted, without needing
// a real job.Manager.
type fakeJobRunner struct {
	created chan job.Spec
}

func (f *fakeJobRunner) CreateJob(spec job.Spec) (*job.Job, error) {
	f.created <- spec
	return nil, errors.New("fakeJobRunner: CreateJob not implemented")
}

func (f *fakeJobRunner) Observe(observability.Event) {}

// TestCronJob_TriggerNow_MergesCronJobLevelHooks is the end-to-end
// counterpart to the CreateJobFromTemplate unit tests above: those only
// checked the merge helper in isolation, which is how a production bug
// slipped through — cronjob.go's executeJob() used to build the job spec
// straight from JobTemplate, never calling CreateJobFromTemplate at all, so
// CronJob-level lifecycle hooks were silently dropped on every real
// scheduled run despite the merge helper itself being fully covered. This
// drives an actual TriggerNow() and inspects the spec that reaches
// JobRunner.CreateJob.
func TestCronJob_TriggerNow_MergesCronJobLevelHooks(t *testing.T) {
	spec := CronJobSpec{
		Name:     "trigger-test",
		Schedule: "@every 1h",
		JobTemplate: job.Spec{
			Name:    "trigger-test",
			Command: "echo hi",
		},
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{Name: "cronjob-level", Command: "echo cronjob-level", FailureMode: process.FailureModeFail},
			},
		},
	}

	runner := &fakeJobRunner{created: make(chan job.Spec, 1)}
	cj := NewCronJob(spec, runner)
	cj.TriggerNow()

	select {
	case created := <-runner.created:
		if len(created.Lifecycle.PreStart) != 1 || created.Lifecycle.PreStart[0].Name != "cronjob-level" {
			t.Fatalf("expected the actual scheduled run to include the cronjob-level PreStart hook, got %+v", created.Lifecycle.PreStart)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for CreateJob to be called")
	}
}

func TestCronJobSpec_WithLifecycleHooks(t *testing.T) {
	spec := CronJobSpec{
		Name:     "test-cronjob",
		Schedule: "0 0 * * *", // Daily at midnight
		JobTemplate: job.Spec{
			Name:    "template-job",
			Command: "echo 'job from cronjob'",
			Lifecycle: process.LifecycleHooks{
				PreStart: []process.Hook{
					{
						Name:        "job-setup",
						Command:     "echo 'job-level setup'",
						FailureMode: process.FailureModeFail,
					},
				},
			},
		},
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{
					Name:        "cronjob-setup",
					Command:     "echo 'cronjob-level setup'",
					FailureMode: process.FailureModeFail,
				},
			},
			PostStop: []process.Hook{
				{
					Name:        "cronjob-cleanup",
					Command:     "echo 'cronjob-level cleanup'",
					FailureMode: process.FailureModeIgnore,
				},
			},
		},
	}

	// Apply defaults and test validation
	spec.GetDefaults()
	err := spec.Validate()
	if err != nil {
		t.Errorf("CronJobSpec.Validate() failed: %v", err)
	}

	// Test job creation from template
	jobSpec := spec.CreateJobFromTemplate("test-job-instance")

	if jobSpec.Name != "test-job-instance" {
		t.Errorf("CreateJobFromTemplate() Name = %v, want test-job-instance", jobSpec.Name)
	}

	// Test lifecycle hooks are merged (CronJob hooks should come first)
	if len(jobSpec.Lifecycle.PreStart) != 2 {
		t.Errorf("CreateJobFromTemplate() PreStart hooks count = %d, want 2", len(jobSpec.Lifecycle.PreStart))
	}

	// CronJob hooks should come first
	if jobSpec.Lifecycle.PreStart[0].Name != "cronjob-setup" {
		t.Errorf("CreateJobFromTemplate() first PreStart hook = %v, want cronjob-setup", jobSpec.Lifecycle.PreStart[0].Name)
	}
	if jobSpec.Lifecycle.PreStart[1].Name != "job-setup" {
		t.Errorf("CreateJobFromTemplate() second PreStart hook = %v, want job-setup", jobSpec.Lifecycle.PreStart[1].Name)
	}

	// Test PostStop hooks from CronJob level
	if len(jobSpec.Lifecycle.PostStop) != 1 {
		t.Errorf("CreateJobFromTemplate() PostStop hooks count = %d, want 1", len(jobSpec.Lifecycle.PostStop))
	}
	if jobSpec.Lifecycle.PostStop[0].Name != "cronjob-cleanup" {
		t.Errorf("CreateJobFromTemplate() PostStop hook = %v, want cronjob-cleanup", jobSpec.Lifecycle.PostStop[0].Name)
	}
}

func TestCronJobSpec_LifecycleValidation(t *testing.T) {
	tests := []struct {
		name    string
		cronjob CronJobSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid cronjob with lifecycle",
			cronjob: CronJobSpec{
				Name:     "valid-cronjob",
				Schedule: "0 0 * * *",
				JobTemplate: job.Spec{
					Name:    "template",
					Command: "echo test",
				},
				Lifecycle: process.LifecycleHooks{
					PreStart: []process.Hook{
						{Name: "setup", Command: "echo setup"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid lifecycle hooks",
			cronjob: CronJobSpec{
				Name:     "invalid-cronjob",
				Schedule: "0 0 * * *",
				JobTemplate: job.Spec{
					Name:    "template",
					Command: "echo test",
				},
				Lifecycle: process.LifecycleHooks{
					PreStart: []process.Hook{
						{Name: "", Command: "echo test"}, // Invalid: empty name
					},
				},
			},
			wantErr: true,
			errMsg:  "lifecycle validation failed",
		},
		{
			name: "invalid job template with lifecycle",
			cronjob: CronJobSpec{
				Name:     "invalid-template-cronjob",
				Schedule: "0 0 * * *",
				JobTemplate: job.Spec{
					Name:    "template",
					Command: "echo test",
					Lifecycle: process.LifecycleHooks{
						PreStart: []process.Hook{
							{Name: "dup", Command: "echo test1"},
						},
						PostStart: []process.Hook{
							{Name: "dup", Command: "echo test2"}, // Duplicate name
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid job template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cronjob.GetDefaults()
			err := tt.cronjob.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("CronJobSpec.Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("CronJobSpec.Validate() error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("CronJobSpec.Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestCronJobSpec_CreateJobFromTemplate_NoLifecycleHooks(t *testing.T) {
	spec := CronJobSpec{
		Name:     "simple-cronjob",
		Schedule: "0 0 * * *",
		JobTemplate: job.Spec{
			Name:    "template-job",
			Command: "echo 'simple job'",
		},
		// No lifecycle hooks at CronJob level
	}

	jobSpec := spec.CreateJobFromTemplate("simple-job-instance")

	if jobSpec.Name != "simple-job-instance" {
		t.Errorf("CreateJobFromTemplate() Name = %v, want simple-job-instance", jobSpec.Name)
	}

	// Should have no lifecycle hooks
	if jobSpec.Lifecycle.HasAnyHooks() {
		t.Error("CreateJobFromTemplate() should have no lifecycle hooks for simple cronjob")
	}
}

func TestCronJobSpec_CreateJobFromTemplate_OnlyJobTemplateHooks(t *testing.T) {
	spec := CronJobSpec{
		Name:     "template-hooks-cronjob",
		Schedule: "0 0 * * *",
		JobTemplate: job.Spec{
			Name:    "template-job",
			Command: "echo 'job with hooks'",
			Lifecycle: process.LifecycleHooks{
				PreStart: []process.Hook{
					{Name: "template-setup", Command: "echo 'template setup'"},
				},
			},
		},
		// No lifecycle hooks at CronJob level
	}

	jobSpec := spec.CreateJobFromTemplate("template-job-instance")

	// Should have only JobTemplate hooks
	if len(jobSpec.Lifecycle.PreStart) != 1 {
		t.Errorf("CreateJobFromTemplate() PreStart hooks count = %d, want 1", len(jobSpec.Lifecycle.PreStart))
	}
	if jobSpec.Lifecycle.PreStart[0].Name != "template-setup" {
		t.Errorf("CreateJobFromTemplate() PreStart hook = %v, want template-setup", jobSpec.Lifecycle.PreStart[0].Name)
	}
}

func TestCronJobSpec_CreateJobFromTemplate_HookMerging(t *testing.T) {
	spec := CronJobSpec{
		Name:     "merging-cronjob",
		Schedule: "*/5 * * * *", // Every 5 minutes
		JobTemplate: job.Spec{
			Name:    "template-job",
			Command: "echo 'merging job'",
			Lifecycle: process.LifecycleHooks{
				PreStart: []process.Hook{
					{Name: "template-pre", Command: "echo 'template pre'"},
				},
				PostStart: []process.Hook{
					{Name: "template-post", Command: "echo 'template post'"},
				},
			},
		},
		Lifecycle: process.LifecycleHooks{
			PreStart: []process.Hook{
				{Name: "cronjob-pre", Command: "echo 'cronjob pre'"},
			},
			PreStop: []process.Hook{
				{Name: "cronjob-stop", Command: "echo 'cronjob stop'"},
			},
		},
	}

	jobSpec := spec.CreateJobFromTemplate("merged-job-instance")

	// Test PreStart merging (CronJob first, then JobTemplate)
	if len(jobSpec.Lifecycle.PreStart) != 2 {
		t.Errorf("CreateJobFromTemplate() PreStart hooks count = %d, want 2", len(jobSpec.Lifecycle.PreStart))
	}
	if jobSpec.Lifecycle.PreStart[0].Name != "cronjob-pre" {
		t.Errorf("CreateJobFromTemplate() first PreStart hook = %v, want cronjob-pre", jobSpec.Lifecycle.PreStart[0].Name)
	}
	if jobSpec.Lifecycle.PreStart[1].Name != "template-pre" {
		t.Errorf("CreateJobFromTemplate() second PreStart hook = %v, want template-pre", jobSpec.Lifecycle.PreStart[1].Name)
	}

	// Test PostStart (only from JobTemplate)
	if len(jobSpec.Lifecycle.PostStart) != 1 {
		t.Errorf("CreateJobFromTemplate() PostStart hooks count = %d, want 1", len(jobSpec.Lifecycle.PostStart))
	}
	if jobSpec.Lifecycle.PostStart[0].Name != "template-post" {
		t.Errorf("CreateJobFromTemplate() PostStart hook = %v, want template-post", jobSpec.Lifecycle.PostStart[0].Name)
	}

	// Test PreStop (only from CronJob)
	if len(jobSpec.Lifecycle.PreStop) != 1 {
		t.Errorf("CreateJobFromTemplate() PreStop hooks count = %d, want 1", len(jobSpec.Lifecycle.PreStop))
	}
	if jobSpec.Lifecycle.PreStop[0].Name != "cronjob-stop" {
		t.Errorf("CreateJobFromTemplate() PreStop hook = %v, want cronjob-stop", jobSpec.Lifecycle.PreStop[0].Name)
	}
}
