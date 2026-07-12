package store

import (
	"context"
	"testing"
	"time"
)

func newTestSQLiteAuthStore(t *testing.T) *SQLiteAuthStore {
	t.Helper()
	s, err := NewSQLiteAuthStore(Config{Type: "sqlite", Path: ":memory:"})
	if err != nil {
		t.Fatalf("NewSQLiteAuthStore() error: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteAuthStoreCanSkipMigrations(t *testing.T) {
	migrate := false
	s, err := NewSQLiteAuthStore(Config{Type: "sqlite", Path: ":memory:", Migrate: &migrate})
	if err != nil {
		t.Fatalf("NewSQLiteAuthStore() error: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, _, err := s.ListUsers(context.Background(), 0, 10); err == nil {
		t.Fatal("ListUsers() succeeded without a pre-migrated schema")
	}
}

func TestSQLiteAuthStore_UserCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestSQLiteAuthStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	user := &User{
		ID:           "u1",
		Username:     "alice",
		PasswordHash: "hash1",
		Email:        "alice@example.com",
		Roles:        []string{"admin", "user"},
		Metadata:     map[string]string{"team": "core"},
		CreatedAt:    now,
		UpdatedAt:    now,
		Active:       true,
	}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	if err := s.CreateUser(ctx, user); err != ErrUserAlreadyExists {
		t.Fatalf("expected ErrUserAlreadyExists on duplicate, got %v", err)
	}

	got, err := s.GetUser(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUser() error: %v", err)
	}
	if got.Username != "alice" || len(got.Roles) != 2 || got.Metadata["team"] != "core" {
		t.Errorf("unexpected user: %+v", got)
	}

	byName, err := s.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername() error: %v", err)
	}
	if byName.ID != "u1" {
		t.Errorf("expected id u1, got %s", byName.ID)
	}

	if _, err := s.GetUser(ctx, "missing"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	got.Email = "alice@newdomain.com"
	got.Roles = []string{"user"}
	if err := s.UpdateUser(ctx, got); err != nil {
		t.Fatalf("UpdateUser() error: %v", err)
	}
	updated, err := s.GetUser(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUser() after update error: %v", err)
	}
	if updated.Email != "alice@newdomain.com" || len(updated.Roles) != 1 {
		t.Errorf("update did not persist: %+v", updated)
	}

	if err := s.UpdateUser(ctx, &User{ID: "missing"}); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound on update of missing user, got %v", err)
	}

	second := &User{ID: "u2", Username: "bob", PasswordHash: "hash2", CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second), Active: true}
	if err := s.CreateUser(ctx, second); err != nil {
		t.Fatalf("CreateUser(second) error: %v", err)
	}

	list, total, err := s.ListUsers(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListUsers() error: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("expected 2 users, got total=%d len=%d", total, len(list))
	}
	if list[0].Username != "bob" {
		t.Errorf("expected newest user first (bob), got %s", list[0].Username)
	}

	if err := s.DeleteUser(ctx, "u1"); err != nil {
		t.Fatalf("DeleteUser() error: %v", err)
	}
	if err := s.DeleteUser(ctx, "u1"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound on second delete, got %v", err)
	}
}

func TestSQLiteAuthStore_ClientCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestSQLiteAuthStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	client := &ClientCredential{
		ID:           "c1",
		ClientID:     "client-1",
		ClientSecret: "secret1",
		Name:         "Test Client",
		Scopes:       []string{"read", "write"},
		Metadata:     map[string]string{"owner": "team-a"},
		CreatedAt:    now,
		UpdatedAt:    now,
		Active:       true,
	}
	if err := s.CreateClient(ctx, client); err != nil {
		t.Fatalf("CreateClient() error: %v", err)
	}
	if err := s.CreateClient(ctx, client); err != ErrClientAlreadyExists {
		t.Fatalf("expected ErrClientAlreadyExists on duplicate, got %v", err)
	}

	got, err := s.GetClient(ctx, "c1")
	if err != nil {
		t.Fatalf("GetClient() error: %v", err)
	}
	if got.ClientID != "client-1" || len(got.Scopes) != 2 {
		t.Errorf("unexpected client: %+v", got)
	}

	byClientID, err := s.GetClientByClientID(ctx, "client-1")
	if err != nil {
		t.Fatalf("GetClientByClientID() error: %v", err)
	}
	if byClientID.ID != "c1" {
		t.Errorf("expected id c1, got %s", byClientID.ID)
	}

	if _, err := s.GetClient(ctx, "missing"); err != ErrClientNotFound {
		t.Fatalf("expected ErrClientNotFound, got %v", err)
	}

	got.Name = "Renamed Client"
	if err := s.UpdateClient(ctx, got); err != nil {
		t.Fatalf("UpdateClient() error: %v", err)
	}
	updated, err := s.GetClient(ctx, "c1")
	if err != nil {
		t.Fatalf("GetClient() after update error: %v", err)
	}
	if updated.Name != "Renamed Client" {
		t.Errorf("update did not persist: %+v", updated)
	}

	list, total, err := s.ListClients(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListClients() error: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected 1 client, got total=%d len=%d", total, len(list))
	}

	if err := s.DeleteClient(ctx, "c1"); err != nil {
		t.Fatalf("DeleteClient() error: %v", err)
	}
	if err := s.DeleteClient(ctx, "c1"); err != ErrClientNotFound {
		t.Fatalf("expected ErrClientNotFound on second delete, got %v", err)
	}
}

func TestSQLiteAuthStore_PingAndClose(t *testing.T) {
	s := newTestSQLiteAuthStore(t)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestNewAuthStore_Dispatch(t *testing.T) {
	s, err := NewAuthStore(Config{Type: "sqlite", Path: ":memory:"})
	if err != nil {
		t.Fatalf("NewAuthStore(sqlite) error: %v", err)
	}
	_ = s.Close()

	if _, err := NewAuthStore(Config{Type: "unsupported"}); err == nil {
		t.Fatal("expected error for unsupported store type")
	}
}
