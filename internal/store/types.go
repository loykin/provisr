package store

import (
	"context"
	"time"
)

// Config represents configuration for different store types
type Config struct {
	Type    string `toml:"type" yaml:"type" json:"type"` // "sqlite", "postgresql", "memory", etc.
	Migrate *bool  `toml:"migrate,omitempty" yaml:"migrate,omitempty" json:"migrate,omitempty"`

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
	Options map[string]string `toml:"options,omitempty" yaml:"options,omitempty" json:"options,omitempty"`
}

// Store defines the connection lifecycle required by authentication storage.
type Store interface {
	Close() error
	Ping(ctx context.Context) error
}
