package clickhouse

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/loykin/provisr/internal/history"
)

// Sink sends events to ClickHouse using the official ClickHouse Go client.
type Sink struct {
	conn  driver.Conn
	table string
}

func New(dsn, table string) (*Sink, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Test the connection
	if err := conn.Ping(context.Background()); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	return &Sink{
		conn:  conn,
		table: table,
	}, nil
}

func (s *Sink) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *Sink) Send(ctx context.Context, e history.Event) error {
	query := fmt.Sprintf(`INSERT INTO %s (type, occurred_at, record_name, record_pid, record_started_at, record_stopped_at, record_running, record_exit_err, record_uniq) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.table)

	err := s.conn.Exec(ctx, query,
		string(e.Type),
		e.OccurredAt,
		e.Record.Name,
		e.Record.PID,
		e.Record.StartedAt,
		e.Record.StoppedAt,
		e.Record.Running,
		e.Record.ExitErr,
		e.Record.Uniq,
	)

	if err != nil {
		return fmt.Errorf("failed to insert event into ClickHouse: %w", err)
	}

	return nil
}
