package database

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
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

func TestInTransactionCommitsOrRollsBack(t *testing.T) {
	ctx := context.Background()
	pool := databasetest.NewPool(t)
	_, err := pool.Exec(ctx, "CREATE TABLE items (name text)")
	require.NoError(t, err)

	require.NoError(t, InTransaction(ctx, pool, func(transaction pgx.Tx) error {
		_, err := transaction.Exec(ctx, "INSERT INTO items VALUES ('kept')")
		return err
	}))
	failure := errors.New("work failed")
	err = InTransaction(ctx, pool, func(transaction pgx.Tx) error {
		if _, err := transaction.Exec(ctx, "INSERT INTO items VALUES ('discarded')"); err != nil {
			return err
		}
		return failure
	})

	require.ErrorIs(t, err, failure)
	var names []string
	rows, err := pool.Query(ctx, "SELECT name FROM items")
	require.NoError(t, err)
	names, err = pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	assert.Equal(t, []string{"kept"}, names)
}
