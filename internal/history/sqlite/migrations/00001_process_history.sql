-- +goose Up
CREATE TABLE IF NOT EXISTS process_history(
    timestamp TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    pid INTEGER NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    error TEXT
);

CREATE INDEX IF NOT EXISTS idx_process_history_name_timestamp
    ON process_history(name, timestamp DESC);

-- +goose Down
DROP TABLE IF EXISTS process_history;
