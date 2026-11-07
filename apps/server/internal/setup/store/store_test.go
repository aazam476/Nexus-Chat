package store

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	return New(pool)
}

func completion(username string) Completion {
	hash := "hash"
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	return Completion{
		Admin: accountsstore.InsertUserParams{
			ID: uuid.Must(uuid.NewV7()), Username: username, DisplayName: "Admin",
			Email: username + "@example.com", PasswordHash: &hash, IsInstanceAdmin: true, CreatedAt: now,
		},
		Settings:    map[string]string{instanceconfig.KeyInstanceName: "Example", instanceconfig.KeyInstanceDomain: "chat.example.com"},
		CompletedAt: now,
	}
}

func TestCompleteSavesEverything(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	complete, err := store.IsComplete(ctx)
	require.NoError(t, err)
	require.False(t, complete)

	require.NoError(t, store.Complete(ctx, completion("admin")))

	complete, err = store.IsComplete(ctx)
	require.NoError(t, err)
	assert.True(t, complete)
	domain, err := store.StoredString(ctx, instanceconfig.KeyInstanceDomain)
	require.NoError(t, err)
	assert.Equal(t, "chat.example.com", domain)
	user, err := accountsstore.New(store.pool).GetUserByUsername(ctx, "admin")
	require.NoError(t, err)
	assert.True(t, user.IsInstanceAdmin)
}

func TestCompleteSucceedsOnlyOnceUnderConcurrency(t *testing.T) {
	store := newStore(t)
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		waitGroup.Go(func() { results <- store.Complete(context.Background(), completion(name)) })
	}
	waitGroup.Wait()
	close(results)

	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		require.ErrorIs(t, err, ErrAlreadyComplete)
		conflicts++
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, conflicts)
	admins, err := accountsstore.New(store.pool).CountInstanceAdmins(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), admins, "the losing transaction must not leave an admin behind")
}

func TestCompleteRollsBackOnInvalidAdmin(t *testing.T) {
	store := newStore(t)
	bad := completion("UPPERCASE") // violates the username CHECK constraint

	err := store.Complete(context.Background(), bad)

	require.Error(t, err)
	complete, err := store.IsComplete(context.Background())
	require.NoError(t, err)
	assert.False(t, complete, "setup must not be marked complete when the admin insert fails")
}

func TestStoredStringMissingAndCorrupt(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	value, err := store.StoredString(ctx, "missing.key")
	require.NoError(t, err)
	assert.Empty(t, value)

	_, err = store.pool.Exec(ctx, "INSERT INTO instance_settings (key, value) VALUES ('number.key', '42')")
	require.NoError(t, err)
	_, err = store.StoredString(ctx, "number.key")
	assert.ErrorContains(t, err, "decoding setting")
}

func TestReadsFailWhenDatabaseIsGone(t *testing.T) {
	store := newStore(t)
	store.pool.Close()

	_, err := store.IsComplete(context.Background())
	require.ErrorContains(t, err, "reading setup state")
	_, err = store.StoredString(context.Background(), "x")
	require.ErrorContains(t, err, "reading setting")
	assert.Error(t, store.Complete(context.Background(), completion("admin")))
}
