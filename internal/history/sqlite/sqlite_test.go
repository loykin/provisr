package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	corehistory "github.com/loykin/provisr/core/history"
)

func TestSinkSendAndList(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "history.db")
	sink, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	events := []corehistory.Event{
		{Type: corehistory.EventStart, OccurredAt: base, Record: corehistory.Record{Name: "svc-a", PID: 100, LastStatus: "running"}},
		{Type: corehistory.EventStop, OccurredAt: base.Add(time.Minute), Record: corehistory.Record{Name: "svc-a", PID: 100, LastStatus: "stopped"}},
		{Type: corehistory.EventStart, OccurredAt: base.Add(2 * time.Minute), Record: corehistory.Record{Name: "svc-b", PID: 200, LastStatus: "running"}},
	}
	for _, e := range events {
		if err := sink.Send(ctx, e); err != nil {
			t.Fatalf("Send() error: %v", err)
		}
	}

	all, err := sink.List(ctx, "", 0, 0)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(all))
	}
	// newest first
	if all[0].Name != "svc-b" || all[0].Status != "running" {
		t.Errorf("expected newest row to be svc-b/running, got %+v", all[0])
	}

	filtered, err := sink.List(ctx, "svc-a", 0, 0)
	if err != nil {
		t.Fatalf("List(svc-a) error: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 rows for svc-a, got %d", len(filtered))
	}
	if filtered[0].Status != "stopped" || filtered[1].Status != "running" {
		t.Errorf("unexpected order/status for svc-a rows: %+v", filtered)
	}
}

func TestNewRejectsEmptyDSN(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error for empty DSN")
	}
}
