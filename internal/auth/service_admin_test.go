package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func newTestAuthService(t *testing.T) *AuthService {
	t.Helper()
	service, err := NewAuthService(AuthConfig{Store: StoreConfig{Type: "sqlite", Path: t.TempDir() + "/auth.db"}})
	if err != nil {
		t.Fatalf("NewAuthService() error: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func TestBootstrapFirstAdminConcurrent(t *testing.T) {
	service := newTestAuthService(t)
	ctx := context.Background()
	const requests = 8
	start := make(chan struct{})
	errs := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := service.BootstrapFirstAdmin(ctx, fmt.Sprintf("admin-%d", i), "password123")
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	succeeded := 0
	for err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrAlreadyBootstrapped):
		default:
			t.Fatalf("unexpected bootstrap error: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful bootstraps = %d, want 1", succeeded)
	}
	users, total, err := service.ListUsers(ctx, 0, requests)
	if err != nil || total != 1 || len(users) != 1 {
		t.Fatalf("users after bootstrap = %d/%d, err=%v; want 1", len(users), total, err)
	}
}

func TestLastActiveAdminCannotBeRemoved(t *testing.T) {
	service := newTestAuthService(t)
	ctx := context.Background()
	admin, err := service.CreateUser(ctx, "admin", "password123", "", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	admin.Roles = []string{"viewer"}
	if err := service.UpdateUser(ctx, admin); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("demote error = %v, want ErrLastActiveAdmin", err)
	}
	admin.Roles = []string{"admin"}
	admin.Active = false
	if err := service.UpdateUser(ctx, admin); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("deactivate error = %v, want ErrLastActiveAdmin", err)
	}
	if err := service.DeleteUser(ctx, admin.ID); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("delete error = %v, want ErrLastActiveAdmin", err)
	}
}

func TestAdminCanBeRemovedWhenAnotherActiveAdminExists(t *testing.T) {
	service := newTestAuthService(t)
	ctx := context.Background()
	first, err := service.CreateUser(ctx, "admin-1", "password123", "", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateUser(ctx, "admin-2", "password123", "", []string{"admin"}, nil); err != nil {
		t.Fatal(err)
	}
	first.Roles = []string{"viewer"}
	if err := service.UpdateUser(ctx, first); err != nil {
		t.Fatalf("demote with another admin: %v", err)
	}
	if err := service.DeleteUser(ctx, first.ID); err != nil {
		t.Fatalf("delete former admin: %v", err)
	}
}
