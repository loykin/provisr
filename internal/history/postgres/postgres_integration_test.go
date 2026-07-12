//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	corehistory "github.com/loykin/provisr/core/history"
)

// TestSinkSendAndList mirrors the sqlite package's unit test, but against a
// real PostgreSQL server via testcontainers — proving the dbstore-backed
// Sink (goose migration + Pool[*sqlx.DB]) actually works on Postgres, not
// just SQLite.
func TestSinkSendAndList(t *testing.T) {
	ctx := context.Background()

	// The postgres image logs "database system is ready to accept
	// connections" once for its own initdb bootstrap (a temporary server
	// that then shuts down) and again for the real server — waiting for
	// only the first occurrence (the module's default) can let the test
	// connect during that shutdown window, seen as a flaky "connection
	// reset by peer" in CI.
	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("user"),
		tcpostgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
				wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(60*time.Second),
			),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	sink, err := New(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sink.Close() })

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []corehistory.Event{
		{Type: corehistory.EventStart, OccurredAt: base, Record: corehistory.Record{Name: "svc-a", PID: 100, LastStatus: "running"}},
		{Type: corehistory.EventStop, OccurredAt: base.Add(time.Minute), Record: corehistory.Record{Name: "svc-a", PID: 100, LastStatus: "stopped"}},
		{Type: corehistory.EventStart, OccurredAt: base.Add(2 * time.Minute), Record: corehistory.Record{Name: "svc-b", PID: 200, LastStatus: "running"}},
	}
	for _, e := range events {
		require.NoError(t, sink.Send(ctx, e))
	}

	all, err := sink.List(ctx, "", 0, 0)
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, "svc-b", all[0].Name, "expected newest row first")

	filtered, err := sink.List(ctx, "svc-a", 0, 0)
	require.NoError(t, err)
	require.Len(t, filtered, 2)
	require.Equal(t, "stopped", filtered[0].Status)
	require.Equal(t, "running", filtered[1].Status)
}
