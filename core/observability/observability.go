// Package observability defines the outbound event port used by the process
// core. Adapters translate these events to Prometheus, logs, traces, or other
// telemetry systems.
package observability

import "sync"

type Kind string

const (
	ProcessStarted       Kind = "process.started"
	ProcessStopped       Kind = "process.stopped"
	ProcessStateChanged  Kind = "process.state_changed"
	JobStarted           Kind = "job.started"
	JobDeleted           Kind = "job.deleted"
	CronJobActivated     Kind = "cronjob.activated"
	CronJobDeactivated   Kind = "cronjob.deactivated"
	CronJobScheduled     Kind = "cronjob.scheduled"
	CronJobNextScheduled Kind = "cronjob.next_scheduled"
	CronJobCompleted     Kind = "cronjob.completed"
)

type Event struct {
	Kind     Kind
	Name     string
	Phase    string
	From     string
	To       string
	UnixTime float64
	Duration float64
}

type Observer interface {
	Observe(Event)
}

type ObserverFunc func(Event)

func (f ObserverFunc) Observe(event Event) { f(event) }

type Emitter struct {
	mu        sync.RWMutex
	observers []Observer
}

func NewEmitter(observers ...Observer) *Emitter {
	emitter := &Emitter{}
	emitter.SetObservers(observers...)
	return emitter
}

func (e *Emitter) SetObservers(observers ...Observer) {
	e.mu.Lock()
	e.observers = append([]Observer(nil), observers...)
	e.mu.Unlock()
}

func (e *Emitter) Emit(event Event) {
	if e == nil {
		return
	}
	e.mu.RLock()
	observers := append([]Observer(nil), e.observers...)
	e.mu.RUnlock()
	for _, observer := range observers {
		if observer != nil {
			observer.Observe(event)
		}
	}
}
