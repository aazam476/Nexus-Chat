package store

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

func setUp(t *testing.T) (*Queries, *accountsstore.Queries, uuid.UUID) {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	accounts := accountsstore.New(pool)
	userID := uuid.New()
	hash := "hash"
	require.NoError(t, accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{
		ID: userID, Username: "ada", DisplayName: "Ada", Email: "ada@example.com", PasswordHash: &hash, CreatedAt: now,
	}))
	return New(pool), accounts, userID
}

func TestTheSaltIsKeptOnLaterCalls(t *testing.T) {
	backups, _, userID := setUp(t)
	first, err := backups.EnsureBackup(t.Context(), EnsureBackupParams{UserID: userID, Salt: make([]byte, 16), CreatedAt: now})
	require.NoError(t, err)
	second, err := backups.EnsureBackup(t.Context(), EnsureBackupParams{UserID: userID, Salt: []byte("another salt 16b"), CreatedAt: now})
	require.NoError(t, err)

	assert.Equal(t, make([]byte, 16), first)
	assert.Equal(t, first, second)
}

// An email reset can't re-wrap the backup key, so it drops only the password copy.
func TestAPasswordResetDropsThePasswordCopy(t *testing.T) {
	backups, accounts, userID := setUp(t)
	for _, method := range []string{"password", "recovery-phrase"} {
		require.NoError(t, backups.PutWrappedKey(t.Context(), PutWrappedKeyParams{UserID: userID, Method: method, WrappedKey: []byte{1}, UpdatedAt: now}))
	}
	hash := "new hash"
	require.NoError(t, accounts.SetPassword(t.Context(), accountsstore.SetPasswordParams{ID: userID, PasswordHash: &hash}))

	rows, err := backups.ListWrappedKeys(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "recovery-phrase", rows[0].Method)
	user, err := accounts.GetUserByID(t.Context(), userID)
	require.NoError(t, err)
	assert.Equal(t, &hash, user.PasswordHash)
}

func TestTheDatabaseEnforcesNamesAndMethods(t *testing.T) {
	backups, _, userID := setUp(t)

	require.Error(t, backups.PutWrappedKey(t.Context(), PutWrappedKeyParams{UserID: userID, Method: "email", WrappedKey: []byte{1}, UpdatedAt: now}))
	require.Error(t, backups.PutBlob(t.Context(), PutBlobParams{UserID: userID, Name: "../escape", Data: []byte{1}, UpdatedAt: now}))
	require.NoError(t, backups.PutBlob(t.Context(), PutBlobParams{UserID: userID, Name: "identity", Data: []byte{1, 2, 3}, UpdatedAt: now}))
	blobs, err := backups.ListBlobs(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, blobs, 1)
	assert.Equal(t, "identity", blobs[0].Name)
	assert.Equal(t, int32(3), blobs[0].Size)
	assert.True(t, now.Equal(blobs[0].UpdatedAt))
	_, err = backups.GetBlob(t.Context(), GetBlobParams{UserID: userID, Name: "missing"})
	require.ErrorIs(t, err, pgx.ErrNoRows)
}
