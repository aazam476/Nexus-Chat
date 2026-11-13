package servers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
)

func TestInvitesLetPeopleJoin(t *testing.T) {
	f := newFixture(t)
	ada, grace, sam, eve := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "sam"), f.user(t, "eve")
	details := f.server(t, ada, grace)
	owner, member := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}

	once, err := f.service.CreateInvite(t.Context(), member, InviteLimits{MaxUses: ptr(int32(1)), ExpiresIn: ptr(time.Hour)})
	require.NoError(t, err)
	assert.Len(t, once.Code, 16)
	preview, err := f.service.PreviewInvite(t.Context(), once.Code)
	require.NoError(t, err)
	assert.Equal(t, "Book club", preview.Server.Name)
	assert.Equal(t, int64(2), preview.Members)

	joined, err := f.service.Join(t.Context(), once.Code, sam)
	require.NoError(t, err)
	assert.Equal(t, details.Server.ID, joined)
	// Joining again uses nothing; the used-up invite turns others away.
	_, err = f.service.Join(t.Context(), once.Code, sam)
	require.ErrorIs(t, err, ErrInviteInvalid)
	_, err = f.service.Join(t.Context(), once.Code, eve)
	require.ErrorIs(t, err, ErrInviteInvalid)

	later, err := f.service.CreateInvite(t.Context(), member, InviteLimits{ExpiresIn: ptr(time.Minute)})
	require.NoError(t, err)
	again, err := f.service.Join(t.Context(), later.Code, grace)
	require.NoError(t, err)
	assert.Equal(t, details.Server.ID, again)
	f.clock.advance(2 * time.Minute)
	_, err = f.service.PreviewInvite(t.Context(), later.Code)
	require.ErrorIs(t, err, ErrInviteInvalid)
	_, err = f.service.PreviewInvite(t.Context(), "nonexistent")
	require.ErrorIs(t, err, ErrInviteInvalid)

	for _, bad := range []InviteLimits{{MaxUses: ptr(int32(0))}, {ExpiresIn: ptr(-time.Second)}, {ExpiresIn: ptr(400 * 24 * time.Hour)}} {
		_, err := f.service.CreateInvite(t.Context(), member, bad)
		require.ErrorIs(t, err, ErrInvalid)
	}
	_, err = f.service.Invites(t.Context(), member)
	require.ErrorIs(t, err, ErrForbidden)
	list, err := f.service.Invites(t.Context(), owner)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestInvitesCanBeRevoked(t *testing.T) {
	f := newFixture(t)
	ada, grace, sam := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "sam")
	details := f.server(t, ada, grace, sam)
	owner, member, other := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}, Scope{details.Server.ID, sam}
	invite, err := f.service.CreateInvite(t.Context(), member, InviteLimits{})
	require.NoError(t, err)

	require.ErrorIs(t, f.service.RevokeInvite(t.Context(), other, invite.Code), ErrForbidden)
	require.NoError(t, f.service.RevokeInvite(t.Context(), member, invite.Code))
	require.ErrorIs(t, f.service.RevokeInvite(t.Context(), owner, invite.Code), ErrInviteInvalid)
	second, err := f.service.CreateInvite(t.Context(), member, InviteLimits{})
	require.NoError(t, err)
	require.NoError(t, f.service.RevokeInvite(t.Context(), owner, second.Code))

	// Without CREATE_INVITES, no invites.
	require.NoError(t, f.service.UpdateRole(t.Context(), owner, details.Roles[0].ID, RoleSpec{Name: "@everyone", Permissions: permissions.ViewChannel}))
	_, err = f.service.CreateInvite(t.Context(), other, InviteLimits{})
	require.ErrorIs(t, err, ErrForbidden)
	_, err = f.service.CreateInvite(t.Context(), Scope{uuid.New(), sam}, InviteLimits{})
	require.ErrorIs(t, err, ErrNotFound)
}
