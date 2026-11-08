package store

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

var now = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func newStock(t *testing.T) (*Stock, *pgxpool.Pool) {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	return NewStock(pool), pool
}

func newUser(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	hash := "hash"
	require.NoError(t, accountsstore.New(pool).InsertUser(context.Background(), accountsstore.InsertUserParams{
		ID: id, Username: name, DisplayName: name, Email: name + "@example.com", PasswordHash: &hash, CreatedAt: now,
	}))
	return id
}

func newDevice(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	tokenHash := sha256.Sum256(id[:])
	require.NoError(t, sessionstore.New(pool).InsertSession(context.Background(), sessionstore.InsertSessionParams{
		ID: id, UserID: userID, TokenHash: tokenHash[:], DeviceName: "Phone",
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour), AuthenticatedAt: now,
	}))
	return id
}

func packages(prefix byte, count int) [][]byte {
	result := make([][]byte, count)
	for index := range result {
		result[index] = []byte{0, 1, 0, 5, 0, 1, 0, 1, prefix, byte(index)}
	}
	return result
}

func upload(user uuid.UUID, device uuid.UUID, oneTime [][]byte, lastResort []byte) Upload {
	return Upload{SessionID: device, UserID: user, OneTime: oneTime, LastResort: lastResort, Now: now, MaximumOneTime: 5}
}

func TestUploadCountsAndLimitsTheStock(t *testing.T) {
	stock, pool := newStock(t)
	ctx := context.Background()
	user := newUser(t, pool, "ada")
	device := newDevice(t, pool, user)

	count, err := stock.Add(ctx, upload(user, device, packages(1, 3), []byte("last-resort-1")))
	require.NoError(t, err)
	assert.Equal(t, CountKeyPackagesRow{OneTime: 3, HasLastResort: true}, count)

	_, err = stock.Add(ctx, upload(user, device, packages(2, 3), nil))
	require.ErrorIs(t, err, ErrStockFull)
	count, err = stock.Count(ctx, device)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count.OneTime, "a rejected upload stores nothing")

	count, err = stock.Add(ctx, upload(user, device, nil, []byte("last-resort-2")))
	require.NoError(t, err)
	assert.Equal(t, CountKeyPackagesRow{OneTime: 3, HasLastResort: true}, count, "the last-resort package is replaced, not added")
}

func TestClaimsHandOutOnePerDeviceThenTheLastResort(t *testing.T) {
	stock, pool := newStock(t)
	ctx := context.Background()
	user := newUser(t, pool, "grace")
	phone := newDevice(t, pool, user)
	laptop := newDevice(t, pool, user)
	_, err := stock.Add(ctx, upload(user, phone, packages(1, 1), []byte("phone-last-resort")))
	require.NoError(t, err)
	_, err = stock.Add(ctx, upload(user, laptop, packages(2, 2), nil))
	require.NoError(t, err)

	first, err := stock.Claim(ctx, user)
	require.NoError(t, err)
	assert.Len(t, first, 2, "one package for each device")
	for _, claimed := range first {
		assert.False(t, claimed.LastResort)
	}

	second, err := stock.Claim(ctx, user)
	require.NoError(t, err)
	require.Len(t, second, 2)
	byDevice := map[uuid.UUID]Claimed{second[0].SessionID: second[0], second[1].SessionID: second[1]}
	assert.Equal(t, Claimed{SessionID: phone, Data: []byte("phone-last-resort"), LastResort: true}, byDevice[phone])
	assert.False(t, byDevice[laptop].LastResort)

	third, err := stock.Claim(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, []Claimed{{SessionID: phone, Data: []byte("phone-last-resort"), LastResort: true}}, third,
		"the last-resort package is never used up; the laptop has nothing left")
}

func TestPackagesGoWithTheSession(t *testing.T) {
	stock, pool := newStock(t)
	ctx := context.Background()
	user := newUser(t, pool, "linus")
	device := newDevice(t, pool, user)
	_, err := stock.Add(ctx, upload(user, device, packages(1, 2), []byte("last-resort-package")))
	require.NoError(t, err)

	_, err = sessionstore.New(pool).DeleteSession(ctx, sessionstore.DeleteSessionParams{ID: device, UserID: user})
	require.NoError(t, err)

	claimed, err := stock.Claim(ctx, user)
	require.NoError(t, err)
	assert.Empty(t, claimed)
}
