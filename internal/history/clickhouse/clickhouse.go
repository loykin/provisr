// Package clickhouse provides a history.Sink implementation backed by ClickHouse.
// Import this package in addition to github.com/loykin/provisr:
//
//	sink, err := clickhouse.New("clickhouse://default:@localhost:9000?database=default", "process_history")
//	mgr.SetHistorySinks(sink)
package clickhouse

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2" // registers the "clickhouse" database/sql driver
	"github.com/jmoiron/sqlx"
	"github.com/loykin/dbstore"
	prometheusadapter "github.com/loykin/dbstore/adapters/prometheus"
	sqlxadapter "github.com/loykin/dbstore/adapters/sqlx"

	corehistory "github.com/loykin/provisr/core/history"
)

const source = "process_history"

// Sink sends events to ClickHouse over database/sql (via dbstore), using
// the official ClickHouse Go client's stdlib driver.
type Sink struct {
	sqlxadapter.Source
	adapter *sqlxadapter.Adapter
	table   string
}

type Options struct {
	Migrate bool
}

var tableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// New opens a ClickHouse connection using dsn (e.g.
// "clickhouse://user:pass@host:9000?database=default") and returns a Sink
// that writes events to table. table must already exist — this package
// does not manage schema/migrations.
func New(dsn, table string) (*Sink, error) {
	return NewWithOptions(dsn, table, Options{Migrate: true})
}

func NewWithOptions(dsn, table string, options Options) (*Sink, error) {
	if !tableNamePattern.MatchString(table) {
		return nil, fmt.Errorf("clickhouse: invalid table name %q", table)
	}
	adapter := sqlxadapter.New()
	adapter.RegisterDriver(sqlxadapter.DriverClickHouse, sqlxadapter.ClickHouseDriver())
	adapter.SetObserver(prometheusadapter.New("provisr_history_clickhouse", nil))

	if err := adapter.Open(source, dbstore.SourceConfig{
		Driver:     "clickhouse",
		DSN:        dsn,
		PoolConfig: dbstore.DefaultPoolConfig,
	}); err != nil {
		return nil, fmt.Errorf("clickhouse: register pool: %w", err)
	}

	sink := &Sink{Source: sqlxadapter.NewSource(source, adapter.Executor()), adapter: adapter, table: table}

	if err := sink.Run(context.Background(), func(ctx context.Context, db *sqlx.DB) error {
		if err := db.PingContext(ctx); err != nil {
			return err
		}
		if !options.Migrate {
			return nil
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			type String,
			occurred_at DateTime64(9, 'UTC'),
			record_name String,
			record_pid Int32,
			record_status String
		) ENGINE = MergeTree() ORDER BY occurred_at`, table)); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s ADD COLUMN IF NOT EXISTS record_status String`, table))
		return err
	}); err != nil {
		adapter.Close()
		return nil, fmt.Errorf("clickhouse: ping: %w", err)
	}

	return sink, nil
}

// Close releases the underlying connection pool.
func (s *Sink) Close() error {
	s.adapter.Close()
	return nil
}

// Send writes a lifecycle event to ClickHouse.
func (s *Sink) Send(ctx context.Context, e corehistory.Event) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (type, occurred_at, record_name, record_pid, record_status) VALUES (?, ?, ?, ?, ?)`,
		s.table,
	)
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		_, err := db.ExecContext(ctx, query, string(e.Type), e.OccurredAt, e.Record.Name, e.Record.PID, e.Record.LastStatus)
		return err
	})
}

// List returns recent history rows, newest first. If name is empty, rows
// for all processes are returned. limit is capped at 500 (defaults to 100).
func (s *Sink) List(ctx context.Context, name string, limit, offset int) ([]corehistory.Entry, error) {
	name = strings.TrimSpace(name)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var rows []corehistory.Entry
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.SelectContext(ctx, &rows,
				fmt.Sprintf(`SELECT occurred_at AS timestamp, record_pid AS pid, record_name AS name, record_status AS status, NULL AS error FROM %s ORDER BY occurred_at DESC LIMIT ? OFFSET ?`, s.table),
				limit, offset)
		}
		return db.SelectContext(ctx, &rows,
			fmt.Sprintf(`SELECT occurred_at AS timestamp, record_pid AS pid, record_name AS name, record_status AS status, NULL AS error FROM %s WHERE positionCaseInsensitive(record_name, ?) > 0 ORDER BY occurred_at DESC LIMIT ? OFFSET ?`, s.table),
			name, limit, offset)
	})
	return rows, err
}

func (s *Sink) Count(ctx context.Context, name string) (int, error) {
	name = strings.TrimSpace(name)
	var total int
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.GetContext(ctx, &total, fmt.Sprintf(`SELECT count() FROM %s`, s.table))
		}
		return db.GetContext(ctx, &total,
			fmt.Sprintf(`SELECT count() FROM %s WHERE positionCaseInsensitive(record_name, ?) > 0`, s.table), name)
	})
	return total, err
}

func (s *Sink) PruneBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if err := db.GetContext(ctx, &total,
			fmt.Sprintf(`SELECT count() FROM %s WHERE occurred_at < ?`, s.table), cutoff.UTC()); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx,
			fmt.Sprintf(`ALTER TABLE %s DELETE WHERE occurred_at < ?`, s.table), cutoff.UTC())
		return err
	})
	return total, err
}

// compile-time check that Sink satisfies corehistory.Sink
var _ corehistory.Sink = (*Sink)(nil)
var _ corehistory.Reader = (*Sink)(nil)
var _ corehistory.Pruner = (*Sink)(nil)
