package store

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

func newQueries(t *testing.T) *Queries {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	return New(pool)
}

func TestInsertSettingIfMissingKeepsTheFirstValue(t *testing.T) {
	queries := newQueries(t)
	ctx := context.Background()

	first, err := queries.InsertSettingIfMissing(ctx, InsertSettingIfMissingParams{Key: "test.key", Value: []byte(`1`)})
	require.NoError(t, err)
	second, err := queries.InsertSettingIfMissing(ctx, InsertSettingIfMissingParams{Key: "test.key", Value: []byte(`2`)})
	require.NoError(t, err)

	assert.JSONEq(t, `1`, string(first.Value))
	assert.JSONEq(t, `1`, string(second.Value))
	assert.Equal(t, first.UpdatedAt, second.UpdatedAt)
}

func TestGetSetting(t *testing.T) {
	queries := newQueries(t)
	ctx := context.Background()
	_, err := queries.InsertSettingIfMissing(ctx, InsertSettingIfMissingParams{Key: "test.key", Value: []byte(`{"a":1}`)})
	require.NoError(t, err)

	setting, err := queries.GetSetting(ctx, "test.key")
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(setting.Value))

	_, err = queries.GetSetting(ctx, "missing.key")
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestSettingKeysMustBeLowercaseIdentifiers(t *testing.T) {
	queries := newQueries(t)

	for _, key := range []string{"", "Upper", "has space", "1starts.with.digit"} {
		_, err := queries.InsertSettingIfMissing(context.Background(), InsertSettingIfMissingParams{Key: key, Value: []byte(`1`)})
		assert.Error(t, err, "key %q should be rejected by the CHECK constraint", key)
	}
}
