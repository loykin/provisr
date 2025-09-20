package metrics

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterIdempotentAndCountersWork(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := Register(reg); err != nil {
		t.Fatalf("first register: %v", err)
	}
	// idempotent: calling again should be no-op
	if err := Register(reg); err != nil {
		t.Fatalf("second register: %v", err)
	}

	// Exercise helpers; they should work only after Register
	IncStart("a")
	IncStart("a")
	IncRestart("a")
	IncStop("a")
	ObserveStartDuration("a", 1.25)
	SetRunningInstances("base", 3)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	// Very basic assertions that our metric names exist and have samples
	wantNames := map[string]bool{
		"provisr_process_starts_total":           false,
		"provisr_process_restarts_total":         false,
		"provisr_process_stops_total":            false,
		"provisr_process_start_duration_seconds": false,
		"provisr_process_running_instances":      false,
	}
	for _, mf := range mfs {
		n := mf.GetName()
		if _, ok := wantNames[n]; ok {
			wantNames[n] = true
			if len(mf.GetMetric()) == 0 {
				t.Fatalf("metric %s has no samples", n)
			}
		}
	}
	for n, ok := range wantNames {
		if !ok {
			t.Fatalf("expected to find metric %s", n)
		}
	}
}

func TestHandlerServesMetrics(t *testing.T) {
	// Ensure collectors are registered with the default registry used by Handler().
	// Reset regOK gate to allow registration in this test regardless of previous tests.
	regOK.Store(false)
	if err := Register(prometheus.DefaultRegisterer); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(Handler())
	defer srv.Close()

	// touch some metrics
	IncStart("x")

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	s := string(b)
	if !strings.Contains(s, "provisr_process_starts_total") {
		t.Fatalf("metrics output missing starts_total: %s", s[:min(200, len(s))])
	}
}

func TestConcurrentIncrements(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := Register(reg); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			IncStart("c")
			IncRestart("c")
			IncStop("c")
		}()
	}
	wg.Wait()
	// Ensure gather succeeds under race detector
	if _, err := reg.Gather(); err != nil {
		t.Fatalf("gather: %v", err)
	}
}

func TestStateTransitionMetrics(t *testing.T) {
	// Test state transition recording before registration (should be no-ops)
	originalState := regOK.Load()
	regOK.Store(false)

	// These should not panic
	RecordStateTransition("test-proc", "starting", "running")
	RecordStateTransition("test-proc", "running", "stopping")
	RecordStateTransition("test-proc", "stopping", "stopped")

	// Restore original state
	regOK.Store(originalState)

	// Test after registration
	if regOK.Load() {
		// These should work if already registered
		RecordStateTransition("registered-proc", "start", "run")
	}
}

func TestCurrentStateMetrics(t *testing.T) {
	// Test current state setting before registration (should be no-ops)
	originalState := regOK.Load()
	regOK.Store(false)

	// These should not panic
	SetCurrentState("test-proc", "running", true)
	SetCurrentState("test-proc", "stopped", false)
	SetCurrentState("another-proc", "starting", true)

	// Restore original state
	regOK.Store(originalState)

	// Test after registration
	if regOK.Load() {
		// These should work if already registered
		SetCurrentState("registered-proc", "active", true)
	}
}

func TestMetricsBeforeRegister(t *testing.T) {
	// Reset registration status to test behavior before registration
	originalState := regOK.Load()
	regOK.Store(false)
	defer regOK.Store(originalState)

	// These should be no-ops and not panic when called before Register
	IncStart("test")
	IncRestart("test")
	IncStop("test")
	ObserveStartDuration("test", 1.0)
	SetRunningInstances("test", 5)
	RecordStateTransition("test", "start", "run")
	SetCurrentState("test", "running", true)

	// No crash means success
}

func TestRegisterError(t *testing.T) {
	// Test that Register handles errors appropriately
	// Create a custom registerer that returns a non-AlreadyRegisteredError
	errorRegisterer := &errorRegisterer{
		shouldError: true,
	}

	// Reset regOK to allow testing registration failure
	originalState := regOK.Load()
	regOK.Store(false)
	defer regOK.Store(originalState)

	// Now Register should return the error
	err := Register(errorRegisterer)
	if err == nil {
		t.Fatal("Register should return error from failing registerer")
	}
	if err.Error() != "test registration error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Custom registerer for testing error handling
type errorRegisterer struct {
	shouldError bool
}

func (e *errorRegisterer) Register(prometheus.Collector) error {
	if e.shouldError {
		return errors.New("test registration error")
	}
	return nil
}

func (e *errorRegisterer) MustRegister(...prometheus.Collector) {}
func (e *errorRegisterer) Unregister(prometheus.Collector) bool { return false }
