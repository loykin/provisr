// Package history re-exports the Sink interface and event types from
// github.com/loykin/provisr/core/history. It exists for backward
// compatibility — new code should import the core/history package directly.
package history

import corehistory "github.com/loykin/provisr/core/history"

// Type aliases — identical to the types in core/history; no conversion needed.
type (
	Sink      = corehistory.Sink
	Event     = corehistory.Event
	EventType = corehistory.EventType
	Record    = corehistory.Record
)

const (
	EventStart = corehistory.EventStart
	EventStop  = corehistory.EventStop
)
