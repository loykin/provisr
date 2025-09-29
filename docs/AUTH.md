# Authentication & Authorization

Provisr provides comprehensive authentication and authorization features for secure process management.

## Overview

The authentication system supports multiple authentication methods:
- **Basic Authentication**: Username/password
- **Client Credentials**: Client ID/secret for API access
- **JWT Tokens**: Token-based authentication with expiration

## Quick Start

### 1. Enable Authentication

Add authentication configuration to your `config.toml`:

```toml
[auth]
enabled = true
database_path = "auth.db"
database_type = "sqlite"

[auth.jwt]
secret = "your-secret-key-change-this-in-production"
expires_in = "24h"

[auth.admin]
auto_create = true
username = "admin"
password = "admin"
email = "admin@localhost"
```

### 2. Login via CLI

```bash
# Login with username/password
provisr login --username=admin --password=admin

# Login with client credentials
provisr login --method=client_secret --client-id=client_123 --client-secret=secret456

# Login to remote server
provisr login --server-url=http://remote:8080/api --username=admin --password=secret
```

### 3. Use Authenticated Commands

Once logged in, all commands automatically use your saved session:

```bash
provisr status
provisr start --name=myapp
provisr stop --name=myapp
```

### 4. Logout

```bash
provisr logout
```

## User Management

### Create Users

```bash
# Create admin user
provisr auth user create --username=admin --password=secret --roles=admin

# Create operator user
provisr auth user create --username=operator --password=secret --email=op@example.com --roles=operator,user
```

### List Users

```bash
provisr auth user list
```

### Delete Users

```bash
provisr auth user delete --username=olduser
```

### Reset Password

```bash
provisr auth user password --username=operator --new-password=newpassword
```

## Client Management

### Create Client Credentials

```bash
# Create API client
provisr auth client create --name="API Client" --scopes=operator

# Create admin client
provisr auth client create --name="Admin Client" --scopes=admin,operator
```

### List Clients

```bash
provisr auth client list
```

### Delete Clients

```bash
provisr auth client delete --client-id=client_123
```

## HTTP API Authentication

### Login Endpoint

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "method": "basic",
    "username": "admin",
    "password": "secret"
  }'
```

Response:
```json
{
  "success": true,
  "user_id": "user-123",
  "username": "admin",
  "roles": ["admin"],
  "token": {
    "type": "Bearer",
    "value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2025-09-30T11:16:15+09:00"
  }
}
```

### Using JWT Tokens

Include the JWT token in the Authorization header:

```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  http://localhost:8080/api/status
```

## Session Management

### Session Storage

Login sessions are automatically saved to `~/.provisr/session.json`:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_at": "2025-09-30T11:16:15+09:00",
  "username": "admin",
  "user_id": "user-123",
  "roles": ["admin"],
  "server_url": "http://localhost:8080/api"
}
```

### Security Features

- Session file has restricted permissions (0600)
- Expired tokens are automatically removed
- Server URL validation prevents token reuse on wrong servers

## Roles and Permissions

### Built-in Roles

- **admin**: Full access to all operations
- **operator**: Process management operations
- **user**: Read-only access

### Role Assignment

Users can have multiple roles:

```bash
provisr auth user create --username=manager --password=secret --roles=admin,operator,user
```

## Database Support

### SQLite (Default)

```toml
[auth]
database_type = "sqlite"
database_path = "auth.db"
```

### PostgreSQL

```toml
[auth]
database_type = "postgresql"
database_url = "postgres://user:password@localhost/provisr_auth?sslmode=disable"
```

## Testing Authentication

### Test CLI Authentication

```bash
# Test basic auth
provisr auth test --method=basic --username=admin --password=secret

# Test client credentials
provisr auth test --method=client_secret --client-id=client_123 --client-secret=secret456
```

### Test HTTP Authentication

```bash
# Test with curl
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"method":"basic","username":"admin","password":"secret"}'
```

## Configuration Reference

### Full Authentication Configuration

```toml
[auth]
# Enable authentication
enabled = true

# Database configuration
database_path = "auth.db"
database_type = "sqlite"  # or "postgresql"
# database_url = "postgres://user:password@localhost/provisr_auth"

[auth.jwt]
# JWT signing secret (keep secure!)
secret = "your-secret-key-change-this-in-production"
# Token expiration time
expires_in = "24h"

[auth.admin]
# Auto-create admin user on startup
auto_create = true
username = "admin"
password = "admin"  # Change this immediately!
email = "admin@localhost"
```

## Troubleshooting

### Common Issues

1. **Login fails**: Check server is running and credentials are correct
2. **Permission denied**: Ensure user has required roles
3. **Session expired**: Login again to refresh token
4. **Database errors**: Check database path and permissions

### Debug Commands

```bash
# Check current session
cat ~/.provisr/session.json

# Test server connectivity
curl http://localhost:8080/api/status

# View server logs
provisr serve config.toml
```