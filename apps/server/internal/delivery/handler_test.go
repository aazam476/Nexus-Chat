package delivery

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

func as(t *testing.T, userID uuid.UUID) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: userID, SessionID: uuid.New()})
}

func TestDeliveryEndpoints(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service)
	dm, ada, grace := f.dm(t)
	group := dm.MLSGroupID
	outsider := f.user(t, "mallory")
	commit := func(who uuid.UUID, body api.MlsCommit) api.SubmitMlsCommitResponseObject {
		response, err := handler.SubmitMlsCommit(as(t, who), api.SubmitMlsCommitRequestObject{ConversationId: dm.ID, Body: &body})
		require.NoError(t, err)
		return response
	}
	send := func(who uuid.UUID, body api.MlsSend) api.SendMlsMessageResponseObject {
		response, err := handler.SendMlsMessage(as(t, who), api.SendMlsMessageRequestObject{ConversationId: dm.ID, Body: &body})
		require.NoError(t, err)
		return response
	}

	state, err := handler.GetMlsGroupState(as(t, ada), api.GetMlsGroupStateRequestObject{ConversationId: dm.ID})
	require.NoError(t, err)
	assert.Equal(t, api.GetMlsGroupState200JSONResponse{GroupId: group, Epoch: 0}, state)
	hidden, err := handler.GetMlsGroupState(as(t, outsider), api.GetMlsGroupStateRequestObject{ConversationId: dm.ID})
	require.NoError(t, err)
	assert.IsType(t, api.GetMlsGroupState404JSONResponse{}, hidden)

	welcomeBytes := welcome
	assert.IsType(t, api.SubmitMlsCommit422JSONResponse{}, commit(ada, api.MlsCommit{Commit: []byte{1}, GroupInfo: groupInfo}))
	assert.IsType(t, api.SubmitMlsCommit403JSONResponse{}, commit(outsider, api.MlsCommit{Commit: privateMessage(group, 0, 3), GroupInfo: groupInfo}))
	assert.Equal(t, api.SubmitMlsCommit200JSONResponse{Epoch: 1}, commit(ada, api.MlsCommit{Commit: privateMessage(group, 0, 3), GroupInfo: groupInfo, Welcome: &welcomeBytes}))
	stale := commit(grace, api.MlsCommit{Commit: privateMessage(group, 0, 3), GroupInfo: groupInfo})
	require.IsType(t, api.SubmitMlsCommit409JSONResponse{}, stale)
	assert.Equal(t, "wrong_epoch", stale.(api.SubmitMlsCommit409JSONResponse).Code) //nolint:forcetypeassert // checked above

	withInfo, err := handler.GetMlsGroupState(as(t, ada), api.GetMlsGroupStateRequestObject{ConversationId: dm.ID})
	require.NoError(t, err)
	assert.Equal(t, groupInfo, *withInfo.(api.GetMlsGroupState200JSONResponse).GroupInfo) //nolint:forcetypeassert // the success type

	messageID := uuid.New()
	sent := send(grace, api.MlsSend{Message: privateMessage(group, 1, 1), Archive: &api.ArchiveCopy{MessageId: messageID, Sealed: []byte("sealed")}})
	assert.IsType(t, api.SendMlsMessage201JSONResponse{}, sent)
	repeated := send(grace, api.MlsSend{Message: privateMessage(group, 1, 1), Archive: &api.ArchiveCopy{MessageId: messageID, Sealed: []byte("x")}})
	require.IsType(t, api.SendMlsMessage409JSONResponse{}, repeated)
	assert.Equal(t, "duplicate_message", repeated.(api.SendMlsMessage409JSONResponse).Code) //nolint:forcetypeassert // checked above
	assert.IsType(t, api.SendMlsMessage409JSONResponse{}, send(grace, api.MlsSend{Message: privateMessage(group, 0, 1)}))
	assert.IsType(t, api.SendMlsMessage422JSONResponse{}, send(grace, api.MlsSend{Message: []byte{1}}))
	assert.IsType(t, api.SendMlsMessage403JSONResponse{}, send(outsider, api.MlsSend{Message: privateMessage(group, 1, 1)}))

	after := int64(1)
	limit := 50
	listed, err := handler.ListMlsMessages(as(t, ada), api.ListMlsMessagesRequestObject{ConversationId: dm.ID, Params: api.ListMlsMessagesParams{After: &after, Limit: &limit}})
	require.NoError(t, err)
	list := listed.(api.ListMlsMessages200JSONResponse) //nolint:forcetypeassert // the success type
	require.Len(t, list, 2)
	assert.Equal(t, api.MlsMessageKind("welcome"), list[0].Kind)
	archived, err := handler.ListArchivedMessages(as(t, ada), api.ListArchivedMessagesRequestObject{ConversationId: dm.ID})
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.Equal(t, messageID, archived.(api.ListArchivedMessages200JSONResponse)[0].MessageId) //nolint:forcetypeassert // the success type

	for _, response := range []any{
		must(handler.ListMlsMessages(as(t, outsider), api.ListMlsMessagesRequestObject{ConversationId: dm.ID})),
		must(handler.ListArchivedMessages(as(t, outsider), api.ListArchivedMessagesRequestObject{ConversationId: dm.ID})),
	} {
		assert.Contains(t, []string{"ListMlsMessages404JSONResponse", "ListArchivedMessages404JSONResponse"}, typeName(response))
	}
}

