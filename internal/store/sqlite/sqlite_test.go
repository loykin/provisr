package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/store"
)

func TestSQLiteStoreLifecycleAndQueries(t *testing.T) {
	// Open in-memory DB and ensure schema twice (idempotent)
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := db.EnsureSchema(ctx); err != nil { // idempotent
		t.Fatalf("ensure schema 2: %v", err)
	}

	// Record a start
	start := time.Now().Add(-2 * time.Second).UTC()
	rec := store.Record{Name: "svc", PID: 1111, StartedAt: start}
	if err := db.RecordStart(ctx, rec); err != nil {
		t.Fatalf("record start: %v", err)
	}

	// It should appear in GetRunning and GetByName
	runs, err := db.GetRunning(ctx, "svc")
	if err != nil {
		t.Fatalf("get running: %v", err)
	}
	if len(runs) != 1 || !runs[0].Running || runs[0].Name != "svc" {
		t.Fatalf("unexpected running rows: %+v", runs)
	}
	hist, err := db.GetByName(ctx, "svc", 10)
	if err != nil || len(hist) < 1 {
		t.Fatalf("get by name: %v len=%d", err, len(hist))
	}

	// Stop it with an error string
	uniq := rec.Key()
	stoppedAt := time.Now().UTC()
	stopErr := sql.ErrNoRows // arbitrary error to capture string
	if err := db.RecordStop(ctx, uniq, stoppedAt, stopErr); err != nil {
		t.Fatalf("record stop: %v", err)
	}

	// Now there should be no running rows, and history should reflect stoppedAt+exit_err
	runs2, err := db.GetRunning(ctx, "svc")
	if err != nil {
		t.Fatalf("get running2: %v", err)
	}
	if len(runs2) != 0 {
		t.Fatalf("expected 0 running after stop, got %d", len(runs2))
	}
	hist2, err := db.GetByName(ctx, "svc", 10)
	if err != nil || len(hist2) < 1 {
		t.Fatalf("get by name2: %v len=%d", err, len(hist2))
	}
	if hist2[0].Running {
		t.Fatalf("expected latest not running: %+v", hist2[0])
	}
	if !hist2[0].StoppedAt.Valid {
		t.Fatalf("expected stopped_at to be set")
	}
	if !hist2[0].ExitErr.Valid || hist2[0].ExitErr.String == "" {
		t.Fatalf("expected exit_err string to be set")
	}

	// Upsert a new status with different pid/start and Running=true
	start2 := time.Now().UTC()
	rec2 := store.Record{Name: "svc", PID: 2222, StartedAt: start2, Running: true}
	if err := db.UpsertStatus(ctx, rec2); err != nil {
		t.Fatalf("upsert status: %v", err)
	}
	// Prefix filtering: use just "s" to match
	runs3, err := db.GetRunning(ctx, "s")
	if err != nil {
		t.Fatalf("get running3: %v", err)
	}
	if len(runs3) < 1 {
		t.Fatalf("expected running rows after upsert")
	}
	// Ensure returned record corresponds to the new uniq
	if runs3[0].PID != 2222 || !runs3[0].Running {
		t.Fatalf("unexpected running rec after upsert: %+v", runs3[0])
	}

	// Purge should delete only non-running records older than the threshold.
	// Use a future threshold so the previous stopped row (updated earlier) is purged.
	// The latest running row must not be purged.
	deleted, err := db.PurgeOlderThan(ctx, time.Now().Add(1*time.Second))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted < 1 {
		t.Fatalf("expected at least 1 row purged, got %d", deleted)
	}
	// Still 1 running
	runs4, _ := db.GetRunning(ctx, "svc")
	if len(runs4) != 1 {
		t.Fatalf("expected 1 running after purge, got %d", len(runs4))
	}
}

func TestSQLiteHistoryToggle(t *testing.T) {
	t.Skip("history moved to internal/history; store drivers no longer handle history")
}
