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

func TestSinkListFiltersByNameContains(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "history.db")
	sink, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []corehistory.Event{
		{Type: corehistory.EventStart, OccurredAt: base, Record: corehistory.Record{Name: "api-server", PID: 100, LastStatus: "running"}},
		{Type: corehistory.EventStart, OccurredAt: base.Add(time.Minute), Record: corehistory.Record{Name: "cron-clean", PID: 200, LastStatus: "running"}},
		{Type: corehistory.EventStart, OccurredAt: base.Add(2 * time.Minute), Record: corehistory.Record{Name: "worker_100%", PID: 300, LastStatus: "running"}},
	}
	for _, e := range events {
		if err := sink.Send(ctx, e); err != nil {
			t.Fatalf("Send() error: %v", err)
		}
	}

	filtered, err := sink.List(ctx, "server", 20, 0)
	if err != nil {
		t.Fatalf("List(server) error: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "api-server" {
		t.Fatalf("expected api-server contains match, got %+v", filtered)
	}

	total, err := sink.Count(ctx, "clean")
	if err != nil {
		t.Fatalf("Count(clean) error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected count 1 for clean contains match, got %d", total)
	}

	literal, err := sink.List(ctx, "_100%", 20, 0)
	if err != nil {
		t.Fatalf("List(_100%%) error: %v", err)
	}
	if len(literal) != 1 || literal[0].Name != "worker_100%" {
		t.Fatalf("expected literal wildcard characters to match worker_100%%, got %+v", literal)
	}
}

func TestNewRejectsEmptyDSN(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error for empty DSN")
	}
}

func TestNewWithOptionsCanSkipMigrations(t *testing.T) {
	sink, err := NewWithOptions(filepath.Join(t.TempDir(), "history.db"), Options{Migrate: false})
	if err != nil {
		t.Fatalf("NewWithOptions() error: %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	err = sink.Send(context.Background(), corehistory.Event{
		OccurredAt: time.Now(),
		Record:     corehistory.Record{Name: "svc"},
	})
	if err == nil {
		t.Fatal("Send() succeeded without a pre-migrated schema")
	}
}

func TestSinkPruneBefore(t *testing.T) {
	sink, err := New(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, occurredAt := range []time.Time{base, base.Add(24 * time.Hour)} {
		if err := sink.Send(ctx, corehistory.Event{OccurredAt: occurredAt, Record: corehistory.Record{Name: "svc"}}); err != nil {
			t.Fatalf("Send() error: %v", err)
		}
	}

	deleted, err := sink.PruneBefore(ctx, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("PruneBefore() error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	total, err := sink.Count(ctx, "")
	if err != nil || total != 1 {
		t.Fatalf("Count() = %d, %v; want 1, nil", total, err)
	}
}
