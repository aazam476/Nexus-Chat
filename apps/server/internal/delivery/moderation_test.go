package delivery

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// directives decodes the signed directives the user's devices fetch from the group.
func directives(t *testing.T, f fixture, user uuid.UUID, groupID uuid.UUID) []Directive {
	t.Helper()
	messages, err := f.service.Messages(t.Context(), user, groupID, MessagePage{})
	require.NoError(t, err)
	out := []Directive{}
	for _, message := range messages {
		if message.Kind != "directive" {
			continue
		}
		var signed SignedDirective
		require.NoError(t, json.Unmarshal(message.Data, &signed))
		require.True(t, VerifyDirective(f.key, signed))
		var directive Directive
		require.NoError(t, json.Unmarshal(signed.Directive, &directive))
		out = append(out, directive)
	}
	return out
}

func applications(t *testing.T, f fixture, user uuid.UUID, groupID uuid.UUID) int {
	t.Helper()
	messages, err := f.service.Messages(t.Context(), user, groupID, MessagePage{})
	require.NoError(t, err)
	count := 0
	for _, message := range messages {
		if message.Kind == "application" {
			count++
		}
	}
	return count
}

func TestModeratorsDeleteChannelMessages(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	audienceID := f.grants.grant.AudienceID
	reply, err := setup.reply(t, setup.sam)
	require.NoError(t, err)
	mod := setup.ref(setup.ada)

	require.ErrorIs(t, f.service.DeleteChannelMessage(t.Context(), mod, reply), ErrNotAllowed, "only with MANAGE_MESSAGES")
	f.grants.grant.MayManageMessages = true
	require.ErrorIs(t, f.service.DeleteChannelMessage(t.Context(), mod, uuid.New()), ErrNoMessage)

	require.NoError(t, f.service.DeleteChannelMessage(t.Context(), mod, reply))
	assert.Equal(t, 1, applications(t, f, setup.sam, audienceID), "only the root's live copy is left")
	archived, err := f.service.ChannelArchive(t.Context(), mod, ArchivePage{Before: now.Add(time.Hour)})
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.Equal(t, setup.root, archived[0].MessageID)
	assert.Equal(t, int32(0), setup.threads(t, setup.ada)[0].ReplyCount)
	assert.Equal(t, []DeletedMessage{{MessageID: reply, SenderID: &setup.sam}}, f.grants.deleted)

	sent := directives(t, f, setup.sam, audienceID)
	require.Len(t, sent, 1)
	assert.Equal(t, DirectiveDelete, sent[0].Type)
	assert.Equal(t, &reply, sent[0].MessageID)
	assert.Equal(t, &setup.channel, sent[0].ChannelID)
	assert.Equal(t, setup.sam, sent[0].UserID)

	// The root is not in a thread; deleting it leaves the thread's count alone.
	require.NoError(t, f.service.DeleteChannelMessage(t.Context(), mod, setup.root))
	assert.Zero(t, applications(t, f, setup.sam, audienceID))

	f.grants.err = ErrNotFound
	require.ErrorIs(t, f.service.DeleteChannelMessage(t.Context(), mod, reply), ErrNotFound)
	f.grants.err = nil
	f.pool.Close()
	require.Error(t, f.service.DeleteChannelMessage(t.Context(), mod, reply))
}

func TestABanPurgesRecentMessages(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	audienceID := f.grants.grant.AudienceID
	_, err := setup.reply(t, setup.sam)
	require.NoError(t, err)
	_, err = setup.reply(t, setup.ada)
	require.NoError(t, err)

	purged, err := f.service.PurgeMessages(t.Context(), []uuid.UUID{audienceID}, setup.sam, now.Add(-time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged)
	assert.Equal(t, 2, applications(t, f, setup.ada, audienceID), "Ada's root and reply stay")
	assert.Equal(t, int32(1), setup.threads(t, setup.ada)[0].ReplyCount, "counts follow")
	sent := directives(t, f, setup.ada, audienceID)
	require.Len(t, sent, 1)
	assert.Equal(t, DirectivePurge, sent[0].Type)
	assert.True(t, now.Add(-time.Minute).Equal(*sent[0].Since))

	purged, err = f.service.PurgeMessages(t.Context(), []uuid.UUID{audienceID}, setup.ada, now.Add(time.Minute))
	require.NoError(t, err)
	assert.Zero(t, purged, "nothing that new")

	_, err = f.service.PurgeMessages(t.Context(), []uuid.UUID{uuid.New()}, setup.ada, now)
	require.Error(t, err, "a group that does not exist cannot be told")
	f.pool.Close()
	_, err = f.service.PurgeMessages(t.Context(), []uuid.UUID{audienceID}, setup.ada, now)
	require.Error(t, err)
}

func TestTheDeletionEndpoint(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	handler := NewHandler(f.service)
	remove := func(messageID uuid.UUID) api.DeleteChannelMessageResponseObject {
		return must(handler.DeleteChannelMessage(as(t, setup.ada), api.DeleteChannelMessageRequestObject{ChannelId: setup.channel, MessageId: messageID}))
	}
	assert.IsType(t, api.DeleteChannelMessage403JSONResponse{}, remove(setup.root))
	f.grants.grant.MayManageMessages = true
	assert.IsType(t, api.DeleteChannelMessage404JSONResponse{}, remove(uuid.New()))
	first, err := setup.reply(t, setup.sam)
	require.NoError(t, err)
	reply, err := setup.reply(t, setup.sam)
	require.NoError(t, err)
	f.grants.auditErr = errors.New("audit log is down")
	require.Error(t, second(handler.DeleteChannelMessage(as(t, setup.ada), api.DeleteChannelMessageRequestObject{ChannelId: setup.channel, MessageId: first})))
	f.grants.auditErr = nil
	assert.IsType(t, api.DeleteChannelMessage204Response{}, remove(reply))
	f.grants.err = ErrNotFound
	assert.IsType(t, api.DeleteChannelMessage404JSONResponse{}, remove(reply))
}
