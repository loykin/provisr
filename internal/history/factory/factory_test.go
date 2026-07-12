package factory

import (
	"path/filepath"
	"testing"
)

func TestFactoryDSNTypes(t *testing.T) {
	tempDir := t.TempDir()
	sqliteFile := filepath.Join(tempDir, "test.db")
	sqliteDSN := "sqlite:///" + filepath.ToSlash(sqliteFile)

	tests := []struct {
		name        string
		dsn         string
		expectError bool
		skipTest    bool
	}{
		{"Empty DSN", "", true, false},
		{"Invalid scheme", "invalid://test", true, false},
		{"ClickHouse DSN", "clickhouse://localhost:9000?table=events", false, true},
		{"OpenSearch DSN", "opensearch://localhost:9200/process-logs", false, true},
		{"PostgreSQL DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable", false, true},
		{"PostgreSQL DSN alt", "postgresql://user:pass@localhost:5432/db", false, true},
		{"SQLite file DSN", sqliteDSN, false, false},
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

			if closer, ok := sink.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		})
	}
}

func TestFactoryCanDisableSQLiteMigration(t *testing.T) {
	dsn := "sqlite://" + filepath.Join(t.TempDir(), "history.db")
	sink, err := NewSinkFromDSNWithOptions(dsn, Options{Migrate: false})
	if err != nil {
		t.Fatalf("NewSinkFromDSNWithOptions() error: %v", err)
	}
	if closer, ok := sink.(interface{ Close() error }); ok {
		_ = closer.Close()
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
