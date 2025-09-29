# Basic Authentication Example

This example demonstrates the core authentication functionality of Provisr, including user management, client credentials, and JWT token authentication.

## What it demonstrates

- Creating and managing users with roles
- Creating API clients with scopes
- Basic username/password authentication
- Client credentials authentication
- JWT token validation
- Listing users and clients

## Prerequisites

No special setup required - this example creates its own SQLite database.

## Running the Example

```bash
cd examples/auth_basic
go run main.go
```

## Expected Output

The example will:

1. **Create a user**: Creates an admin user with password
2. **Create API client**: Creates a client credential for API access
3. **Test basic auth**: Authenticates with username/password
4. **Test client auth**: Authenticates with client credentials
5. **Test JWT auth**: Validates JWT tokens
6. **List resources**: Shows created users and clients

Sample output:
```
Creating user...
Created user: admin

Creating client credential...
Created client: API Client (ID: client_abc123)
Client Secret: secret_xyz789

Testing basic authentication...
✓ Basic auth successful: admin
  Token expires: 2025-09-30T11:16:15Z

Testing client credentials authentication...
✓ Client auth successful: API Client
  Scopes: [operator]

Testing JWT authentication...
✓ JWT auth successful: admin

Listing users...
Found 1 users:
  - admin (admin@example.com)

Listing clients...
Found 1 clients:
  - API Client (client_abc123)

✓ Auth example completed successfully!
```

## Key Components

- **AuthService**: Core authentication service
- **Users**: Username/password authentication with roles
- **Clients**: Client credentials for API access
- **JWT Tokens**: Stateless authentication tokens
- **SQLite Storage**: Local database for auth data

## Configuration

The example uses a simple configuration:

```go
config := auth.AuthConfig{
    Store: auth.StoreConfig{
        Type: "sqlite",
        Path: "example_auth.db",
    },
    JWTSecret:  "example-secret-key-change-in-production",
    TokenTTL:   24 * time.Hour,
    BcryptCost: 10,
}
```

## Files Created

- `example_auth.db`: SQLite database with auth data
- Remove this file to reset the example

## Next Steps

- See `../auth_login/` for CLI login examples
- See `../store_basic/` for advanced storage options
- Check `/docs/AUTH.md` for complete authentication documentation