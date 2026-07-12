// Package history exposes the Sink interface and event types needed to implement
// or consume history backends. External modules (e.g. provisr/history/clickhouse)
// import this package to satisfy the Sink contract without depending on internal/.
package history

import (
	"context"
	"time"
)

// EventType defines the kind of lifecycle event.
type EventType string

const (
	EventStart EventType = "start"
	EventStop  EventType = "stop"
)

// Record is a minimal process record used for history events.
type Record struct {
	Name       string    `json:"name"`
	PID        int       `json:"pid"`
	LastStatus string    `json:"last_status"`
	UpdatedAt  time.Time `json:"updated_at"`
	SpecJSON   string    `json:"spec_json"`
}

// Event represents a lifecycle event to be exported to external systems.
type Event struct {
	Type       EventType `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	Record     Record    `json:"record"`
}

// Sink is a destination for history events.
// Implementations must be safe for concurrent use.
type Sink interface {
	Send(ctx context.Context, e Event) error
}

// Entry is the backend-neutral representation returned by history readers.
// Storage adapters may keep additional internal fields, but transports must
// depend on this contract rather than a concrete database record type.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	PID       int       `json:"pid"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Error     *string   `json:"error,omitempty"`
}

// Reader provides paginated access to stored process lifecycle history.
// Implementations may be backed by SQLite, PostgreSQL, OpenSearch, or any
// other adapter with equivalent query semantics.
type Reader interface {
	List(ctx context.Context, name string, limit, offset int) ([]Entry, error)
	Count(ctx context.Context, name string) (int, error)
}

// Pruner deletes history entries older than a cutoff. Persistent history
// adapters implement this contract so retention remains storage-agnostic.
type Pruner interface {
	PruneBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
