package manager

import (
	"testing"

	"github.com/loykin/provisr/core/observability"
)

func TestManagersHaveIndependentObservers(t *testing.T) {
	first, second := NewManager(), NewManager()
	var firstCount, secondCount int
	first.SetObservers(observability.ObserverFunc(func(observability.Event) { firstCount++ }))
	second.SetObservers(observability.ObserverFunc(func(observability.Event) { secondCount++ }))

	first.Observe(observability.Event{Kind: observability.ProcessStarted})
	if firstCount != 1 || secondCount != 0 {
		t.Fatalf("observer counts = %d/%d, want 1/0", firstCount, secondCount)
	}
}
