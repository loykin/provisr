package manager

import (
	"testing"
	"time"
)

func TestStopUnknownProcess(t *testing.T) {
	mgr := NewManager()
	err := mgr.Stop("nope", 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error stopping unknown process")
	}
}
