// Package auth re-exports internal/auth for external consumers.
package auth

import (
	"github.com/loykin/provisr/internal/auth"
	"github.com/loykin/provisr/internal/auth/store"
)

type (
	AuthService  = auth.AuthService
	AuthConfig   = auth.AuthConfig
	StoreConfig  = auth.StoreConfig
	AuthMethod   = auth.AuthMethod
	AuthResult   = auth.AuthResult
	Token        = auth.Token
	LoginRequest = auth.LoginRequest
	User         = store.User
)

const (
	AuthMethodBasic = auth.AuthMethodBasic
	AuthMethodJWT   = auth.AuthMethodJWT
)

// NewAuthService creates a new AuthService from the given config.
func NewAuthService(config AuthConfig) (*AuthService, error) {
	return auth.NewAuthService(config)
}
