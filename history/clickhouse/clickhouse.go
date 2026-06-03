// Package clickhouse provides a history.Sink implementation backed by ClickHouse.
// Import this package in addition to github.com/loykin/provisr:
//
//	sink, err := clickhouse.New("localhost:9000", "process_history")
//	mgr.SetHistorySinks(sink)
package clickhouse

import (
	"context"
	"fmt"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	corehistory "github.com/loykin/provisr/core/history"
)

// Sink sends events to ClickHouse using the official ClickHouse Go client.
type Sink struct {
	conn  driver.Conn
	table string
}

// New opens a connection to ClickHouse at host (e.g. "localhost:9000") and
// returns a Sink that writes events to table.
func New(host, table string) (*Sink, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{host},
		Auth: ch.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse: connect: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clickhouse: ping: %w", err)
	}

	return &Sink{conn: conn, table: table}, nil
}

// Close releases the underlying connection.
func (s *Sink) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Send writes a lifecycle event to ClickHouse.
func (s *Sink) Send(ctx context.Context, e corehistory.Event) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (type, occurred_at, record_name, record_pid) VALUES (?, ?, ?, ?)`,
		s.table,
	)
	if err := s.conn.Exec(ctx, query,
		string(e.Type),
		e.OccurredAt,
		e.Record.Name,
		e.Record.PID,
	); err != nil {
		return fmt.Errorf("clickhouse: insert: %w", err)
	}
	return nil
}

// compile-time check that Sink satisfies corehistory.Sink
var _ corehistory.Sink = (*Sink)(nil)
