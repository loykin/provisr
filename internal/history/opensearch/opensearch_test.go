package opensearch

import "testing"

// TestNewDoesNotDial confirms Sink construction (Pool.Register ->
// opensearchapi.NewClient) succeeds without contacting a real server —
// the client only builds an HTTP transport, it doesn't ping eagerly.
func TestNewDoesNotDial(t *testing.T) {
	sink, err := New("http://127.0.0.1:19200", "provisr-history-test")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if sink == nil {
		t.Fatal("expected non-nil sink")
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}
