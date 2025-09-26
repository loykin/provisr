# Provisr TLS Configuration Examples

This example shows various ways to configure TLS in Provisr.

## Run

```bash
go run main.go
```

## Key Features

### 1. Server TLS Configuration

#### Auto-generated Certificates (Development)
```yaml
# config.toml
[server]
listen = ":8443"

[server.tls]
enabled = true
dir = "./tls"
auto_generate = true

[server.tls.auto_gen]
common_name = "localhost"
dns_names = ["localhost", "127.0.0.1", "provisr.local"]
ip_addresses = ["127.0.0.1"]
valid_days = 365
```

#### Manual Certificate Configuration (Production)
```yaml
# config.toml
[server]
listen = ":8443"

[server.tls]
enabled = true
cert_file = "/etc/ssl/provisr.crt"
key_file = "/etc/ssl/provisr.key"
```

#### Directory-based Certificates
```yaml
# config.toml
[server]
listen = ":8443"

[server.tls]
enabled = true
dir = "/etc/provisr/tls"
# In this case, it looks for tls.crt, tls.key files
```

### 2. Client TLS Configuration

#### Basic HTTPS Client
```go
config := client.DefaultTLSConfig()
c := client.New(config)
```

#### Insecure Client (Development/Testing)
```go
config := client.InsecureConfig() // Skip TLS verification
c := client.New(config)
```

#### Custom TLS Client
```go
config := client.Config{
    BaseURL: "https://provisr.company.com:8443/api",
    TLS: &client.TLSClientConfig{
        Enabled:    true,
        CACert:     "/path/to/ca.crt",        // CA certificate
        ClientCert: "/path/to/client.crt",    // Client certificate
        ClientKey:  "/path/to/client.key",    // Client private key
        ServerName: "provisr.company.com",    // Server name for SNI
        SkipVerify: false,                    // Certificate verification
    },
}
c := client.New(config)
```

### 3. Programmatic TLS Configuration

#### Builder Pattern
```go
tlsConfig := tls.NewTLSBuilder().
    WithDir("/etc/provisr/tls").
    WithAutoGenerate(true).
    WithAutoGenConfig("provisr.local", []string{"provisr.local", "localhost"}, 365).
    Build()
```

#### Using Presets
```go
// Development
devConfig := tls.Presets.Development("/tmp/dev-tls")

// Production
prodConfig := tls.Presets.Production("/etc/ssl/cert.pem", "/etc/ssl/key.pem")

// Testing (temporary certificates)
testConfig, _ := tls.Presets.Testing()
```

#### Quick Configuration
```go
// Quick development TLS setup
tlsConfig, _ := tls.CreateDevTLS("/tmp/provisr-dev")

// Quick self-signed certificate
quickTLS, _ := tls.QuickSelfSignedTLS("/tmp/quick-tls")
```

## TLS Configuration Priority

1. **cert_file + key_file**: Use explicitly specified certificate files
2. **dir + auto_generate=true**: Auto-generate in directory
3. **dir**: Use existing certificates in directory (tls.crt, tls.key)

## File Structure

The following files are generated during auto-generation:

```
tls/
├── tls.crt        # Server certificate
├── tls.key        # Server private key
└── tls_ca.crt     # CA certificate (self-signed)
```

## Security Considerations

### Development Environment
- Use `auto_generate = true`
- `insecure = true` client option can be used

### Production Environment
- Use certificates issued by trusted CA
- Set `skip_verify = false`
- Set appropriate file permissions (0600)

## Troubleshooting

### Certificate-related Errors
```bash
# Check certificate information
openssl x509 -in tls.crt -text -noout

# Check private key
openssl rsa -in tls.key -check

# Check certificate and key matching
openssl x509 -noout -modulus -in tls.crt | openssl md5
openssl rsa -noout -modulus -in tls.key | openssl md5
```

### Client Connection Issues
1. Check server address and certificate CN/SAN
2. Verify CA certificate path
3. Check firewall/ports
4. Test with `insecure = true` then gradually strengthen security