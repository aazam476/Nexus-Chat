package servers

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
)

func TestModerationHandlersAnswerTheContract(t *testing.T) {
	m := newModerated(t, permissions.KickMembers|permissions.BanMembers|permissions.TimeoutMembers)
	handler := NewHandler(m.f.service)
	server, mod, plain := m.details.Server.ID, as(t, m.grace), as(t, m.kim)
	reason, long, week := "spam", strings.Repeat("x", 600), 7*24*3600

	timeout, err := handler.TimeoutServerMember(mod, api.TimeoutServerMemberRequestObject{ServerId: server, UserId: m.sam, Body: &api.TimeoutSpec{Seconds: 3600}})
	require.NoError(t, err)
	require.IsType(t, api.TimeoutServerMember204Response{}, timeout)
	members, err := handler.ListServerMembers(mod, api.ListServerMembersRequestObject{ServerId: server})
	require.NoError(t, err)
	timedOut := 0
	for _, member := range members.(api.ListServerMembers200JSONResponse) { //nolint:forcetypeassert // checked by the test
		if member.TimedOutUntil != nil {
			timedOut++
		}
	}
	assert.Equal(t, 1, timedOut, "the member list shows who is timed out")
	timeout, err = handler.TimeoutServerMember(mod, api.TimeoutServerMemberRequestObject{ServerId: server, UserId: m.sam, Body: &api.TimeoutSpec{}})
	require.NoError(t, err)
	require.IsType(t, api.TimeoutServerMember422JSONResponse{}, timeout, "zero seconds is not a timeout")
	timeout, err = handler.TimeoutServerMember(plain, api.TimeoutServerMemberRequestObject{ServerId: server, UserId: m.sam, Body: &api.TimeoutSpec{Seconds: 60}})
	require.NoError(t, err)
	require.IsType(t, api.TimeoutServerMember403JSONResponse{}, timeout)
	ended, err := handler.EndServerMemberTimeout(mod, api.EndServerMemberTimeoutRequestObject{ServerId: server, UserId: m.sam})
	require.NoError(t, err)
	require.IsType(t, api.EndServerMemberTimeout204Response{}, ended)
	ended, err = handler.EndServerMemberTimeout(mod, api.EndServerMemberTimeoutRequestObject{ServerId: server, UserId: uuid.New()})
	require.NoError(t, err)
	require.IsType(t, api.EndServerMemberTimeout404JSONResponse{}, ended)

	kicked, err := handler.KickServerMember(mod, api.KickServerMemberRequestObject{ServerId: server, UserId: m.sam, Body: &api.Sanction{Reason: &long}})
	require.NoError(t, err)
	require.IsType(t, api.KickServerMember422JSONResponse{}, kicked)
	kicked, err = handler.KickServerMember(mod, api.KickServerMemberRequestObject{ServerId: server, UserId: m.sam, Body: &api.Sanction{Reason: &reason}})
	require.NoError(t, err)
	require.IsType(t, api.KickServerMember204Response{}, kicked)

	banned, err := handler.BanServerMember(mod, api.BanServerMemberRequestObject{ServerId: server, UserId: m.kim, Body: &api.BanSpec{Reason: &reason, DeleteMessageSeconds: &week}})
	require.NoError(t, err)
	require.IsType(t, api.BanServerMember204Response{}, banned)
	banned, err = handler.BanServerMember(mod, api.BanServerMemberRequestObject{ServerId: server, UserId: m.ada, Body: &api.BanSpec{}})
	require.NoError(t, err)
	require.IsType(t, api.BanServerMember403JSONResponse{}, banned)
	bans, err := handler.ListServerBans(mod, api.ListServerBansRequestObject{ServerId: server})
	require.NoError(t, err)
	list := bans.(api.ListServerBans200JSONResponse) //nolint:forcetypeassert // checked by the test
	require.Len(t, list, 1)
	assert.Equal(t, "kim", list[0].User.Username)
	assert.Equal(t, reason, list[0].Reason)
	bans, err = handler.ListServerBans(as(t, m.sam), api.ListServerBansRequestObject{ServerId: server})
	require.NoError(t, err)
	require.IsType(t, api.ListServerBans404JSONResponse{}, bans, "kicked, so no longer in the server")

	unbanned, err := handler.UnbanServerMember(mod, api.UnbanServerMemberRequestObject{ServerId: server, UserId: m.kim})
	require.NoError(t, err)
	require.IsType(t, api.UnbanServerMember204Response{}, unbanned)
	unbanned, err = handler.UnbanServerMember(mod, api.UnbanServerMemberRequestObject{ServerId: server, UserId: m.kim})
	require.NoError(t, err)
	require.IsType(t, api.UnbanServerMember404JSONResponse{}, unbanned, "not banned")
}

func TestBannedUsersCannotJoin(t *testing.T) {
	m := newModerated(t, permissions.BanMembers)
	handler := NewHandler(m.f.service)
	require.NoError(t, m.f.service.Ban(t.Context(), m.mod, BanRequest{UserID: m.sam}))
	invite, err := m.f.service.CreateInvite(t.Context(), m.owner, InviteLimits{})
	require.NoError(t, err)
	response, err := handler.JoinServer(as(t, m.sam), api.JoinServerRequestObject{Code: invite.Code})
	require.NoError(t, err)
	refused, ok := response.(api.JoinServer403JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "banned", refused.Code)
}