func TestCommitsCanConflict(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service)
	dm, ada, _ := f.dm(t)
	// The epoch moved between reading the state and storing: simulated by
	// committing through the store directly first.
	_, err := handler.GetMlsGroupState(as(t, ada), api.GetMlsGroupStateRequestObject{ConversationId: dm.ID})
	require.NoError(t, err)
	conflicting := conflictOnce{deliveryStore: f.service.dependencies.Store}
	f.service.dependencies.Store = &conflicting

	response, err := handler.SubmitMlsCommit(as(t, ada), api.SubmitMlsCommitRequestObject{ConversationId: dm.ID, Body: &api.MlsCommit{
		Commit: privateMessage(dm.MLSGroupID, 0, 3), GroupInfo: groupInfo,
	}})
	require.NoError(t, err)
	require.IsType(t, api.SubmitMlsCommit409JSONResponse{}, response)
	assert.Equal(t, "epoch_conflict", response.(api.SubmitMlsCommit409JSONResponse).Code) //nolint:forcetypeassert // checked above
}

func TestEndpointsNeedASession(t *testing.T) {
	handler := NewHandler(NewService(Dependencies{}))
	ctx := t.Context()
	id := uuid.New()
	for _, call := range []func() error{
		func() error {
			return second(handler.GetMlsGroupState(ctx, api.GetMlsGroupStateRequestObject{ConversationId: id}))
		},
		func() error {
			return second(handler.SubmitMlsCommit(ctx, api.SubmitMlsCommitRequestObject{ConversationId: id, Body: &api.MlsCommit{}}))
		},
		func() error {
			return second(handler.SendMlsMessage(ctx, api.SendMlsMessageRequestObject{ConversationId: id, Body: &api.MlsSend{}}))
		},
		func() error {
			return second(handler.ListMlsMessages(ctx, api.ListMlsMessagesRequestObject{ConversationId: id}))
		},
		func() error {
			return second(handler.ListArchivedMessages(ctx, api.ListArchivedMessagesRequestObject{ConversationId: id}))
		},
		func() error {
			return second(handler.SendChannelMessage(ctx, api.SendChannelMessageRequestObject{ChannelId: id, Body: &api.MlsSend{}}))
		},
		func() error {
			return second(handler.ListChannelArchive(ctx, api.ListChannelArchiveRequestObject{ChannelId: id}))
		},
	} {
		require.ErrorIs(t, call(), errNoPrincipal)
	}
}

func TestChannelEndpoints(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service)
	ada := f.user(t, "ada")
	audienceID, channelID, group := f.audience(t, ada, ada)
	f.grants.grant = ChannelGrant{AudienceID: audienceID, MaySend: true}
	send := func(body api.MlsSend) api.SendChannelMessageResponseObject {
		return must(handler.SendChannelMessage(as(t, ada), api.SendChannelMessageRequestObject{ChannelId: channelID, Body: &body}))
	}
	messageID := uuid.New()
	archive := &api.ArchiveCopy{MessageId: messageID, Sealed: []byte("sealed")}

	assert.IsType(t, api.SendChannelMessage201JSONResponse{}, send(api.MlsSend{Message: privateMessage(group, 0, 1), Archive: archive}))
	assert.IsType(t, api.SendChannelMessage409JSONResponse{}, send(api.MlsSend{Message: privateMessage(group, 0, 1), Archive: archive}))
	assert.IsType(t, api.SendChannelMessage409JSONResponse{}, send(api.MlsSend{Message: privateMessage(group, 4, 1)}))
	assert.IsType(t, api.SendChannelMessage422JSONResponse{}, send(api.MlsSend{Message: []byte{1}}))
	listed := must(handler.ListMlsMessages(as(t, ada), api.ListMlsMessagesRequestObject{ConversationId: audienceID}))
	assert.Equal(t, &channelID, listed.(api.ListMlsMessages200JSONResponse)[0].ChannelId) //nolint:forcetypeassert // the success type
	archived := must(handler.ListChannelArchive(as(t, ada), api.ListChannelArchiveRequestObject{ChannelId: channelID}))
	list := archived.(api.ListChannelArchive200JSONResponse) //nolint:forcetypeassert // the success type
	require.Len(t, list, 1)
	assert.Equal(t, group, list[0].MlsGroupId)

	f.grants.grant.MaySend = false
	assert.IsType(t, api.SendChannelMessage403JSONResponse{}, send(api.MlsSend{Message: privateMessage(group, 0, 1)}))
	f.grants.err = ErrNotFound
	assert.IsType(t, api.SendChannelMessage404JSONResponse{}, send(api.MlsSend{Message: privateMessage(group, 0, 1)}))
	assert.IsType(t, api.ListChannelArchive404JSONResponse{}, must(handler.ListChannelArchive(as(t, ada), api.ListChannelArchiveRequestObject{ChannelId: channelID})))
	f.grants.err = errors.New("database is down")
	require.Error(t, second(handler.SendChannelMessage(as(t, ada), api.SendChannelMessageRequestObject{ChannelId: channelID, Body: &api.MlsSend{}})))
	require.Error(t, second(handler.ListChannelArchive(as(t, ada), api.ListChannelArchiveRequestObject{ChannelId: channelID})))
}
