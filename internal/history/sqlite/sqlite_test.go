package sqlite

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/store"
)

func TestSQLiteSink_Integration(t *testing.T) {
	// Create temporary database file
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	// Create sink
	sink, err := New("file:" + dbPath)
	if err != nil {
		t.Fatalf("Failed to create sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Errorf("Failed to close sink: %v", err)
		}
		_ = os.Remove(dbPath)
	}()

	ctx := context.Background()

	// Create test record
	testRecord := store.Record{
		Name:      "test-process",
		PID:       12345,
		StartedAt: time.Now().Add(-time.Minute).UTC(),
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

	t.Log("SQLite sink integration test completed successfully")
}

func TestSQLiteSink_InMemory(t *testing.T) {
	// Create in-memory sink
	sink, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Errorf("Failed to close sink: %v", err)
		}
	}()

	ctx := context.Background()

	// Create test record
	testRecord := store.Record{
		Name:      "mem-test-process",
		PID:       54321,
		StartedAt: time.Now().UTC(),
		Running:   true,
		Uniq:      "mem-test-unique-key",
	}

	event := history.Event{
		Type:       history.EventStart,
		OccurredAt: time.Now().UTC(),
		Record:     testRecord,
	}

	// Send event
	if err := sink.Send(ctx, event); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	t.Log("SQLite in-memory sink test completed successfully")
}

func TestSQLiteSink_ContextCancellation(t *testing.T) {
	// Create in-memory sink
	sink, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Errorf("Failed to close sink: %v", err)
		}
	}()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create test record
	testRecord := store.Record{
		Name:      "cancelled-process",
		PID:       99999,
		StartedAt: time.Now().UTC(),
		Running:   true,
		Uniq:      "cancelled-unique-key",
	}

	event := history.Event{
		Type:       history.EventStart,
		OccurredAt: time.Now().UTC(),
		Record:     testRecord,
	}

	// Send event with cancelled context - should handle gracefully
	err = sink.Send(ctx, event)
	if err != nil {
		t.Logf("Expected error with cancelled context: %v", err)
	}

	t.Log("SQLite context cancellation test completed")
}
