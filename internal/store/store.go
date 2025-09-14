package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Record represents a single observation/state of a process instance.
// Unique identity is derived from PID + StartedAt to disambiguate PID reuse.
//
// Uniq is a stable unique key computed by UniqueKey(pid, startedAt).
// Drivers must enforce uniqueness on Uniq.
//
// ExitErr stores a textual representation of the last error (if any) when the
// process exited; empty means clean exit or still running.
//
// Timestamps should be UTC.
// Running indicates whether the process is considered currently alive.
// For consecutive writes on the same Uniq, the latest row overwrites previous.

type Record struct {
	ID        int64
	Name      string
	PID       int
	StartedAt time.Time
	StoppedAt sql.NullTime
	Running   bool
	ExitErr   sql.NullString
	Uniq      string
	UpdatedAt time.Time
}

// UniqueKey returns a persistent unique key based on pid and start time.
// Format: pid-<unixNano>. The caller should use UTC for start.
func UniqueKey(pid int, startedAt time.Time) string {
	return fmt.Sprintf("%d-%d", pid, startedAt.UTC().UnixNano())
}

// Key returns the unique key of the record (computed if empty).
func (r *Record) Key() string {
	if r.Uniq != "" {
		return r.Uniq
	}
	r.Uniq = UniqueKey(r.PID, r.StartedAt)
	return r.Uniq
}

// Store is a pluggable persistence interface for process state.
// Implementations must be safe for concurrent use by multiple goroutines.
//
// Typical flow:
//   - RecordStart on successful start
//   - RecordStop when the process has stopped
//   - UpsertStatus for periodic refreshes, if desired
//
// Store should be resilient to duplicate RecordStart calls for the same Uniq.

type Store interface {
	EnsureSchema(ctx context.Context) error
	RecordStart(ctx context.Context, rec Record) error
	RecordStop(ctx context.Context, uniq string, stoppedAt time.Time, exitErr error) error
	UpsertStatus(ctx context.Context, rec Record) error
	GetByName(ctx context.Context, name string, limit int) ([]Record, error)
	GetRunning(ctx context.Context, namePrefix string) ([]Record, error)
	PurgeOlderThan(ctx context.Context, olderThan time.Time) (int64, error)
	Close() error
}
