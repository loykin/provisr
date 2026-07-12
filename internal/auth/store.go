package auth

import (
	"errors"

	"github.com/loykin/provisr/internal/store"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAlreadyBootstrapped = errors.New("an admin user already exists")
	ErrLastActiveAdmin     = errors.New("at least one active admin must remain")
)

// Auth uses the store contracts as its persistence boundary. OAuth-style
// client credentials are intentionally excluded; service accounts are users
// with restricted roles.
type (
	User      = store.User
	UserStore = store.UserStore
	Store     = store.AuthStore
)

var (
	ErrUserNotFound      = store.ErrUserNotFound
	ErrUserAlreadyExists = store.ErrUserAlreadyExists
)

// StoreConfig represents configuration for the auth store
type StoreConfig struct {
	Type         string `toml:"type" yaml:"type" json:"type"` // "sqlite" or "postgresql"
	Migrate      *bool  `toml:"migrate,omitempty" yaml:"migrate,omitempty" json:"migrate,omitempty"`
	Path         string `toml:"path,omitempty" yaml:"path,omitempty" json:"path,omitempty"`
	Host         string `toml:"host,omitempty" yaml:"host,omitempty" json:"host,omitempty"`
	Port         int    `toml:"port,omitempty" yaml:"port,omitempty" json:"port,omitempty"`
	Database     string `toml:"database,omitempty" yaml:"database,omitempty" json:"database,omitempty"`
	Username     string `toml:"username,omitempty" yaml:"username,omitempty" json:"username,omitempty"`
	Password     string `toml:"password,omitempty" yaml:"password,omitempty" json:"password,omitempty"`
	SSLMode      string `toml:"ssl_mode,omitempty" yaml:"ssl_mode,omitempty" json:"ssl_mode,omitempty"`
	MaxOpenConns int    `toml:"max_open_conns,omitempty" yaml:"max_open_conns,omitempty" json:"max_open_conns,omitempty"`
	MaxIdleConns int    `toml:"max_idle_conns,omitempty" yaml:"max_idle_conns,omitempty" json:"max_idle_conns,omitempty"`
}

// NewStore creates a new auth store based on the configuration
func NewStore(config StoreConfig) (Store, error) {
	storeConfig := store.Config{
		Type:         config.Type,
		Migrate:      config.Migrate,
		Path:         config.Path,
		Host:         config.Host,
		Port:         config.Port,
		Database:     config.Database,
		Username:     config.Username,
		Password:     config.Password,
		SSLMode:      config.SSLMode,
		MaxOpenConns: config.MaxOpenConns,
		MaxIdleConns: config.MaxIdleConns,
	}

	return store.NewAuthStore(storeConfig)
}
