package servers

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// audienceMembers lists who is entitled to an audience's group.
func (f fixture) audienceMembers(t *testing.T, audienceID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := f.pool.Query(t.Context(), "SELECT user_id FROM conversation_members WHERE conversation_id = $1", audienceID)
	require.NoError(t, err)
	defer rows.Close()
	var users []uuid.UUID
	for rows.Next() {
		var user uuid.UUID
		require.NoError(t, rows.Scan(&user))
		users = append(users, user)
	}
	slices.SortFunc(users, func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) })
	return users
}

func sorted(users ...uuid.UUID) []uuid.UUID {
	slices.SortFunc(users, func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) })
	return users
}

func (f fixture) channel(t *testing.T, scope Scope, name string) Channel {
	t.Helper()
	details, err := f.service.Details(t.Context(), scope)
	require.NoError(t, err)
	return channelNamed(t, details, name)
}

func TestChannelsWithOneAudienceShareAGroup(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner := Scope{details.Server.ID, ada}
	general := channelNamed(t, details, "general")
	require.NotNil(t, general.AudienceID)
	assert.Len(t, general.MLSGroupID, 16)
	assert.Nil(t, channelNamed(t, details, "General").AudienceID, "voice channels have no group")

	_, err := f.service.CreateChannel(t.Context(), owner, NewChannel{Kind: KindText, Name: "books"})
	require.NoError(t, err)
	books := f.channel(t, owner, "books")
	assert.Equal(t, general.AudienceID, books.AudienceID)
	assert.Equal(t, general.MLSGroupID, books.MLSGroupID)
	assert.Equal(t, sorted(ada, grace), f.audienceMembers(t, *general.AudienceID))
	assert.Empty(t, f.leaves.removals)
}

func TestARestrictedChannelMovesToItsOwnGroup(t *testing.T) {
	f := newFixture(t)
	ada, grace, sam := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "sam")
	details := f.server(t, ada, grace, sam)
	owner := Scope{details.Server.ID, ada}
	everyone, general := details.Roles[0].ID, channelNamed(t, details, "general")
	_, err := f.service.CreateChannel(t.Context(), owner, NewChannel{Kind: KindText, Name: "staff"})
	require.NoError(t, err)
	staff := f.channel(t, owner, "staff")

	// Hidden from @everyone but shown to Grace: the open channel keeps the
	// group, and the staff channel gets a new one for Ada and Grace.
	require.NoError(t, f.service.SetOverride(t.Context(), owner, staff.ID, Override{TargetKind: TargetRole, TargetID: everyone, Deny: permissions.ViewChannel}))
	require.NoError(t, f.service.SetOverride(t.Context(), owner, staff.ID, Override{TargetKind: TargetMember, TargetID: grace, Allow: permissions.ViewChannel}))
	moved := f.channel(t, owner, "staff")
	assert.Equal(t, general.AudienceID, f.channel(t, owner, "general").AudienceID)
	require.NotEqual(t, general.AudienceID, moved.AudienceID)
	assert.Equal(t, sorted(ada, grace), f.audienceMembers(t, *moved.AudienceID))
	assert.Equal(t, sorted(ada, grace, sam), f.audienceMembers(t, *general.AudienceID))
	assert.Empty(t, f.leaves.removals, "nobody loses a channel they could see")

	// Grace loses the open channel: she leaves its group, and only hers.
	require.NoError(t, f.service.SetOverride(t.Context(), owner, general.ID, Override{TargetKind: TargetMember, TargetID: grace, Deny: permissions.ViewChannel}))
	assert.Equal(t, []removal{{audienceID: *general.AudienceID, userID: grace}}, f.leaves.removals)
	assert.Equal(t, sorted(ada, sam), f.audienceMembers(t, *general.AudienceID))
}

func TestLeavingTheServerLeavesEveryGroup(t *testing.T) {
	f := newFixture(t)
	ada, sam := f.user(t, "ada"), f.user(t, "sam")
	details := f.server(t, ada, sam)
	owner := Scope{details.Server.ID, ada}
	general := channelNamed(t, details, "general")
	_, err := f.service.CreateChannel(t.Context(), owner, NewChannel{Kind: KindText, Name: "staff"})
	require.NoError(t, err)
	staff := f.channel(t, owner, "staff")

	// When a channel's whole audience changes, it keeps its group and the
	// group's members change, as when someone joins.
	require.NoError(t, f.service.SetOverride(t.Context(), owner, staff.ID, Override{TargetKind: TargetRole, TargetID: details.Roles[0].ID, Deny: permissions.ViewChannel}))
	restricted := f.channel(t, owner, "staff")
	assert.NotEqual(t, general.AudienceID, restricted.AudienceID)
	assert.Equal(t, []uuid.UUID{ada}, f.audienceMembers(t, *restricted.AudienceID))

	require.NoError(t, f.service.Leave(t.Context(), Scope{details.Server.ID, sam}))
	assert.Equal(t, []removal{{audienceID: *general.AudienceID, userID: sam}}, f.leaves.removals)
	assert.Equal(t, []uuid.UUID{ada}, f.audienceMembers(t, *general.AudienceID))

	// Now both channels have the same audience again and share a group.
	require.NoError(t, f.service.ClearOverride(t.Context(), owner, staff.ID, Target{Kind: TargetRole, ID: details.Roles[0].ID}))
	assert.Equal(t, f.channel(t, owner, "general").AudienceID, f.channel(t, owner, "staff").AudienceID)
}

