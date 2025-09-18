package factory

import (
	"testing"
)

func TestFactoryDSNTypes(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		expectError bool
		skipTest    bool
	}{
		{"Empty DSN", "", true, false},
		{"Invalid scheme", "invalid://test", true, false},
		{"ClickHouse DSN", "clickhouse://localhost:8123?table=events", false, true},
		{"OpenSearch DSN", "opensearch://localhost:9200/process-logs", false, true},
		{"PostgreSQL DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable", false, true},
		{"PostgreSQL DSN alt", "postgresql://user:pass@localhost:5432/db", false, true},
		{"SQLite file DSN", "sqlite:///tmp/test.db", false, false},
		{"SQLite memory DSN", "sqlite://:memory:", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip("Skipping test that requires external database connection")
			}

			sink, err := NewSinkFromDSN(tt.dsn)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for DSN %q, got nil", tt.dsn)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for DSN %q: %v", tt.dsn, err)
				return
			}

			if sink == nil {
				t.Errorf("expected non-nil sink for DSN %q", tt.dsn)
			}

			// Clean up if closeable
			if closer, ok := sink.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		})
	}
}
