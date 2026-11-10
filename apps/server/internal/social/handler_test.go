package social

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

func as(t *testing.T, userID uuid.UUID) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: userID, SessionID: uuid.New()})
}

func codeOf(t *testing.T, response any) string {
	t.Helper()
	switch typed := response.(type) {
	case api.SendFriendRequest403JSONResponse:
		return typed.Code
	case api.SendFriendRequest404JSONResponse:
		return typed.Code
	case api.SendFriendRequest409JSONResponse:
		return typed.Code
	case api.SendFriendRequest422JSONResponse:
		return typed.Code
	case api.SendFriendRequest429JSONResponse:
		return typed.Code
	}
	t.Fatalf("unexpected response %T", response)
	return ""
}

func TestFriendEndpoints(t *testing.T) {
	f := newFixture(t, NoServers{})
	handler := NewHandler(f.service)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	send := func(from uuid.UUID, username string) api.SendFriendRequestResponseObject {
		response, err := handler.SendFriendRequest(as(t, from), api.SendFriendRequestRequestObject{Body: &api.SendFriendRequestJSONRequestBody{Username: username}})
		require.NoError(t, err)
		return response
	}

	assert.Equal(t, api.SendFriendRequest200JSONResponse{UserId: grace, Relationship: api.RelationshipOutgoing}, send(ada, "grace"))
	assert.Equal(t, "already_requested", codeOf(t, send(ada, "grace")))
	assert.Equal(t, "user_not_found", codeOf(t, send(ada, "nobody")))
	assert.Equal(t, "cannot_befriend_self", codeOf(t, send(ada, "ada")))

	listed, err := handler.ListFriends(as(t, grace), api.ListFriendsRequestObject{})
	require.NoError(t, err)
	incoming := listed.(api.ListFriends200JSONResponse).Incoming //nolint:forcetypeassert // the only success type
	require.Len(t, incoming, 1)
	assert.Equal(t, api.UserSummary{Id: ada, Username: "ada", DisplayName: "ada"}, incoming[0].User)

	accepted, err := handler.AcceptFriendRequest(as(t, grace), api.AcceptFriendRequestRequestObject{UserId: ada})
	require.NoError(t, err)
	assert.Equal(t, api.AcceptFriendRequest204Response{}, accepted)
	again, err := handler.AcceptFriendRequest(as(t, grace), api.AcceptFriendRequestRequestObject{UserId: ada})
	require.NoError(t, err)
	assert.IsType(t, api.AcceptFriendRequest404JSONResponse{}, again)
	assert.Equal(t, "already_friends", codeOf(t, send(ada, "grace")))

	removed, err := handler.RemoveFriend(as(t, ada), api.RemoveFriendRequestObject{UserId: grace})
	require.NoError(t, err)
	assert.Equal(t, api.RemoveFriend204Response{}, removed)
	gone, err := handler.RemoveFriend(as(t, ada), api.RemoveFriendRequestObject{UserId: grace})
	require.NoError(t, err)
	assert.IsType(t, api.RemoveFriend404JSONResponse{}, gone)

	send(ada, "grace")
	declined, err := handler.DeleteFriendRequest(as(t, grace), api.DeleteFriendRequestRequestObject{UserId: ada})
	require.NoError(t, err)
	assert.Equal(t, api.DeleteFriendRequest204Response{}, declined)
	none, err := handler.DeleteFriendRequest(as(t, grace), api.DeleteFriendRequestRequestObject{UserId: ada})
	require.NoError(t, err)
	assert.IsType(t, api.DeleteFriendRequest404JSONResponse{}, none)

	_, err = handler.BlockUser(as(t, grace), api.BlockUserRequestObject{UserId: ada})
	require.NoError(t, err)
	assert.Equal(t, "friend_request_not_allowed", codeOf(t, send(ada, "grace")))
	f.service.dependencies.Limiter = limiter{deny: true}
	assert.Equal(t, "rate_limited", codeOf(t, send(ada, "grace")))
}

