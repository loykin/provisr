// Package store re-exports internal/store for external consumers.
package store

import (
	"github.com/loykin/provisr/internal/store"
)

type (
	Config          = store.Config
	Store           = store.Store
	AuthStore       = store.AuthStore
	User            = store.User
	StoreOptions    = store.StoreOptions
	TimeSeriesPoint = store.TimeSeriesPoint
)

var (
	ErrUserNotFound      = store.ErrUserNotFound
	ErrUserAlreadyExists = store.ErrUserAlreadyExists
)

// CreateStore creates a new store from the given config.
func CreateStore(config Config) (Store, error) { return store.CreateStore(config) }

// NewAuthStore creates a new auth-specific store from the given config.
func NewAuthStore(config Config) (AuthStore, error) { return store.NewAuthStore(config) }

// SupportedTypes returns a list of supported store backend types.
func SupportedTypes() []string { return store.SupportedTypes() }
