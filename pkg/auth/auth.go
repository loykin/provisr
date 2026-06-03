// Package auth re-exports internal/auth for external consumers.
package auth

import (
	"github.com/loykin/provisr/internal/auth"
	"github.com/loykin/provisr/internal/store"
)

type (
	AuthService      = auth.AuthService
	AuthConfig       = auth.AuthConfig
	StoreConfig      = auth.StoreConfig
	AuthMethod       = auth.AuthMethod
	AuthResult       = auth.AuthResult
	Token            = auth.Token
	LoginRequest     = auth.LoginRequest
	User             = store.User
	ClientCredential = store.ClientCredential
)

const (
	AuthMethodBasic        = auth.AuthMethodBasic
	AuthMethodClientSecret = auth.AuthMethodClientSecret
	AuthMethodJWT          = auth.AuthMethodJWT
)

var (
	ErrInvalidCredentials  = auth.ErrInvalidCredentials
	ErrUserNotFound        = auth.ErrUserNotFound
	ErrUserAlreadyExists   = auth.ErrUserAlreadyExists
	ErrClientNotFound      = auth.ErrClientNotFound
	ErrClientAlreadyExists = auth.ErrClientAlreadyExists
)

// NewAuthService creates a new AuthService from the given config.
func NewAuthService(config AuthConfig) (*AuthService, error) {
	return auth.NewAuthService(config)
}
