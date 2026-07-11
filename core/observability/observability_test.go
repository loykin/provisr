package observability

import "testing"

func TestEmitNotifiesConfiguredObservers(t *testing.T) {
	var got Event
	emitter := NewEmitter(ObserverFunc(func(event Event) { got = event }))
	want := Event{Kind: ProcessStarted, Name: "worker"}
	emitter.Emit(want)
	if got != want {
		t.Fatalf("observed event = %+v, want %+v", got, want)
	}
}
