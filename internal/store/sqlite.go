package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store interface using SQLite
type SQLiteStore struct {
	db     *sql.DB
	config Config
}

// SQLiteTransaction wraps a SQLite transaction
type SQLiteTransaction struct {
	*SQLiteStore
	tx *sql.Tx
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(config Config) (*SQLiteStore, error) {
	path := config.Path
	if path == "" {
		path = ":memory:" // In-memory database if no path specified
	}

	db, err := sql.Open("sqlite3", path+"?_journal=WAL&_timeout=5000&_fk=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(1) // SQLite works best with single connection
	}

	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}

	if config.ConnMaxAge > 0 {
		db.SetConnMaxLifetime(config.ConnMaxAge)
	}

	store := &SQLiteStore{
		db:     db,
		config: config,
	}

	// Test connection
	if err := store.Ping(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	return store, nil
}

// Close closes the SQLite connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Ping tests the database connection
func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// BeginTx starts a new transaction
func (s *SQLiteStore) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &SQLiteTransaction{
		SQLiteStore: &SQLiteStore{db: s.db, config: s.config},
		tx:          tx,
	}, nil
}

// GetDB returns the underlying database connection or transaction
func (s *SQLiteStore) GetDB() interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
} {
	return s.db
}

// Commit commits the transaction
func (t *SQLiteTransaction) Commit() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction")
	}
	return t.tx.Commit()
}

// Rollback rolls back the transaction
func (t *SQLiteTransaction) Rollback() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction")
	}
	return t.tx.Rollback()
}

// GetDB returns the transaction instead of the main connection
func (t *SQLiteTransaction) GetDB() interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
} {
	return t.tx
}

// CreateTables creates the required tables with the given schema
func (s *SQLiteStore) CreateTables(schemas []string) error {
	for _, schema := range schemas {
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("failed to create table with schema %q: %w", schema, err)
		}
	}
	return nil
}

// EnableForeignKeys enables foreign key constraints
func (s *SQLiteStore) EnableForeignKeys() error {
	_, err := s.db.Exec("PRAGMA foreign_keys = ON")
	return err
}

// GetTablePrefix returns the configured table prefix
func (s *SQLiteStore) GetTablePrefix() string {
	return s.config.TablePrefix
}
