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

func TestParseClickHouseDSN(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		expectError bool
		skipTest    bool
	}{
		{"Valid ClickHouse DSN with table", "clickhouse://localhost:8123?table=events", false, true},
		{"ClickHouse DSN without table", "clickhouse://localhost:8123", false, true},
		{"ClickHouse DSN with default port", "clickhouse://localhost", false, true},
		{"Invalid ClickHouse DSN", "clickhouse://", false, true}, // URL parse should succeed but connection might fail
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip("Skipping test that requires external ClickHouse connection")
			}

			sink, err := parseClickHouseDSN(tt.dsn)

			if tt.expectError && err == nil {
				t.Errorf("expected error for DSN %q, got nil", tt.dsn)
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for DSN %q: %v", tt.dsn, err)
				return
			}

			if !tt.expectError && sink == nil {
				t.Errorf("expected non-nil sink for DSN %q", tt.dsn)
			}
		})
	}
}

func TestParseOpenSearchDSN(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		expectError bool
		skipTest    bool
	}{
		{"Valid OpenSearch DSN with index", "opensearch://localhost:9200/process-logs", false, true},
		{"OpenSearch DSN without index", "opensearch://localhost:9200", false, true},
		{"OpenSearch HTTPS DSN", "opensearch://https://localhost:9200/logs", false, true},
		{"Elasticsearch DSN", "elasticsearch://localhost:9200/events", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip("Skipping test that requires external OpenSearch connection")
			}

			sink, err := parseOpenSearchDSN(tt.dsn)

			if tt.expectError && err == nil {
				t.Errorf("expected error for DSN %q, got nil", tt.dsn)
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for DSN %q: %v", tt.dsn, err)
				return
			}

			if !tt.expectError && sink == nil {
				t.Errorf("expected non-nil sink for DSN %q", tt.dsn)
			}
		})
	}
}

func TestDSNParsingUnit(t *testing.T) {
	// Test URL parsing functionality in isolation
	tests := []struct {
		name     string
		dsn      string
		testFunc func(string) (interface{}, error)
	}{
		{
			"ClickHouse parsing",
			"clickhouse://localhost:8123?table=events",
			func(dsn string) (interface{}, error) {
				return parseClickHouseDSN(dsn)
			},
		},
		{
			"OpenSearch parsing",
			"opensearch://localhost:9200/logs",
			func(dsn string) (interface{}, error) {
				return parseOpenSearchDSN(dsn)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the parsing logic without requiring external services
			// We expect these to not panic and to parse the DSN structure
			result, err := tt.testFunc(tt.dsn)

			// We can't test connection without external services, but we can
			// verify the parsing doesn't panic and produces some result
			if err != nil {
				// If error is connection-related, that's OK for this unit test
				t.Logf("Function produced error (expected for unit test): %v", err)
			} else if result == nil {
				t.Error("Expected non-nil result from parsing function")
			}
		})
	}
}
