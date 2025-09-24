package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/loykin/provisr/internal/history"
)

// Sink writes history events to PostgreSQL database.
type Sink struct {
	db *sql.DB
}

// New creates a new PostgreSQL history sink.
// DSN format: postgres://user:pass@host:port/db?sslmode=disable
func New(dsn string) (*Sink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty PostgreSQL DSN")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	sink := &Sink{db: db}
	if err := sink.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return sink, nil
}

func (s *Sink) ensureSchema(ctx context.Context) error {
	// Simple audit table with no primary key; timestamp defaults to now
	stmt := `CREATE TABLE IF NOT EXISTS process_history(
		timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		pid INTEGER NOT NULL,
		name TEXT NOT NULL,
		status TEXT NOT NULL,
		error TEXT
	);`
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

func (s *Sink) Send(ctx context.Context, e history.Event) error {
	rec := e.Record
	occur := e.OccurredAt.UTC()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO process_history(timestamp, pid, name, status, error)
		VALUES($1, $2, $3, $4, NULL);`,
		occur, rec.PID, rec.Name, rec.LastStatus)
	return err
}

func (s *Sink) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
