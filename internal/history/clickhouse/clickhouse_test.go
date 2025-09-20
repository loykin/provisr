package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/clickhouse"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/store"
)

// setupClickHouseContainer starts a ClickHouse container for testing
func setupClickHouseContainer(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
	t.Helper()

	// Start ClickHouse container
	clickHouseContainer, err := clickhouse.Run(ctx,
		"clickhouse/clickhouse-server:24.3.2.23",
		clickhouse.WithUsername("default"),
		clickhouse.WithPassword(""),
		clickhouse.WithDatabase("default"),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/ping").
				WithPort("8123/tcp").
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start ClickHouse container: %v", err)
	}

	// Get connection details
	host, err := clickHouseContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := clickHouseContainer.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}

	dsn := host + ":" + port.Port()
	return clickHouseContainer, dsn
}

// setupSinkWithTable creates a sink and sets up the test table
func setupSinkWithTable(ctx context.Context, t *testing.T, dsn string, tableName string) *Sink {
	t.Helper()

	// Create sink
	sink, err := New(dsn, tableName)
	if err != nil {
		t.Fatalf("Failed to create sink: %v", err)
	}

	// Create the table
	err = sink.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS `+tableName+` (
			type String,
			occurred_at DateTime64(6),
			record_name String,
			record_pid UInt32,
			record_started_at DateTime64(6),
			record_stopped_at Nullable(DateTime64(6)),
			record_running Bool,
			record_exit_err Nullable(String),
			record_uniq String
		) ENGINE = MergeTree()
		ORDER BY (occurred_at, record_uniq)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	return sink
}

func TestClickHouseSink_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup ClickHouse container
	clickHouseContainer, dsn := setupClickHouseContainer(ctx, t)
	defer func() {
		if err := clickHouseContainer.Terminate(ctx); err != nil {
			t.Errorf("Failed to terminate ClickHouse container: %v", err)
		}
	}()

	// Setup sink with table
	sink := setupSinkWithTable(ctx, t, dsn, "process_history")
	defer func() {
		if err := sink.Close(); err != nil {
			t.Errorf("Failed to close sink: %v", err)
		}
	}()

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

	// Wait a moment for data to be written
	time.Sleep(100 * time.Millisecond)

	// Verify data was written
	rows := sink.conn.QueryRow(ctx, "SELECT COUNT(*) FROM process_history WHERE record_uniq = ?", testRecord.Uniq)
	var count uint64
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 events, got %d", count)
	}

	t.Log("ClickHouse sink integration test completed successfully")
}

func TestClickHouseSink_ConnectionError(t *testing.T) {
	// Test with invalid connection
	_, err := New("invalid-host:9000", "test_table")
	if err == nil {
		t.Error("Expected error with invalid connection, got nil")
	}
}

func TestClickHouseSink_Send_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup ClickHouse container
	clickHouseContainer, dsn := setupClickHouseContainer(ctx, t)
	defer func() {
		if err := clickHouseContainer.Terminate(ctx); err != nil {
			t.Errorf("Failed to terminate ClickHouse container: %v", err)
		}
	}()

	// Setup sink with table
	sink := setupSinkWithTable(ctx, t, dsn, "process_history")
	defer func() {
		if err := sink.Close(); err != nil {
			t.Errorf("Failed to close sink: %v", err)
		}
	}()

	// Create cancelled context
	cancelCtx, cancel := context.WithCancel(ctx)
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
	err := sink.Send(cancelCtx, event)
	if err != nil {
		t.Logf("Expected error with cancelled context: %v", err)
	}

	t.Log("ClickHouse context cancellation test completed")
}
