package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Session represents a user session
type Session struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	UserID    string    `json:"user_id"`
	Roles     []string  `json:"roles"`
	ServerURL string    `json:"server_url"`
}

// SessionManager handles session storage and retrieval
type SessionManager struct {
	sessionPath string
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		homeDir = "."
	}

	sessionDir := filepath.Join(homeDir, ".provisr")
	_ = os.MkdirAll(sessionDir, 0o700) // Create directory if it doesn't exist

	return &SessionManager{
		sessionPath: filepath.Join(sessionDir, "session.json"),
	}
}

// SaveSession saves a session to disk
func (sm *SessionManager) SaveSession(session *Session) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sm.sessionPath, data, 0o600) // Only user can read/write
}

// LoadSession loads a session from disk
func (sm *SessionManager) LoadSession() (*Session, error) {
	data, err := os.ReadFile(sm.sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No session exists
		}
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		_ = sm.ClearSession() // Remove expired session
		return nil, nil
	}

	return &session, nil
}

// ClearSession removes the session file
func (sm *SessionManager) ClearSession() error {
	if err := os.Remove(sm.sessionPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsLoggedIn checks if there's a valid session
func (sm *SessionManager) IsLoggedIn() bool {
	session, err := sm.LoadSession()
	return err == nil && session != nil
}

// GetSessionPath returns the path to the session file
func (sm *SessionManager) GetSessionPath() string {
	return sm.sessionPath
}
