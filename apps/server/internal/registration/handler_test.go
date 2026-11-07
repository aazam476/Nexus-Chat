package registration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

type fakeUsers struct {
	admin bool
	err   error
}

func (fake fakeUsers) GetUserByID(_ context.Context, id uuid.UUID) (accountsstore.User, error) {
	return accountsstore.User{ID: id, IsInstanceAdmin: fake.admin}, fake.err
}

func registerBody(invite *string) *api.RegisterJSONRequestBody {
	return &api.RegisterJSONRequestBody{Username: "bob", DisplayName: "Bob", Email: "bob@example.com", Password: "a long password", DeviceName: "Phone", InviteCode: invite}
}

func TestRegisterResponses(t *testing.T) {
	invite := "ABCD"
	tests := []struct {
		name   string
		mode   string
		change func(*Dependencies)
		body   *api.RegisterJSONRequestBody
		want   any
		code   string
	}{
		{name: "created", mode: "open", body: registerBody(nil), want: api.Register201JSONResponse{}},
		{name: "with invite", mode: "invite_only", body: registerBody(&invite), want: api.Register201JSONResponse{}},
		{name: "closed", mode: "closed", body: registerBody(nil), want: api.Register403JSONResponse{}, code: "registration_closed"},
		{name: "invite required", mode: "invite_only", body: registerBody(nil), want: api.Register403JSONResponse{}, code: "invite_required"},
		{name: "invalid invite", mode: "open", change: func(d *Dependencies) { d.Accounts = &fakeCreator{err: store.ErrInvalidInvite} },
			body: registerBody(&invite), want: api.Register403JSONResponse{}, code: "invalid_invite"},
		{name: "rate limited", mode: "open", change: func(d *Dependencies) { d.Limiter = fakeLimiter{deny: true} }, body: registerBody(nil), want: api.Register429JSONResponse{}},
		{name: "invalid", mode: "open", body: &api.RegisterJSONRequestBody{Username: "x", DeviceName: "x"}, want: api.Register422JSONResponse{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setup := newFixture(test.mode, test.change)

			response, err := NewHandler(setup.service, nil, nil).Register(t.Context(), api.RegisterRequestObject{Body: test.body})

			require.NoError(t, err)
			assert.IsType(t, test.want, response)
			if forbidden, ok := response.(api.Register403JSONResponse); ok {
				assert.Equal(t, test.code, forbidden.Code)
			}
		})
	}
}

func TestRegisterPassesOnUnexpectedErrors(t *testing.T) {
	setup := newFixture("open", func(d *Dependencies) { d.Accounts = &fakeCreator{err: errors.New("down")} })

	_, err := NewHandler(setup.service, nil, nil).Register(t.Context(), api.RegisterRequestObject{Body: registerBody(nil)})

	assert.Error(t, err)
}

func adminContext(t *testing.T) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New()})
}

func TestInviteEndpointsRequireAnAdmin(t *testing.T) {
	handler := NewHandler(nil, NewInvites(&fakeInviteStore{}, clock.Fixed{Time: now}), fakeUsers{admin: false})
	ctx := adminContext(t)

	created, err := handler.CreateInvite(ctx, api.CreateInviteRequestObject{Body: &api.CreateInviteRequest{}})
	require.NoError(t, err)
	assert.IsType(t, api.CreateInvite403JSONResponse{}, created)
	listed, err := handler.ListInvites(ctx, api.ListInvitesRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.ListInvites403JSONResponse{}, listed)
	deleted, err := handler.DeleteInvite(ctx, api.DeleteInviteRequestObject{InviteId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.DeleteInvite403JSONResponse{}, deleted)
}

func TestInviteEndpointsForAdmins(t *testing.T) {
	invites := &fakeInviteStore{}
	handler := NewHandler(nil, NewInvites(invites, clock.Fixed{Time: now}), fakeUsers{admin: true})
	ctx := adminContext(t)
	maxUses := int32(5)

	created, err := handler.CreateInvite(ctx, api.CreateInviteRequestObject{Body: &api.CreateInviteRequest{MaxUses: &maxUses}})
	require.NoError(t, err)
	assert.IsType(t, api.CreateInvite201JSONResponse{}, created)
	assert.Equal(t, int32(5), *invites.inserted[0].MaxUses)

	listed, err := handler.ListInvites(ctx, api.ListInvitesRequestObject{})
	require.NoError(t, err)
	assert.Len(t, listed, 1)

	missing, err := handler.DeleteInvite(ctx, api.DeleteInviteRequestObject{InviteId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.DeleteInvite404JSONResponse{}, missing)
	invites.deleted = 1
	gone, err := handler.DeleteInvite(ctx, api.DeleteInviteRequestObject{InviteId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.DeleteInvite204Response{}, gone)
}

func TestInviteEndpointsPassOnErrors(t *testing.T) {
	failure := errors.New("down")
	lookupFails := NewHandler(nil, NewInvites(&fakeInviteStore{}, clock.Fixed{}), fakeUsers{err: failure})
	storeFails := NewHandler(nil, NewInvites(&fakeInviteStore{err: failure}, clock.Fixed{}), fakeUsers{admin: true})
	ctx := adminContext(t)

	for _, handler := range []*Handler{lookupFails, storeFails} {
		_, err := handler.CreateInvite(ctx, api.CreateInviteRequestObject{Body: &api.CreateInviteRequest{}})
		require.ErrorIs(t, err, failure)
		_, err = handler.ListInvites(ctx, api.ListInvitesRequestObject{})
		require.ErrorIs(t, err, failure)
		_, err = handler.DeleteInvite(ctx, api.DeleteInviteRequestObject{InviteId: uuid.New()})
		require.ErrorIs(t, err, failure)
	}
}
