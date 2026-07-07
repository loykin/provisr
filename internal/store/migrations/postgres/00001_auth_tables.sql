-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    email VARCHAR(255),
    roles JSONB,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(active);
CREATE INDEX IF NOT EXISTS idx_users_roles ON users USING GIN(roles);

CREATE TABLE IF NOT EXISTS client_credentials (
    id TEXT PRIMARY KEY,
    client_id VARCHAR(255) UNIQUE NOT NULL,
    client_secret TEXT NOT NULL,
    name VARCHAR(255) NOT NULL,
    scopes JSONB,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_clients_client_id ON client_credentials(client_id);
CREATE INDEX IF NOT EXISTS idx_clients_active ON client_credentials(active);
CREATE INDEX IF NOT EXISTS idx_clients_scopes ON client_credentials USING GIN(scopes);

-- +goose Down
DROP TABLE IF EXISTS client_credentials;
DROP TABLE IF EXISTS users;
