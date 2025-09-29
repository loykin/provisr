# Authentication Login Example

This example demonstrates how to use the `provisr login` and `provisr logout` commands for session management.

## Prerequisites

1. Start the provisr server:
```bash
provisr serve config.toml
```

2. Create a user account:
```bash
provisr auth user create --username=demo --password=demopass --roles=operator
```

## Basic Login Workflow

### 1. Login

```bash
# Login with username/password
provisr login --username=demo --password=demopass
```

Expected output:
```
Login successful! Logged in as demo
Session saved to /Users/username/.provisr/session.json
Token expires at: 2025-09-30T11:16:15+09:00
```

### 2. Use Authenticated Commands

Now you can use other provisr commands without specifying credentials:

```bash
# These commands now use your saved session
provisr status
provisr start --name=myapp
provisr stop --name=myapp
```

### 3. Check Session Status

```bash
# View your session file
cat ~/.provisr/session.json
```

### 4. Logout

```bash
provisr logout
```

Expected output:
```
Logged out successfully
```

## Client Credentials Login

### 1. Create API Client

```bash
provisr auth client create --name="Demo Client" --scopes=operator
```

Note the returned client_id and client_secret.

### 2. Login with Client Credentials

```bash
provisr login --method=client_secret \
  --client-id=client_abc123 \
  --client-secret=secret_xyz789
```

## Remote Server Login

### 1. Login to Remote Server

```bash
provisr login --server-url=http://remote-server:8080/api \
  --username=demo --password=demopass
```

### 2. Commands Use Remote Session

All subsequent commands will connect to the remote server:

```bash
provisr status  # Connects to remote-server:8080
```

## Session Management

### Session File Location

Sessions are stored in `~/.provisr/session.json` with the following structure:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_at": "2025-09-30T11:16:15+09:00",
  "username": "demo",
  "user_id": "user-123",
  "roles": ["operator"],
  "server_url": "http://localhost:8080/api"
}
```

### Session Security

- File permissions are set to 0600 (user read/write only)
- Expired sessions are automatically removed
- Server URL prevents token reuse on wrong servers

### Multiple Server Sessions

To work with multiple servers, logout and login to switch:

```bash
# Work with server A
provisr login --server-url=http://server-a:8080/api --username=user --password=pass
provisr status

# Switch to server B
provisr logout
provisr login --server-url=http://server-b:8080/api --username=user --password=pass
provisr status
```

## Error Handling

### No Session

```bash
$ provisr logout
No active session found
```

### Expired Session

```bash
$ provisr status
Error: authentication failed: token expired
# Need to login again
$ provisr login --username=demo --password=demopass
```

### Server Unreachable

```bash
$ provisr login --username=demo --password=demopass
Error: server not reachable at http://localhost:8080/api - please start daemon first with 'provisr serve'
```

## Best Practices

1. **Change default passwords**: Always change default admin password
2. **Use client credentials for automation**: Prefer client credentials for scripts/CI
3. **Regular logout**: Logout when done to clear sensitive tokens
4. **Secure config**: Keep JWT secrets secure and rotate regularly
5. **Monitor sessions**: Check ~/.provisr/session.json for unexpected sessions

## Integration with Scripts

### Shell Script Example

```bash
#!/bin/bash

# Login
if ! provisr login --username=admin --password=secret; then
    echo "Login failed"
    exit 1
fi

# Do work
provisr start --name=web-server
provisr status --name=web-server

# Cleanup
provisr logout
```

### CI/CD Example

```bash
# Use client credentials in CI
provisr login --method=client_secret \
  --client-id="$PROVISR_CLIENT_ID" \
  --client-secret="$PROVISR_CLIENT_SECRET"

# Deploy
provisr start --name=production-api
provisr status

# No need to logout in CI (ephemeral environment)
```