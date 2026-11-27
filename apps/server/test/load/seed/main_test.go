package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/mlsheader"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

func TestSeedMakesUsersInOneServer(t *testing.T) {
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))

	seed, err := build(t.Context(), pool, 3)

	require.NoError(t, err)
	assert.Len(t, seed.Tokens, 3)
	var members int
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT count(*) FROM conversation_members WHERE conversation_id = $1", seed.AudienceID).Scan(&members))
	assert.Equal(t, 3, members, "everyone is in #general's audience")
	var groupID []byte
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT mls_group_id FROM conversations WHERE id = $1", seed.AudienceID).Scan(&groupID))
	header, err := mlsheader.Parse(applicationMessage(groupID))
	require.NoError(t, err)
	assert.Equal(t, mlsheader.Header{WireFormat: mlsheader.WireFormatPrivateMessage, GroupID: groupID, ContentType: mlsheader.ContentTypeApplication}, header)
}

func TestSeedNeedsADatabaseAndUsers(t *testing.T) {
	require.Error(t, run(t.Context(), "", 5))
	require.Error(t, run(t.Context(), "postgres://nobody@127.0.0.1:1/none?connect_timeout=1", 5))
}
