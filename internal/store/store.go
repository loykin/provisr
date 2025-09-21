package store

import (
	"context"
	"time"
)

// Record is the minimal unit of state we persist for a managed process.
// Name is unique across all managed processes.
// LastStatus is an arbitrary string like "starting", "running", "stopped".
// UpdatedAt should be in UTC.
// PID is the latest observed PID for the named process.
// This is intentionally minimal to support PID recovery on restart.

type Record struct {
	Name       string
	PID        int
	LastStatus string
	UpdatedAt  time.Time
}

// Store is a minimal persistence interface to keep last known PID and status
// for a uniquely named managed process.

type Store interface {
	EnsureSchema(ctx context.Context) error
	Record(ctx context.Context, rec Record) error
	GetByName(ctx context.Context, name string) (Record, error)
	Delete(ctx context.Context, name string) error
	Close() error
}
