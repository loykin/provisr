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

func TestPostgresMinimalAPI(t *testing.T) {
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
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := db.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	// Record running
	now := time.Now().UTC()
	rec := store.Record{Name: "pgsvc", PID: 4321, LastStatus: "running", UpdatedAt: now}
	if err := db.Record(ctx, rec); err != nil {
		t.Fatalf("record running: %v", err)
	}
	got, err := db.GetByName(ctx, "pgsvc")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.PID != 4321 || got.LastStatus != "running" {
		t.Fatalf("unexpected record: %+v", got)
	}

	// Update to stopped
	rec2 := store.Record{Name: "pgsvc", PID: 4321, LastStatus: "stopped", UpdatedAt: time.Now().UTC()}
	if err := db.Record(ctx, rec2); err != nil {
		t.Fatalf("record stopped: %v", err)
	}
	got2, err := db.GetByName(ctx, "pgsvc")
	if err != nil {
		t.Fatalf("get by name2: %v", err)
	}
	if got2.LastStatus != "stopped" {
		t.Fatalf("expected stopped, got %q", got2.LastStatus)
	}

	// Delete and verify not found
	if err := db.Delete(ctx, "pgsvc"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetByName(ctx, "pgsvc"); err == nil {
		t.Fatalf("expected error after delete, got nil")
	}
}
