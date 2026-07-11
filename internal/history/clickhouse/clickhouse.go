// Package clickhouse provides a history.Sink implementation backed by ClickHouse.
// Import this package in addition to github.com/loykin/provisr:
//
//	sink, err := clickhouse.New("clickhouse://default:@localhost:9000?database=default", "process_history")
//	mgr.SetHistorySinks(sink)
package clickhouse

import (
	"context"
	"fmt"
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

// Record is a single stored history row.
type Record struct {
	Type       string    `db:"type" json:"type"`
	OccurredAt time.Time `db:"occurred_at" json:"occurred_at"`
	Name       string    `db:"record_name" json:"record_name"`
	PID        int       `db:"record_pid" json:"record_pid"`
}

// New opens a ClickHouse connection using dsn (e.g.
// "clickhouse://user:pass@host:9000?database=default") and returns a Sink
// that writes events to table. table must already exist — this package
// does not manage schema/migrations.
func New(dsn, table string) (*Sink, error) {
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
		return db.PingContext(ctx)
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
		`INSERT INTO %s (type, occurred_at, record_name, record_pid) VALUES (?, ?, ?, ?)`,
		s.table,
	)
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		_, err := db.ExecContext(ctx, query, string(e.Type), e.OccurredAt, e.Record.Name, e.Record.PID)
		return err
	})
}

// List returns recent history rows, newest first. If name is empty, rows
// for all processes are returned. limit is capped at 500 (defaults to 100).
func (s *Sink) List(ctx context.Context, name string, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []Record
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.SelectContext(ctx, &rows,
				fmt.Sprintf(`SELECT type, occurred_at, record_name, record_pid FROM %s ORDER BY occurred_at DESC LIMIT ?`, s.table),
				limit)
		}
		return db.SelectContext(ctx, &rows,
			fmt.Sprintf(`SELECT type, occurred_at, record_name, record_pid FROM %s WHERE record_name = ? ORDER BY occurred_at DESC LIMIT ?`, s.table),
			name, limit)
	})
	return rows, err
}

// compile-time check that Sink satisfies corehistory.Sink
var _ corehistory.Sink = (*Sink)(nil)
