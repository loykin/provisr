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
	prometheusadapter "github.com/loykin/dbstore/adapters/prometheus"
	sqlxadapter "github.com/loykin/dbstore/adapters/sqlx"
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
	sqlxadapter.Source
	adapter *sqlxadapter.Adapter
}

type Options struct {
	Migrate bool
}

// New creates a new PostgreSQL-backed history sink.
// DSN format: postgres://user:pass@host:port/db?sslmode=disable
func New(dsn string) (*Sink, error) {
	return NewWithOptions(dsn, Options{Migrate: true})
}

func NewWithOptions(dsn string, options Options) (*Sink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty PostgreSQL DSN")
	}

	adapter := sqlxadapter.New()
	adapter.RegisterDriver("pgx", sqlxadapter.NewDriver("pgx"))
	adapter.SetObserver(prometheusadapter.New("provisr_history_postgres", nil))
	if err := adapter.Open(source, dbstore.SourceConfig{
		Driver:     "pgx",
		DSN:        dsn,
		PoolConfig: dbstore.DefaultPoolConfig,
	}); err != nil {
		return nil, fmt.Errorf("register postgres pool: %w", err)
	}

	src := sqlxadapter.NewSource(source, adapter.Executor())
	if options.Migrate {
		if err := src.Run(context.Background(), func(ctx context.Context, db *sqlx.DB) error {
			return migrate(ctx, db)
		}); err != nil {
			adapter.Close()
			return nil, err
		}
	}

	return &Sink{Source: src, adapter: adapter}, nil
}

func migrate(ctx context.Context, db *sqlx.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
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
			`INSERT INTO process_history(timestamp, pid, name, status, error) VALUES($1, $2, $3, $4, NULL)`,
			e.OccurredAt.UTC(), rec.PID, rec.Name, rec.LastStatus)
		return err
	})
}

// List returns the most recent history rows, newest first. If name is empty,
// rows for all processes are returned. limit is capped at 500 (defaults to 100).
func (s *Sink) List(ctx context.Context, name string, limit, offset int) ([]corehistory.Entry, error) {
	name = strings.TrimSpace(name)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var rows []corehistory.Entry
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.SelectContext(ctx, &rows,
				`SELECT timestamp, pid, name, status, error FROM process_history ORDER BY timestamp DESC LIMIT $1 OFFSET $2`, limit, offset)
		}
		return db.SelectContext(ctx, &rows,
			`SELECT timestamp, pid, name, status, error FROM process_history WHERE name ILIKE $1 ESCAPE E'\\' ORDER BY timestamp DESC LIMIT $2 OFFSET $3`, containsPattern(name), limit, offset)
	})
	return rows, err
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

func (s *Sink) Count(ctx context.Context, name string) (int, error) {
	name = strings.TrimSpace(name)
	var total int
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if name == "" {
			return db.GetContext(ctx, &total, `SELECT COUNT(*) FROM process_history`)
		}
		return db.GetContext(ctx, &total,
			`SELECT COUNT(*) FROM process_history WHERE name ILIKE $1 ESCAPE E'\\'`, containsPattern(name))
	})
	return total, err
}

func (s *Sink) PruneBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var deleted int64
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		result, err := db.ExecContext(ctx, `DELETE FROM process_history WHERE timestamp < $1`, cutoff.UTC())
		if err != nil {
			return err
		}
		deleted, err = result.RowsAffected()
		return err
	})
	return deleted, err
}

func (s *Sink) Close() error {
	s.adapter.Close()
	return nil
}

// compile-time check that Sink satisfies corehistory.Sink
var _ corehistory.Sink = (*Sink)(nil)
var _ corehistory.Reader = (*Sink)(nil)
var _ corehistory.Pruner = (*Sink)(nil)
