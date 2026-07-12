package auth

import (
	"errors"

	"github.com/loykin/provisr/internal/auth/store"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAlreadyBootstrapped = errors.New("an admin user already exists")
	ErrLastActiveAdmin     = errors.New("at least one active admin must remain")
)

// Auth uses the store contracts as its persistence boundary.
type (
	User      = store.User
	UserStore = store.UserStore
	Store     = store.AuthStore
)

var (
	ErrUserNotFound      = store.ErrUserNotFound
	ErrUserAlreadyExists = store.ErrUserAlreadyExists
)

type StoreConfig = store.Config

// NewStore creates a new auth store based on the configuration
func NewStore(config StoreConfig) (Store, error) {
	return store.NewAuthStore(config)
}
