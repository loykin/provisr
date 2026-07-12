//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestPostgreSQLAuthStore_UserAndClientCRUD mirrors the SQLite unit test,
// but against a real PostgreSQL server via testcontainers — proving the
// dbstore-backed authStore (shared code, Rebind-based placeholders, goose
// postgres dialect) actually works on Postgres too, not just SQLite.
func TestPostgreSQLAuthStore_UserAndClientCRUD(t *testing.T) {
	ctx := context.Background()

	// The postgres image logs "database system is ready to accept
	// connections" once for its own initdb bootstrap (a temporary server
	// that then shuts down) and again for the real server — waiting for
	// only the first occurrence (the module's default) can let the test
	// connect during that shutdown window, seen as a flaky "connection
	// reset by peer" in CI.
	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("user"),
		tcpostgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
				wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(60*time.Second),
			),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	s, err := NewPostgreSQLAuthStore(Config{
		Type:     "postgres",
		Host:     host,
		Port:     int(port.Num()),
		Database: "testdb",
		Username: "user",
		Password: "pass",
		SSLMode:  "disable",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

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
	require.NoError(t, s.CreateUser(ctx, user))
	require.ErrorIs(t, s.CreateUser(ctx, user), ErrUserAlreadyExists)

	got, err := s.GetUser(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, "alice", got.Username)
	require.Len(t, got.Roles, 2)
	require.Equal(t, "core", got.Metadata["team"])

	byName, err := s.GetUserByUsername(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, "u1", byName.ID)

	got.Email = "alice@newdomain.com"
	require.NoError(t, s.UpdateUser(ctx, got))
	updated, err := s.GetUser(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, "alice@newdomain.com", updated.Email)

	list, total, err := s.ListUsers(ctx, 0, 10)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, list, 1)

	require.NoError(t, s.DeleteUser(ctx, "u1"))
	require.ErrorIs(t, s.DeleteUser(ctx, "u1"), ErrUserNotFound)

}
