package job

import (
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/core/internal/manager"
)

func TestParallelJobCompletes(t *testing.T) {
	processes := manager.NewManager()
	jobs := NewManager(processes)
	t.Cleanup(func() { _ = jobs.Shutdown() })

	parallelism := int32(2)
	completions := int32(2)
	j, err := jobs.CreateJob(Spec{
		Name:          "parallel-completion",
		Command:       "go version",
		Parallelism:   &parallelism,
		Completions:   &completions,
		RestartPolicy: string(RestartPolicyNever),
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-j.Done():
		status := j.GetStatus()
		if status.Phase != JobPhaseSucceeded || status.Succeeded != completions {
			t.Fatalf("job status = %+v, want %s with %d successes", status, JobPhaseSucceeded, completions)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("parallel job did not complete: %+v", j.GetStatus())
	}
}

func TestUpdateJobValidationFailurePreservesOriginal(t *testing.T) {
	jobs := NewManager(manager.NewManager())
	original := Spec{Name: "keep-me", Command: "sleep 30"}
	if _, err := jobs.CreateJob(original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = jobs.Shutdown() })

	err := jobs.UpdateJob(original.Name, Spec{Name: original.Name})
	if err == nil || !strings.Contains(err.Error(), "requires command or args") {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	got, ok := jobs.GetJob(original.Name)
	if !ok || got.GetSpec().Command != original.Command {
		t.Fatalf("original job was not preserved: ok=%v", ok)
	}
}

func TestUpdateJobMissingDependencyPreservesOriginal(t *testing.T) {
	jobs := NewManager(manager.NewManager())
	original := Spec{Name: "keep-me", Command: "sleep 30"}
	if _, err := jobs.CreateJob(original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = jobs.Shutdown() })

	updated := Spec{Name: original.Name, Command: "echo updated", DependsOn: []string{"missing"}}
	if err := jobs.UpdateJob(original.Name, updated); err == nil || !strings.Contains(err.Error(), "dependency") {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	got, ok := jobs.GetJob(original.Name)
	if !ok || got.GetSpec().Command != original.Command {
		t.Fatalf("original job was not preserved: ok=%v", ok)
	}
}
