package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/loykin/dbstore"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const genericStoreSource = "generic"

// sqlStore is the dbstore-backed implementation shared by both SQLite and
// PostgreSQL generic stores — same pattern as authStore (authstore.go): the
// only per-dialect difference is the DSN/driver, handled in the constructors
// below. Embedding SQLRepo (not just holding a raw db handle) means Ping
// goes through the pool's throttle/lifecycle like every other query, same
// as the repo pattern used by the history sinks and authStore.
type sqlStore struct {
	dbstore.SQLRepo
	pool *dbstore.Pool[*sqlx.DB]
	db   *sqlx.DB // direct handle, kept only for BeginTx/Transaction compat
}

// SQLiteStore implements Store backed by SQLite.
type SQLiteStore struct{ *sqlStore }

// PostgreSQLStore implements Store backed by PostgreSQL.
type PostgreSQLStore struct{ *sqlStore }

type genericDriverAdapter struct {
	driverName string
	db         *sqlx.DB
}

func (d *genericDriverAdapter) Open(cfg dbstore.DriverConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect(d.driverName, cfg.DSN)
	if err != nil {
		return nil, err
	}
	d.db = db
	return db, nil
}

func (d *genericDriverAdapter) ApplyPoolConfig(db *sqlx.DB, cfg dbstore.PoolConfig) {
	dbstore.DefaultApplyPoolConfig(db, cfg)
}

func newSQLStore(driverName, dsn string, poolCfg dbstore.PoolConfig) (*sqlStore, error) {
	adapter := &genericDriverAdapter{driverName: driverName}
	registry := dbstore.NewDriverRegistry[*sqlx.DB]()
	registry.Register(driverName, adapter)
	pool := dbstore.NewPool(registry)
	if err := pool.Register(genericStoreSource, dbstore.DriverConfig{
		Driver:     driverName,
		DSN:        dsn,
		PoolConfig: poolCfg,
	}); err != nil {
		return nil, fmt.Errorf("register store pool: %w", err)
	}

	executor := dbstore.NewExecutor(pool)
	return &sqlStore{
		SQLRepo: dbstore.NewSQLRepo(genericStoreSource, executor),
		pool:    pool,
		db:      adapter.db,
	}, nil
}

// NewSQLiteStore creates a new SQLite store.
func NewSQLiteStore(config Config) (*SQLiteStore, error) {
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

	s, err := newSQLStore("sqlite", dsn, poolCfg)
	if err != nil {
		return nil, err
	}
	return &SQLiteStore{sqlStore: s}, nil
}

// NewPostgreSQLStore creates a new PostgreSQL store.
func NewPostgreSQLStore(config Config) (*PostgreSQLStore, error) {
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

	s, err := newSQLStore("pgx", dsn, poolCfg)
	if err != nil {
		return nil, err
	}
	return &PostgreSQLStore{sqlStore: s}, nil
}

func (s *sqlStore) Close() error {
	s.pool.RemoveAll()
	return nil
}

func (s *sqlStore) Ping(ctx context.Context) error {
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		return db.PingContext(ctx)
	})
}

// BeginTx starts a new transaction. Kept for Store interface compatibility.
func (s *sqlStore) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &sqlStoreTx{sqlStore: s, tx: tx}, nil
}

type sqlStoreTx struct {
	*sqlStore
	tx *sqlx.Tx
}

func (t *sqlStoreTx) Commit() error   { return t.tx.Commit() }
func (t *sqlStoreTx) Rollback() error { return t.tx.Rollback() }
