package servers

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/reports"
)

// moderated is a server owned by ada, where grace holds a "Mods" role with
// p and sam and kim are plain members.
type moderated struct {
	f                    fixture
	details              Details
	ada, grace, sam, kim uuid.UUID
	owner, mod           Scope
	mods                 uuid.UUID
}

func newModerated(t *testing.T, p permissions.Permission) moderated {
	t.Helper()
	f := newFixture(t)
	m := moderated{f: f, ada: f.user(t, "ada"), grace: f.user(t, "grace"), sam: f.user(t, "sam"), kim: f.user(t, "kim")}
	m.details = f.server(t, m.ada, m.grace, m.sam, m.kim)
	m.owner, m.mod = Scope{m.details.Server.ID, m.ada}, Scope{m.details.Server.ID, m.grace}
	role, err := f.service.CreateRole(t.Context(), m.owner, RoleSpec{Name: "Mods", Permissions: p})
	require.NoError(t, err)
	require.NoError(t, f.service.SetMemberRole(t.Context(), m.owner, RoleChange{UserID: m.grace, RoleID: role, Give: true}))
	m.mods = role
	return m
}

func (m moderated) isMember(t *testing.T, userID uuid.UUID) bool {
	t.Helper()
	_, err := m.f.service.load(t.Context(), Scope{m.details.Server.ID, userID})
	if err == nil {
		return true
	}
	require.ErrorIs(t, err, ErrNotFound)
	return false
}

func TestKickingTakesAMemberOut(t *testing.T) {
	m := newModerated(t, permissions.KickMembers)
	ctx := t.Context()
	require.ErrorIs(t, m.f.service.Kick(ctx, Scope{m.details.Server.ID, m.sam}, Sanction{UserID: m.kim}), ErrForbidden, "only with KICK_MEMBERS")
	require.ErrorIs(t, m.f.service.Kick(ctx, m.mod, Sanction{UserID: m.ada}), ErrForbidden, "never the owner")
	require.ErrorIs(t, m.f.service.Kick(ctx, m.mod, Sanction{UserID: m.grace}), ErrForbidden, "never oneself")
	require.ErrorIs(t, m.f.service.Kick(ctx, m.mod, Sanction{UserID: uuid.New()}), ErrNoMember)
	require.ErrorIs(t, m.f.service.Kick(ctx, m.mod, Sanction{UserID: m.sam, Reason: strings.Repeat("x", 513)}), ErrBadSanction)

	require.NoError(t, m.f.service.Kick(ctx, m.mod, Sanction{UserID: m.sam, Reason: "spam"}))
	assert.False(t, m.isMember(t, m.sam))
	assert.Contains(t, m.f.events.changed, m.sam, "the kicked member's devices are told")
	entries, err := m.f.service.AuditLog(ctx, m.owner, AuditFilter{Action: ActionMemberKick})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "spam", entries[0].Details["reason"])
	assert.Equal(t, m.sam.String(), entries[0].TargetID)

	// Someone at or above the moderator's highest role is out of reach.
	require.NoError(t, m.f.service.SetMemberRole(ctx, m.owner, RoleChange{UserID: m.kim, RoleID: m.mods, Give: true}))
	require.ErrorIs(t, m.f.service.Kick(ctx, m.mod, Sanction{UserID: m.kim}), ErrForbidden)
	require.NoError(t, m.f.service.Kick(ctx, m.owner, Sanction{UserID: m.kim}), "the owner outranks everyone")
}

