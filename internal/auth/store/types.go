package store

import (
	"context"
)

// Config represents configuration for different store types
type Config struct {
	Type    string `mapstructure:"type" toml:"type" yaml:"type" json:"type"`
	Migrate *bool  `mapstructure:"migrate" toml:"migrate,omitempty" yaml:"migrate,omitempty" json:"migrate,omitempty"`

	// SQLite specific
	Path string `mapstructure:"path" toml:"path,omitempty" yaml:"path,omitempty" json:"path,omitempty"`

	// PostgreSQL specific
	Host     string `mapstructure:"host" toml:"host,omitempty" yaml:"host,omitempty" json:"host,omitempty"`
	Port     int    `mapstructure:"port" toml:"port,omitempty" yaml:"port,omitempty" json:"port,omitempty"`
	Database string `mapstructure:"database" toml:"database,omitempty" yaml:"database,omitempty" json:"database,omitempty"`
	Username string `mapstructure:"username" toml:"username,omitempty" yaml:"username,omitempty" json:"username,omitempty"`
	Password string `mapstructure:"password" toml:"password,omitempty" yaml:"password,omitempty" json:"password,omitempty"`
	SSLMode  string `mapstructure:"ssl_mode" toml:"ssl_mode,omitempty" yaml:"ssl_mode,omitempty" json:"ssl_mode,omitempty"`

	// Connection pooling
	MaxOpenConns int `mapstructure:"max_open_conns" toml:"max_open_conns,omitempty" yaml:"max_open_conns,omitempty" json:"max_open_conns,omitempty"`
	MaxIdleConns int `mapstructure:"max_idle_conns" toml:"max_idle_conns,omitempty" yaml:"max_idle_conns,omitempty" json:"max_idle_conns,omitempty"`
}

// Store defines the connection lifecycle required by authentication storage.
type Store interface {
	Close() error
	Ping(ctx context.Context) error
}
