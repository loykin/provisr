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
	// For in-memory databases, ensure a single underlying connection so the
	// schema and data are visible across all operations. With multiple
	// connections, each would get its own isolated :memory: DB.
	if p == ":memory:" {
		d.SetMaxOpenConns(1)
	}
	// busy timeout helps with short concurrent locks
	_, _ = d.Exec("PRAGMA busy_timeout=3000;")
	return &DB{db: d}, nil
}

func (s *DB) EnsureSchema(ctx context.Context) error {
	stmt := `CREATE TABLE IF NOT EXISTS process_state(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		pid INTEGER NOT NULL,
		last_status TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);`
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

func (s *DB) Close() error { return s.db.Close() }

// Record upserts the last known state for a process name.
func (s *DB) Record(ctx context.Context, rec store.Record) error {
	if strings.TrimSpace(rec.Name) == "" {
		return errors.New("empty name")
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO process_state(name, pid, last_status, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			pid=excluded.pid,
			last_status=excluded.last_status,
			updated_at=excluded.updated_at;`,
		rec.Name, rec.PID, rec.LastStatus, rec.UpdatedAt)
	return err
}

// GetByName returns the last known record for the given name. If not found, returns zero-value record and sql.ErrNoRows.
func (s *DB) GetByName(ctx context.Context, name string) (store.Record, error) {
	var r store.Record
	row := s.db.QueryRowContext(ctx, `SELECT name, pid, last_status, updated_at FROM process_state WHERE name=?;`, name)
	err := row.Scan(&r.Name, &r.PID, &r.LastStatus, &r.UpdatedAt)
	if err != nil {
		return store.Record{}, err
	}
	return r, nil
}

// Delete removes a record by name. It is not an error if the record does not exist.
func (s *DB) Delete(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("empty name")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM process_state WHERE name=?;`, name)
	return err
}
