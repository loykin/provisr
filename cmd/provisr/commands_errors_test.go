package main

import (
	"testing"

	"github.com/loykin/provisr"
)

func TestCmdCronMissingConfig(t *testing.T) {
	mgr := provisr.New()
	if err := cmdCron(mgr, CronFlags{}); err == nil {
		t.Fatalf("expected error when --config is missing for cron")
	}
}

func TestGroupCommandsMissingFlags(t *testing.T) {
	mgr := provisr.New()
	// group-start without config
	if err := runGroupStart(mgr, GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-start")
	}
	// group-start without group
	if err := runGroupStart(mgr, GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-start")
	}
	// group-status
	if err := runGroupStatus(mgr, GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-status")
	}
	if err := runGroupStatus(mgr, GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-status")
	}
	// group-stop
	if err := runGroupStop(mgr, GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-stop")
	}
	if err := runGroupStop(mgr, GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-stop")
	}
}
