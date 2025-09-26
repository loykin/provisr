package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

// CertConfig holds configuration for certificate generation
type CertConfig struct {
	CommonName   string
	Organization string
	DNSNames     []string
	IPAddresses  []string
	NotAfter     time.Time
	CertPath     string
	KeyPath      string
	CACertPath   string
}

// GenerateSelfSignedCert generates a self-signed certificate and private key
func GenerateSelfSignedCert(config CertConfig) error {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   config.CommonName,
			Organization: []string{config.Organization},
		},
		NotBefore:             time.Now(),
		NotAfter:              config.NotAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add DNS names
	template.DNSNames = config.DNSNames

	// Add IP addresses
	for _, ipStr := range config.IPAddresses {
		if ip := net.ParseIP(ipStr); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		}
	}

	// Create the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate
	certFile, err := os.Create(config.CertPath)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer func() { err = certFile.Close() }()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Save private key
	keyFile, err := os.Create(config.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer func() { _ = keyFile.Close() }()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Save CA certificate (same as certificate for self-signed)
	if config.CACertPath != "" {
		caCertFile, err := os.Create(config.CACertPath)
		if err != nil {
			return fmt.Errorf("failed to create CA certificate file: %w", err)
		}
		defer func() { _ = caCertFile.Close() }()

		if err := pem.Encode(caCertFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
			return fmt.Errorf("failed to write CA certificate: %w", err)
		}
	}

	return nil
}
