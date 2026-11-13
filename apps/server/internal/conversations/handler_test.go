package conversations

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/social"
)

func as(t *testing.T, userID uuid.UUID) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: userID, SessionID: uuid.New()})
}

func TestDirectMessageEndpoints(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service)
	ada, grace, shy := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "shy")
	require.NoError(t, f.social.SetPrivacy(t.Context(), shy, social.Privacy{WhoCanDirectMessage: social.DirectMessagesFromFriends, WhoCanFriendRequest: social.FriendRequestsFromEveryone}))
	open := func(other uuid.UUID) api.OpenDirectMessageResponseObject {
		response, err := handler.OpenDirectMessage(as(t, ada), api.OpenDirectMessageRequestObject{Body: &api.OpenDirectMessageJSONRequestBody{UserId: other}})
		require.NoError(t, err)
		return response
	}

	created := open(grace)
	require.IsType(t, api.OpenDirectMessage201JSONResponse{}, created)
	dm := created.(api.OpenDirectMessage201JSONResponse) //nolint:forcetypeassert // checked above
	assert.Equal(t, api.ConversationKind("dm"), dm.Kind)
	assert.Len(t, dm.Members, 2)
	assert.IsType(t, api.OpenDirectMessage200JSONResponse{}, open(grace))
	assert.IsType(t, api.OpenDirectMessage403JSONResponse{}, open(shy))
	assert.IsType(t, api.OpenDirectMessage404JSONResponse{}, open(uuid.New()))
	assert.IsType(t, api.OpenDirectMessage422JSONResponse{}, open(ada))

	listed, err := handler.ListConversations(as(t, grace), api.ListConversationsRequestObject{})
	require.NoError(t, err)
	assert.Len(t, listed, 1)
	got, err := handler.GetConversation(as(t, grace), api.GetConversationRequestObject{ConversationId: dm.Id})
	require.NoError(t, err)
	assert.IsType(t, api.GetConversation200JSONResponse{}, got)
	hidden, err := handler.GetConversation(as(t, shy), api.GetConversationRequestObject{ConversationId: dm.Id})
	require.NoError(t, err)
	assert.IsType(t, api.GetConversation404JSONResponse{}, hidden)
	left, err := handler.LeaveConversation(as(t, ada), api.LeaveConversationRequestObject{ConversationId: dm.Id})
	require.NoError(t, err)
	assert.IsType(t, api.LeaveConversation422JSONResponse{}, left)
}

func TestGroupEndpoints(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service)
	ada, grace, alan, linus, shy := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "alan"), f.user(t, "linus"), f.user(t, "shy")
	require.NoError(t, f.social.SetPrivacy(t.Context(), shy, social.Privacy{WhoCanDirectMessage: social.DirectMessagesFromFriends, WhoCanFriendRequest: social.FriendRequestsFromEveryone}))
	create := func(members ...uuid.UUID) api.CreateGroupConversationResponseObject {
		response, err := handler.CreateGroupConversation(as(t, ada), api.CreateGroupConversationRequestObject{Body: &api.NewGroupConversation{MemberIds: members}})
		require.NoError(t, err)
		return response
	}
	assert.IsType(t, api.CreateGroupConversation422JSONResponse{}, create(grace))
	assert.IsType(t, api.CreateGroupConversation403JSONResponse{}, create(grace, shy))
	assert.IsType(t, api.CreateGroupConversation404JSONResponse{}, create(grace, uuid.New()))
	created := create(grace, alan)
	require.IsType(t, api.CreateGroupConversation201JSONResponse{}, created)
	group := created.(api.CreateGroupConversation201JSONResponse).Id //nolint:forcetypeassert // checked above

	add := func(who uuid.UUID, conversation uuid.UUID) api.AddConversationMemberResponseObject {
		response, err := handler.AddConversationMember(as(t, ada), api.AddConversationMemberRequestObject{ConversationId: conversation, Body: &api.DirectMessageTarget{UserId: who}})
		require.NoError(t, err)
		return response
	}
	assert.IsType(t, api.AddConversationMember204Response{}, add(linus, group))
	assert.IsType(t, api.AddConversationMember409JSONResponse{}, add(linus, group))
	assert.IsType(t, api.AddConversationMember403JSONResponse{}, add(shy, group))
	assert.IsType(t, api.AddConversationMember404JSONResponse{}, add(uuid.New(), group))
	assert.IsType(t, api.AddConversationMember404JSONResponse{}, add(linus, uuid.New()))
	dm, _, err := f.service.OpenDirectMessage(t.Context(), ada, alan)
	require.NoError(t, err)
	assert.IsType(t, api.AddConversationMember422JSONResponse{}, add(linus, dm.ID))

	name := "Compilers"
	rename := func(conversation uuid.UUID, value *string) api.RenameConversationResponseObject {
		response, err := handler.RenameConversation(as(t, ada), api.RenameConversationRequestObject{ConversationId: conversation, Body: &api.ConversationRename{Name: value}})
		require.NoError(t, err)
		return response
	}
	assert.IsType(t, api.RenameConversation204Response{}, rename(group, &name))
	assert.IsType(t, api.RenameConversation422JSONResponse{}, rename(dm.ID, &name))
	assert.IsType(t, api.RenameConversation404JSONResponse{}, rename(uuid.New(), &name))

	left, err := handler.LeaveConversation(as(t, alan), api.LeaveConversationRequestObject{ConversationId: group})
	require.NoError(t, err)
	assert.IsType(t, api.LeaveConversation204Response{}, left)
	again, err := handler.LeaveConversation(as(t, alan), api.LeaveConversationRequestObject{ConversationId: group})
	require.NoError(t, err)
	assert.IsType(t, api.LeaveConversation404JSONResponse{}, again)
}

func TestEndpointsNeedASession(t *testing.T) {
	handler := NewHandler(NewService(Dependencies{}))
	ctx := t.Context()
	id := uuid.New()
	calls := []func() error{
		func() error {
			_, err := handler.ListConversations(ctx, api.ListConversationsRequestObject{})
			return err
		},
		func() error {
			_, err := handler.GetConversation(ctx, api.GetConversationRequestObject{ConversationId: id})
			return err
		},
		func() error {
			_, err := handler.OpenDirectMessage(ctx, api.OpenDirectMessageRequestObject{Body: &api.OpenDirectMessageJSONRequestBody{}})
			return err
		},
		func() error {
			_, err := handler.CreateGroupConversation(ctx, api.CreateGroupConversationRequestObject{Body: &api.NewGroupConversation{}})
			return err
		},
		func() error {
			_, err := handler.RenameConversation(ctx, api.RenameConversationRequestObject{ConversationId: id, Body: &api.ConversationRename{}})
			return err
		},
		func() error {
			_, err := handler.AddConversationMember(ctx, api.AddConversationMemberRequestObject{ConversationId: id, Body: &api.DirectMessageTarget{}})
			return err
		},
		func() error {
			_, err := handler.LeaveConversation(ctx, api.LeaveConversationRequestObject{ConversationId: id})
			return err
		},
	}
	for _, call := range calls {
		require.ErrorIs(t, call(), errNoPrincipal)
	}
}
