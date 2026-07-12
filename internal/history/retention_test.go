package history

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordingPruner struct {
	mu      sync.Mutex
	cutoffs []time.Time
}

func (p *recordingPruner) PruneBefore(_ context.Context, cutoff time.Time) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cutoffs = append(p.cutoffs, cutoff)
	return 1, nil
}

func TestStartRetentionRunsImmediatelyAndPeriodically(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pruner := &recordingPruner{}
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	results := make(chan int64, 2)

	go StartRetention(ctx, pruner, 24*time.Hour, time.Millisecond, func() time.Time { return now }, func(deleted int64, err error) {
		if err != nil {
			t.Errorf("cleanup error: %v", err)
		}
		results <- deleted
	})

	for i := 0; i < 2; i++ {
		select {
		case deleted := <-results:
			if deleted != 1 {
				t.Fatalf("deleted = %d, want 1", deleted)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for cleanup")
		}
	}
	cancel()

	pruner.mu.Lock()
	defer pruner.mu.Unlock()
	for _, cutoff := range pruner.cutoffs {
		if want := now.Add(-24 * time.Hour); !cutoff.Equal(want) {
			t.Fatalf("cutoff = %v, want %v", cutoff, want)
		}
	}
}

func TestStartRetentionDisabled(t *testing.T) {
	pruner := &recordingPruner{}
	StartRetention(context.Background(), pruner, 0, time.Millisecond, nil, nil)
	if len(pruner.cutoffs) != 0 {
		t.Fatalf("cleanup ran with retention disabled")
	}
}
