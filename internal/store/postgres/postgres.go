package postgres

import (
	"context"
	"database/sql"
	"errors"
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
	stmt := `CREATE TABLE IF NOT EXISTS process_state(
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		pid INTEGER NOT NULL,
		last_status TEXT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL,
		spec_json TEXT
	);`
	_, err := p.db.ExecContext(ctx, stmt)
	return err
}

func (p *DB) Close() error { return p.db.Close() }

func (p *DB) Record(ctx context.Context, rec store.Record) error {
	if rec.Name == "" {
		return errors.New("empty name")
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO process_state(name, pid, last_status, updated_at, spec_json)
		VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(name) DO UPDATE SET
			pid=EXCLUDED.pid,
			last_status=EXCLUDED.last_status,
			updated_at=EXCLUDED.updated_at,
			spec_json=EXCLUDED.spec_json;`,
		rec.Name, rec.PID, rec.LastStatus, rec.UpdatedAt, rec.SpecJSON)
	return err
}

func (p *DB) GetByName(ctx context.Context, name string) (store.Record, error) {
	var r store.Record
	row := p.db.QueryRowContext(ctx, `SELECT name, pid, last_status, updated_at, spec_json FROM process_state WHERE name=$1;`, name)
	err := row.Scan(&r.Name, &r.PID, &r.LastStatus, &r.UpdatedAt, &r.SpecJSON)
	if err != nil {
		return store.Record{}, err
	}
	return r, nil
}

func (p *DB) Delete(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("empty name")
	}
	_, err := p.db.ExecContext(ctx, `DELETE FROM process_state WHERE name=$1;`, name)
	return err
}

// List returns all records from postgres store.
func (p *DB) List(ctx context.Context) ([]store.Record, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT name, pid, last_status, updated_at, spec_json FROM process_state ORDER BY updated_at ASC;`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var res []store.Record
	for rows.Next() {
		var r store.Record
		if err := rows.Scan(&r.Name, &r.PID, &r.LastStatus, &r.UpdatedAt, &r.SpecJSON); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}
