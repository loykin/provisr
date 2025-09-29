package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Client operations for SQLite

func (s *SQLiteAuthStore) CreateClient(ctx context.Context, client *ClientCredential) error {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	scopesJSON, _ := json.Marshal(client.Scopes)
	metadataJSON, _ := json.Marshal(client.Metadata)

	query := fmt.Sprintf(`INSERT INTO %sclient_credentials (id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, prefix)

	_, err := db.ExecContext(ctx, query,
		client.ID, client.ClientID, client.ClientSecret, client.Name,
		string(scopesJSON), string(metadataJSON),
		client.CreatedAt, client.UpdatedAt, client.Active)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrClientAlreadyExists
		}
		return fmt.Errorf("failed to create client: %w", err)
	}

	return nil
}

func (s *SQLiteAuthStore) GetClient(ctx context.Context, id string) (*ClientCredential, error) {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	query := fmt.Sprintf(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			  FROM %sclient_credentials WHERE id = ?`, prefix)

	return s.scanClient(db.QueryRowContext(ctx, query, id))
}

func (s *SQLiteAuthStore) GetClientByClientID(ctx context.Context, clientID string) (*ClientCredential, error) {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	query := fmt.Sprintf(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			  FROM %sclient_credentials WHERE client_id = ? AND active = 1`, prefix)

	return s.scanClient(db.QueryRowContext(ctx, query, clientID))
}

func (s *SQLiteAuthStore) UpdateClient(ctx context.Context, client *ClientCredential) error {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	scopesJSON, _ := json.Marshal(client.Scopes)
	metadataJSON, _ := json.Marshal(client.Metadata)
	client.UpdatedAt = time.Now().UTC()

	query := fmt.Sprintf(`UPDATE %sclient_credentials SET client_id = ?, client_secret = ?, name = ?, scopes = ?, metadata = ?, updated_at = ?, active = ?
			  WHERE id = ?`, prefix)

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
}

func (s *SQLiteAuthStore) DeleteClient(ctx context.Context, id string) error {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	query := fmt.Sprintf(`DELETE FROM %sclient_credentials WHERE id = ?`, prefix)

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
}

func (s *SQLiteAuthStore) ListClients(ctx context.Context, offset, limit int) ([]*ClientCredential, int, error) {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	// Get total count
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %sclient_credentials`, prefix)
	if err := db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to get client count: %w", err)
	}

	// Get clients with pagination
	query := fmt.Sprintf(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			  FROM %sclient_credentials ORDER BY created_at DESC LIMIT ? OFFSET ?`, prefix)

	rows, err := db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list clients: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var clients []*ClientCredential
	for rows.Next() {
		client, err := s.scanClientFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		clients = append(clients, client)
	}

	return clients, total, nil
}

// scanClient scans a client from a single row (SQLite)
func (s *SQLiteAuthStore) scanClient(row *sql.Row) (*ClientCredential, error) {
	var client ClientCredential
	var scopesJSON, metadataJSON sql.NullString

	err := row.Scan(
		&client.ID, &client.ClientID, &client.ClientSecret, &client.Name,
		&scopesJSON, &metadataJSON,
		&client.CreatedAt, &client.UpdatedAt, &client.Active,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrClientNotFound
		}
		return nil, fmt.Errorf("failed to scan client: %w", err)
	}

	if scopesJSON.Valid {
		_ = json.Unmarshal([]byte(scopesJSON.String), &client.Scopes)
	}
	if metadataJSON.Valid {
		_ = json.Unmarshal([]byte(metadataJSON.String), &client.Metadata)
	}

	return &client, nil
}

// scanClientFromRows scans a client from multiple rows (SQLite)
func (s *SQLiteAuthStore) scanClientFromRows(rows *sql.Rows) (*ClientCredential, error) {
	var client ClientCredential
	var scopesJSON, metadataJSON sql.NullString

	err := rows.Scan(
		&client.ID, &client.ClientID, &client.ClientSecret, &client.Name,
		&scopesJSON, &metadataJSON,
		&client.CreatedAt, &client.UpdatedAt, &client.Active,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan client: %w", err)
	}

	if scopesJSON.Valid {
		_ = json.Unmarshal([]byte(scopesJSON.String), &client.Scopes)
	}
	if metadataJSON.Valid {
		_ = json.Unmarshal([]byte(metadataJSON.String), &client.Metadata)
	}

	return &client, nil
}

// Client operations for PostgreSQL

func (s *PostgreSQLAuthStore) CreateClient(ctx context.Context, client *ClientCredential) error {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	scopesJSON, _ := json.Marshal(client.Scopes)
	metadataJSON, _ := json.Marshal(client.Metadata)

	query := fmt.Sprintf(`INSERT INTO %sclient_credentials (id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, prefix)

	_, err := db.ExecContext(ctx, query,
		client.ID, client.ClientID, client.ClientSecret, client.Name,
		scopesJSON, metadataJSON,
		client.CreatedAt, client.UpdatedAt, client.Active)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrClientAlreadyExists
		}
		return fmt.Errorf("failed to create client: %w", err)
	}

	return nil
}

