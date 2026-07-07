package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// clientRow mirrors the client_credentials table for scanning; Scopes/
// Metadata are stored as JSON text and converted to/from ClientCredential's
// typed fields at the boundary.
type clientRow struct {
	ID           string         `db:"id"`
	ClientID     string         `db:"client_id"`
	ClientSecret string         `db:"client_secret"`
	Name         string         `db:"name"`
	Scopes       sql.NullString `db:"scopes"`
	Metadata     sql.NullString `db:"metadata"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
	Active       bool           `db:"active"`
}

func (r clientRow) toClient() *ClientCredential {
	c := &ClientCredential{
		ID:           r.ID,
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		Name:         r.Name,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
		Active:       r.Active,
	}
	if r.Scopes.Valid {
		_ = json.Unmarshal([]byte(r.Scopes.String), &c.Scopes)
	}
	if r.Metadata.Valid {
		_ = json.Unmarshal([]byte(r.Metadata.String), &c.Metadata)
	}
	return c
}

func (s *authStore) CreateClient(ctx context.Context, client *ClientCredential) error {
	scopesJSON, _ := json.Marshal(client.Scopes)
	metadataJSON, _ := json.Marshal(client.Metadata)

	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`INSERT INTO client_credentials (id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		_, err := db.ExecContext(ctx, query,
			client.ID, client.ClientID, client.ClientSecret, client.Name,
			string(scopesJSON), string(metadataJSON),
			client.CreatedAt, client.UpdatedAt, client.Active)
		if err != nil {
			if isUniqueViolation(err) {
				return ErrClientAlreadyExists
			}
			return fmt.Errorf("failed to create client: %w", err)
		}
		return nil
	})
}

func (s *authStore) GetClient(ctx context.Context, id string) (*ClientCredential, error) {
	var row clientRow
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			FROM client_credentials WHERE id = ?`)
		return db.GetContext(ctx, &row, query, id)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return row.toClient(), nil
}

func (s *authStore) GetClientByClientID(ctx context.Context, clientID string) (*ClientCredential, error) {
	var row clientRow
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			FROM client_credentials WHERE client_id = ? AND active = true`)
		return db.GetContext(ctx, &row, query, clientID)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return row.toClient(), nil
}

func (s *authStore) UpdateClient(ctx context.Context, client *ClientCredential) error {
	scopesJSON, _ := json.Marshal(client.Scopes)
	metadataJSON, _ := json.Marshal(client.Metadata)
	client.UpdatedAt = time.Now().UTC()

	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`UPDATE client_credentials SET client_id = ?, client_secret = ?, name = ?, scopes = ?, metadata = ?, updated_at = ?, active = ?
			WHERE id = ?`)
		result, err := db.ExecContext(ctx, query,
			client.ClientID, client.ClientSecret, client.Name,
			string(scopesJSON), string(metadataJSON),
			client.UpdatedAt, client.Active, client.ID)
		if err != nil {
			return fmt.Errorf("failed to update client: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}
		if affected == 0 {
			return ErrClientNotFound
		}
		return nil
	})
}

func (s *authStore) DeleteClient(ctx context.Context, id string) error {
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`DELETE FROM client_credentials WHERE id = ?`)
		result, err := db.ExecContext(ctx, query, id)
		if err != nil {
			return fmt.Errorf("failed to delete client: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}
		if affected == 0 {
			return ErrClientNotFound
		}
		return nil
	})
}

func (s *authStore) ListClients(ctx context.Context, offset, limit int) ([]*ClientCredential, int, error) {
	var total int
	var rows []clientRow
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if err := db.GetContext(ctx, &total, `SELECT COUNT(*) FROM client_credentials`); err != nil {
			return fmt.Errorf("failed to get client count: %w", err)
		}
		query := db.Rebind(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			FROM client_credentials ORDER BY created_at DESC LIMIT ? OFFSET ?`)
		return db.SelectContext(ctx, &rows, query, limit, offset)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list clients: %w", err)
	}

	clients := make([]*ClientCredential, len(rows))
	for i, row := range rows {
		clients[i] = row.toClient()
	}
	return clients, total, nil
}
