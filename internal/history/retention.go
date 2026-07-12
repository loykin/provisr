package history

import (
	"context"
	"time"

	corehistory "github.com/loykin/provisr/core/history"
)

const defaultCleanupInterval = time.Hour

// StartRetention runs cleanup immediately and then periodically until ctx is
// canceled. A non-positive retention disables cleanup.
func StartRetention(
	ctx context.Context,
	pruner corehistory.Pruner,
	retention time.Duration,
	interval time.Duration,
	now func() time.Time,
	onResult func(deleted int64, err error),
) {
	if pruner == nil || retention <= 0 {
		return
	}
	if interval <= 0 {
		interval = defaultCleanupInterval
	}
	if now == nil {
		now = time.Now
	}

	cleanup := func() {
		deleted, err := pruner.PruneBefore(ctx, now().Add(-retention))
		if onResult != nil {
			onResult(deleted, err)
		}
	}
	cleanup()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}