func TestFailedRemovalsAreReported(t *testing.T) {
	f := newFixture(t)
	ada, sam := f.user(t, "ada"), f.user(t, "sam")
	details := f.server(t, ada, sam)
	f.leaves.fail = errors.New("delivery is down")
	require.ErrorIs(t, f.service.Leave(t.Context(), Scope{details.Server.ID, sam}), f.leaves.fail)
}

func TestChannelGrants(t *testing.T) {
	f := newFixture(t)
	ada, sam, eve := f.user(t, "ada"), f.user(t, "sam"), f.user(t, "eve")
	details := f.server(t, ada, sam)
	owner := Scope{details.Server.ID, ada}
	general, voice := channelNamed(t, details, "general"), channelNamed(t, details, "General")
	ref := func(user uuid.UUID, channel uuid.UUID) delivery.ChannelRef {
		return delivery.ChannelRef{ServerID: details.Server.ID, ChannelID: channel, UserID: user}
	}

	grant, err := f.service.ChannelGrant(t.Context(), ref(sam, general.ID))
	require.NoError(t, err)
	assert.Equal(t, delivery.ChannelGrant{AudienceID: *general.AudienceID, MaySend: true}, grant)
	for _, refused := range []delivery.ChannelRef{ref(eve, general.ID), ref(sam, voice.ID), ref(sam, uuid.New())} {
		_, err := f.service.ChannelGrant(t.Context(), refused)
		require.ErrorIs(t, err, delivery.ErrNotFound)
	}

	// Without READ_HISTORY_BEFORE_JOIN, history starts when Sam joined;
	// without SEND_MESSAGES he only reads; hidden, he gets nothing.
	limited := permissions.ViewChannel
	require.NoError(t, f.service.UpdateRole(t.Context(), owner, details.Roles[0].ID, RoleSpec{Name: "@everyone", Permissions: limited}))
	grant, err = f.service.ChannelGrant(t.Context(), ref(sam, general.ID))
	require.NoError(t, err)
	assert.False(t, grant.MaySend)
	assert.True(t, start.Equal(grant.HistorySince), "history starts when Sam joined")
	require.NoError(t, f.service.SetOverride(t.Context(), owner, general.ID, Override{TargetKind: TargetMember, TargetID: sam, Deny: permissions.ViewChannel}))
	_, err = f.service.ChannelGrant(t.Context(), ref(sam, general.ID))
	require.ErrorIs(t, err, delivery.ErrNotFound)
}

func TestChannelGrantsReportDatabaseFailures(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	f.pool.Close()
	_, err := f.service.ChannelGrant(t.Context(), delivery.ChannelRef{ServerID: details.Server.ID, ChannelID: details.Channels[0].ID, UserID: ada})
	require.Error(t, err)
	require.NotErrorIs(t, err, delivery.ErrNotFound)
}

// An unused group with exactly a channel's audience (a timed-out member
// still sees the channel) is reused rather than a new one made, and
// someone who left the server leaves the groups no channel uses.
func TestPlanningReusesAnUnusedGroup(t *testing.T) {
	server, owner, member, departed := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	used, unused, retired, channel := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	later := time.Now().Add(time.Hour)
	snapshot := store.AudienceSnapshot{
		Server:  store.Server{ID: server, OwnerID: owner},
		Roles:   []store.ServerRole{{ID: uuid.New(), Everyone: true, Permissions: int64(permissions.DefaultEveryone)}},
		Members: []store.ListServerMembersRow{{UserID: owner}, {UserID: member, TimedOutUntil: &later}},
		Channels: []store.ServerChannel{
			{ID: channel, Kind: KindText}, {ID: uuid.New(), Kind: KindVoice},
		},
		Audiences: []store.ListAudiencesRow{{ID: used}, {ID: unused}, {ID: retired}},
		AudienceMembers: []store.ListAudienceMembersRow{
			{ConversationID: used, UserID: owner}, {ConversationID: unused, UserID: owner}, {ConversationID: unused, UserID: member},
			{ConversationID: retired, UserID: owner}, {ConversationID: retired, UserID: departed},
		},
	}
	now := time.Now()
	plan := planAudiences(snapshot, now, func() (uuid.UUID, []byte) {
		t.Fatal("no new group is needed")
		return uuid.Nil, nil
	})
	assert.Empty(t, plan.Create)
	assert.Equal(t, []store.SetChannelAudienceParams{{ID: channel, AudienceID: &unused}}, plan.Assign)
	assert.Empty(t, plan.Add, "the timed-out member is already in the reused group")
	assert.Equal(t, []store.RemoveAudienceMemberParams{{ConversationID: retired, UserID: departed}}, plan.Remove)
}
