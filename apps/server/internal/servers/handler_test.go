package servers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

func as(t *testing.T, userID uuid.UUID) context.Context {
	t.Helper()
	return session.WithPrincipal(t.Context(), session.Principal{UserID: userID, SessionID: uuid.New()})
}

// calls runs every operation once with ctx, returning each one's error.
func calls(ctx context.Context, handler *Handler, serverID uuid.UUID) []error {
	id := uuid.New()
	spec := api.RoleSpec{Name: "x", Permissions: []api.PermissionName{}}
	errs := []error{}
	add := func(_ any, err error) { errs = append(errs, err) }
	add(handler.ListServers(ctx, api.ListServersRequestObject{}))
	add(handler.CreateServer(ctx, api.CreateServerRequestObject{Body: &api.NewServer{Name: "x"}}))
	add(handler.GetServer(ctx, api.GetServerRequestObject{ServerId: serverID}))
	add(handler.UpdateServer(ctx, api.UpdateServerRequestObject{ServerId: serverID, Body: &api.ServerSettings{Name: "x"}}))
	add(handler.DeleteServer(ctx, api.DeleteServerRequestObject{ServerId: serverID}))
	add(handler.LeaveServer(ctx, api.LeaveServerRequestObject{ServerId: serverID}))
	add(handler.ListServerMembers(ctx, api.ListServerMembersRequestObject{ServerId: serverID}))
	add(handler.GiveServerRole(ctx, api.GiveServerRoleRequestObject{ServerId: serverID, UserId: id, RoleId: id}))
	add(handler.TakeServerRole(ctx, api.TakeServerRoleRequestObject{ServerId: serverID, UserId: id, RoleId: id}))
	add(handler.CreateServerChannel(ctx, api.CreateServerChannelRequestObject{ServerId: serverID, Body: &api.NewServerChannel{Kind: "text", Name: "x"}}))
	add(handler.UpdateServerChannel(ctx, api.UpdateServerChannelRequestObject{ServerId: serverID, ChannelId: id, Body: &api.ChannelText{Name: "x"}}))
	add(handler.DeleteServerChannel(ctx, api.DeleteServerChannelRequestObject{ServerId: serverID, ChannelId: id}))
	add(handler.OrderServerChannels(ctx, api.OrderServerChannelsRequestObject{ServerId: serverID, Body: &[]api.ChannelPlacement{}}))
	override := api.OverrideSpec{Allow: []api.PermissionName{}, Deny: []api.PermissionName{}}
	add(handler.SetChannelOverride(ctx, api.SetChannelOverrideRequestObject{ServerId: serverID, ChannelId: id, TargetKind: "member", TargetId: id, Body: &override}))
	add(handler.ClearChannelOverride(ctx, api.ClearChannelOverrideRequestObject{ServerId: serverID, ChannelId: id, TargetKind: "member", TargetId: id}))
	add(handler.CreateServerRole(ctx, api.CreateServerRoleRequestObject{ServerId: serverID, Body: &spec}))
	add(handler.UpdateServerRole(ctx, api.UpdateServerRoleRequestObject{ServerId: serverID, RoleId: id, Body: &spec}))
	add(handler.DeleteServerRole(ctx, api.DeleteServerRoleRequestObject{ServerId: serverID, RoleId: id}))
	add(handler.OrderServerRoles(ctx, api.OrderServerRolesRequestObject{ServerId: serverID, Body: &api.RoleOrder{RoleIds: []uuid.UUID{}}}))
	add(handler.ListServerInvites(ctx, api.ListServerInvitesRequestObject{ServerId: serverID}))
	add(handler.CreateServerInvite(ctx, api.CreateServerInviteRequestObject{ServerId: serverID, Body: &api.NewServerInvite{}}))
	add(handler.RevokeServerInvite(ctx, api.RevokeServerInviteRequestObject{ServerId: serverID, Code: "x"}))
	add(handler.PreviewInvite(ctx, api.PreviewInviteRequestObject{Code: "x"}))
	add(handler.JoinServer(ctx, api.JoinServerRequestObject{Code: "x"}))
	return errs
}

func TestHandlersNeedAPrincipal(t *testing.T) {
	errs := calls(t.Context(), NewHandler(newFixture(t).service), uuid.New())
	preview := len(errs) - 2 // previewing an invite does not need to know who asks
	for i, err := range errs {
		if i == preview {
			require.NoError(t, err)
			continue
		}
		require.ErrorIs(t, err, errNoPrincipal, "call %d", i)
	}
}

