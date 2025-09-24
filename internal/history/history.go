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

// Sink is a destination for history events (analytics/statistics systems).
// Implementations must be safe for concurrent use.
type Sink interface {
	Send(ctx context.Context, e Event) error
}
