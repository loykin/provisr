package job

import (
	"strings"
	"testing"

	"github.com/loykin/provisr/core/internal/manager"
)

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
