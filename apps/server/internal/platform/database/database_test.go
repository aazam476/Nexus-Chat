package database

import (
	"context"
	"log/slog"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

func TestOpenConnectsToTheDatabase(t *testing.T) {
	pool, err := Open(context.Background(), databasetest.NewDatabaseURL(t))
	require.NoError(t, err)
	defer pool.Close()

	assert.NoError(t, pool.Ping(context.Background()))
}

func TestOpenRejectsMalformedURL(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user:pass@host:notaport/db")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuring database connection")
}

func TestOpenFailsWhenDatabaseDoesNotExist(t *testing.T) {
	parsed, err := url.Parse(databasetest.NewDatabaseURL(t))
	require.NoError(t, err)
	parsed.Path += "_missing"

	_, err = Open(context.Background(), parsed.String())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connecting to database")
}

func TestMigrateCreatesSchemaAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := databasetest.NewPool(t)
	logger := slog.New(slog.DiscardHandler)

	require.NoError(t, Migrate(ctx, pool, logger))
	require.NoError(t, Migrate(ctx, pool, logger), "running again must apply nothing and succeed")

	var exists bool
	err := pool.QueryRow(ctx, "SELECT to_regclass('instance_settings') IS NOT NULL").Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestMigrateFailsWhenDatabaseIsClosed(t *testing.T) {
	pool := databasetest.NewPool(t)
	pool.Close()

	err := Migrate(context.Background(), pool, slog.New(slog.DiscardHandler))

	assert.Error(t, err)
}
