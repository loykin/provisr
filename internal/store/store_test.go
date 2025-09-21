package store

import (
	"testing"
	"time"
)

func TestMinimalRecordFields(t *testing.T) {
	r := Record{Name: "demo", PID: 1234, LastStatus: "running", UpdatedAt: time.Now().UTC()}
	if r.Name != "demo" || r.PID != 1234 || r.LastStatus == "" {
		t.Fatalf("unexpected record: %+v", r)
	}
}
