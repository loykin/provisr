//go:build integration

package opensearch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcopensearch "github.com/testcontainers/testcontainers-go/modules/opensearch"

	corehistory "github.com/loykin/provisr/core/history"
)

// TestSink_Integration_SendAndList proves the dbstore-backed Sink actually
// works against a real OpenSearch server (Pool[*opensearchapi.Client]), not
// just that it constructs. This is the provisr-side counterpart to
// dbstore's own opensearch_repo_test.go, exercised against provisr's real
// history.Sink implementation instead of a throwaway test repo.
func TestSink_Integration_SendAndList(t *testing.T) {
	ctx := context.Background()

	ctr, err := tcopensearch.Run(ctx, "opensearchproject/opensearch:2.11.1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	addr, err := ctr.Address(ctx)
	require.NoError(t, err)

	sink, err := NewWithOptions(addr, "process_history", Options{Migrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sink.Close() })

	occurredAt := time.Now().UTC().Truncate(time.Millisecond)
	require.NoError(t, sink.Send(ctx, corehistory.Event{
		Type:       corehistory.EventStart,
		OccurredAt: occurredAt,
		Record:     corehistory.Record{Name: "svc-a", PID: 123, LastStatus: "running"},
	}))
	require.NoError(t, sink.Send(ctx, corehistory.Event{
		Type:       corehistory.EventStop,
		OccurredAt: occurredAt.Add(time.Minute),
		Record:     corehistory.Record{Name: "svc-b", PID: 456, LastStatus: "stopped"},
	}))

	// OpenSearch indexing is near-real-time: a freshly created document
	// isn't guaranteed to be visible to _search until the index refreshes.
	require.Eventually(t, func() bool {
		all, err := sink.List(ctx, "", 10, 0)
		return err == nil && len(all) == 2
	}, 10*time.Second, 200*time.Millisecond, "expected both events to become searchable")

	filtered, err := sink.List(ctx, "svc-a", 10, 0)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "svc-a", filtered[0].Name)
	require.Equal(t, 123, filtered[0].PID)
	require.Equal(t, "running", filtered[0].Status)
	total, err := sink.Count(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 2, total)
	deleted, err := sink.PruneBefore(ctx, occurredAt.Add(30*time.Second))
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	require.Eventually(t, func() bool {
		count, err := sink.Count(ctx, "")
		return err == nil && count == 1
	}, 10*time.Second, 200*time.Millisecond, "expected retention delete to become visible")
}
