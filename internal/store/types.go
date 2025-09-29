package store

import (
	"context"
	"time"
)

// Config represents configuration for different store types
type Config struct {
	Type string `toml:"type" yaml:"type" json:"type"` // "sqlite", "postgresql", "memory", etc.

	// SQLite specific
	Path string `toml:"path,omitempty" yaml:"path,omitempty" json:"path,omitempty"`

	// PostgreSQL specific
	Host     string `toml:"host,omitempty" yaml:"host,omitempty" json:"host,omitempty"`
	Port     int    `toml:"port,omitempty" yaml:"port,omitempty" json:"port,omitempty"`
	Database string `toml:"database,omitempty" yaml:"database,omitempty" json:"database,omitempty"`
	Username string `toml:"username,omitempty" yaml:"username,omitempty" json:"username,omitempty"`
	Password string `toml:"password,omitempty" yaml:"password,omitempty" json:"password,omitempty"`
	SSLMode  string `toml:"ssl_mode,omitempty" yaml:"ssl_mode,omitempty" json:"ssl_mode,omitempty"`

	// Connection pooling
	MaxOpenConns int           `toml:"max_open_conns,omitempty" yaml:"max_open_conns,omitempty" json:"max_open_conns,omitempty"`
	MaxIdleConns int           `toml:"max_idle_conns,omitempty" yaml:"max_idle_conns,omitempty" json:"max_idle_conns,omitempty"`
	ConnMaxAge   time.Duration `toml:"conn_max_age,omitempty" yaml:"conn_max_age,omitempty" json:"conn_max_age,omitempty"`

	// Additional options
	TablePrefix string            `toml:"table_prefix,omitempty" yaml:"table_prefix,omitempty" json:"table_prefix,omitempty"`
	Options     map[string]string `toml:"options,omitempty" yaml:"options,omitempty" json:"options,omitempty"`
}

// Store represents a generic data store interface
type Store interface {
	// Connection management
	Close() error
	Ping(ctx context.Context) error

	// Transaction support
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
	Store
	Commit() error
	Rollback() error
}

// Repository represents a generic repository pattern
type Repository[T any] interface {
	// CRUD operations
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id string) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, offset, limit int) ([]*T, int, error)

	// Query operations
	FindBy(ctx context.Context, criteria map[string]interface{}) ([]*T, error)
	Count(ctx context.Context, criteria map[string]interface{}) (int, error)
}

// KeyValueStore represents a simple key-value store
type KeyValueStore interface {
	Store
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Keys(ctx context.Context, pattern string) ([]string, error)
}

// CacheStore represents a caching layer
type CacheStore interface {
	KeyValueStore
	Flush(ctx context.Context) error
	Size(ctx context.Context) (int64, error)
}

// TimeSeriesStore represents a time-series data store
type TimeSeriesStore interface {
	Store
	Write(ctx context.Context, series string, timestamp time.Time, value interface{}, tags map[string]string) error
	Query(ctx context.Context, series string, start, end time.Time, tags map[string]string) ([]TimeSeriesPoint, error)
}

// TimeSeriesPoint represents a single point in time series data
type TimeSeriesPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     interface{}            `json:"value"`
	Tags      map[string]string      `json:"tags"`
	Fields    map[string]interface{} `json:"fields"`
}

// StoreOptions represents options for store operations
type StoreOptions struct {
	Timeout     time.Duration
	Retries     int
	Consistency string // "strong", "eventual", etc.
}

// Factory represents a store factory interface
type Factory interface {
	CreateStore(config Config) (Store, error)
	SupportedTypes() []string
}
