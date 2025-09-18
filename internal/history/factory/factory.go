package factory

import (
	"errors"
	"net/url"
	"strings"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/history/clickhouse"
	"github.com/loykin/provisr/internal/history/opensearch"
	"github.com/loykin/provisr/internal/history/postgres"
	"github.com/loykin/provisr/internal/history/sqlite"
)

// NewSinkFromDSN creates a history sink based on DSN format.
// Supported formats:
//   - "clickhouse://host:port?database=db&table=table"
//   - "opensearch://host:port/index"
//   - "postgres://user:pass@host:port/db?sslmode=disable"
//   - "postgresql://user:pass@host:port/db?sslmode=disable"
//   - "sqlite:///path/to/file.db" or "sqlite://:memory:"
//   - "/path/to/file.db" (defaults to SQLite)
func NewSinkFromDSN(dsn string) (history.Sink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty DSN")
	}

	lower := strings.ToLower(dsn)

	// ClickHouse
	if strings.HasPrefix(lower, "clickhouse://") {
		return parseClickHouseDSN(dsn)
	}

	// OpenSearch / Elasticsearch
	if strings.HasPrefix(lower, "opensearch://") || strings.HasPrefix(lower, "elasticsearch://") {
		return parseOpenSearchDSN(dsn)
	}

	// PostgreSQL
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return postgres.New(dsn)
	}

	// SQLite (explicit or implicit)
	if strings.HasPrefix(lower, "sqlite://") || !strings.Contains(dsn, "://") {
		return sqlite.New(dsn)
	}

	return nil, errors.New("unsupported DSN format: " + dsn)
}

func parseClickHouseDSN(dsn string) (history.Sink, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	// Extract host:port
	host := u.Host
	if host == "" {
		host = "localhost:9000" // default ClickHouse native port
	}

	// Get table from query params
	table := u.Query().Get("table")
	if table == "" {
		table = "process_history" // default table name
	}

	return clickhouse.New(host, table)
}

func parseOpenSearchDSN(dsn string) (history.Sink, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	// Extract base URL (scheme + host + port)
	baseURL := u.Scheme + "://" + u.Host

	// Extract index from path
	index := strings.Trim(u.Path, "/")
	if index == "" {
		index = "process-history" // default index name
	}

	return opensearch.New(baseURL, index), nil
}
