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
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	corehistory "github.com/loykin/provisr/core/history"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const source = "process_history"

// Sink writes history events to SQLite via dbstore, and can read them back.
type Sink struct {
	dbstore.BaseRepo[*sqlx.DB]
	pool *dbstore.Pool[*sqlx.DB]
}

// Record is a single stored history row.
type Record struct {
	Timestamp time.Time `db:"timestamp" json:"timestamp"`
	PID       int       `db:"pid" json:"pid"`
	Name      string    `db:"name" json:"name"`
	Status    string    `db:"status" json:"status"`
	Error     *string   `db:"error" json:"error,omitempty"`
}

type driverAdapter struct {
	db *sqlx.DB
}

func (d *driverAdapter) Open(cfg dbstore.DriverConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite", cfg.DSN)
	if err != nil {
		return nil, err
	}
	d.db = db
	return db, nil
}

func (d *driverAdapter) ApplyPoolConfig(db *sqlx.DB, cfg dbstore.PoolConfig) {
	dbstore.DefaultApplyPoolConfig(db, cfg)
	// sqlite only supports a single writer at a time.
	db.SetMaxOpenConns(1)
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

	adapter := &driverAdapter{}
	registry := dbstore.NewDriverRegistry[*sqlx.DB]()
	registry.Register("sqlite", adapter)
	pool := dbstore.NewPool(registry)
	if err := pool.Register(source, dbstore.DriverConfig{
		Driver: "sqlite",
		DSN:    dsn + "?_journal=WAL&_timeout=5000&_fk=1",
		PoolConfig: dbstore.PoolConfig{
			MaxOpenConns: 1,
			MaxIdleConns: 1,
			MaxIdleTime:  5 * time.Minute,
		},
	}); err != nil {
		return nil, fmt.Errorf("register sqlite pool: %w", err)
	}

	if err := migrate(adapter.db); err != nil {
		pool.RemoveAll()
		return nil, err
	}

	executor := dbstore.NewExecutor(pool)
	return &Sink{BaseRepo: dbstore.NewBaseRepo(source, executor), pool: pool}, nil
}

func migrate(db *sqlx.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.RunContext(context.Background(), "up", db.DB, "migrations"); err != nil {
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

// List returns history rows newest-first, paginated by limit/offset. If name
// is empty, rows for all processes are returned. limit is capped at 500
// (defaults to 100); offset must be >= 0.
func (s *Sink) List(ctx context.Context, name string, limit, offset int) ([]Record, error) {
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
			`SELECT timestamp, pid, name, status, error FROM process_history WHERE name = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`, name, limit, offset)
	})
	return rows, err
}

// Count returns the total number of history rows, optionally filtered by
// name, so callers can compute page counts for List.
func (s *Sink) Count(ctx context.Context, name string) (int, error) {
	var total int
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.GetContext(ctx, &total, `SELECT COUNT(*) FROM process_history`)
		}
		return db.GetContext(ctx, &total, `SELECT COUNT(*) FROM process_history WHERE name = ?`, name)
	})
	return total, err
}

func (s *Sink) Close() error {
	s.pool.RemoveAll()
	return nil
}