func TestBanningKeepsSomeoneOut(t *testing.T) {
	m := newModerated(t, permissions.BanMembers)
	ctx := t.Context()
	m.f.clock.advance(time.Hour)
	require.ErrorIs(t, m.f.service.Ban(ctx, m.mod, BanRequest{UserID: m.sam, DeleteMessages: MaximumPurge + time.Second}), ErrBadSanction)
	require.ErrorIs(t, m.f.service.Ban(ctx, m.mod, BanRequest{UserID: m.sam, DeleteMessages: -time.Second}), ErrBadSanction)
	require.ErrorIs(t, m.f.service.Ban(ctx, m.mod, BanRequest{UserID: m.ada}), ErrForbidden)
	require.ErrorIs(t, m.f.service.Ban(ctx, m.mod, BanRequest{UserID: uuid.New()}), ErrNoMember, "no such user")

	require.NoError(t, m.f.service.Ban(ctx, m.mod, BanRequest{UserID: m.sam, Reason: "raids", DeleteMessages: time.Hour}))
	assert.False(t, m.isMember(t, m.sam))
	require.Len(t, m.f.leaves.purges, 1, "their last hour of messages goes")
	purged := m.f.leaves.purges[0]
	assert.Equal(t, m.sam, purged.userID)
	assert.Equal(t, start, purged.since)
	assert.NotEmpty(t, purged.audienceIDs)

	invite, err := m.f.service.CreateInvite(ctx, m.owner, InviteLimits{})
	require.NoError(t, err)
	_, err = m.f.service.Join(ctx, invite.Code, m.sam)
	require.ErrorIs(t, err, ErrBanned)

	// Someone not in the server can be banned ahead of time, keeping their messages.
	outsider := m.f.user(t, "troll")
	m.f.clock.advance(time.Minute)
	require.NoError(t, m.f.service.Ban(ctx, m.mod, BanRequest{UserID: outsider}))
	assert.Len(t, m.f.leaves.purges, 1)
	bans, err := m.f.service.Bans(ctx, m.mod)
	require.NoError(t, err)
	require.Len(t, bans, 2)
	assert.Equal(t, "troll", bans[0].Username, "most recent first")
	assert.Equal(t, "raids", bans[1].Reason)
	assert.Equal(t, &m.grace, bans[1].BannedBy)

	_, err = m.f.service.Bans(ctx, Scope{m.details.Server.ID, m.kim})
	require.ErrorIs(t, err, ErrForbidden)
	require.ErrorIs(t, m.f.service.Unban(ctx, Scope{m.details.Server.ID, m.kim}, m.sam), ErrForbidden)
	require.NoError(t, m.f.service.Unban(ctx, m.mod, m.sam))
	require.ErrorIs(t, m.f.service.Unban(ctx, m.mod, m.sam), ErrNotBanned)
	_, err = m.f.service.Join(ctx, invite.Code, m.sam)
	require.NoError(t, err, "unbanned, they may join again")

	entries, err := m.f.service.AuditLog(ctx, m.owner, AuditFilter{Action: "member"})
	require.NoError(t, err)
	assert.Equal(t, []string{ActionMemberUnban, ActionMemberBan, ActionMemberBan}, actions(entries)[:3])
	assert.InDelta(t, 3600, entries[2].Details["deleteSeconds"], 0)
}

func TestBanningStopsWhenPurgingFails(t *testing.T) {
	m := newModerated(t, permissions.BanMembers)
	m.f.leaves.fail = assert.AnError
	require.ErrorIs(t, m.f.service.Ban(t.Context(), m.mod, BanRequest{UserID: m.sam, DeleteMessages: time.Minute}), assert.AnError)
}

