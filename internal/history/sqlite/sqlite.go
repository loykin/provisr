package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/loykin/provisr/internal/history"
)

// Sink writes history events to SQLite database.
type Sink struct {
	db *sql.DB
}

// New creates a new SQLite history sink.
// DSN format:
//   - "sqlite:///path/to/file.db"
//   - "sqlite://:memory:"
//   - "/path/to/file.db" (without prefix)
//   - ":memory:" (in-memory database)
func New(dsn string) (*Sink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty SQLite DSN")
	}

	// Handle sqlite:// prefix
	if strings.HasPrefix(strings.ToLower(dsn), "sqlite://") {
		dsn = strings.TrimPrefix(dsn, "sqlite://")
	}

	db, err := sql.Open("sqlite", dsn)
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
	// Simple audit table, no primary key. Timestamp defaults to CURRENT_TIMESTAMP when not provided.
	stmt := `CREATE TABLE IF NOT EXISTS process_history(
		timestamp TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP),
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
		VALUES(?, ?, ?, ?, NULL);`,
		occur, rec.PID, rec.Name, rec.LastStatus)
	return err
}

func (s *Sink) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
