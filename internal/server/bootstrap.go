package server

import (
	"context"
	"fmt"

	"github.com/loykin/provisr/internal/auth"
	"github.com/loykin/provisr/internal/config"
)

// newAuthServiceFromConfig builds an auth.AuthService the same way
// newRouterFromConfig does, factored out so callers that need a standalone
// AuthService (e.g. the serve command's initial-admin bootstrap) don't have
// to duplicate the Store field mapping a third time.
func newAuthServiceFromConfig(authCfg *config.AuthConfig) (*auth.AuthService, error) {
	return auth.NewAuthService(auth.AuthConfig{
		Store: auth.StoreConfig{
			Type:         authCfg.Store.Type,
			Path:         authCfg.Store.Path,
			Host:         authCfg.Store.Host,
			Port:         authCfg.Store.Port,
			Database:     authCfg.Store.Database,
			Username:     authCfg.Store.Username,
			Password:     authCfg.Store.Password,
			SSLMode:      authCfg.Store.SSLMode,
			MaxOpenConns: authCfg.Store.MaxOpenConns,
			MaxIdleConns: authCfg.Store.MaxIdleConns,
		},
		JWTSecret:  authCfg.JWTSecret,
		TokenTTL:   authCfg.TokenTTL,
		BcryptCost: authCfg.BcryptCost,
	})
}

// EnsureInitialAdmin creates an "admin" user with a random password if the
// configured auth store has no users yet — otherwise enabling [server.auth]
// on a fresh store locks every operator out until someone with separate
// shell access runs `provisr auth user create`. A no-op (created=false, no
// error) once any user exists, so it's safe to call on every `provisr
// serve` startup. The returned password is only ever held in memory; the
// caller must display it once and is responsible for not logging it
// anywhere durable.
func EnsureInitialAdmin(authCfg *config.AuthConfig) (password string, created bool, err error) {
	if authCfg == nil || !authCfg.Enabled {
		return "", false, nil
	}

	authService, err := newAuthServiceFromConfig(authCfg)
	if err != nil {
		return "", false, fmt.Errorf("failed to open auth store: %w", err)
	}
	defer func() { _ = authService.Close() }()

	helper := auth.NewCLIHelper(authService)
	return helper.EnsureInitialAdmin(context.Background(), "admin")
}
