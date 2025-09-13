package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/loykin/provisr/internal/store"
)

// DB implements store.Store for SQLite (modernc.org/sqlite driver, CGO-free).
// DSN is a filesystem path to the SQLite database file. Use ":memory:" for in-memory.

type DB struct {
	db *sql.DB
}

// New opens a SQLite database at path.
func New(path string) (*DB, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, errors.New("empty sqlite path")
	}
	d, err := sql.Open("sqlite", p)
	if err != nil {
		return nil, err
	}
	// busy timeout helps with short concurrent locks
	_, _ = d.Exec("PRAGMA busy_timeout=3000;")
	return &DB{db: d}, nil
}

func (s *DB) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS process_state(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			pid INTEGER NOT NULL,
			started_at TIMESTAMP NOT NULL,
			stopped_at TIMESTAMP NULL,
			running BOOLEAN NOT NULL,
			exit_err TEXT NULL,
			uniq TEXT NOT NULL UNIQUE,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_process_state_name ON process_state(name);`,
		`CREATE INDEX IF NOT EXISTS idx_process_state_running ON process_state(running);`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *DB) Close() error { return s.db.Close() }

func (s *DB) RecordStart(ctx context.Context, rec store.Record) error {
	rec.Running = true
	rec.StoppedAt = sql.NullTime{}
	rec.ExitErr = sql.NullString{}
	rec.UpdatedAt = time.Now().UTC()
	uniq := rec.Key()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO process_state(name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at)
		VALUES(?, ?, ?, NULL, 1, NULL, ?, ?)
		ON CONFLICT(uniq) DO UPDATE SET
			name=excluded.name,
			pid=excluded.pid,
			started_at=excluded.started_at,
			running=excluded.running,
			stopped_at=NULL,
			exit_err=NULL,
			updated_at=excluded.updated_at;`,
		rec.Name, rec.PID, rec.StartedAt.UTC(), uniq, rec.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *DB) RecordStop(ctx context.Context, uniq string, stoppedAt time.Time, exitErr error) error {
	var errStr sql.NullString
	if exitErr != nil {
		errStr = sql.NullString{String: exitErr.Error(), Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE process_state
		SET running=0, stopped_at=?, exit_err=?, updated_at=?
		WHERE uniq=?;`,
		stoppedAt.UTC(), errStr, time.Now().UTC(), uniq)
	if err != nil {
		return err
	}
	return nil
}

func (s *DB) UpsertStatus(ctx context.Context, rec store.Record) error {
	rec.UpdatedAt = time.Now().UTC()
	uniq := rec.Key()
	stoppedAt := interface{}(nil)
	if rec.StoppedAt.Valid {
		stoppedAt = rec.StoppedAt.Time.UTC()
	}
	var exitErr any
	if rec.ExitErr.Valid {
		exitErr = rec.ExitErr.String
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO process_state(name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(uniq) DO UPDATE SET
			name=excluded.name,
			pid=excluded.pid,
			started_at=excluded.started_at,
			stopped_at=excluded.stopped_at,
			running=excluded.running,
			exit_err=excluded.exit_err,
			updated_at=excluded.updated_at;`,
		rec.Name, rec.PID, rec.StartedAt.UTC(), stoppedAt, rec.Running, exitErr, uniq, rec.UpdatedAt)
	return err
}

func (s *DB) GetByName(ctx context.Context, name string, limit int) ([]store.Record, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at
		FROM process_state
		WHERE name=?
		ORDER BY started_at DESC
		LIMIT ?;`, name, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanRecords(rows)
}

func (s *DB) GetRunning(ctx context.Context, namePrefix string) ([]store.Record, error) {
	like := strings.TrimSpace(namePrefix) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at
		FROM process_state
		WHERE running=1 AND name LIKE ?
		ORDER BY updated_at DESC;`, like)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanRecords(rows)
}

func (s *DB) PurgeOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM process_state WHERE running=0 AND updated_at < ?;`, olderThan.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func scanRecords(rows *sql.Rows) ([]store.Record, error) {
	out := make([]store.Record, 0)
	for rows.Next() {
		var r store.Record
		if err := rows.Scan(&r.ID, &r.Name, &r.PID, &r.StartedAt, &r.StoppedAt, &r.Running, &r.ExitErr, &r.Uniq, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
