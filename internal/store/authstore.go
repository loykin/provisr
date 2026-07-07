package store

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/loykin/dbstore"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

//go:embed migrations/sqlite/*.sql
var sqliteAuthMigrationsFS embed.FS

//go:embed migrations/postgres/*.sql
var postgresAuthMigrationsFS embed.FS

const authSource = "auth"

// authStore is the dbstore-backed implementation shared by both SQLite and
// PostgreSQL auth stores. The only per-dialect differences are the
// DSN/driver/migration dialect, handled entirely in the constructors below;
// every CRUD method (auth.go, auth_client.go) is written once against
// *sqlx.DB and uses db.Rebind to adapt "?" placeholders to each driver.
type authStore struct {
	dbstore.SQLRepo
	pool *dbstore.Pool[*sqlx.DB]
	db   *sqlx.DB // direct handle, kept only for BeginTx/Transaction compat
}

// SQLiteAuthStore implements AuthStore backed by SQLite.
type SQLiteAuthStore struct{ *authStore }

// PostgreSQLAuthStore implements AuthStore backed by PostgreSQL.
type PostgreSQLAuthStore struct{ *authStore }

type authDriverAdapter struct {
	driverName string
	db         *sqlx.DB
}

func (d *authDriverAdapter) Open(cfg dbstore.DriverConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect(d.driverName, cfg.DSN)
	if err != nil {
		return nil, err
	}
	d.db = db
	return db, nil
}

func (d *authDriverAdapter) ApplyPoolConfig(db *sqlx.DB, cfg dbstore.PoolConfig) {
	dbstore.DefaultApplyPoolConfig(db, cfg)
}

func newAuthStore(driverName, dsn string, poolCfg dbstore.PoolConfig, migrationsFS embed.FS, dialect goose.Dialect) (*authStore, error) {
	adapter := &authDriverAdapter{driverName: driverName}
	registry := dbstore.NewDriverRegistry[*sqlx.DB]()
	registry.Register(driverName, adapter)
	pool := dbstore.NewPool(registry)
	if err := pool.Register(authSource, dbstore.DriverConfig{
		Driver:     driverName,
		DSN:        dsn,
		PoolConfig: poolCfg,
	}); err != nil {
		return nil, fmt.Errorf("register auth store pool: %w", err)
	}

	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect(string(dialect)); err != nil {
		pool.RemoveAll()
		return nil, fmt.Errorf("goose set dialect: %w", err)
	}
	dir := "migrations/sqlite"
	if dialect == goose.DialectPostgres {
		dir = "migrations/postgres"
	}
	if err := goose.RunContext(context.Background(), "up", adapter.db.DB, dir); err != nil {
		pool.RemoveAll()
		return nil, fmt.Errorf("goose up: %w", err)
	}

	executor := dbstore.NewExecutor(pool)
	return &authStore{
		SQLRepo: dbstore.NewSQLRepo(authSource, executor),
		pool:    pool,
		db:      adapter.db,
	}, nil
}

// NewSQLiteAuthStore creates a new SQLite auth store.
func NewSQLiteAuthStore(config Config) (*SQLiteAuthStore, error) {
	path := config.Path
	if path == "" {
		path = ":memory:"
	}
	dsn := path + "?_journal=WAL&_timeout=5000&_fk=1"

	poolCfg := dbstore.PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1, MaxIdleTime: 5 * time.Minute}
	if config.MaxOpenConns > 0 {
		poolCfg.MaxOpenConns = config.MaxOpenConns
	}
	if config.MaxIdleConns > 0 {
		poolCfg.MaxIdleConns = config.MaxIdleConns
	}

	s, err := newAuthStore("sqlite", dsn, poolCfg, sqliteAuthMigrationsFS, goose.DialectSQLite3)
	if err != nil {
		return nil, err
	}
	return &SQLiteAuthStore{authStore: s}, nil
}

// NewPostgreSQLAuthStore creates a new PostgreSQL auth store.
func NewPostgreSQLAuthStore(config Config) (*PostgreSQLAuthStore, error) {
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port == 0 {
		config.Port = 5432
	}
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.Username, config.Password, config.Database, config.SSLMode)
	for key, value := range config.Options {
		dsn += fmt.Sprintf(" %s=%s", key, value)
	}

	poolCfg := dbstore.DefaultPoolConfig
	if config.MaxOpenConns > 0 {
		poolCfg.MaxOpenConns = config.MaxOpenConns
	}
	if config.MaxIdleConns > 0 {
		poolCfg.MaxIdleConns = config.MaxIdleConns
	}
	if config.ConnMaxAge > 0 {
		poolCfg.MaxLifetime = config.ConnMaxAge
	}

	s, err := newAuthStore("pgx", dsn, poolCfg, postgresAuthMigrationsFS, goose.DialectPostgres)
	if err != nil {
		return nil, err
	}
	return &PostgreSQLAuthStore{authStore: s}, nil
}

func (s *authStore) Close() error {
	s.pool.RemoveAll()
	return nil
}

func (s *authStore) Ping(ctx context.Context) error {
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		return db.PingContext(ctx)
	})
}

// BeginTx starts a new transaction. Kept for Store interface compatibility;
// none of provisr's own code calls it today.
func (s *authStore) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &authStoreTx{authStore: s, tx: tx}, nil
}

type authStoreTx struct {
	*authStore
	tx *sqlx.Tx
}

func (t *authStoreTx) Commit() error   { return t.tx.Commit() }
func (t *authStoreTx) Rollback() error { return t.tx.Rollback() }
