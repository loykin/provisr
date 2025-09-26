package tls

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/loykin/provisr/internal/config"
)

// Builder provides a fluent interface for TLS configuration
type Builder struct {
	cfg *config.TLSConfig
}

// NewTLSBuilder creates a new TLS configuration builder
func NewTLSBuilder() *Builder {
	return &Builder{
		cfg: &config.TLSConfig{
			Enabled: true,
		},
	}
}

// WithCertFiles sets certificate and key files
func (b *Builder) WithCertFiles(certFile, keyFile string) *Builder {
	b.cfg.CertFile = certFile
	b.cfg.KeyFile = keyFile
	return b
}

// WithDir sets the certificate directory
func (b *Builder) WithDir(dir string) *Builder {
	b.cfg.Dir = dir
	return b
}

// WithAutoGenerate enables automatic certificate generation
func (b *Builder) WithAutoGenerate(enable bool) *Builder {
	b.cfg.AutoGenerate = enable
	return b
}

// WithAutoGenConfig configures auto-generation settings
func (b *Builder) WithAutoGenConfig(commonName string, dnsNames []string, validDays int) *Builder {
	if b.cfg.AutoGen == nil {
		b.cfg.AutoGen = &config.AutoGenTLS{}
	}

	b.cfg.AutoGen.CommonName = commonName
	b.cfg.AutoGen.DNSNames = dnsNames
	b.cfg.AutoGen.ValidDays = validDays
	return b
}

// Build returns the configured TLS config
func (b *Builder) Build() *config.TLSConfig {
	return b.cfg
}

// Presets provides common TLS configurations
type Presets struct{}

// Development returns a development-friendly TLS config with self-signed certs
func (p Presets) Development(certDir string) *config.TLSConfig {
	return NewTLSBuilder().
		WithDir(certDir).
		WithAutoGenerate(true).
		WithAutoGenConfig("localhost", []string{"localhost", "127.0.0.1"}, 365).
		Build()
}

// Production returns a production TLS config requiring manual certificates
func (p Presets) Production(certFile, keyFile string) *config.TLSConfig {
	return NewTLSBuilder().
		WithCertFiles(certFile, keyFile).
		Build()
}

// Testing returns a testing TLS config with temporary certificates
func (p Presets) Testing() (*config.TLSConfig, error) {
	tmpDir, err := os.MkdirTemp("", "provisr-tls-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	return NewTLSBuilder().
		WithDir(tmpDir).
		WithAutoGenerate(true).
		WithAutoGenConfig("test", []string{"test", "localhost"}, 1).
		Build(), nil
}

var Default = Presets{}

// CreateDevTLS creates a development TLS configuration
func CreateDevTLS(baseDir string) (*config.TLSConfig, error) {
	certDir := filepath.Join(baseDir, "tls")
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create TLS directory: %w", err)
	}

	return Default.Development(certDir), nil
}
