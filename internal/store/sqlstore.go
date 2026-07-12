package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/loykin/dbstore"
	prometheusadapter "github.com/loykin/dbstore/adapters/prometheus"
	sqlxadapter "github.com/loykin/dbstore/adapters/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const genericStoreSource = "generic"

// sqlStore is the dbstore-backed implementation shared by both SQLite and
// PostgreSQL generic stores — same pattern as authStore (authstore.go): the
// only per-dialect difference is the DSN/driver, handled in the constructors
// below. Embedding sqlxadapter.Source means Ping goes through dbstore's
// throttle/lifecycle like every other query, same as the repo pattern used
// by the history sinks and authStore.
type sqlStore struct {
	sqlxadapter.Source
	adapter *sqlxadapter.Adapter
}

// SQLiteStore implements Store backed by SQLite.
type SQLiteStore struct{ *sqlStore }

// PostgreSQLStore implements Store backed by PostgreSQL.
type PostgreSQLStore struct{ *sqlStore }

func newSQLStore(driverName, dsn string, poolCfg dbstore.PoolConfig) (*sqlStore, error) {
	adapter := sqlxadapter.New()
	adapter.RegisterDriver(driverName, sqlxadapter.NewDriver(driverName))
	adapter.SetObserver(prometheusadapter.New("provisr_store", nil))
	if err := adapter.Open(genericStoreSource, dbstore.SourceConfig{
		Driver:     driverName,
		DSN:        dsn,
		PoolConfig: poolCfg,
	}); err != nil {
		return nil, fmt.Errorf("register store pool: %w", err)
	}

	return &sqlStore{
		Source:  sqlxadapter.NewSource(genericStoreSource, adapter.Executor()),
		adapter: adapter,
	}, nil
}

// NewSQLiteStore creates a new SQLite store.
func NewSQLiteStore(config Config) (*SQLiteStore, error) {
	path := config.Path
	if path == "" {
		path = ":memory:"
	}
	dsn := path + "?_journal=WAL&_timeout=5000&_fk=1"

	poolCfg := dbstore.PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1, MaxIdleTime: 5 * time.Minute, MaxConcurrency: 1}
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
	s.adapter.Close()
	return nil
}

func (s *sqlStore) Ping(ctx context.Context) error {
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		return db.PingContext(ctx)
	})
}
