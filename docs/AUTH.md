# Authentication and authorization

Provisr uses username/password authentication to issue JWT access tokens. User
data is stored through the configured SQLite or PostgreSQL auth store.

## Configuration

```toml
[server.auth]
enabled = true
jwt_secret = "replace-with-a-stable-secret"
token_ttl = "24h"
bcrypt_cost = 10

[server.auth.store]
type = "sqlite"
path = "auth.db"
migrate = true
max_open_conns = 1
max_idle_conns = 1
```

Relative SQLite paths are resolved from the directory containing the main
config file. `migrate = false` skips Goose migrations and requires the schema
to have been created before Provisr starts.

PostgreSQL uses the same hierarchy:

```toml
[server.auth.store]
type = "postgresql"
migrate = true
host = "localhost"
port = 5432
database = "provisr_auth"
username = "provisr"
password = "replace-me"
ssl_mode = "require"
max_open_conns = 10
max_idle_conns = 5
```

## First administrator

Credentials are never read from the config file and an administrator is not
created automatically. When the user store is empty, create the first admin
through the bootstrap endpoint:

```bash
curl -X POST http://localhost:8080/api/auth/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"replace-me","email":"admin@example.com"}'
```

Bootstrap is rejected after the first user has been created.

## Login

```bash
provisr login \
  --server-url=http://localhost:8080/api \
  --username=admin \
  --password=replace-me
```

Or call the HTTP endpoint directly:

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"method":"basic","username":"admin","password":"replace-me"}'
```

The response contains the JWT used by protected API requests.

## User management

`--roles` is required and accepts `admin`, `operator`, or `viewer`.

```bash
provisr auth user create \
  --config config/config.toml \
  --username operator \
  --password replace-me \
  --email operator@example.com \
  --roles operator

provisr auth user list --config config/config.toml
provisr auth user password --config config/config.toml \
  --username operator --new-password new-password
provisr auth user delete --config config/config.toml --username operator
```

The CLI opens the same `[server.auth.store]` configured for the server.

## Roles

- `admin`: full access, including user administration and write operations.
- `operator`: operational access granted by the server permission table.
- `viewer`: read-only access.

Authorization is enforced by server middleware. UI role checks only hide
controls and are not a security boundary.

## Security notes

- Configure a stable `jwt_secret`; generated secrets invalidate tokens after a
  restart and are unsuitable for multiple server instances.
- Use TLS for remote access.
- Use PostgreSQL with TLS for distributed deployments.
- Keep database credentials and JWT secrets outside committed sample files.
- Set `migrate = false` when schema changes are managed by deployment tooling
  or a database administrator.
