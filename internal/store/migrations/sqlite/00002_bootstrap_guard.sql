-- +goose Up
CREATE TABLE IF NOT EXISTS auth_bootstrap_guard (
    id INTEGER PRIMARY KEY CHECK (id = 1)
);
INSERT OR IGNORE INTO auth_bootstrap_guard (id)
SELECT 1 WHERE EXISTS (SELECT 1 FROM users);

-- +goose Down
DROP TABLE IF EXISTS auth_bootstrap_guard;
