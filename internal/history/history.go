package history

import corehistory "github.com/loykin/provisr/core/history"

type EventType = corehistory.EventType

const (
	EventStart = corehistory.EventStart
	EventStop  = corehistory.EventStop
)

type Record = corehistory.Record
type Event = corehistory.Event
type Sink = corehistory.Sink
