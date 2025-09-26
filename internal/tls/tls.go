package tls

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/config"
)

const (
	tlsCaCrt = "tls_ca.crt"
	tlsCrt   = "tls.crt"
	tlsKey   = "tls.key"
)

// parseTLSVersion parses TLS version string and returns the corresponding constant
func parseTLSVersion(ver string) (uint16, bool) {
	switch ver {
	case "", "default":
		return tls.VersionTLS13, false
	case "1.2", "TLS1.2", "tls1.2":
		return tls.VersionTLS12, true
	case "1.3", "TLS1.3", "tls1.3":
		return tls.VersionTLS13, true
	default:
		return 0, false
	}
}

// resolveTLSVersions resolves minimum and maximum TLS versions from server config
func resolveTLSVersions(cfg config.ServerConfig) (min uint16, max uint16) {
	// Defaults: 1.3
	min = tls.VersionTLS13
	max = tls.VersionTLS13
	if v, ok := parseTLSVersion(cfg.TLSMinVersion); ok {
		min = v
	}
	if v, ok := parseTLSVersion(cfg.TLSMaxVersion); ok {
		max = v
	}
	return
}

// safeReadFile reads file content safely within base directory
func safeReadFile(baseDir, p string) ([]byte, error) {
	clean := filepath.Clean(p)
	if baseDir != "" {
		absBase, _ := filepath.Abs(baseDir)
		absFile, _ := filepath.Abs(clean)
		if !strings.HasPrefix(absFile, absBase+string(filepath.Separator)) && absFile != absBase {
			return nil, errors.New("file path outside of allowed directory")
		}
	}
	return os.ReadFile(clean)
}

// getCertificationFunc returns a function that loads certificates dynamically
func getCertificationFunc(certFile, keyFile string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	baseDir := filepath.Dir(certFile)
	return func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		if readCert, err := safeReadFile(baseDir, certFile); err != nil {
			return nil, err
		} else if readKey, err := safeReadFile(baseDir, keyFile); err != nil {
			return nil, err
		} else {
			certificate, err := tls.X509KeyPair(readCert, readKey)
			return &certificate, err
		}
	}
}

// SetupTLS configures TLS settings for the server with improved usability
func SetupTLS(server config.ServerConfig) (*tls.Config, error) {
	if server.TLS == nil || !server.TLS.Enabled {
		return nil, nil
	}

	minVer, maxVer := resolveTLSVersions(server)

	// Priority 1: Use specific cert/key files if provided
	if server.TLS.CertFile != "" && server.TLS.KeyFile != "" {
		return createTLSConfig(server.TLS.CertFile, server.TLS.KeyFile, minVer, maxVer)
	}

	// Priority 2: Use directory-based certificates
	if server.TLS.Dir != "" {
		keyPath := filepath.Join(server.TLS.Dir, tlsKey)
		certPath := filepath.Join(server.TLS.Dir, tlsCrt)

		// Auto-generate if enabled and certificates don't exist
		if server.TLS.AutoGenerate && !certificatesExist(certPath, keyPath) {
			if err := generateCertificate(server.TLS, server.TLS.Dir); err != nil {
				return nil, fmt.Errorf("certificate generation failed: %w", err)
			}
		}

		return createTLSConfig(certPath, keyPath, minVer, maxVer)
	}

	return nil, errors.New("TLS enabled but no valid certificate configuration found")
}

// helper functions
func getOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func getOrDefaultSlice(value, defaultValue []string) []string {
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

// EasyTLSSetup provides a simplified interface for TLS setup
func EasyTLSSetup(listen string, certDir string, autoGen bool) (*tls.Config, error) {
	serverConfig := config.ServerConfig{
		Listen: listen,
		TLS: &config.TLSConfig{
			Enabled:      true,
			Dir:          certDir,
			AutoGenerate: autoGen,
		},
	}

	return SetupTLS(serverConfig)
}

// QuickSelfSignedTLS generates a quick self-signed certificate for testing
func QuickSelfSignedTLS(certDir string) (*tls.Config, error) {
	return EasyTLSSetup("localhost:8080", certDir, true)
}

// createTLSConfig creates TLS configuration with certificate files
func createTLSConfig(certPath, keyPath string, minVer, maxVer uint16) (*tls.Config, error) {
	// #nosec G402 TLS backward compatibility considered
	return &tls.Config{
		GetCertificate: getCertificationFunc(certPath, keyPath),
		MinVersion:     minVer,
		MaxVersion:     maxVer,
	}, nil
}

// certificatesExist checks if both certificate files exist
func certificatesExist(certPath, keyPath string) bool {
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	return certErr == nil && keyErr == nil
}

// generateCertificate generates self-signed certificates with improved defaults
func generateCertificate(tlsConfig *config.TLSConfig, destDir string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Set up defaults
	autoGen := tlsConfig.AutoGen
	if autoGen == nil {
		autoGen = &config.AutoGenTLS{}
	}

	commonName := getOrDefault(autoGen.CommonName, "localhost")
	organization := getOrDefault(autoGen.Organization, "provisr")
	dnsNames := getOrDefaultSlice(autoGen.DNSNames, []string{"localhost", "127.0.0.1"})
	ipAddresses := getOrDefaultSlice(autoGen.IPAddresses, []string{"127.0.0.1"})

	// Calculate expiration date
	validDays := autoGen.ValidDays
	if validDays <= 0 {
		validDays = 365 * 5 // Default: 5 years
	}
	notAfter := time.Now().AddDate(0, 0, validDays)

	// Generate the certificate
	return GenerateSelfSignedCert(CertConfig{
		CommonName:   commonName,
		Organization: organization,
		DNSNames:     dnsNames,
		IPAddresses:  ipAddresses,
		NotAfter:     notAfter,
		CertPath:     filepath.Join(destDir, tlsCrt),
		KeyPath:      filepath.Join(destDir, tlsKey),
		CACertPath:   filepath.Join(destDir, tlsCaCrt),
	})
}
