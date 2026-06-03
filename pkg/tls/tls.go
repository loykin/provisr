// Package tls re-exports internal/tls for external consumers.
package tls

import (
	cryptotls "crypto/tls"

	cfg "github.com/loykin/provisr/internal/config"
	internaltls "github.com/loykin/provisr/internal/tls"
)

type (
	Builder = internaltls.Builder
	Presets = internaltls.Presets
	// TLSConfig is the configuration struct used by provisr's TLS helpers.
	// It is also accessible as provisr.TLSConfig.
	TLSConfig = cfg.TLSConfig
)

// Default is the preset factory for common TLS configurations.
var Default = internaltls.Default

// NewTLSBuilder returns a builder for constructing TLSConfig values.
func NewTLSBuilder() *Builder { return internaltls.NewTLSBuilder() }

// CreateDevTLS creates a development TLSConfig with auto-generated certificates stored in baseDir.
func CreateDevTLS(baseDir string) (*TLSConfig, error) { return internaltls.CreateDevTLS(baseDir) }

// EasyTLSSetup builds a *crypto/tls.Config for a server listening on listen,
// storing certificates in certDir, optionally auto-generating them.
func EasyTLSSetup(listen, certDir string, autoGen bool) (*cryptotls.Config, error) {
	return internaltls.EasyTLSSetup(listen, certDir, autoGen)
}

// QuickSelfSignedTLS generates a self-signed certificate in certDir and returns
// a *crypto/tls.Config ready for use.
func QuickSelfSignedTLS(certDir string) (*cryptotls.Config, error) {
	return internaltls.QuickSelfSignedTLS(certDir)
}

// SetupTLS builds a *crypto/tls.Config from a ServerConfig.
func SetupTLS(serverConfig cfg.ServerConfig) (*cryptotls.Config, error) {
	return internaltls.SetupTLS(serverConfig)
}
