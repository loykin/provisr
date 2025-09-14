package history

import (
	"context"
	"time"

	"github.com/loykin/provisr/internal/store"
)

// EventType defines the kind of lifecycle event.
type EventType string

const (
	EventStart EventType = "start"
	EventStop  EventType = "stop"
)

// Event represents a lifecycle event to be exported to external systems.
type Event struct {
	Type       EventType    `json:"type"`
	OccurredAt time.Time    `json:"occurred_at"`
	Record     store.Record `json:"record"`
}

// Sink is a destination for history events (analytics/statistics systems).
// Implementations must be safe for concurrent use.
type Sink interface {
	Send(ctx context.Context, e Event) error
}
