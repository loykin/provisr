package cron

import (
	"testing"
	"time"

	mgrpkg "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
)

func TestParseEvery(t *testing.T) {
	if _, err := parseEvery("@every 100ms"); err != nil {
		t.Fatalf("parse every: %v", err)
	}
	if _, err := parseEvery("* * * * *"); err == nil {
		t.Fatalf("expected error for unsupported cron expr")
	}
}

func TestSchedulerRunsAndNonOverlap(t *testing.T) {
	mgr := mgrpkg.NewManager()
	sch := NewScheduler(mgr)
	job := Job{
		Name:     "j1",
		Spec:     process.Spec{Name: "cron-1", Command: "sleep 0.2", AutoRestart: false},
		Schedule: "@every 100ms",
		// Singleton default true -> no overlap
	}
	if err := sch.Add(&job); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := sch.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer sch.Stop()
	// Wait some time; since job runs for 200ms and ticks every 100ms, with singleton=true
	// we expect around 1 running instance at any time, not multiple. Ensure at least one start happened.
	deadline := time.Now().Add(700 * time.Millisecond)
	var started bool
	for time.Now().Before(deadline) {
		st, err := mgr.Status("cron-1")
		if err == nil && (st.Running || st.StoppedAt.After(st.StartedAt)) {
			started = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !started {
		t.Fatalf("expected cron job to start at least once")
	}
}

func TestSchedulerRejectsAutoRestartAndInstances(t *testing.T) {
	mgr := mgrpkg.NewManager()
	sch := NewScheduler(mgr)
	if err := sch.Add(&Job{Name: "bad1", Spec: process.Spec{Name: "x", Command: "true", AutoRestart: true}, Schedule: "@every 1s"}); err == nil {
		t.Fatalf("expected error for autorestart=true")
	}
	if err := sch.Add(&Job{Name: "bad2", Spec: process.Spec{Name: "y", Command: "true", Instances: 2}, Schedule: "@every 1s"}); err == nil {
		t.Fatalf("expected error for instances>1")
	}
}

func TestParseEveryInvalid(t *testing.T) {
	if _, err := parseEvery("every 1s"); err == nil { // missing '@'
		t.Fatalf("expected error for bad format")
	}
	if _, err := parseEvery("@every -1s"); err == nil { // non-positive
		t.Fatalf("expected error for non-positive duration")
	}
}

func TestSchedulerAddValidation(t *testing.T) {
	mgr := mgrpkg.NewManager()
	s := NewScheduler(mgr)

	// empty name
	j := &Job{Name: "", Spec: process.Spec{Command: "true"}, Schedule: "@every 1s"}
	if err := s.Add(j); err == nil {
		t.Fatalf("expected error for empty job name")
	}
	// empty schedule
	j = &Job{Name: "a", Spec: process.Spec{Command: "true"}, Schedule: ""}
	if err := s.Add(j); err == nil {
		t.Fatalf("expected error for empty schedule")
	}
	// instances > 1
	j = &Job{Name: "b", Spec: process.Spec{Command: "true", Instances: 2}, Schedule: "@every 1s"}
	if err := s.Add(j); err == nil {
		t.Fatalf("expected error for instances>1")
	}
	// autorestart true
	j = &Job{Name: "c", Spec: process.Spec{Command: "true", AutoRestart: true}, Schedule: "@every 1s"}
	if err := s.Add(j); err == nil {
		t.Fatalf("expected error for autorestart true")
	}

	// valid job; Singleton defaults to true when false is passed
	j = &Job{Name: "ok", Spec: process.Spec{Command: "true"}, Schedule: "@every 1s", Singleton: false}
	if err := s.Add(j); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.jobs[len(s.jobs)-1].Singleton {
		t.Fatalf("expected Singleton defaulted to true")
	}

	// Start/Stop with invalid schedule string on run: ensure Start returns error when parse fails
	ss := NewScheduler(mgr)
	bad := &Job{Name: "bad", Spec: process.Spec{Command: "true"}, Schedule: "not@every"}
	_ = ss.Add(bad)
	if err := ss.Start(); err == nil {
		t.Fatalf("expected error on Start for invalid schedule")
	}

	// start valid and stop (no running jobs, just ensure no panic)
	sv := NewScheduler(mgr)
	good := &Job{Name: "good", Spec: process.Spec{Name: "g", Command: "sleep 0.01", StartDuration: 0}, Schedule: "@every 10ms"}
	if err := sv.Add(good); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := sv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	sv.Stop()
}
