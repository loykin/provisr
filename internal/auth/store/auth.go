package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

// User represents a user in the auth system
type User struct {
	ID           string            `json:"id" db:"id"`
	Username     string            `json:"username" db:"username"`
	PasswordHash string            `json:"-" db:"password_hash"`
	Email        string            `json:"email,omitempty" db:"email"`
	Roles        []string          `json:"roles" db:"roles"`
	Metadata     map[string]string `json:"metadata,omitempty" db:"metadata"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at" db:"updated_at"`
	Active       bool              `json:"active" db:"active"`
}

// UserStore defines the interface for user storage operations
type UserStore interface {
	CreateUser(ctx context.Context, user *User) error
	CreateFirstUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, offset, limit int) ([]*User, int, error)
}

// CreateFirstUser inserts user only when the users table is still empty. The
// condition and insert live in one SQL statement, so concurrent bootstrap
// requests cannot both create an administrator.
func (s *authStore) CreateFirstUser(ctx context.Context, user *User) error {
	rolesJSON, _ := json.Marshal(user.Roles)
	metadataJSON, _ := json.Marshal(user.Metadata)

	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin first-user transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Every bootstrap competes for the same primary key. This is stronger
		// than a plain NOT EXISTS(users) predicate on PostgreSQL, where two
		// READ COMMITTED snapshots could otherwise both observe an empty table.
		guardQuery := tx.Rebind(`INSERT INTO auth_bootstrap_guard (id)
			SELECT ? WHERE NOT EXISTS (SELECT 1 FROM users)`)
		result, err := tx.ExecContext(ctx, guardQuery, 1)
		if err != nil {
			if isUniqueViolation(err) {
				return ErrUserAlreadyExists
			}
			return fmt.Errorf("claim bootstrap guard: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("get bootstrap guard affected rows: %w", err)
		}
		if affected == 0 {
			return ErrUserAlreadyExists
		}

		query := tx.Rebind(`INSERT INTO users (id, username, password_hash, email, roles, metadata, created_at, updated_at, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		_, err = tx.ExecContext(ctx, query,
			user.ID, user.Username, user.PasswordHash, user.Email,
			string(rolesJSON), string(metadataJSON),
			user.CreatedAt, user.UpdatedAt, user.Active)
		if err != nil {
			if isUniqueViolation(err) {
				return ErrUserAlreadyExists
			}
			return fmt.Errorf("failed to create first user: %w", err)
		}
		if err := tx.Commit(); err != nil {
			if isUniqueViolation(err) {
				return ErrUserAlreadyExists
			}
			return fmt.Errorf("commit first-user transaction: %w", err)
		}
		return nil
	})
}

// AuthStore provides user persistence and connection lifecycle operations.
type AuthStore interface {
	Store
	UserStore
}

// NewAuthStore creates a new auth store based on the configuration
func NewAuthStore(config Config) (AuthStore, error) {
	switch config.Type {
	case "sqlite", "":
		return NewSQLiteAuthStore(config)
	case "postgresql", "postgres":
		return NewPostgreSQLAuthStore(config)
	default:
		return nil, fmt.Errorf("unsupported auth store type: %s", config.Type)
	}
}

func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate key")
}

// userRow mirrors the users table for scanning; Roles/Metadata are stored
// as JSON text (TEXT on SQLite, JSONB on PostgreSQL) and converted to/from
// User's typed fields at the boundary.
type userRow struct {
	ID           string         `db:"id"`
	Username     string         `db:"username"`
	PasswordHash string         `db:"password_hash"`
	Email        sql.NullString `db:"email"`
	Roles        sql.NullString `db:"roles"`
	Metadata     sql.NullString `db:"metadata"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
	Active       bool           `db:"active"`
}

func (r userRow) toUser() *User {
	u := &User{
		ID:           r.ID,
		Username:     r.Username,
		PasswordHash: r.PasswordHash,
		Email:        r.Email.String,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
		Active:       r.Active,
	}
	if r.Roles.Valid {
		_ = json.Unmarshal([]byte(r.Roles.String), &u.Roles)
	}
	if r.Metadata.Valid {
		_ = json.Unmarshal([]byte(r.Metadata.String), &u.Metadata)
	}
	return u
}

func (s *authStore) CreateUser(ctx context.Context, user *User) error {
	rolesJSON, _ := json.Marshal(user.Roles)
	metadataJSON, _ := json.Marshal(user.Metadata)

	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`INSERT INTO users (id, username, password_hash, email, roles, metadata, created_at, updated_at, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		_, err := db.ExecContext(ctx, query,
			user.ID, user.Username, user.PasswordHash, user.Email,
			string(rolesJSON), string(metadataJSON),
			user.CreatedAt, user.UpdatedAt, user.Active)
		if err != nil {
			if isUniqueViolation(err) {
				return ErrUserAlreadyExists
			}
			return fmt.Errorf("failed to create user: %w", err)
		}
		return nil
	})
}

func (s *authStore) GetUser(ctx context.Context, id string) (*User, error) {
	var row userRow
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`SELECT id, username, password_hash, email, roles, metadata, created_at, updated_at, active
			FROM users WHERE id = ?`)
		return db.GetContext(ctx, &row, query, id)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return row.toUser(), nil
}

func (s *authStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var row userRow
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`SELECT id, username, password_hash, email, roles, metadata, created_at, updated_at, active
			FROM users WHERE username = ? AND active = true`)
		return db.GetContext(ctx, &row, query, username)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return row.toUser(), nil
}

func (s *authStore) UpdateUser(ctx context.Context, user *User) error {
	rolesJSON, _ := json.Marshal(user.Roles)
	metadataJSON, _ := json.Marshal(user.Metadata)
	user.UpdatedAt = time.Now().UTC()

	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`UPDATE users SET username = ?, password_hash = ?, email = ?, roles = ?, metadata = ?, updated_at = ?, active = ?
			WHERE id = ?`)
		result, err := db.ExecContext(ctx, query,
			user.Username, user.PasswordHash, user.Email,
			string(rolesJSON), string(metadataJSON),
			user.UpdatedAt, user.Active, user.ID)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}
		if affected == 0 {
			return ErrUserNotFound
		}
		return nil
	})
}

func (s *authStore) DeleteUser(ctx context.Context, id string) error {
	return s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		query := db.Rebind(`DELETE FROM users WHERE id = ?`)
		result, err := db.ExecContext(ctx, query, id)
		if err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}
		if affected == 0 {
			return ErrUserNotFound
		}
		return nil
	})
}

func (s *authStore) ListUsers(ctx context.Context, offset, limit int) ([]*User, int, error) {
	var total int
	var rows []userRow
	err := s.Run(ctx, func(ctx context.Context, db *sqlx.DB) error {
		if err := db.GetContext(ctx, &total, `SELECT COUNT(*) FROM users`); err != nil {
			return fmt.Errorf("failed to get user count: %w", err)
		}
		query := db.Rebind(`SELECT id, username, password_hash, email, roles, metadata, created_at, updated_at, active
			FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`)
		return db.SelectContext(ctx, &rows, query, limit, offset)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	users := make([]*User, len(rows))
	for i, row := range rows {
		users[i] = row.toUser()
	}
	return users, total, nil
}
