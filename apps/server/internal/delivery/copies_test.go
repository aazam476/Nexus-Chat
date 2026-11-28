package delivery

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

func TestReportsFindStoredCopiesOnlyForReaders(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	found, err := f.service.ChannelCopy(t.Context(), setup.ref(setup.sam), setup.root)
	require.NoError(t, err)
	assert.Equal(t, setup.root, found.MessageID)
	assert.Equal(t, &setup.ada, found.SenderID)
	assert.Equal(t, []byte("sealed"), found.Sealed)
	assert.Equal(t, &setup.channel, found.ChannelID)

	_, err = f.service.ChannelCopy(t.Context(), setup.ref(setup.sam), uuid.New())
	require.ErrorIs(t, err, ErrNoMessage)
	other := setup.ref(setup.sam)
	other.ChannelID = uuid.New()
	_, err = f.service.ChannelCopy(t.Context(), other, setup.root)
	require.ErrorIs(t, err, ErrNoMessage, "a message of another channel in the same audience")
	_, err = f.service.ConversationCopy(t.Context(), setup.sam, f.grants.grant.AudienceID, setup.root)
	require.ErrorIs(t, err, ErrNotFound, "a channel's message is reported by its channel")
	f.grants.err = ErrNotFound
	_, err = f.service.ChannelCopy(t.Context(), setup.ref(setup.sam), setup.root)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestReportsFindDirectMessageCopies(t *testing.T) {
	f := newFixture(t)
	dm, ada, grace := f.dm(t)
	messageID := uuid.New()
	_, err := f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(dm.MLSGroupID, 0, 1), Archive: &Archive{MessageID: messageID, Sealed: []byte("sealed")}})
	require.NoError(t, err)

	found, err := f.service.ConversationCopy(t.Context(), grace, dm.ID, messageID)
	require.NoError(t, err)
	assert.Equal(t, &ada, found.SenderID)
	assert.Nil(t, found.ChannelID)
	_, err = f.service.ConversationCopy(t.Context(), f.user(t, "eve"), dm.ID, messageID)
	require.ErrorIs(t, err, ErrNotFound, "only members")
	f.pool.Close()
	_, err = f.service.ConversationCopy(t.Context(), grace, dm.ID, messageID)
	require.Error(t, err)
}

func TestArchiveCopyEndpoints(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	handler := NewHandler(f.service)
	get := func(messageID uuid.UUID) api.GetChannelArchivedMessageResponseObject {
		return must(handler.GetChannelArchivedMessage(as(t, setup.sam), api.GetChannelArchivedMessageRequestObject{ChannelId: setup.channel, MessageId: messageID}))
	}
	found, ok := get(setup.root).(api.GetChannelArchivedMessage200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, setup.group, found.MlsGroupId)
	assert.Equal(t, setup.root, found.MessageId)
	assert.IsType(t, api.GetChannelArchivedMessage404JSONResponse{}, get(uuid.New()))
	f.grants.grant.HistorySince = now.Add(time.Hour)
	assert.IsType(t, api.GetChannelArchivedMessage404JSONResponse{}, get(setup.root), "from before they may read")
	f.grants.err = ErrNotFound
	assert.IsType(t, api.GetChannelArchivedMessage404JSONResponse{}, get(setup.root))
	f.grants.err = errors.New("servers are down")
	require.Error(t, second(handler.GetChannelArchivedMessage(as(t, setup.sam), api.GetChannelArchivedMessageRequestObject{ChannelId: setup.channel, MessageId: setup.root})))
	require.ErrorIs(t, second(handler.GetChannelArchivedMessage(t.Context(), api.GetChannelArchivedMessageRequestObject{})), errNoPrincipal)
	require.ErrorIs(t, second(handler.GetArchivedMessage(t.Context(), api.GetArchivedMessageRequestObject{})), errNoPrincipal)
}

func TestTheConversationArchiveCopyEndpoint(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service)
	dm, ada, grace := f.dm(t)
	messageID := uuid.New()
	_, err := f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(dm.MLSGroupID, 0, 1), Archive: &Archive{MessageID: messageID, Sealed: []byte("sealed")}})
	require.NoError(t, err)
	get := func(messageID uuid.UUID) api.GetArchivedMessageResponseObject {
		return must(handler.GetArchivedMessage(as(t, grace), api.GetArchivedMessageRequestObject{ConversationId: dm.ID, MessageId: messageID}))
	}
	found, ok := get(messageID).(api.GetArchivedMessage200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, []byte("sealed"), found.Sealed)
	assert.IsType(t, api.GetArchivedMessage404JSONResponse{}, get(uuid.New()))
	f.pool.Close()
	require.Error(t, second(handler.GetArchivedMessage(as(t, grace), api.GetArchivedMessageRequestObject{ConversationId: dm.ID, MessageId: messageID})))
}