func TestTimeoutsSilenceAMemberForAWhile(t *testing.T) {
	m := newModerated(t, permissions.TimeoutMembers)
	ctx := t.Context()
	general := channelNamed(t, m.details, "general")
	ref := delivery.ChannelRef{ServerID: m.details.Server.ID, ChannelID: general.ID, UserID: m.sam}

	require.ErrorIs(t, m.f.service.Timeout(ctx, m.mod, m.sam, 59*time.Second), ErrTimeoutRange)
	require.ErrorIs(t, m.f.service.Timeout(ctx, m.mod, m.sam, MaximumTimeout+time.Second), ErrTimeoutRange)
	require.ErrorIs(t, m.f.service.Timeout(ctx, m.mod, uuid.New(), time.Hour), ErrNoMember)
	require.ErrorIs(t, m.f.service.Timeout(ctx, Scope{m.details.Server.ID, m.kim}, m.sam, time.Hour), ErrForbidden)

	require.NoError(t, m.f.service.Timeout(ctx, m.mod, m.sam, time.Hour))
	grant, err := m.f.service.ChannelGrant(ctx, ref)
	require.NoError(t, err, "they can still read")
	assert.False(t, grant.MaySend)
	assert.Contains(t, m.f.events.changed, m.sam)

	m.f.clock.advance(time.Hour + time.Second)
	grant, err = m.f.service.ChannelGrant(ctx, ref)
	require.NoError(t, err)
	assert.True(t, grant.MaySend, "it ends by itself")

	require.NoError(t, m.f.service.Timeout(ctx, m.mod, m.sam, MaximumTimeout))
	require.NoError(t, m.f.service.Timeout(ctx, m.mod, m.sam, 0))
	grant, err = m.f.service.ChannelGrant(ctx, ref)
	require.NoError(t, err)
	assert.True(t, grant.MaySend, "or a moderator lifts it")
	entries, err := m.f.service.AuditLog(ctx, m.owner, AuditFilter{Action: ActionMemberTimeout})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, true, entries[0].Details["cleared"])
	assert.InDelta(t, 3600, entries[2].Details["seconds"], 0)
}

func TestModeratorDeletionsAreAudited(t *testing.T) {
	m := newModerated(t, permissions.ManageMessages)
	general := channelNamed(t, m.details, "general")
	ref := delivery.ChannelRef{ServerID: m.details.Server.ID, ChannelID: general.ID, UserID: m.grace}
	grant, err := m.f.service.ChannelGrant(t.Context(), ref)
	require.NoError(t, err)
	assert.True(t, grant.MayManageMessages)

	message := uuid.New()
	require.NoError(t, m.f.service.MessageDeleted(t.Context(), ref, delivery.DeletedMessage{MessageID: message, SenderID: &m.sam}))
	require.NoError(t, m.f.service.MessageDeleted(t.Context(), ref, delivery.DeletedMessage{MessageID: uuid.New()}))
	entries, err := m.f.service.AuditLog(t.Context(), m.owner, AuditFilter{Action: "message"})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, message.String(), entries[1].TargetID)
	assert.Equal(t, m.sam.String(), entries[1].Details["senderId"])
	assert.Equal(t, general.ID.String(), entries[1].Details["channelId"])
	assert.Equal(t, &m.grace, entries[1].ActorID)
	assert.NotContains(t, entries[0].Details, "senderId")
}

func TestModeratorsHandleReports(t *testing.T) {
	m := newModerated(t, permissions.ManageMessages)
	ctx := t.Context()
	may, err := m.f.service.MayHandleReports(ctx, m.details.Server.ID, m.grace)
	require.NoError(t, err)
	assert.True(t, may)
	may, err = m.f.service.MayHandleReports(ctx, m.details.Server.ID, m.sam)
	require.NoError(t, err)
	assert.False(t, may)
	_, err = m.f.service.MayHandleReports(ctx, m.details.Server.ID, uuid.New())
	require.ErrorIs(t, err, reports.ErrNoServer)

	report := uuid.New()
	require.NoError(t, m.f.service.ReportHandled(ctx, reports.Handled{ServerID: m.details.Server.ID, ActorID: m.grace, ReportID: report, Escalated: true}))
	require.NoError(t, m.f.service.ReportHandled(ctx, reports.Handled{ServerID: m.details.Server.ID, ActorID: m.grace, ReportID: report}))
	entries, err := m.f.service.AuditLog(ctx, m.owner, AuditFilter{Action: "report"})
	require.NoError(t, err)
	assert.Equal(t, []string{ActionReportResolve, ActionReportEscalate}, actions(entries))
	assert.Equal(t, report.String(), entries[0].TargetID)

	m.f.pool.Close()
	_, err = m.f.service.MayHandleReports(ctx, m.details.Server.ID, m.grace)
	require.Error(t, err)
}
