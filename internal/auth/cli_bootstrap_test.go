package auth

import (
	"context"
	"testing"
)

func newTestAuthService(t *testing.T) *AuthService {
	t.Helper()
	svc, err := NewAuthService(AuthConfig{
		Store: StoreConfig{Type: "sqlite", Path: ":memory:"},
	})
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestCLIHelper_EnsureInitialAdmin_CreatesOnEmptyStore(t *testing.T) {
	svc := newTestAuthService(t)
	helper := NewCLIHelper(svc)
	ctx := context.Background()

	password, created, err := helper.EnsureInitialAdmin(ctx, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Fatal("expected created=true on an empty store")
	}
	if password == "" {
		t.Fatal("expected a non-empty generated password")
	}

	result, err := svc.Authenticate(ctx, LoginRequest{
		Method:   AuthMethodBasic,
		Username: "admin",
		Password: password,
	})
	if err != nil {
		t.Fatalf("unexpected error authenticating with the generated password: %v", err)
	}
	if !result.Success {
		t.Fatal("expected the generated admin password to authenticate successfully")
	}
	if len(result.Roles) != 1 || result.Roles[0] != "admin" {
		t.Fatalf("expected roles=[admin], got %v", result.Roles)
	}
}

func TestCLIHelper_EnsureInitialAdmin_NoopWhenUsersExist(t *testing.T) {
	svc := newTestAuthService(t)
	helper := NewCLIHelper(svc)
	ctx := context.Background()

	if _, err := svc.CreateUser(ctx, "someone", "irrelevant-password", "", []string{"viewer"}, nil); err != nil {
		t.Fatalf("failed to seed an existing user: %v", err)
	}

	password, created, err := helper.EnsureInitialAdmin(ctx, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Fatal("expected created=false when a user already exists")
	}
	if password != "" {
		t.Fatal("expected an empty password when nothing was created")
	}

	if _, err := svc.store.GetUserByUsername(ctx, "admin"); err == nil {
		t.Fatal("expected no admin user to have been created")
	}
}
