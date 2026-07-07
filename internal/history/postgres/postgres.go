package postgres

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

	_ "github.com/jackc/pgx/v5/stdlib"

	corehistory "github.com/loykin/provisr/core/history"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const source = "process_history"

// Sink writes history events to PostgreSQL via dbstore, and can read them
// back.
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
	db, err := sqlx.Connect("pgx", cfg.DSN)
	if err != nil {
		return nil, err
	}
	d.db = db
	return db, nil
}

func (d *driverAdapter) ApplyPoolConfig(db *sqlx.DB, cfg dbstore.PoolConfig) {
	dbstore.DefaultApplyPoolConfig(db, cfg)
}

// New creates a new PostgreSQL-backed history sink.
// DSN format: postgres://user:pass@host:port/db?sslmode=disable
func New(dsn string) (*Sink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty PostgreSQL DSN")
	}

	adapter := &driverAdapter{}
	registry := dbstore.NewDriverRegistry[*sqlx.DB]()
	registry.Register("pgx", adapter)
	pool := dbstore.NewPool(registry)
	if err := pool.Register(source, dbstore.DriverConfig{
		Driver:     "pgx",
		DSN:        dsn,
		PoolConfig: dbstore.DefaultPoolConfig,
	}); err != nil {
		return nil, fmt.Errorf("register postgres pool: %w", err)
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
	if err := goose.SetDialect("postgres"); err != nil {
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
			`INSERT INTO process_history(timestamp, pid, name, status, error) VALUES($1, $2, $3, $4, NULL)`,
			e.OccurredAt.UTC(), rec.PID, rec.Name, rec.LastStatus)
		return err
	})
}

// List returns the most recent history rows, newest first. If name is empty,
// rows for all processes are returned. limit is capped at 500 (defaults to 100).
func (s *Sink) List(ctx context.Context, name string, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []Record
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.SelectContext(ctx, &rows,
				`SELECT timestamp, pid, name, status, error FROM process_history ORDER BY timestamp DESC LIMIT $1`, limit)
		}
		return db.SelectContext(ctx, &rows,
			`SELECT timestamp, pid, name, status, error FROM process_history WHERE name = $1 ORDER BY timestamp DESC LIMIT $2`, name, limit)
	})
	return rows, err
}

func (s *Sink) Close() error {
	s.pool.RemoveAll()
	return nil
}

// compile-time check that Sink satisfies corehistory.Sink
var _ corehistory.Sink = (*Sink)(nil)
