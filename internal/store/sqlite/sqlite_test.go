package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/store"
)

func TestSQLiteMinimalAPI(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := db.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	// Record running
	now := time.Now().UTC()
	rec := store.Record{Name: "svc", PID: 1111, LastStatus: "running", UpdatedAt: now}
	if err := db.Record(ctx, rec); err != nil {
		t.Fatalf("record running: %v", err)
	}
	// Verify
	got, err := db.GetByName(ctx, "svc")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.PID != 1111 || got.LastStatus != "running" {
		t.Fatalf("unexpected record: %+v", got)
	}

	// Update to stopped
	rec2 := store.Record{Name: "svc", PID: 1111, LastStatus: "stopped", UpdatedAt: time.Now().UTC()}
	if err := db.Record(ctx, rec2); err != nil {
		t.Fatalf("record stopped: %v", err)
	}
	got2, err := db.GetByName(ctx, "svc")
	if err != nil {
		t.Fatalf("get by name2: %v", err)
	}
	if got2.LastStatus != "stopped" {
		t.Fatalf("expected stopped, got %q", got2.LastStatus)
	}

	// Delete and verify not found
	if err := db.Delete(ctx, "svc"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetByName(ctx, "svc"); err == nil {
		t.Fatalf("expected error after delete, got nil")
	}
}
