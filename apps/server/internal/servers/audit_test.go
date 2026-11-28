package servers

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
)

// actions lists the entries' actions, newest first.
func actions(entries []AuditEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Action)
	}
	return out
}

func TestConfigurationChangesAreAudited(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner, member := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}
	ctx := t.Context()

	channel, err := f.service.CreateChannel(ctx, owner, NewChannel{Kind: KindText, Name: "books"})
	require.NoError(t, err)
	require.NoError(t, f.service.UpdateChannel(ctx, owner, channel, ChannelText{Name: "reading", Topic: "What now"}))
	require.NoError(t, f.service.SetOverride(ctx, owner, channel, Override{TargetKind: TargetMember, TargetID: grace, Deny: permissions.SendMessages}))
	require.NoError(t, f.service.ClearOverride(ctx, owner, channel, Target{Kind: TargetMember, ID: grace}))
	role, err := f.service.CreateRole(ctx, owner, RoleSpec{Name: "Auditors", Permissions: permissions.ViewAuditLog})
	require.NoError(t, err)
	require.NoError(t, f.service.UpdateRole(ctx, owner, role, RoleSpec{Name: "Auditors", Permissions: permissions.ViewAuditLog}))
	require.NoError(t, f.service.SetMemberRole(ctx, owner, RoleChange{UserID: grace, RoleID: role, Give: true}))
	require.NoError(t, f.service.Update(ctx, owner, Settings{Name: "Book club", Description: "Still reading"}))
	require.NoError(t, f.service.DeleteChannel(ctx, owner, channel))

	entries, err := f.service.AuditLog(ctx, member, AuditFilter{})
	require.NoError(t, err, "VIEW_AUDIT_LOG reads the log")
	assert.Equal(t, []string{
		ActionChannelDelete, ActionServerUpdate, ActionMemberRoleGive, ActionRoleUpdate, ActionRoleCreate,
		ActionOverrideClear, ActionOverrideSet, ActionChannelUpdate, ActionChannelCreate, ActionInviteCreate,
	}, actions(entries), "the fixture's invite first")
	newest := entries[0]
	assert.Equal(t, &ada, newest.ActorID)
	assert.Equal(t, TargetKindChannel, newest.TargetKind)
	assert.Equal(t, channel.String(), newest.TargetID)
	assert.Equal(t, map[string]any{"name": "reading", "kind": "text"}, newest.Details)
	assert.True(t, start.Equal(newest.CreatedAt))
	assert.Equal(t, []any{"SEND_MESSAGES"}, entries[6].Details["deny"])

	require.NoError(t, f.service.SetMemberRole(ctx, owner, RoleChange{UserID: grace, RoleID: role}))
	_, err = f.service.AuditLog(ctx, member, AuditFilter{})
	require.ErrorIs(t, err, ErrForbidden, "without it, the log is closed")
	_, err = f.service.AuditLog(ctx, Scope{uuid.New(), ada}, AuditFilter{})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestTheAuditLogIsFilteredAndPaged(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner := Scope{details.Server.ID, ada}
	ctx := t.Context()
	role, err := f.service.CreateRole(ctx, owner, RoleSpec{Name: "Mods", Permissions: permissions.ManageChannels})
	require.NoError(t, err)
	require.NoError(t, f.service.SetMemberRole(ctx, owner, RoleChange{UserID: grace, RoleID: role, Give: true}))
	mod := Scope{details.Server.ID, grace}
	general := channelNamed(t, details, "general")
	require.NoError(t, f.service.UpdateChannel(ctx, mod, general.ID, ChannelText{Name: "chat"}))
	require.NoError(t, f.service.OrderChannels(ctx, mod, orderOf(details.Channels)))

	byGrace, err := f.service.AuditLog(ctx, owner, AuditFilter{ActorID: &grace})
	require.NoError(t, err)
	assert.Equal(t, []string{ActionChannelOrder, ActionChannelUpdate}, actions(byGrace))
	assert.Empty(t, byGrace[0].TargetKind, "ordering has no single target")

	roles, err := f.service.AuditLog(ctx, owner, AuditFilter{Action: "role"})
	require.NoError(t, err)
	assert.Equal(t, []string{ActionRoleCreate}, actions(roles), "a kind matches its actions, not member.role_give")
	exact, err := f.service.AuditLog(ctx, owner, AuditFilter{Action: ActionMemberRoleGive})
	require.NoError(t, err)
	assert.Len(t, exact, 1)
	aboutGeneral, err := f.service.AuditLog(ctx, owner, AuditFilter{TargetID: general.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, []string{ActionChannelUpdate}, actions(aboutGeneral))

	first, err := f.service.AuditLog(ctx, owner, AuditFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, first, 2)
	rest, err := f.service.AuditLog(ctx, owner, AuditFilter{Before: first[1].ID, Limit: 1000})
	require.NoError(t, err)
	assert.Equal(t, []string{ActionMemberRoleGive, ActionRoleCreate, ActionInviteCreate}, actions(rest))
}

func orderOf(channels []Channel) []Placement {
	order := make([]Placement, 0, len(channels))
	for _, channel := range channels {
		order = append(order, Placement{ID: channel.ID, ParentID: channel.ParentID})
	}
	return order
}

func TestInvitesAndEmojiAreAuditedWithoutTheirCodes(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	owner := Scope{details.Server.ID, ada}
	ctx := t.Context()
	invite, err := f.service.CreateInvite(ctx, owner, InviteLimits{})
	require.NoError(t, err)
	require.NoError(t, f.service.RevokeInvite(ctx, owner, invite.Code))
	wave, err := f.service.AddEmoji(ctx, owner, "wave", bigPNG(t))
	require.NoError(t, err)
	require.NoError(t, f.service.RemoveEmoji(ctx, owner, wave))
	require.NoError(t, f.service.SetIcon(ctx, owner, bigPNG(t)))
	require.NoError(t, f.service.RemoveIcon(ctx, owner))

	entries, err := f.service.AuditLog(ctx, owner, AuditFilter{})
	require.NoError(t, err)
	// The fixture's own invite comes first.
	assert.Equal(t, []string{ActionServerIcon, ActionServerIcon, ActionEmojiRemove, ActionEmojiAdd, ActionInviteRevoke, ActionInviteCreate, ActionInviteCreate}, actions(entries))
	assert.Equal(t, "wave", entries[2].Details["name"])
	assert.Equal(t, invite.Code[:4]+"…", entries[4].TargetID)
	assert.NotContains(t, entries[5].TargetID, invite.Code)
	assert.Equal(t, "ab", inviteHint("ab"))
}

func TestOldAuditEntriesAreForgotten(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	owner := Scope{details.Server.ID, ada}
	f.clock.advance(AuditRetention - time.Hour)
	require.NoError(t, f.service.Update(t.Context(), owner, Settings{Name: "Later"}))
	f.clock.advance(2 * time.Hour)

	pruned, err := f.service.PruneAuditLog(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(1), pruned, "the invite made 90 days ago")
	entries, err := f.service.AuditLog(t.Context(), owner, AuditFilter{})
	require.NoError(t, err)
	assert.Equal(t, []string{ActionServerUpdate}, actions(entries))
}

func TestThePrunerRunsUntilStopped(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	AuditPruner{Service: f.service, Logger: slog.New(slog.DiscardHandler)}.Run(ctx, time.Hour)

	f.pool.Close()
	ctx, cancel = context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	AuditPruner{Service: f.service, Logger: slog.New(slog.DiscardHandler)}.Run(ctx, 10*time.Millisecond)
}

func TestTheAuditLogHandlerAnswersTheContract(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	handler := NewHandler(f.service)
	require.NoError(t, f.service.Update(t.Context(), Scope{details.Server.ID, ada}, Settings{Name: "Renamed"}))

	before, limit, action, target := int64(1<<40), 1, "server", details.Server.ID.String()
	response, err := handler.ListServerAuditLog(as(t, ada), api.ListServerAuditLogRequestObject{
		ServerId: details.Server.ID,
		Params:   api.ListServerAuditLogParams{Before: &before, Limit: &limit, Action: &action, TargetId: &target, ActorId: &ada},
	})
	require.NoError(t, err)
	entries, ok := response.(api.ListServerAuditLog200JSONResponse)
	require.True(t, ok)
	require.Len(t, entries, 1)
	assert.Equal(t, ActionServerUpdate, entries[0].Action)
	assert.Equal(t, api.AuditEntryTargetKind("server"), *entries[0].TargetKind)
	assert.Equal(t, "Renamed", entries[0].Details["name"])

	response, err = handler.ListServerAuditLog(as(t, ada), api.ListServerAuditLogRequestObject{ServerId: details.Server.ID, Params: api.ListServerAuditLogParams{Action: &[]string{"member"}[0]}})
	require.NoError(t, err)
	assert.Empty(t, response.(api.ListServerAuditLog200JSONResponse)) //nolint:forcetypeassert // checked by the test
	response, err = handler.ListServerAuditLog(as(t, ada), api.ListServerAuditLogRequestObject{ServerId: details.Server.ID})
	require.NoError(t, err)
	assert.Len(t, response, 2) // the rename and the fixture's invite

	response, err = handler.ListServerAuditLog(as(t, grace), api.ListServerAuditLogRequestObject{ServerId: details.Server.ID})
	require.NoError(t, err)
	require.IsType(t, api.ListServerAuditLog403JSONResponse{}, response)
	response, err = handler.ListServerAuditLog(as(t, grace), api.ListServerAuditLogRequestObject{ServerId: uuid.New()})
	require.NoError(t, err)
	require.IsType(t, api.ListServerAuditLog404JSONResponse{}, response)
}