func TestHandlersReportDatabaseFailures(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	f.pool.Close()
	for i, err := range calls(as(t, ada), NewHandler(f.service), details.Server.ID) {
		assert.Error(t, err, "call %d", i)
	}
}

func TestHandlersRefuseUnknownPermissions(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	handler, ctx, id := NewHandler(f.service), as(t, ada), details.Server.ID
	bad := []api.PermissionName{"FLY"}

	created, err := handler.CreateServerRole(ctx, api.CreateServerRoleRequestObject{ServerId: id, Body: &api.RoleSpec{Name: "x", Permissions: bad}})
	require.NoError(t, err)
	assert.Equal(t, "unknown_permission", created.(api.CreateServerRole422JSONResponse).Code)
	updated, err := handler.UpdateServerRole(ctx, api.UpdateServerRoleRequestObject{ServerId: id, RoleId: details.Roles[0].ID, Body: &api.RoleSpec{Name: "x", Permissions: bad}})
	require.NoError(t, err)
	assert.IsType(t, api.UpdateServerRole422JSONResponse{}, updated)
	channel := details.Channels[0].ID
	for _, spec := range []api.OverrideSpec{{Allow: bad, Deny: []api.PermissionName{}}, {Allow: []api.PermissionName{}, Deny: bad}} {
		set, err := handler.SetChannelOverride(ctx, api.SetChannelOverrideRequestObject{ServerId: id, ChannelId: channel, TargetKind: "member", TargetId: ada, Body: &spec})
		require.NoError(t, err)
		assert.IsType(t, api.SetChannelOverride422JSONResponse{}, set)
	}
	// An absurd lifetime is clamped, then refused, rather than overflowing.
	huge := int64(1) << 62
	invite, err := handler.CreateServerInvite(ctx, api.CreateServerInviteRequestObject{ServerId: id, Body: &api.NewServerInvite{ExpiresInSeconds: &huge}})
	require.NoError(t, err)
	assert.IsType(t, api.CreateServerInvite422JSONResponse{}, invite)
}

func TestHandlersAnswerTheContract(t *testing.T) {
	f := newFixture(t)
	ada, sam := f.user(t, "ada"), f.user(t, "sam")
	details := f.server(t, ada)
	handler := NewHandler(f.service)
	for i, err := range calls(as(t, sam), handler, details.Server.ID) {
		require.NoError(t, err, "call %d: refusals are answers, not errors", i)
	}
	description := "Read."
	topic := "Books"
	ctx := as(t, ada)
	created, err := handler.CreateServer(ctx, api.CreateServerRequestObject{Body: &api.NewServer{Name: "Club", Description: &description}})
	require.NoError(t, err)
	server := created.(api.CreateServer201JSONResponse)
	assert.Equal(t, "Read.", server.Server.Description)
	assert.Equal(t, []uuid.UUID{}, server.MyRoleIds)
	channel, err := handler.CreateServerChannel(ctx, api.CreateServerChannelRequestObject{ServerId: server.Server.Id, Body: &api.NewServerChannel{Kind: "voice", Name: "Room", Topic: &topic}})
	require.NoError(t, err)
	assert.IsType(t, api.CreateServerChannel201JSONResponse{}, channel)
	_, err = handler.UpdateServerChannel(ctx, api.UpdateServerChannelRequestObject{ServerId: server.Server.Id, ChannelId: server.Channels[0].Id, Body: &api.ChannelText{Name: "x", Topic: &topic}})
	require.NoError(t, err)
	_, err = handler.CreateServerInvite(ctx, api.CreateServerInviteRequestObject{ServerId: server.Server.Id, Body: &api.NewServerInvite{}})
	require.NoError(t, err)
	invites, err := handler.ListServerInvites(ctx, api.ListServerInvitesRequestObject{ServerId: server.Server.Id})
	require.NoError(t, err)
	assert.Len(t, invites.(api.ListServerInvites200JSONResponse), 1)
	members, err := handler.ListServerMembers(ctx, api.ListServerMembersRequestObject{ServerId: server.Server.Id})
	require.NoError(t, err)
	listed := members.(api.ListServerMembers200JSONResponse)
	require.Len(t, listed, 1)
	assert.Equal(t, "ada", listed[0].User.Username)
	assert.Equal(t, []uuid.UUID{}, listed[0].RoleIds)
	bad, err := handler.CreateServer(ctx, api.CreateServerRequestObject{Body: &api.NewServer{Name: ""}})
	require.NoError(t, err)
	assert.IsType(t, api.CreateServer422JSONResponse{}, bad)
}
