//go:build integration

package clickhouse

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"

	corehistory "github.com/loykin/provisr/core/history"
)

// TestSink_Integration_SendAndList proves the Sink actually works against a
// real ClickHouse server over database/sql (via dbstore's Pool[*sqlx.DB]),
// not just that it builds. ClickHouse was hypothesized to be a "non-SQL"
// backend requiring the generic Pool[T] work, but its official driver
// registers a real database/sql driver — this test is the concrete proof
// that Pool[*sqlx.DB] (unchanged) is sufficient.
func TestSink_Integration_SendAndList(t *testing.T) {
	ctx := context.Background()

	ctr, err := tcclickhouse.Run(ctx, "clickhouse/clickhouse-server:24.3")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.ConnectionHost(ctx)
	require.NoError(t, err)

	dsn := fmt.Sprintf("clickhouse://%s:%s@%s/%s", ctr.User, ctr.Password, host, ctr.DbName)

	sink, err := New(dsn, "process_history")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sink.Close() })

	// Schema ownership is the caller's — create the table the sink expects.
	require.NoError(t, sink.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE process_history (
				type String,
				occurred_at DateTime,
				record_name String,
				record_pid Int32
			) ENGINE = MergeTree() ORDER BY occurred_at`)
		return err
	}))

	occurredAt := time.Now().UTC().Truncate(time.Second)
	event := corehistory.Event{
		Type:       corehistory.EventStart,
		OccurredAt: occurredAt,
		Record:     corehistory.Record{Name: "svc-a", PID: 123, LastStatus: "running"},
	}
	require.NoError(t, sink.Send(ctx, event))
	require.NoError(t, sink.Send(ctx, corehistory.Event{
		Type:       corehistory.EventStop,
		OccurredAt: occurredAt.Add(time.Minute),
		Record:     corehistory.Record{Name: "svc-b", PID: 456, LastStatus: "stopped"},
	}))

	all, err := sink.List(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, all, 2)

	filtered, err := sink.List(ctx, "svc-a", 10)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "svc-a", filtered[0].Name)
	require.Equal(t, 123, filtered[0].PID)
	require.Equal(t, "start", filtered[0].Type)
}
