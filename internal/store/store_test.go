package store

import (
	"testing"
	"time"
)

func TestUniqueKeyAndRecordKey(t *testing.T) {
	start := time.Unix(1700000000, 123456789).UTC()
	k := UniqueKey(1234, start)
	if k != "1234-1700000000123456789" {
		t.Fatalf("unexpected key: %s", k)
	}
	r := Record{Name: "demo", PID: 1234, StartedAt: start}
	if r.Key() != k {
		t.Fatalf("record key mismatch: %s vs %s", r.Key(), k)
	}
}
