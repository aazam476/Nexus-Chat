package servers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
)

func TestChannelsAreManagedWithManageChannels(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner, member := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}
	textCategory := channelNamed(t, details, "Text channels")

	_, err := f.service.CreateChannel(t.Context(), member, NewChannel{Kind: KindText, Name: "nope"})
	require.ErrorIs(t, err, ErrForbidden)
	id, err := f.service.CreateChannel(t.Context(), owner, NewChannel{Kind: KindText, Name: "books", Topic: "What we read", ParentID: &textCategory.ID})
	require.NoError(t, err)
	voice, err := f.service.CreateChannel(t.Context(), owner, NewChannel{Kind: KindVoice, Name: "Lounge"})
	require.NoError(t, err)
	for _, bad := range []NewChannel{
		{Kind: "forum", Name: "x"}, {Kind: KindText, Name: ""},
		{Kind: KindCategory, Name: "x", ParentID: &textCategory.ID}, {Kind: KindText, Name: "x", ParentID: &voice},
		{Kind: KindText, Name: "x", ParentID: ptr(uuid.New())},
	} {
		_, err := f.service.CreateChannel(t.Context(), owner, bad)
		require.ErrorIs(t, err, ErrInvalid, "%+v", bad)
	}

	require.NoError(t, f.service.UpdateChannel(t.Context(), owner, id, ChannelText{Name: "reading", Topic: "Now"}))
	require.ErrorIs(t, f.service.UpdateChannel(t.Context(), owner, id, ChannelText{Name: ""}), ErrInvalid)
	require.ErrorIs(t, f.service.UpdateChannel(t.Context(), owner, uuid.New(), ChannelText{Name: "x"}), ErrNoChannel)
	require.ErrorIs(t, f.service.UpdateChannel(t.Context(), member, id, ChannelText{Name: "x"}), ErrForbidden)
	seen, err := f.service.Details(t.Context(), member)
	require.NoError(t, err)
	assert.Equal(t, "Now", channelNamed(t, seen, "reading").Topic)

	require.ErrorIs(t, f.service.DeleteChannel(t.Context(), member, voice), ErrForbidden)
	require.ErrorIs(t, f.service.DeleteChannel(t.Context(), owner, uuid.New()), ErrNoChannel)
	require.NoError(t, f.service.DeleteChannel(t.Context(), owner, voice))
}

func ptr[T any](value T) *T { return &value }

func TestChannelsAreOrderedByDragAndDrop(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	owner := Scope{details.Server.ID, ada}
	text, voice := channelNamed(t, details, "Text channels"), channelNamed(t, details, "Voice channels")
	general, lounge := channelNamed(t, details, "general"), channelNamed(t, details, "General")

	// Voice category first; both channels into it, the voice one on top.
	order := []Placement{{ID: voice.ID}, {ID: text.ID}, {ID: lounge.ID, ParentID: &voice.ID}, {ID: general.ID, ParentID: &voice.ID}}
	require.NoError(t, f.service.OrderChannels(t.Context(), owner, order))
	seen, err := f.service.Details(t.Context(), owner)
	require.NoError(t, err)
	assert.Equal(t, int32(0), channelNamed(t, seen, "Voice channels").Position)
	assert.Equal(t, int32(1), channelNamed(t, seen, "general").Position)
	assert.Equal(t, &voice.ID, channelNamed(t, seen, "general").ParentID)

	for _, bad := range [][]Placement{
		order[:3],
		{{ID: voice.ID}, {ID: voice.ID}, {ID: lounge.ID}, {ID: general.ID}},
		{{ID: voice.ID, ParentID: &text.ID}, {ID: text.ID}, {ID: lounge.ID}, {ID: general.ID}},
		{{ID: voice.ID}, {ID: text.ID}, {ID: lounge.ID, ParentID: &general.ID}, {ID: general.ID}},
		{{ID: voice.ID}, {ID: text.ID}, {ID: uuid.New()}, {ID: general.ID}},
	} {
		require.ErrorIs(t, f.service.OrderChannels(t.Context(), owner, bad), ErrBadOrder)
	}
}

func TestOverridesHideChannelsAndCannotEscalate(t *testing.T) {
	f := newFixture(t)
	ada, grace, sam := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "sam")
	details := f.server(t, ada, grace, sam)
	owner, mod, plain := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}, Scope{details.Server.ID, sam}
	general := channelNamed(t, details, "general")
	everyone := details.Roles[0].ID
	mods, err := f.service.CreateRole(t.Context(), owner, RoleSpec{Name: "Mods", Permissions: permissions.ManageChannels})
	require.NoError(t, err)
	require.NoError(t, f.service.SetMemberRole(t.Context(), owner, RoleChange{UserID: grace, RoleID: mods, Give: true}))

	// Hidden from @everyone, shown to Mods.
	require.NoError(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: TargetRole, TargetID: everyone, Deny: permissions.ViewChannel}))
	require.NoError(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: TargetRole, TargetID: mods, Allow: permissions.ViewChannel}))
	samSees, err := f.service.Details(t.Context(), plain)
	require.NoError(t, err)
	for _, channel := range samSees.Channels {
		assert.NotEqual(t, "general", channel.Name)
	}
	graceSees, err := f.service.Details(t.Context(), mod)
	require.NoError(t, err)
	assert.Len(t, channelNamed(t, graceSees, "general").Overrides, 2)

	// Grace cannot hand out what she does not have, or contradict herself.
	require.ErrorIs(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: TargetMember, TargetID: sam, Allow: permissions.ManageMessages}), ErrForbidden)
	require.ErrorIs(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: TargetMember, TargetID: sam, Allow: permissions.ViewChannel, Deny: permissions.ViewChannel}), ErrForbidden)
	require.ErrorIs(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: TargetMember, TargetID: uuid.New()}), ErrNoMember)
	require.ErrorIs(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: TargetRole, TargetID: uuid.New()}), ErrNoRole)
	require.ErrorIs(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: "team", TargetID: sam}), ErrInvalid)
	require.ErrorIs(t, f.service.SetOverride(t.Context(), plain, general.ID, Override{TargetKind: TargetMember, TargetID: sam}), ErrForbidden)
	require.ErrorIs(t, f.service.SetOverride(t.Context(), mod, uuid.New(), Override{TargetKind: TargetMember, TargetID: sam}), ErrNoChannel)

	// Sam gets in by a member override, then out again when it is cleared.
	require.NoError(t, f.service.SetOverride(t.Context(), mod, general.ID, Override{TargetKind: TargetMember, TargetID: sam, Allow: permissions.ViewChannel}))
	samSees, err = f.service.Details(t.Context(), plain)
	require.NoError(t, err)
	assert.True(t, channelNamed(t, samSees, "general").Permissions.Has(permissions.ViewChannel|permissions.SendMessages))
	require.NoError(t, f.service.ClearOverride(t.Context(), mod, general.ID, Target{Kind: TargetMember, ID: sam}))
	require.ErrorIs(t, f.service.ClearOverride(t.Context(), plain, general.ID, Target{Kind: TargetMember, ID: sam}), ErrForbidden)
	require.ErrorIs(t, f.service.ClearOverride(t.Context(), mod, uuid.New(), Target{Kind: TargetMember, ID: sam}), ErrNoChannel)

	// Someone a channel is hidden from cannot list every channel, so cannot reorder.
	require.NoError(t, f.service.SetOverride(t.Context(), owner, general.ID, Override{TargetKind: TargetMember, TargetID: grace, Deny: permissions.ViewChannel}))
	require.ErrorIs(t, f.service.OrderChannels(t.Context(), mod, nil), ErrForbidden)
}
