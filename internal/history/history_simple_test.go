package history

import (
	"testing"
	"time"
)

func TestEvent_Creation(t *testing.T) {
	record := Record{
		Name:       "test-process",
		PID:        12345,
		LastStatus: "running",
		UpdatedAt:  time.Now(),
		SpecJSON:   `{"name":"test-process","command":"echo hello"}`,
	}

	event := Event{
		Type:       EventStart,
		OccurredAt: time.Now(),
		Record:     record,
	}

	if event.Type != EventStart {
		t.Errorf("Expected event type %s, got %s", EventStart, event.Type)
	}
	if event.Record.Name != "test-process" {
		t.Errorf("Expected process name test-process, got %s", event.Record.Name)
	}
	if event.Record.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", event.Record.PID)
	}
}

func TestEvent_Types(t *testing.T) {
	testCases := []struct {
		name      string
		eventType EventType
	}{
		{"start event", EventStart},
		{"stop event", EventStop},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			record := Record{
				Name:       "test-process",
				PID:        12345,
				LastStatus: "running",
				UpdatedAt:  time.Now(),
			}

			event := Event{
				Type:       tc.eventType,
				OccurredAt: time.Now(),
				Record:     record,
			}

			if event.Type != tc.eventType {
				t.Errorf("Expected event type %s, got %s", tc.eventType, event.Type)
			}
		})
	}
}

func TestRecord_Fields(t *testing.T) {
	now := time.Now()
	record := Record{
		Name:       "test-process",
		PID:        12345,
		LastStatus: "running",
		UpdatedAt:  now,
		SpecJSON:   `{"name":"test-process","command":"echo hello","work_dir":"/tmp"}`,
	}

	if record.Name == "" {
		t.Error("Expected name to be set")
	}
	if record.PID <= 0 {
		t.Error("Expected PID to be positive")
	}
	if record.LastStatus == "" {
		t.Error("Expected last status to be set")
	}
	if record.UpdatedAt.IsZero() {
		t.Error("Expected updated at to be set")
	}
	if record.SpecJSON == "" {
		t.Error("Expected spec JSON to be set")
	}
}

func TestEvent_Validation(t *testing.T) {
	testCases := []struct {
		name  string
		event Event
		valid bool
	}{
		{
			name: "valid_start_event",
			event: Event{
				Type:       EventStart,
				OccurredAt: time.Now(),
				Record: Record{
					Name:       "test-process",
					PID:        12345,
					LastStatus: "starting",
					UpdatedAt:  time.Now(),
				},
			},
			valid: true,
		},
		{
			name: "valid_stop_event",
			event: Event{
				Type:       EventStop,
				OccurredAt: time.Now(),
				Record: Record{
					Name:       "test-process",
					PID:        12345,
					LastStatus: "stopped",
					UpdatedAt:  time.Now(),
				},
			},
			valid: true,
		},
		{
			name: "empty_type",
			event: Event{
				Type:       "",
				OccurredAt: time.Now(),
				Record: Record{
					Name: "test-process",
				},
			},
			valid: false,
		},
		{
			name: "zero_time",
			event: Event{
				Type:       EventStart,
				OccurredAt: time.Time{},
				Record: Record{
					Name: "test-process",
				},
			},
			valid: false,
		},
		{
			name: "empty_process_name",
			event: Event{
				Type:       EventStart,
				OccurredAt: time.Now(),
				Record: Record{
					Name: "",
				},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := tc.event.Type != "" &&
				!tc.event.OccurredAt.IsZero() &&
				tc.event.Record.Name != ""

			if tc.valid && !isValid {
				t.Error("Expected event to be valid")
			}
			if !tc.valid && isValid {
				t.Error("Expected event to be invalid")
			}
		})
	}
}
