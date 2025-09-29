package auth

import (
	"time"
)

// AuthMethod represents the type of authentication
type AuthMethod string

const (
	AuthMethodBasic        AuthMethod = "basic"         // username/password
	AuthMethodClientSecret AuthMethod = "client_secret" // client_id/client_secret
	AuthMethodJWT          AuthMethod = "jwt"           // JWT token
)

// User and ClientCredential types are now imported from the store package

// AuthResult represents the result of authentication
type AuthResult struct {
	Success  bool              `json:"success"`
	UserID   string            `json:"user_id,omitempty"`
	Username string            `json:"username,omitempty"`
	Roles    []string          `json:"roles,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Token    *Token            `json:"token,omitempty"`
}

// Token represents a JWT token
type Token struct {
	Type      string    `json:"type"`  // "Bearer"
	Value     string    `json:"value"` // JWT token string
	ExpiresAt time.Time `json:"expires_at"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Method       AuthMethod `json:"method"`
	Username     string     `json:"username,omitempty"`
	Password     string     `json:"password,omitempty"`
	ClientID     string     `json:"client_id,omitempty"`
	ClientSecret string     `json:"client_secret,omitempty"`
	Token        string     `json:"token,omitempty"`
}

// Permission represents a permission in the system
type Permission struct {
	Resource string `json:"resource"` // e.g., "process", "job", "cronjob"
	Action   string `json:"action"`   // e.g., "read", "write", "delete"
}

// Role represents a role with associated permissions
type Role struct {
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
}
