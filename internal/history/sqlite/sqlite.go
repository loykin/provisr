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
	queries := []string{
		`CREATE TABLE IF NOT EXISTS process_history(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at TIMESTAMP NOT NULL,
			event TEXT NOT NULL,
			name TEXT NOT NULL,
			pid INTEGER NOT NULL,
			started_at TIMESTAMP NOT NULL,
			stopped_at TIMESTAMP NULL,
			running BOOLEAN NOT NULL,
			exit_err TEXT NULL,
			uniq TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_process_history_name ON process_history(name);`,
		`CREATE INDEX IF NOT EXISTS idx_process_history_uniq ON process_history(uniq);`,
		`CREATE INDEX IF NOT EXISTS idx_process_history_occurred_at ON process_history(occurred_at);`,
	}

	for _, query := range queries {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sink) Send(ctx context.Context, e history.Event) error {
	rec := e.Record
	occur := e.OccurredAt.UTC()

	var stopped interface{} = nil
	if rec.StoppedAt.Valid {
		stopped = rec.StoppedAt.Time.UTC()
	}

	var exitErr interface{} = nil
	if rec.ExitErr.Valid {
		exitErr = rec.ExitErr.String
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO process_history(occurred_at, event, name, pid, started_at, stopped_at, running, exit_err, uniq)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		occur, string(e.Type), rec.Name, rec.PID, rec.StartedAt.UTC(), stopped, rec.Running, exitErr, rec.Key())

	return err
}

func (s *Sink) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
