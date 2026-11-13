package servers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
)

func TestRolesFollowTheHierarchy(t *testing.T) {
	f := newFixture(t)
	ada, grace, sam := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "sam")
	details := f.server(t, ada, grace, sam)
	owner, admin, plain := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}, Scope{details.Server.ID, sam}
	everyone := details.Roles[0].ID
	red := int32(0xFF0000)

	managers, err := f.service.CreateRole(t.Context(), owner, RoleSpec{Name: "Managers", Color: &red, Permissions: permissions.ManageRoles | permissions.KickMembers})
	require.NoError(t, err)
	helpers, err := f.service.CreateRole(t.Context(), owner, RoleSpec{Name: "Helpers"})
	require.NoError(t, err)
	// New roles go just above @everyone: Helpers (1) below Managers (2).
	require.NoError(t, f.service.SetMemberRole(t.Context(), owner, RoleChange{UserID: grace, RoleID: managers, Give: true}))

	// Grace manages roles below her own: Helpers and @everyone, not Managers.
	require.NoError(t, f.service.UpdateRole(t.Context(), admin, helpers, RoleSpec{Name: "Helpers", Permissions: permissions.KickMembers}))
	require.ErrorIs(t, f.service.UpdateRole(t.Context(), admin, managers, RoleSpec{Name: "Bosses"}), ErrForbidden)
	require.ErrorIs(t, f.service.UpdateRole(t.Context(), admin, helpers, RoleSpec{Name: "Helpers", Permissions: permissions.BanMembers}), ErrForbidden)
	require.NoError(t, f.service.UpdateRole(t.Context(), admin, everyone, RoleSpec{Name: "ignored", Permissions: permissions.DefaultEveryone &^ permissions.CreateInvites}))
	_, err = f.service.CreateRole(t.Context(), admin, RoleSpec{Name: "Gods", Permissions: permissions.Administrator})
	require.ErrorIs(t, err, ErrForbidden)
	require.ErrorIs(t, f.service.SetMemberRole(t.Context(), admin, RoleChange{UserID: sam, RoleID: managers, Give: true}), ErrForbidden)
	require.NoError(t, f.service.SetMemberRole(t.Context(), admin, RoleChange{UserID: sam, RoleID: helpers, Give: true}))
	require.ErrorIs(t, f.service.SetMemberRole(t.Context(), admin, RoleChange{UserID: sam, RoleID: everyone, Give: false}), ErrForbidden)
	require.ErrorIs(t, f.service.SetMemberRole(t.Context(), admin, RoleChange{UserID: uuid.New(), RoleID: helpers, Give: true}), ErrNoMember)
	require.ErrorIs(t, f.service.SetMemberRole(t.Context(), admin, RoleChange{UserID: sam, RoleID: uuid.New(), Give: true}), ErrNoRole)

	members, err := f.service.Members(t.Context(), plain)
	require.NoError(t, err)
	require.Len(t, members, 3)
	for _, member := range members {
		if member.UserID == sam {
			assert.Equal(t, []uuid.UUID{helpers}, member.RoleIDs)
		}
	}
	seen, err := f.service.Details(t.Context(), plain)
	require.NoError(t, err)
	assert.False(t, seen.Permissions.Has(permissions.CreateInvites))
	assert.True(t, seen.Permissions.Has(permissions.KickMembers))
	assert.Equal(t, []uuid.UUID{helpers}, seen.MyRoleIDs)
	assert.Equal(t, "@everyone", seen.Roles[0].Name)

	// Ordering: Grace may move Helpers but not Managers; the owner may do anything.
	require.ErrorIs(t, f.service.OrderRoles(t.Context(), admin, []uuid.UUID{managers, helpers}), ErrForbidden)
	require.ErrorIs(t, f.service.OrderRoles(t.Context(), owner, []uuid.UUID{managers}), ErrBadOrder)
	require.ErrorIs(t, f.service.OrderRoles(t.Context(), owner, []uuid.UUID{managers, managers}), ErrBadOrder)
	require.NoError(t, f.service.OrderRoles(t.Context(), owner, []uuid.UUID{managers, helpers}))
	require.ErrorIs(t, f.service.OrderRoles(t.Context(), plain, []uuid.UUID{managers, helpers}), ErrForbidden)

	require.ErrorIs(t, f.service.DeleteRole(t.Context(), owner, everyone), ErrForbidden)
	require.ErrorIs(t, f.service.DeleteRole(t.Context(), admin, helpers), ErrForbidden) // now above her
	require.NoError(t, f.service.DeleteRole(t.Context(), owner, helpers))
	require.NoError(t, f.service.SetMemberRole(t.Context(), owner, RoleChange{UserID: grace, RoleID: managers, Give: false}))
	_, err = f.service.CreateRole(t.Context(), owner, RoleSpec{Name: "x", Color: ptr(int32(0x1000000))})
	require.ErrorIs(t, err, ErrInvalid)
	require.ErrorIs(t, f.service.UpdateRole(t.Context(), owner, managers, RoleSpec{Name: ""}), ErrInvalid)
}

func TestTimedOutMembersOnlyRead(t *testing.T) {
	f := newFixture(t)
	ada, sam := f.user(t, "ada"), f.user(t, "sam")
	details := f.server(t, ada, sam)
	until := start.Add(1e12)
	_, err := f.pool.Exec(t.Context(), "UPDATE server_members SET timed_out_until = $1 WHERE user_id = $2", until, sam)
	require.NoError(t, err)
	seen, err := f.service.Details(t.Context(), Scope{details.Server.ID, sam})
	require.NoError(t, err)
	assert.Equal(t, permissions.ViewChannel|permissions.ReadHistoryBeforeJoin, channelNamed(t, seen, "general").Permissions)
	f.clock.advance(2e12)
	seen, err = f.service.Details(t.Context(), Scope{details.Server.ID, sam})
	require.NoError(t, err)
	assert.True(t, channelNamed(t, seen, "general").Permissions.Has(permissions.SendMessages))
}
