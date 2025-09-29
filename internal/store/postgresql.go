package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgreSQLStore implements Store interface using PostgreSQL
type PostgreSQLStore struct {
	db     *sql.DB
	config Config
}

// PostgreSQLTransaction wraps a PostgreSQL transaction
type PostgreSQLTransaction struct {
	*PostgreSQLStore
	tx *sql.Tx
}

// NewPostgreSQLStore creates a new PostgreSQL store
func NewPostgreSQLStore(config Config) (*PostgreSQLStore, error) {
	// Set defaults
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

	// Add additional options
	for key, value := range config.Options {
		dsn += fmt.Sprintf(" %s=%s", key, value)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgresql database: %w", err)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25) // Default for PostgreSQL
	}

	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}

	if config.ConnMaxAge > 0 {
		db.SetConnMaxLifetime(config.ConnMaxAge)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	store := &PostgreSQLStore{
		db:     db,
		config: config,
	}

	// Test connection
	if err := store.Ping(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping postgresql database: %w", err)
	}

	return store, nil
}

// Close closes the PostgreSQL connection
func (s *PostgreSQLStore) Close() error {
	return s.db.Close()
}

// Ping tests the database connection
func (s *PostgreSQLStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// BeginTx starts a new transaction
func (s *PostgreSQLStore) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &PostgreSQLTransaction{
		PostgreSQLStore: &PostgreSQLStore{db: s.db, config: s.config},
		tx:              tx,
	}, nil
}

// GetDB returns the underlying database connection or transaction
func (s *PostgreSQLStore) GetDB() interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
} {
	return s.db
}

// Commit commits the transaction
func (t *PostgreSQLTransaction) Commit() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction")
	}
	return t.tx.Commit()
}

// Rollback rolls back the transaction
func (t *PostgreSQLTransaction) Rollback() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction")
	}
	return t.tx.Rollback()
}

// GetDB returns the transaction instead of the main connection
func (t *PostgreSQLTransaction) GetDB() interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
} {
	return t.tx
}

// CreateTables creates the required tables with the given schema
func (s *PostgreSQLStore) CreateTables(schemas []string) error {
	for _, schema := range schemas {
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("failed to create table with schema %q: %w", schema, err)
		}
	}
	return nil
}

// CreateExtension creates a PostgreSQL extension if it doesn't exist
func (s *PostgreSQLStore) CreateExtension(name string) error {
	query := fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %q", name)
	_, err := s.db.Exec(query)
	return err
}

// GetTablePrefix returns the configured table prefix
func (s *PostgreSQLStore) GetTablePrefix() string {
	return s.config.TablePrefix
}
