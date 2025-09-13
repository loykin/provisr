package postgres

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/loykin/provisr/internal/store"
)

type DB struct {
	db *sql.DB
}

func New(dsn string) (*DB, error) {
	d, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return &DB{db: d}, nil
}

func (p *DB) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS process_state(
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			pid INTEGER NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			stopped_at TIMESTAMPTZ NULL,
			running BOOLEAN NOT NULL,
			exit_err TEXT NULL,
			uniq TEXT NOT NULL UNIQUE,
			updated_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_process_state_name ON process_state(name);`,
		`CREATE INDEX IF NOT EXISTS idx_process_state_running ON process_state(running);`,
		`CREATE TABLE IF NOT EXISTS process_history(
			id BIGSERIAL PRIMARY KEY,
			occurred_at TIMESTAMPTZ NOT NULL,
			event TEXT NOT NULL,
			name TEXT NOT NULL,
			pid INTEGER NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			stopped_at TIMESTAMPTZ NULL,
			running BOOLEAN NOT NULL,
			exit_err TEXT NULL,
			uniq TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_process_history_name ON process_history(name);`,
		`CREATE INDEX IF NOT EXISTS idx_process_history_uniq ON process_history(uniq);`,
	}
	for _, q := range stmts {
		if _, err := p.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (p *DB) Close() error { return p.db.Close() }

func (p *DB) RecordStart(ctx context.Context, rec store.Record) error {
	rec.Running = true
	rec.StoppedAt = sql.NullTime{}
	rec.ExitErr = sql.NullString{}
	rec.UpdatedAt = time.Now().UTC()
	uniq := rec.Key()
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO process_state(name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at)
		VALUES($1,$2,$3,NULL,true,NULL,$4,$5)
		ON CONFLICT(uniq) DO UPDATE SET
			name=EXCLUDED.name,
			pid=EXCLUDED.pid,
			started_at=EXCLUDED.started_at,
			running=EXCLUDED.running,
			stopped_at=NULL,
			exit_err=NULL,
			updated_at=EXCLUDED.updated_at;`,
		rec.Name, rec.PID, rec.StartedAt.UTC(), uniq, rec.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (p *DB) RecordStop(ctx context.Context, uniq string, stoppedAt time.Time, exitErr error) error {
	var errStr sql.NullString
	if exitErr != nil {
		errStr = sql.NullString{String: exitErr.Error(), Valid: true}
	}
	_, err := p.db.ExecContext(ctx, `
		UPDATE process_state
		SET running=false, stopped_at=$1, exit_err=$2, updated_at=$3
		WHERE uniq=$4;`, stoppedAt.UTC(), errStr, time.Now().UTC(), uniq)
	if err != nil {
		return err
	}
	return nil
}

func (p *DB) UpsertStatus(ctx context.Context, rec store.Record) error {
	rec.UpdatedAt = time.Now().UTC()
	uniq := rec.Key()
	var stoppedAt any
	if rec.StoppedAt.Valid {
		stoppedAt = rec.StoppedAt.Time.UTC()
	}
	var exitErr any
	if rec.ExitErr.Valid {
		exitErr = rec.ExitErr.String
	}
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO process_state(name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(uniq) DO UPDATE SET
			name=EXCLUDED.name,
			pid=EXCLUDED.pid,
			started_at=EXCLUDED.started_at,
			stopped_at=EXCLUDED.stopped_at,
			running=EXCLUDED.running,
			exit_err=EXCLUDED.exit_err,
			updated_at=EXCLUDED.updated_at;`,
		rec.Name, rec.PID, rec.StartedAt.UTC(), stoppedAt, rec.Running, exitErr, uniq, rec.UpdatedAt)
	return err
}

func (p *DB) GetByName(ctx context.Context, name string, limit int) ([]store.Record, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at
		FROM process_state
		WHERE name=$1
		ORDER BY started_at DESC
		LIMIT $2;`, name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (p *DB) GetRunning(ctx context.Context, namePrefix string) ([]store.Record, error) {
	like := namePrefix + "%"
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, name, pid, started_at, stopped_at, running, exit_err, uniq, updated_at
		FROM process_state
		WHERE running=true AND name LIKE $1
		ORDER BY updated_at DESC;`, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (p *DB) PurgeOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := p.db.ExecContext(ctx, `DELETE FROM process_state WHERE running=false AND updated_at < $1;`, olderThan.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func scanRecords(rows *sql.Rows) ([]store.Record, error) {
	out := make([]store.Record, 0)
	for rows.Next() {
		var r store.Record
		if err := rows.Scan(&r.ID, &r.Name, &r.PID, &r.StartedAt, &r.StoppedAt, &r.Running, &r.ExitErr, &r.Uniq, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