func (s *PostgreSQLAuthStore) GetClient(ctx context.Context, id string) (*ClientCredential, error) {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	query := fmt.Sprintf(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			  FROM %sclient_credentials WHERE id = $1`, prefix)

	return s.scanClient(db.QueryRowContext(ctx, query, id))
}

func (s *PostgreSQLAuthStore) GetClientByClientID(ctx context.Context, clientID string) (*ClientCredential, error) {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	query := fmt.Sprintf(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			  FROM %sclient_credentials WHERE client_id = $1 AND active = TRUE`, prefix)

	return s.scanClient(db.QueryRowContext(ctx, query, clientID))
}

func (s *PostgreSQLAuthStore) UpdateClient(ctx context.Context, client *ClientCredential) error {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	scopesJSON, _ := json.Marshal(client.Scopes)
	metadataJSON, _ := json.Marshal(client.Metadata)
	client.UpdatedAt = time.Now().UTC()

	query := fmt.Sprintf(`UPDATE %sclient_credentials SET client_id = $1, client_secret = $2, name = $3, scopes = $4, metadata = $5, updated_at = $6, active = $7
			  WHERE id = $8`, prefix)

	result, err := db.ExecContext(ctx, query,
		client.ClientID, client.ClientSecret, client.Name,
		scopesJSON, metadataJSON,
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
}

func (s *PostgreSQLAuthStore) DeleteClient(ctx context.Context, id string) error {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	query := fmt.Sprintf(`DELETE FROM %sclient_credentials WHERE id = $1`, prefix)

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
}

func (s *PostgreSQLAuthStore) ListClients(ctx context.Context, offset, limit int) ([]*ClientCredential, int, error) {
	db := s.GetDB()
	prefix := s.GetTablePrefix()

	// Get total count
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %sclient_credentials`, prefix)
	if err := db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to get client count: %w", err)
	}

	// Get clients with pagination
	query := fmt.Sprintf(`SELECT id, client_id, client_secret, name, scopes, metadata, created_at, updated_at, active
			  FROM %sclient_credentials ORDER BY created_at DESC LIMIT $1 OFFSET $2`, prefix)

	rows, err := db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list clients: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var clients []*ClientCredential
	for rows.Next() {
		client, err := s.scanClientFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		clients = append(clients, client)
	}

	return clients, total, nil
}

// scanClient for PostgreSQL
func (s *PostgreSQLAuthStore) scanClient(row *sql.Row) (*ClientCredential, error) {
	var client ClientCredential
	var scopesJSON, metadataJSON []byte

	err := row.Scan(
		&client.ID, &client.ClientID, &client.ClientSecret, &client.Name,
		&scopesJSON, &metadataJSON,
		&client.CreatedAt, &client.UpdatedAt, &client.Active,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrClientNotFound
		}
		return nil, fmt.Errorf("failed to scan client: %w", err)
	}

	if len(scopesJSON) > 0 {
		_ = json.Unmarshal(scopesJSON, &client.Scopes)
	}
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &client.Metadata)
	}

	return &client, nil
}

// scanClientFromRows for PostgreSQL
func (s *PostgreSQLAuthStore) scanClientFromRows(rows *sql.Rows) (*ClientCredential, error) {
	var client ClientCredential
	var scopesJSON, metadataJSON []byte

	err := rows.Scan(
		&client.ID, &client.ClientID, &client.ClientSecret, &client.Name,
		&scopesJSON, &metadataJSON,
		&client.CreatedAt, &client.UpdatedAt, &client.Active,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan client: %w", err)
	}

	if len(scopesJSON) > 0 {
		_ = json.Unmarshal(scopesJSON, &client.Scopes)
	}
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &client.Metadata)
	}

	return &client, nil
}
