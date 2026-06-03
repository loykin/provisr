package factory

import (
	"errors"
	"net/url"
	"strings"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/history/opensearch"
	"github.com/loykin/provisr/internal/history/postgres"
	"github.com/loykin/provisr/internal/history/sqlite"
)

// NewSinkFromDSN creates a history sink based on DSN format.
// Supported formats:
//   - "opensearch://host:port/index"
//   - "postgres://user:pass@host:port/db?sslmode=disable"
//   - "postgresql://user:pass@host:port/db?sslmode=disable"
//   - "sqlite:///path/to/file.db" or "sqlite://:memory:"
//   - "/path/to/file.db" (defaults to SQLite)
//
// ClickHouse is not built into the main module. Use
// github.com/loykin/provisr/history/clickhouse and pass the result to
// manager.SetHistorySinks().
func NewSinkFromDSN(dsn string) (history.Sink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty DSN")
	}

	lower := strings.ToLower(dsn)

	if strings.HasPrefix(lower, "clickhouse://") {
		return nil, errors.New("clickhouse is a separate module; import github.com/loykin/provisr/history/clickhouse")
	}

	if strings.HasPrefix(lower, "opensearch://") || strings.HasPrefix(lower, "elasticsearch://") {
		return parseOpenSearchDSN(dsn)
	}

	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return postgres.New(dsn)
	}

	if strings.HasPrefix(lower, "sqlite://") || !strings.Contains(dsn, "://") {
		return sqlite.New(dsn)
	}

	return nil, errors.New("unsupported DSN format: " + dsn)
}

func parseOpenSearchDSN(dsn string) (history.Sink, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host

	index := strings.Trim(u.Path, "/")
	if index == "" {
		index = "process-history"
	}

	return opensearch.New(baseURL, index), nil
}
