package factory

import (
	"errors"
	"net/url"
	"strings"

	corehistory "github.com/loykin/provisr/core/history"
	"github.com/loykin/provisr/internal/history/clickhouse"
	"github.com/loykin/provisr/internal/history/opensearch"
	"github.com/loykin/provisr/internal/history/postgres"
	"github.com/loykin/provisr/internal/history/sqlite"
)

// NewSinkFromDSN creates a history sink based on DSN format.
// Supported formats:
//   - "clickhouse://user:pass@host:port/database?table=process_history"
//     (table defaults to "process_history"; the table must already exist)
//   - "opensearch://host:port/index"
//   - "postgres://user:pass@host:port/db?sslmode=disable"
//   - "postgresql://user:pass@host:port/db?sslmode=disable"
//   - "sqlite:///path/to/file.db" or "sqlite://:memory:"
//   - "/path/to/file.db" (defaults to SQLite)
func NewSinkFromDSN(dsn string) (corehistory.Sink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty DSN")
	}

	lower := strings.ToLower(dsn)

	if strings.HasPrefix(lower, "clickhouse://") {
		return parseClickHouseDSN(dsn)
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

func parseClickHouseDSN(dsn string) (corehistory.Sink, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	table := q.Get("table")
	if table == "" {
		table = "process_history"
	}
	q.Del("table")
	u.RawQuery = q.Encode()

	return clickhouse.New(u.String(), table)
}

func parseOpenSearchDSN(dsn string) (corehistory.Sink, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host

	index := strings.Trim(u.Path, "/")
	if index == "" {
		index = "process-history"
	}

	return opensearch.New(baseURL, index)
}
