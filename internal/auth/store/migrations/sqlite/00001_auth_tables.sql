-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    email TEXT,
    roles TEXT,
    metadata TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    active BOOLEAN NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(active);

-- +goose Down
DROP TABLE IF EXISTS users;
