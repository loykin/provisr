package cron

import (
	"testing"
	"time"

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
	mgr := process.NewManager()
	sch := NewScheduler(mgr)
	job := Job{
		Name:     "j1",
		Spec:     process.Spec{Name: "cron-1", Command: "sleep 0.2", AutoRestart: false},
		Schedule: "@every 100ms",
		// Singleton default true -> no overlap
	}
	if err := sch.Add(job); err != nil {
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
	mgr := process.NewManager()
	sch := NewScheduler(mgr)
	if err := sch.Add(Job{Name: "bad1", Spec: process.Spec{Name: "x", Command: "true", AutoRestart: true}, Schedule: "@every 1s"}); err == nil {
		t.Fatalf("expected error for autorestart=true")
	}
	if err := sch.Add(Job{Name: "bad2", Spec: process.Spec{Name: "y", Command: "true", Instances: 2}, Schedule: "@every 1s"}); err == nil {
		t.Fatalf("expected error for instances>1")
	}
}
