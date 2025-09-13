package history

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/store"
)

func TestOpenSearchSink_Send(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/idx/_doc" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = b
		w.WriteHeader(201)
	}))
	defer ts.Close()

	sink := NewOpenSearchSink(ts.URL, "idx")
	e := Event{Type: EventStart, OccurredAt: time.Now().UTC(), Record: store.Record{Name: "a", PID: 1, StartedAt: time.Now().UTC()}}
	if err := sink.Send(context.Background(), e); err != nil {
		t.Fatalf("send: %v", err)
	}
	// Ensure body is JSON and includes name
	var m map[string]any
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	rec, ok := m["record"].(map[string]any)
	if !ok {
		t.Fatalf("missing record in payload: %v", m)
	}
	if rec["Name"] != "a" {
		t.Fatalf("unexpected record name: %v", rec)
	}
}

func TestClickHouseSink_Send(t *testing.T) {
	var gotQuery string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		gotBody = b
		w.WriteHeader(200)
	}))
	defer ts.Close()

	sink := NewClickHouseSink(ts.URL, "default.evts")
	e := Event{Type: EventStop, OccurredAt: time.Now().UTC(), Record: store.Record{Name: "b", PID: 2, StartedAt: time.Now().UTC()}}
	if err := sink.Send(context.Background(), e); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotQuery == "" || gotBody == nil || len(gotBody) == 0 {
		t.Fatalf("expected non-empty query/body")
	}
	// body should be a single JSON line
	if gotBody[len(gotBody)-1] != '\n' {
		t.Fatalf("expected trailing newline in body")
	}
}
