package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/tls"
	"github.com/loykin/provisr/pkg/client"
)

func main() {
	// TLS configuration examples
	fmt.Println("=== Provisr TLS Configuration Examples ===")

	// 1. Development auto-certificate generation example
	fmt.Println("1. Development Auto-Certificate Generation:")
	devExample()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 2. Client TLS configuration example
	fmt.Println("2. Client TLS Configuration:")
	clientExample()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 3. TLS Builder pattern example
	fmt.Println("3. TLS Builder Pattern:")
	builderExample()
}

// devExample demonstrates development TLS setup with auto-generated certificates
func devExample() {
	// Create temporary directory
	baseDir, err := os.MkdirTemp("", "provisr-tls-dev-*")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(baseDir) }()

	fmt.Printf("TLS certificate storage location: %s\n", baseDir)

	// Create development TLS configuration
	tlsConfig, err := tls.CreateDevTLS(baseDir)
	if err != nil {
		log.Fatalf("Failed to create dev TLS config: %v", err)
	}

	fmt.Printf("TLS configuration created successfully:\n")
	fmt.Printf("  - Auto Generate: %t\n", tlsConfig.AutoGenerate)
	fmt.Printf("  - Directory: %s\n", tlsConfig.Dir)
	if tlsConfig.AutoGen != nil {
		fmt.Printf("  - Common Name: %s\n", tlsConfig.AutoGen.CommonName)
		fmt.Printf("  - DNS Names: %v\n", tlsConfig.AutoGen.DNSNames)
		fmt.Printf("  - Valid Days: %d days\n", tlsConfig.AutoGen.ValidDays)
	}

	// Quick test TLS configuration
	quickTLSConfig, err := tls.QuickSelfSignedTLS(filepath.Join(baseDir, "quick"))
	if err != nil {
		log.Printf("Failed to create quick TLS config: %v", err)
	} else {
		fmt.Printf("Quick TLS configuration created successfully\n")
		_ = quickTLSConfig
	}
}

// clientExample demonstrates client TLS configurations
func clientExample() {
	// 1. Basic HTTP client (no TLS)
	httpConfig := client.DefaultConfig()
	httpClient := client.New(httpConfig)
	fmt.Printf("HTTP client created: %s\n", httpConfig.BaseURL)

	// 2. HTTPS client (with TLS verification)
	httpsConfig := client.DefaultTLSConfig()
	httpsClient := client.New(httpsConfig)
	fmt.Printf("HTTPS client created: %s\n", httpsConfig.BaseURL)

	// 3. Insecure client (skip TLS verification)
	insecureConfig := client.InsecureConfig()
	insecureClient := client.New(insecureConfig)
	fmt.Printf("Insecure client created: %s (skip verification: %t)\n",
		insecureConfig.BaseURL, insecureConfig.Insecure)

	// 4. Custom TLS client
	customConfig := client.Config{
		BaseURL: "https://my-provisr-server.com:8443/api",
		Timeout: 30 * time.Second,
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, nil)),
		TLS: &client.TLSClientConfig{
			Enabled:    true,
			ServerName: "my-provisr-server.com",
			SkipVerify: false,
			// CACert: "/path/to/ca.crt", // CA certificate file
			// ClientCert: "/path/to/client.crt", // Client certificate
			// ClientKey: "/path/to/client.key",  // Client private key
		},
	}
	customClient := client.New(customConfig)
	fmt.Printf("Custom TLS client created: %s\n", customConfig.BaseURL)

	// Connection test (will fail as server is not running)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Printf("\nConnection test (expected to fail as server is not running):\n")
	clients := map[string]*client.Client{
		"HTTP":     httpClient,
		"HTTPS":    httpsClient,
		"Insecure": insecureClient,
		"Custom":   customClient,
	}

	for name, c := range clients {
		reachable := c.IsReachable(ctx)
		fmt.Printf("  %s: %t\n", name, reachable)
	}
}

// builderExample demonstrates TLS builder pattern usage
func builderExample() {
	// Generate various TLS configurations using Builder pattern

	// 1. Basic auto-generation configuration
	basicConfig := tls.NewTLSBuilder().
		WithDir("/etc/provisr/tls").
		WithAutoGenerate(true).
		Build()

	fmt.Printf("Basic auto-generation configuration:\n")
	fmt.Printf("  Directory: %s\n", basicConfig.Dir)
	fmt.Printf("  Auto Generate: %t\n", basicConfig.AutoGenerate)

	// 2. Custom auto-generation configuration
	customConfig := tls.NewTLSBuilder().
		WithDir("/var/lib/provisr/ssl").
		WithAutoGenerate(true).
		WithAutoGenConfig("provisr.local", []string{
			"provisr.local",
			"provisr",
			"localhost",
			"*.provisr.local",
		}, 730). // 2 years
		Build()

	fmt.Printf("\nCustom auto-generation configuration:\n")
	fmt.Printf("  Directory: %s\n", customConfig.Dir)
	fmt.Printf("  Common Name: %s\n", customConfig.AutoGen.CommonName)
	fmt.Printf("  DNS Names: %v\n", customConfig.AutoGen.DNSNames)
	fmt.Printf("  Valid Days: %d days\n", customConfig.AutoGen.ValidDays)

	// 3. Manual certificate configuration
	manualConfig := tls.NewTLSBuilder().
		WithCertFiles("/etc/ssl/provisr.crt", "/etc/ssl/provisr.key").
		Build()

	fmt.Printf("\nManual certificate configuration:\n")
	fmt.Printf("  Certificate File: %s\n", manualConfig.CertFile)
	fmt.Printf("  Key File: %s\n", manualConfig.KeyFile)

	// 4. Using presets
	fmt.Printf("\nPreset usage examples:\n")

	// Development preset
	devPreset := tls.Default.Development("/tmp/dev-tls")
	fmt.Printf("Development: %s (auto-generate: %t)\n", devPreset.Dir, devPreset.AutoGenerate)

	// Production preset
	prodPreset := tls.Default.Production("/etc/ssl/cert.pem", "/etc/ssl/key.pem")
	fmt.Printf("Production: %s, %s\n", prodPreset.CertFile, prodPreset.KeyFile)

	// Testing preset
	testPreset, err := tls.Default.Testing()
	if err != nil {
		fmt.Printf("Failed to create testing preset: %v\n", err)
	} else {
		fmt.Printf("Testing: %s (temporary)\n", testPreset.Dir)
		// Cleanup
		defer func() { _ = os.RemoveAll(testPreset.Dir) }()
	}
}
