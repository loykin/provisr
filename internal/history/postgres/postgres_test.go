package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/store"
)

func TestPostgresSink_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Errorf("Failed to terminate PostgreSQL container: %v", err)
		}
	}()

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Create sink
	sink, err := New(connStr)
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Errorf("Failed to close sink: %v", err)
		}
	}()

	// Test event sending
	testRecord := store.Record{
		Name:      "test-process",
		PID:       12345,
		StartedAt: time.Now().UTC(),
		Running:   true,
		Uniq:      "test-unique-key",
	}

	startEvent := history.Event{
		Type:       history.EventStart,
		OccurredAt: time.Now().UTC(),
		Record:     testRecord,
	}

	// Send start event
	if err := sink.Send(ctx, startEvent); err != nil {
		t.Fatalf("Failed to send start event: %v", err)
	}

	// Test stop event
	stopTime := time.Now().UTC()
	testRecord.Running = false
	testRecord.StoppedAt.Time = stopTime
	testRecord.StoppedAt.Valid = true

	stopEvent := history.Event{
		Type:       history.EventStop,
		OccurredAt: stopTime,
		Record:     testRecord,
	}

	// Send stop event
	if err := sink.Send(ctx, stopEvent); err != nil {
		t.Fatalf("Failed to send stop event: %v", err)
	}

	// Verify events were stored
	rows, err := sink.db.QueryContext(ctx, "SELECT COUNT(*) FROM process_history WHERE name = $1", testRecord.Name)
	if err != nil {
		t.Fatalf("Failed to query process_history: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var count int
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			t.Fatalf("Failed to scan count: %v", err)
		}
	}

	if count != 2 {
		t.Errorf("Expected 2 events in history, got %d", count)
	}
}