func TestBlockAndPrivacyEndpoints(t *testing.T) {
	f := newFixture(t, NoServers{})
	handler := NewHandler(f.service)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")

	blocked, err := handler.BlockUser(as(t, ada), api.BlockUserRequestObject{UserId: grace})
	require.NoError(t, err)
	assert.Equal(t, api.BlockUser204Response{}, blocked)
	self, err := handler.BlockUser(as(t, ada), api.BlockUserRequestObject{UserId: ada})
	require.NoError(t, err)
	assert.IsType(t, api.BlockUser422JSONResponse{}, self)
	unknown, err := handler.BlockUser(as(t, ada), api.BlockUserRequestObject{UserId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.BlockUser404JSONResponse{}, unknown)
	list, err := handler.ListBlocked(as(t, ada), api.ListBlockedRequestObject{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	unblocked, err := handler.UnblockUser(as(t, ada), api.UnblockUserRequestObject{UserId: grace})
	require.NoError(t, err)
	assert.Equal(t, api.UnblockUser204Response{}, unblocked)
	notBlocked, err := handler.UnblockUser(as(t, ada), api.UnblockUserRequestObject{UserId: grace})
	require.NoError(t, err)
	assert.IsType(t, api.UnblockUser404JSONResponse{}, notBlocked)

	settings := api.PrivacySettings{WhoCanDirectMessage: api.PrivacySettingsWhoCanDirectMessageEveryone, WhoCanFriendRequest: api.PrivacySettingsWhoCanFriendRequestNobody}
	saved, err := handler.UpdatePrivacySettings(as(t, ada), api.UpdatePrivacySettingsRequestObject{Body: &settings})
	require.NoError(t, err)
	assert.Equal(t, api.UpdatePrivacySettings204Response{}, saved)
	read, err := handler.GetPrivacySettings(as(t, ada), api.GetPrivacySettingsRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.GetPrivacySettings200JSONResponse(settings), read)
	invalid, err := handler.UpdatePrivacySettings(as(t, ada), api.UpdatePrivacySettingsRequestObject{Body: &api.PrivacySettings{WhoCanDirectMessage: "anyone", WhoCanFriendRequest: "nobody"}})
	require.NoError(t, err)
	assert.IsType(t, api.UpdatePrivacySettings400JSONResponse{}, invalid)
}

func TestAvatarsAppearInLists(t *testing.T) {
	avatar := uuid.New()
	listed := contacts([]Contact{{UserID: uuid.New(), Username: "ada", DisplayName: "Ada", AvatarID: &avatar, Since: now}})

	require.NotNil(t, listed[0].User.AvatarUrl)
	assert.Equal(t, "/api/v1/avatars/"+avatar.String(), *listed[0].User.AvatarUrl)
}

func TestEndpointsNeedASession(t *testing.T) {
	handler := NewHandler(NewService(Dependencies{}))
	ctx := t.Context()
	id := uuid.New()
	checks := []func() error{
		func() error { _, err := handler.ListFriends(ctx, api.ListFriendsRequestObject{}); return err },
		func() error {
			_, err := handler.SendFriendRequest(ctx, api.SendFriendRequestRequestObject{Body: &api.SendFriendRequestJSONRequestBody{}})
			return err
		},
		func() error {
			_, err := handler.AcceptFriendRequest(ctx, api.AcceptFriendRequestRequestObject{UserId: id})
			return err
		},
		func() error {
			_, err := handler.DeleteFriendRequest(ctx, api.DeleteFriendRequestRequestObject{UserId: id})
			return err
		},
		func() error {
			_, err := handler.RemoveFriend(ctx, api.RemoveFriendRequestObject{UserId: id})
			return err
		},
		func() error { _, err := handler.ListBlocked(ctx, api.ListBlockedRequestObject{}); return err },
		func() error { _, err := handler.BlockUser(ctx, api.BlockUserRequestObject{UserId: id}); return err },
		func() error { _, err := handler.UnblockUser(ctx, api.UnblockUserRequestObject{UserId: id}); return err },
		func() error {
			_, err := handler.GetPrivacySettings(ctx, api.GetPrivacySettingsRequestObject{})
			return err
		},
		func() error {
			_, err := handler.UpdatePrivacySettings(ctx, api.UpdatePrivacySettingsRequestObject{Body: &api.PrivacySettings{}})
			return err
		},
	}
	for _, check := range checks {
		require.ErrorIs(t, check(), errNoPrincipal)
	}
}

func TestStoreFailuresAreReported(t *testing.T) {
	f := newFixture(t, NoServers{})
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	handler := func(failOn string) *Handler {
		return NewHandler(NewService(Dependencies{
			Store: failing{Queries: f.queries, failOn: failOn}, Notifier: f.notices, Servers: NoServers{}, Limiter: limiter{}, Clock: clock.Fixed{Time: now},
		}))
	}
	send := func(h *Handler) error {
		_, err := h.SendFriendRequest(as(t, ada), api.SendFriendRequestRequestObject{Body: &api.SendFriendRequestJSONRequestBody{Username: "grace"}})
		return err
	}
	for _, call := range []string{"FindUserByUsername", "Relationship", "GetPrivacy", "InsertFriendRequest"} {
		require.ErrorIs(t, send(handler(call)), errDown, call)
	}
	calls := map[string]func(h *Handler) error{
		"AcceptFriendRequest": func(h *Handler) error {
			_, err := h.AcceptFriendRequest(as(t, ada), api.AcceptFriendRequestRequestObject{UserId: grace})
			return err
		},
		"DeleteFriendRequest": func(h *Handler) error {
			_, err := h.DeleteFriendRequest(as(t, ada), api.DeleteFriendRequestRequestObject{UserId: grace})
			return err
		},
		"DeleteFriendship": func(h *Handler) error {
			_, err := h.RemoveFriend(as(t, ada), api.RemoveFriendRequestObject{UserId: grace})
			return err
		},
		"UserExists": func(h *Handler) error {
			_, err := h.BlockUser(as(t, ada), api.BlockUserRequestObject{UserId: grace})
			return err
		},
		"Block": func(h *Handler) error {
			_, err := h.BlockUser(as(t, ada), api.BlockUserRequestObject{UserId: grace})
			return err
		},
		"Unblock": func(h *Handler) error {
			_, err := h.UnblockUser(as(t, ada), api.UnblockUserRequestObject{UserId: grace})
			return err
		},
		"ListBlocked": func(h *Handler) error {
			_, err := h.ListBlocked(as(t, ada), api.ListBlockedRequestObject{})
			return err
		},
		"ListFriends": func(h *Handler) error {
			_, err := h.ListFriends(as(t, ada), api.ListFriendsRequestObject{})
			return err
		},
		"ListIncomingRequests": func(h *Handler) error {
			_, err := h.ListFriends(as(t, ada), api.ListFriendsRequestObject{})
			return err
		},
		"ListOutgoingRequests": func(h *Handler) error {
			_, err := h.ListFriends(as(t, ada), api.ListFriendsRequestObject{})
			return err
		},
		"GetPrivacy": func(h *Handler) error {
			_, err := h.GetPrivacySettings(as(t, ada), api.GetPrivacySettingsRequestObject{})
			return err
		},
		"PutPrivacy": func(h *Handler) error {
			_, err := h.UpdatePrivacySettings(as(t, ada), api.UpdatePrivacySettingsRequestObject{Body: &api.PrivacySettings{WhoCanDirectMessage: "everyone", WhoCanFriendRequest: "nobody"}})
			return err
		},
	}
	for call, run := range calls {
		require.ErrorIs(t, run(handler(call)), errDown, call)
	}

	// Asking whether a DM is allowed fails the same way.
	for _, call := range []string{"Relationship", "GetPrivacy"} {
		service := NewService(Dependencies{Store: failing{Queries: f.queries, failOn: call}, Servers: NoServers{}, Clock: clock.Fixed{Time: now}})
		_, err := service.CanDirectMessage(t.Context(), ada, grace)
		require.ErrorIs(t, err, errDown, call)
	}
	servers := NewService(Dependencies{Store: f.queries, Servers: failingServers{}, Limiter: limiter{}, Notifier: f.notices, Clock: clock.Fixed{Time: now}})
	require.NoError(t, NewService(Dependencies{Store: f.queries, Notifier: f.notices, Clock: clock.Fixed{Time: now}}).SetPrivacy(t.Context(), grace, Privacy{WhoCanDirectMessage: "friends", WhoCanFriendRequest: "server_members"}))
	_, _, err := servers.SendRequest(t.Context(), ada, "grace")
	require.ErrorIs(t, err, errDown)
}
