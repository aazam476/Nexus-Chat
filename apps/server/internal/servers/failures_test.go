package servers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every operation reports a database failure instead of guessing.
func TestDatabaseFailuresAreReported(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	scope, id := Scope{details.Server.ID, ada}, uuid.New()
	f.pool.Close()
	ctx := t.Context()

	_, err := f.service.Create(ctx, ada, "x", "")
	require.Error(t, err)
	_, err = f.service.List(ctx, ada)
	require.Error(t, err)
	_, err = f.service.Details(ctx, scope)
	require.Error(t, err)
	_, err = f.service.Members(ctx, scope)
	require.Error(t, err)
	_, err = f.service.Invites(ctx, scope)
	require.Error(t, err)
	_, err = f.service.PreviewInvite(ctx, "code")
	require.Error(t, err)
	_, err = f.service.Join(ctx, "code", ada)
	require.Error(t, err)
	_, err = f.service.ShareAServer(ctx, ada, id)
	require.Error(t, err)
	_, err = f.service.CreateChannel(ctx, scope, NewChannel{Kind: KindText, Name: "x"})
	require.Error(t, err)
	_, err = f.service.CreateRole(ctx, scope, RoleSpec{Name: "x"})
	require.Error(t, err)
	_, err = f.service.CreateInvite(ctx, scope, InviteLimits{})
	require.Error(t, err)
	for _, call := range []error{
		f.service.Update(ctx, scope, Settings{Name: "x"}),
		f.service.Delete(ctx, scope),
		f.service.Leave(ctx, scope),
		f.service.UpdateChannel(ctx, scope, id, ChannelText{Name: "x"}),
		f.service.DeleteChannel(ctx, scope, id),
		f.service.OrderChannels(ctx, scope, nil),
		f.service.SetOverride(ctx, scope, id, Override{TargetKind: TargetMember, TargetID: ada}),
		f.service.ClearOverride(ctx, scope, id, Target{Kind: TargetMember, ID: ada}),
		f.service.UpdateRole(ctx, scope, id, RoleSpec{Name: "x"}),
		f.service.DeleteRole(ctx, scope, id),
		f.service.OrderRoles(ctx, scope, nil),
		f.service.SetMemberRole(ctx, scope, RoleChange{UserID: ada, RoleID: id, Give: true}),
		f.service.RevokeInvite(ctx, scope, "code"),
	} {
		assert.Error(t, call)
	}
}
