package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grants answers ChannelGrant with whatever the test sets.
type grants struct {
	grant ChannelGrant
	err   error
	asked []ChannelRef
}

func (fake *grants) ChannelGrant(_ context.Context, ref ChannelRef) (ChannelGrant, error) {
	fake.asked = append(fake.asked, ref)
	return fake.grant, fake.err
}

// audience makes a server with a text channel whose audience group has
// the given members, as the servers feature would.
func (f fixture) audience(t *testing.T, owner uuid.UUID, members ...uuid.UUID) (uuid.UUID, uuid.UUID, []byte) {
	t.Helper()
	serverID, audienceID, channelID := uuid.New(), uuid.New(), uuid.New()
	groupID := []byte("audience-group-1")
	statements := []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO servers (id, name, owner_id, created_at) VALUES ($1, 'Club', $2, $3)", []any{serverID, owner, now}},
		{"INSERT INTO conversations (id, kind, mls_group_id, server_id, created_at) VALUES ($1, 'audience', $2, $3, $4)", []any{audienceID, groupID, serverID, now}},
		{"INSERT INTO server_channels (id, server_id, kind, name, position, created_at, audience_id) VALUES ($1, $2, 'text', 'general', 0, $3, $4)", []any{channelID, serverID, now, audienceID}},
	}
	for _, member := range members {
		statements = append(statements, struct {
			sql  string
			args []any
		}{"INSERT INTO conversation_members (conversation_id, user_id, joined_at) VALUES ($1, $2, $3)", []any{audienceID, member, now}})
	}
	for _, statement := range statements {
		_, err := f.pool.Exec(t.Context(), statement.sql, statement.args...)
		require.NoError(t, err)
	}
	return audienceID, channelID, groupID
}

func TestChannelMessagesTravelInTheAudienceGroup(t *testing.T) {
	f := newFixture(t)
	ada, sam := f.user(t, "ada"), f.user(t, "sam")
	audienceID, channelID, group := f.audience(t, ada, ada, sam)
	f.grants.grant = ChannelGrant{AudienceID: audienceID, MaySend: true}
	ref := ChannelRef{ServerID: uuid.New(), ChannelID: channelID, UserID: ada}

	messageID := uuid.New()
	id, err := f.service.SendToChannel(t.Context(), ref, Outgoing{Message: privateMessage(group, 0, 1), Archive: &Archive{MessageID: messageID, Sealed: []byte("sealed")}})
	require.NoError(t, err)
	assert.Equal(t, []ChannelRef{ref}, f.grants.asked)
	assert.ElementsMatch(t, []uuid.UUID{ada, sam}, f.events.users)

	messages, err := f.service.Messages(t.Context(), sam, audienceID, MessagePage{})
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, id, messages[0].ID)
	assert.Equal(t, &channelID, messages[0].ChannelID)

	// The audience's messages always name a channel.
	_, err = f.service.SendMessage(t.Context(), ada, audienceID, Outgoing{Message: privateMessage(group, 0, 1)})
	require.ErrorIs(t, err, ErrNotAllowed)

	archived, err := f.service.ChannelArchive(t.Context(), ref, ArchivePage{Before: now.Add(time.Hour)})
	require.NoError(t, err)
	assert.Equal(t, []ChannelArchived{{MessageID: messageID, MLSGroupID: group, Epoch: 0, SenderID: &ada, Sealed: []byte("sealed"), CreatedAt: archived[0].CreatedAt}}, archived)
	f.grants.grant.HistorySince = now.Add(time.Minute)
	archived, err = f.service.ChannelArchive(t.Context(), ref, ArchivePage{Before: now.Add(time.Hour)})
	require.NoError(t, err)
	assert.Empty(t, archived, "nothing from before the member may read")
}

func TestChannelMessagesFollowTheGrant(t *testing.T) {
	f := newFixture(t)
	lin, eve := f.user(t, "lin"), f.user(t, "eve")
	audienceID, channelID, group := f.audience(t, lin, lin)
	message := Outgoing{Message: privateMessage(group, 0, 1)}
	ref := ChannelRef{ChannelID: channelID, UserID: lin}

	f.grants.grant = ChannelGrant{AudienceID: audienceID}
	_, err := f.service.SendToChannel(t.Context(), ref, message)
	require.ErrorIs(t, err, ErrNotAllowed, "no SEND_MESSAGES")

	f.grants.grant.MaySend = true
	_, err = f.service.SendToChannel(t.Context(), ChannelRef{ChannelID: channelID, UserID: eve}, message)
	require.ErrorIs(t, err, ErrNotAllowed, "not yet in the audience's group")

	f.grants.err = ErrNotFound
	_, err = f.service.SendToChannel(t.Context(), ref, message)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.ChannelArchive(t.Context(), ref, ArchivePage{Before: now})
	require.ErrorIs(t, err, ErrNotFound)

	// A channel message cannot go to a DM's group.
	dm, mine, _ := f.dm(t)
	f.grants.err, f.grants.grant = nil, ChannelGrant{AudienceID: dm.ID, MaySend: true}
	_, err = f.service.SendToChannel(t.Context(), ChannelRef{ChannelID: channelID, UserID: mine}, Outgoing{Message: privateMessage(dm.MLSGroupID, 0, 1)})
	require.ErrorIs(t, err, ErrNotAllowed)

	f.grants.grant = ChannelGrant{AudienceID: audienceID, MaySend: true}
	f.pool.Close()
	_, err = f.service.ChannelArchive(t.Context(), ref, ArchivePage{Before: now})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNotFound)
}
