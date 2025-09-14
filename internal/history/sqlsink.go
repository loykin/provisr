package history

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// SQLSink writes history events into a relational database table process_history.
// It supports SQLite (modernc.org/sqlite) and Postgres (pgx stdlib) based on DSN.
// The schema is created if missing.
// DSN examples:
//   - sqlite:///path/to/file.db or :memory:
//   - postgres://user:pass@host:port/db?sslmode=disable
//
// Note: This sink is independent from the store state; it only appends to history.

type SQLSink struct {
	db      *sql.DB
	dialect string // "sqlite" or "postgres"
}

func NewSQLSinkFromDSN(dsn string) (*SQLSink, error) {
	d := strings.TrimSpace(dsn)
	if d == "" {
		return nil, errors.New("empty DSN for SQL history sink")
	}
	ld := strings.ToLower(d)
	var (
		db      *sql.DB
		drv     string
		dialect string
		path    string
	)
	if strings.HasPrefix(ld, "postgres://") || strings.HasPrefix(ld, "postgresql://") {
		drv = "pgx"
		dialect = "postgres"
		path = d
	} else if strings.HasPrefix(ld, "sqlite://") {
		drv = "sqlite"
		dialect = "sqlite"
		path = strings.TrimPrefix(d, "sqlite://")
	} else {
		// default to sqlite path
		drv = "sqlite"
		dialect = "sqlite"
		path = d
	}
	var err error
	db, err = sql.Open(drv, path)
	if err != nil {
		return nil, err
	}
	s := &SQLSink{db: db, dialect: dialect}
	if err := s.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLSink) ensureSchema(ctx context.Context) error {
	var stmts []string
	if s.dialect == "sqlite" {
		stmts = []string{
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
		}
	} else {
		stmts = []string{
			`CREATE TABLE IF NOT EXISTS process_history(
				id BIGSERIAL PRIMARY KEY,
				occurred_at TIMESTAMPTZ NOT NULL,
				event TEXT NOT NULL,
				name TEXT NOT NULL,
				pid INTEGER NOT NULL,
				started_at TIMESTAMPTZ NOT NULL,
				stopped_at TIMESTAMPTZ NULL,
				running BOOLEAN NOT NULL,
				exit_err TEXT NULL,
				uniq TEXT NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_process_history_name ON process_history(name);`,
			`CREATE INDEX IF NOT EXISTS idx_process_history_uniq ON process_history(uniq);`,
		}
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLSink) Send(ctx context.Context, e Event) error {
	rec := e.Record
	occur := e.OccurredAt.UTC()
	stopped := interface{}(nil)
	if rec.StoppedAt.Valid {
		stopped = rec.StoppedAt.Time.UTC()
	}
	exitErr := interface{}(nil)
	if rec.ExitErr.Valid {
		exitErr = rec.ExitErr.String
	}
	evt := string(e.Type)
	if s.dialect == "sqlite" {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO process_history(occurred_at, event, name, pid, started_at, stopped_at, running, exit_err, uniq)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?);`,
			occur, evt, rec.Name, rec.PID, rec.StartedAt.UTC(), stopped, rec.Running, exitErr, rec.Key())
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO process_history(occurred_at, event, name, pid, started_at, stopped_at, running, exit_err, uniq)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9);`,
		occur, evt, rec.Name, rec.PID, rec.StartedAt.UTC(), stopped, rec.Running, exitErr, rec.Key())
	return err
}

func (s *SQLSink) Close() error { return s.db.Close() }
