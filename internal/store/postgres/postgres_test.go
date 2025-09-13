package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/store"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// startPostgresContainer starts a PostgreSQL container for tests
// and returns a DSN suitable for pgx stdlib. It skips the test if Docker is unavailable.
func startPostgresContainer(t *testing.T) (dsn string, terminate func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	if err != nil {
		cancel()
		t.Skipf("Failed to start PostgreSQL container: %v", err)
		return "", nil // ensure container is never used below
	}

	// container is guaranteed to be non-nil here
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		cancel()
		t.Skipf("Failed to get host info: %v", err)
		return "", nil
	}

	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		cancel()
		t.Skipf("Failed to get mapped port: %v", err)
		return "", nil
	}

	dsn = fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

	terminate = func() {
		_ = container.Terminate(ctx)
		cancel()
	}

	return dsn, terminate
}

func waitForPostgres(t *testing.T, dsn string) {
	// Try to ping until timeout; helps when container reports ready but DB not yet accepting connections
	deadline := time.Now().Add(45 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		db, err := sql.Open("pgx", dsn)
		if err == nil {
			if err = db.PingContext(ctx); err == nil {
				_ = db.Close()
				cancel()
				return
			}
			_ = db.Close()
		}
		cancel()
		if time.Now().After(deadline) {
			t.Fatalf("postgres not ready in time: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func TestPostgresStoreLifecycleAndQueries(t *testing.T) {
	dsn, terminate := startPostgresContainer(t)
	// Ensure DB is ready to accept connections
	waitForPostgres(t, dsn)
	defer func() {
		if terminate != nil {
			terminate()
		}
	}()

	db, err := New(dsn)
	if err != nil {
		t.Fatalf("pg open: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := db.EnsureSchema(ctx); err != nil { // idempotent
		t.Fatalf("ensure schema 2: %v", err)
	}

	// Record a start
	start := time.Now().Add(-2 * time.Second).UTC()
	rec := store.Record{Name: "pgsvc", PID: 4321, StartedAt: start}
	if err := db.RecordStart(ctx, rec); err != nil {
		t.Fatalf("record start: %v", err)
	}

	// It should appear in GetRunning and GetByName
	runs, err := db.GetRunning(ctx, "pgsvc")
	if err != nil {
		t.Fatalf("get running: %v", err)
	}
	if len(runs) != 1 || !runs[0].Running || runs[0].Name != "pgsvc" {
		t.Fatalf("unexpected running rows: %+v", runs)
	}
	hist, err := db.GetByName(ctx, "pgsvc", 10)
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
	runs2, err := db.GetRunning(ctx, "pgsvc")
	if err != nil {
		t.Fatalf("get running2: %v", err)
	}
	if len(runs2) != 0 {
		t.Fatalf("expected 0 running after stop, got %d", len(runs2))
	}
	hist2, err := db.GetByName(ctx, "pgsvc", 10)
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
	rec2 := store.Record{Name: "pgsvc", PID: 5555, StartedAt: start2, Running: true}
	if err := db.UpsertStatus(ctx, rec2); err != nil {
		t.Fatalf("upsert status: %v", err)
	}
	// Prefix filtering: use just "pg" to match
	runs3, err := db.GetRunning(ctx, "pg")
	if err != nil {
		t.Fatalf("get running3: %v", err)
	}
	if len(runs3) < 1 {
		t.Fatalf("expected running rows after upsert")
	}
	if runs3[0].PID != 5555 || !runs3[0].Running {
		t.Fatalf("unexpected running rec after upsert: %+v", runs3[0])
	}

	// Purge should delete only non-running records older than the threshold.
	deleted, err := db.PurgeOlderThan(ctx, time.Now().Add(1*time.Second))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted < 1 {
		t.Fatalf("expected at least 1 row purged, got %d", deleted)
	}
	// Still 1 running
	runs4, _ := db.GetRunning(ctx, "pgsvc")
	if len(runs4) != 1 {
		t.Fatalf("expected 1 running after purge, got %d", len(runs4))
	}
}
