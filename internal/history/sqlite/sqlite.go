package sqlite

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/loykin/dbstore"
	prometheusadapter "github.com/loykin/dbstore/adapters/prometheus"
	sqlxadapter "github.com/loykin/dbstore/adapters/sqlx"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	corehistory "github.com/loykin/provisr/core/history"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const source = "process_history"

// Sink writes history events to SQLite via dbstore, and can read them back.
type Sink struct {
	sqlxadapter.Source
	adapter *sqlxadapter.Adapter
}

// Record is a single stored history row.
type Record struct {
	Timestamp time.Time `db:"timestamp" json:"timestamp"`
	PID       int       `db:"pid" json:"pid"`
	Name      string    `db:"name" json:"name"`
	Status    string    `db:"status" json:"status"`
	Error     *string   `db:"error" json:"error,omitempty"`
}

// New creates a new SQLite-backed history sink.
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

	if strings.HasPrefix(strings.ToLower(dsn), "sqlite://") {
		dsn = strings.TrimPrefix(dsn, "sqlite://")
	}

	adapter := sqlxadapter.New()
	adapter.RegisterDriver(sqlxadapter.DriverSQLite, sqlxadapter.SQLiteDriver())
	adapter.SetObserver(prometheusadapter.New("provisr_history_sqlite", nil))
	if err := adapter.Open(source, dbstore.SourceConfig{
		Driver: "sqlite",
		DSN:    dsn + "?_journal=WAL&_timeout=5000&_fk=1",
		PoolConfig: dbstore.PoolConfig{
			MaxOpenConns:   1,
			MaxIdleConns:   1,
			MaxIdleTime:    5 * time.Minute,
			MaxConcurrency: 1,
		},
	}); err != nil {
		return nil, fmt.Errorf("register sqlite pool: %w", err)
	}

	src := sqlxadapter.NewSource(source, adapter.Executor())
	if err := src.Run(context.Background(), func(ctx context.Context, db *sqlx.DB) error {
		return migrate(ctx, db)
	}); err != nil {
		adapter.Close()
		return nil, err
	}

	return &Sink{Source: src, adapter: adapter}, nil
}

func migrate(ctx context.Context, db *sqlx.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.RunContext(ctx, "up", db.DB, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func (s *Sink) Send(ctx context.Context, e corehistory.Event) error {
	rec := e.Record
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO process_history(timestamp, pid, name, status, error) VALUES(?, ?, ?, ?, NULL)`,
			e.OccurredAt.UTC(), rec.PID, rec.Name, rec.LastStatus)
		return err
	})
}

func containsPattern(value string) string {
	var b strings.Builder
	b.Grow(len(value) + 2)
	b.WriteByte('%')
	for _, r := range value {
		switch r {
		case '%', '_', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('%')
	return b.String()
}

// List returns history rows newest-first, paginated by limit/offset. If name
// is empty, rows for all processes are returned. Otherwise name is treated as
// a case-insensitive contains filter. limit is capped at 500 (defaults to
// 100); offset must be >= 0.
func (s *Sink) List(ctx context.Context, name string, limit, offset int) ([]Record, error) {
	name = strings.TrimSpace(name)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var rows []Record
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.SelectContext(ctx, &rows,
				`SELECT timestamp, pid, name, status, error FROM process_history ORDER BY timestamp DESC LIMIT ? OFFSET ?`, limit, offset)
		}
		return db.SelectContext(ctx, &rows,
			`SELECT timestamp, pid, name, status, error
			 FROM process_history
			 WHERE name LIKE ? ESCAPE '\'
			 ORDER BY timestamp DESC
			 LIMIT ? OFFSET ?`, containsPattern(name), limit, offset)
	})
	return rows, err
}

// Count returns the total number of history rows, optionally filtered by
// name contains search, so callers can compute page counts for List.
func (s *Sink) Count(ctx context.Context, name string) (int, error) {
	name = strings.TrimSpace(name)
	var total int
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.GetContext(ctx, &total, `SELECT COUNT(*) FROM process_history`)
		}
		return db.GetContext(ctx, &total,
			`SELECT COUNT(*) FROM process_history WHERE name LIKE ? ESCAPE '\'`, containsPattern(name))
	})
	return total, err
}

func (s *Sink) Close() error {
	s.adapter.Close()
	return nil
}
