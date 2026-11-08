package oidc

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	registrationstore "github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

type fakeUsers struct {
	admin bool
	err   error
}

func (fake fakeUsers) GetUserByID(_ context.Context, id uuid.UUID) (accountsstore.User, error) {
	return accountsstore.User{ID: id, IsInstanceAdmin: fake.admin}, fake.err
}

type handlerFixture struct {
	signInFixture
	handler *Handler
	user    uuid.UUID
	ctx     context.Context
}

func newHandlerFixture(t *testing.T, users fakeUsers) handlerFixture {
	t.Helper()
	fixture := handlerFixture{signInFixture: newSignInFixture(t), user: uuid.New()}
	fixture.handler = NewHandler(fixture.service, fixture.providers, users, fixture.settings)
	fixture.ctx = session.WithPrincipal(t.Context(), session.Principal{UserID: fixture.user})
	return fixture
}

func (fixture handlerFixture) beginRequest(returnTo string, invite *string) api.BeginProviderSignInRequestObject {
	return api.BeginProviderSignInRequestObject{ProviderId: "company", Body: &api.BeginProviderSignInRequest{ReturnTo: returnTo, InviteCode: invite}}
}

func TestListSignInProviders(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{})

	response, err := fixture.handler.ListSignInProviders(t.Context(), api.ListSignInProvidersRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, api.ListSignInProviders200JSONResponse{{Id: "company", DisplayName: "Company login"}}, response)
	fixture.store.failOn["ListProviders"] = errFailure
	_, err = fixture.handler.ListSignInProviders(t.Context(), api.ListSignInProvidersRequestObject{})
	assert.ErrorIs(t, err, errFailure)
}

func TestSignInThroughTheHandler(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{})
	invite := "CODE"
	begun, err := fixture.handler.BeginProviderSignIn(t.Context(), fixture.beginRequest("/login/oidc", &invite))
	require.NoError(t, err)
	flow, ok := begun.(api.BeginProviderSignIn200JSONResponse)
	require.True(t, ok)
	assert.NotNil(t, fixture.store.flows[flow.FlowId].InviteHash)

	state := fixture.connector.state
	redirected, err := fixture.handler.ProviderCallback(t.Context(), api.ProviderCallbackRequestObject{Params: api.ProviderCallbackParams{State: &state, Code: &[]string{"c"}[0]}})
	require.NoError(t, err)
	location := redirected.(api.ProviderCallback302Response).Headers.Location //nolint:forcetypeassert // checked by the test failing otherwise
	_, fragment, _ := strings.Cut(location, "#")
	values, err := url.ParseQuery(fragment)
	require.NoError(t, err)

	completed, err := fixture.handler.CompleteProviderSignIn(t.Context(), api.CompleteProviderSignInRequestObject{
		Body: &api.CompleteProviderSignInRequest{FlowId: flow.FlowId, Code: values.Get("code"), DeviceName: " Phone "},
	})
	require.NoError(t, err)
	result, ok := completed.(api.CompleteProviderSignIn200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "nxs_token", result.Token)
	assert.Equal(t, []session.Device{{Name: "Phone"}}, fixture.logins.devices)
}

func TestCallbackHandlerErrors(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{})
	response, err := fixture.handler.ProviderCallback(t.Context(), api.ProviderCallbackRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.ProviderCallback400JSONResponse{}, response, "no state at all")

	fixture.begin(t, nil, "")
	fixture.store.failOn["GetFlowByState"] = errFailure
	state := fixture.connector.state
	_, err = fixture.handler.ProviderCallback(t.Context(), api.ProviderCallbackRequestObject{Params: api.ProviderCallbackParams{State: &state, Error: &state}})
	assert.ErrorIs(t, err, errFailure)
}

func TestBeginHandlersMapErrors(t *testing.T) {
	tests := []struct {
		name       string
		change     func(*handlerFixture)
		returnTo   string
		wantSignIn any
		wantLink   any
	}{
		{"unknown provider", func(f *handlerFixture) { delete(f.store.providers, "company") }, "/x",
			api.BeginProviderSignIn404JSONResponse{}, api.BeginProviderLink404JSONResponse{}},
		{"no domain", func(f *handlerFixture) { f.settings.base = "" }, "/x",
			api.BeginProviderSignIn409JSONResponse{}, api.BeginProviderLink409JSONResponse{}},
		{"bad return address", func(*handlerFixture) {}, "https://evil.example",
			api.BeginProviderSignIn422JSONResponse{}, api.BeginProviderLink422JSONResponse{}},
		{"rate limited", func(f *handlerFixture) { f.limiter.denied = true }, "/x",
			api.BeginProviderSignIn429JSONResponse{}, api.BeginProviderLink429JSONResponse{}},
		{"provider down", func(f *handlerFixture) { f.connector.connectErr = errFailure }, "/x",
			api.BeginProviderSignIn502JSONResponse{}, api.BeginProviderLink502JSONResponse{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHandlerFixture(t, fakeUsers{})
			test.change(&fixture)

			signIn, err := fixture.handler.BeginProviderSignIn(t.Context(), fixture.beginRequest(test.returnTo, nil))
			require.NoError(t, err)
			link, err := fixture.handler.BeginProviderLink(fixture.ctx, api.BeginProviderLinkRequestObject{
				ProviderId: "company", Body: &api.BeginProviderLinkRequest{ReturnTo: test.returnTo},
			})
			require.NoError(t, err)

			assert.IsType(t, test.wantSignIn, signIn)
			assert.IsType(t, test.wantLink, link)
		})
	}
}

func TestBeginHandlersPassOnUnexpectedErrors(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{})
	fixture.store.failOn["InsertFlow"] = errFailure

	_, err := fixture.handler.BeginProviderSignIn(t.Context(), fixture.beginRequest("/x", nil))
	require.ErrorIs(t, err, errFailure)
	_, err = fixture.handler.BeginProviderLink(fixture.ctx, api.BeginProviderLinkRequestObject{ProviderId: "company", Body: &api.BeginProviderLinkRequest{ReturnTo: "/x"}})
	require.ErrorIs(t, err, errFailure)
	_, err = fixture.handler.BeginProviderLink(t.Context(), api.BeginProviderLinkRequestObject{ProviderId: "company", Body: &api.BeginProviderLinkRequest{ReturnTo: "/x"}})
	assert.ErrorIs(t, err, errNoPrincipal)
}

func TestLinkThroughTheHandler(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{})
	begun, err := fixture.handler.BeginProviderLink(fixture.ctx, api.BeginProviderLinkRequestObject{ProviderId: "company", Body: &api.BeginProviderLinkRequest{ReturnTo: "/settings"}})
	require.NoError(t, err)
	flowID := begun.(api.BeginProviderLink200JSONResponse).FlowId //nolint:forcetypeassert // the test fails otherwise
	assert.Equal(t, &fixture.user, fixture.store.flows[flowID].UserID)
	code := fixture.callback(t).Get("code")

	response, err := fixture.handler.CompleteProviderLink(fixture.ctx, api.CompleteProviderLinkRequestObject{Body: &api.CompleteProviderLinkRequest{FlowId: flowID, Code: code}})

	require.NoError(t, err)
	assert.Equal(t, api.CompleteProviderLink204Response{}, response)
	listed, err := fixture.handler.ListLinkedProviders(fixture.ctx, api.ListLinkedProvidersRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.ListLinkedProviders200JSONResponse{{ProviderId: "company", DisplayName: "Company login", Email: "ada@example.com", LinkedAt: testNow}}, listed)
}

func TestCompleteSignInHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(*handlerFixture)
		code   string
		want   any
	}{
		{"wrong code", func(*handlerFixture) {}, "wrong", api.CompleteProviderSignIn401JSONResponse{Code: "invalid_flow", Message: ErrInvalidFlow.Error()}},
		{"no account", func(f *handlerFixture) {
			provider := f.store.providers["company"]
			provider.AllowSignup = false
			f.store.providers["company"] = provider
		}, "", api.CompleteProviderSignIn403JSONResponse{Code: "no_account", Message: ErrNoAccount.Error()}},
		{"closed", func(f *handlerFixture) { f.settings.mode = "closed" }, "", api.CompleteProviderSignIn403JSONResponse{Code: "registration_closed", Message: ErrRegistrationClosed.Error()}},
		{"invite", func(f *handlerFixture) { f.settings.mode = "invite_only" }, "", api.CompleteProviderSignIn403JSONResponse{Code: "invite_required", Message: ErrInviteRequired.Error()}},
		{"bad invite", func(f *handlerFixture) { f.accounts.err = registrationstore.ErrInvalidInvite }, "", api.CompleteProviderSignIn403JSONResponse{Code: "invalid_invite", Message: ErrInvalidInvite.Error()}},
		{"email in use", func(f *handlerFixture) { f.accounts.err = registrationstore.ErrEmailTaken }, "", api.CompleteProviderSignIn409JSONResponse{Code: "email_in_use", Message: ErrEmailInUse.Error()}},
		{"identity in use", func(f *handlerFixture) { f.accounts.err = duplicate(store.IdentityKey) }, "", api.CompleteProviderSignIn409JSONResponse{Code: "identity_in_use", Message: ErrIdentityInUse.Error()}},
		{"rate limited", func(f *handlerFixture) { f.limiter.denied = true }, "", api.CompleteProviderSignIn429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse{Code: "rate_limited", Message: ErrRateLimited.Error()}}},
		{"second factor", func(f *handlerFixture) {
			f.logins.result = login.Result{MFAChallenge: "challenge", MFAMethods: []string{"totp", "backup_code"}}
		}, "", api.CompleteProviderSignIn202JSONResponse{ChallengeToken: "challenge", Methods: []api.MfaMethod{"totp", "backup_code"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHandlerFixture(t, fakeUsers{})
			flowID, code := fixture.signIn(t, nil, "")
			test.change(&fixture)
			if test.code != "" {
				code = test.code
			}

			response, err := fixture.handler.CompleteProviderSignIn(t.Context(), api.CompleteProviderSignInRequestObject{
				Body: &api.CompleteProviderSignInRequest{FlowId: flowID, Code: code, DeviceName: "Phone"},
			})

			require.NoError(t, err)
			assert.Equal(t, test.want, response)
		})
	}
}

func TestCompleteSignInEmailRequired(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{})
	fixture.connector.claims.Email = ""
	flowID, code := fixture.signIn(t, nil, "")

	response, err := fixture.handler.CompleteProviderSignIn(t.Context(), api.CompleteProviderSignInRequestObject{
		Body: &api.CompleteProviderSignInRequest{FlowId: flowID, Code: code, DeviceName: "Phone"},
	})

	require.NoError(t, err)
	assert.Equal(t, api.CompleteProviderSignIn403JSONResponse{Code: "email_required", Message: ErrEmailRequired.Error()}, response)
	fixture.connector.claims.Email = "ada@example.com"
	flowID, code = fixture.signIn(t, nil, "")
	fixture.logins.err = errFailure
	_, err = fixture.handler.CompleteProviderSignIn(t.Context(), api.CompleteProviderSignInRequestObject{
		Body: &api.CompleteProviderSignInRequest{FlowId: flowID, Code: code, DeviceName: "Phone"},
	})
	assert.Error(t, err)
}

func TestCompleteLinkHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(*handlerFixture)
		want   any
	}{
		{"wrong code", func(f *handlerFixture) { f.store.failOn["ConsumeFlow"] = nil }, nil},
		{"in use", func(f *handlerFixture) {
			f.store.identities[[2]string{"company", "sub-1"}] = store.OidcIdentity{UserID: uuid.New()}
		}, api.CompleteProviderLink409JSONResponse{Code: "identity_in_use", Message: ErrIdentityInUse.Error()}},
		{"already linked", func(f *handlerFixture) { f.store.duplicateOn = store.IdentityPerUserKey },
			api.CompleteProviderLink409JSONResponse{Code: "already_linked", Message: ErrAlreadyLinked.Error()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHandlerFixture(t, fakeUsers{})
			flowID, code := fixture.signIn(t, &fixture.user, "")
			test.change(&fixture)
			want := test.want
			if want == nil {
				code = "wrong"
				want = api.CompleteProviderLink401JSONResponse{Code: "invalid_flow", Message: ErrInvalidFlow.Error()}
			}

			response, err := fixture.handler.CompleteProviderLink(fixture.ctx, api.CompleteProviderLinkRequestObject{Body: &api.CompleteProviderLinkRequest{FlowId: flowID, Code: code}})

			require.NoError(t, err)
			assert.Equal(t, want, response)
		})
	}
	fixture := newHandlerFixture(t, fakeUsers{})
	fixture.store.failOn["ConsumeFlow"] = errFailure
	_, err := fixture.handler.CompleteProviderLink(fixture.ctx, api.CompleteProviderLinkRequestObject{Body: &api.CompleteProviderLinkRequest{}})
	require.ErrorIs(t, err, errFailure)
}

func TestUnlinkHandler(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{})
	fixture.store.identities[[2]string{"company", "sub-1"}] = store.OidcIdentity{ProviderID: "company", Subject: "sub-1", UserID: fixture.user}
	request := api.UnlinkProviderRequestObject{ProviderId: "company"}

	response, err := fixture.handler.UnlinkProvider(fixture.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, api.UnlinkProvider409JSONResponse{Code: "last_login_method", Message: ErrLastLoginMethod.Error()}, response)

	fixture.store.otherMethods = 1
	response, err = fixture.handler.UnlinkProvider(fixture.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, api.UnlinkProvider204Response{}, response)

	response, err = fixture.handler.UnlinkProvider(fixture.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, api.UnlinkProvider404JSONResponse{Code: "not_linked", Message: ErrNotLinked.Error()}, response)

	fixture.store.failOn["CountOtherLoginMethods"] = errFailure
	_, err = fixture.handler.UnlinkProvider(fixture.ctx, request)
	require.ErrorIs(t, err, errFailure)
	fixture.store.failOn["ListIdentitiesForUser"] = errFailure
	_, err = fixture.handler.ListLinkedProviders(fixture.ctx, api.ListLinkedProvidersRequestObject{})
	assert.ErrorIs(t, err, errFailure)
}

func TestSelfServiceHandlersNeedAPrincipal(t *testing.T) {
	handler := newHandlerFixture(t, fakeUsers{}).handler
	ctx := t.Context()
	_, err := handler.CompleteProviderLink(ctx, api.CompleteProviderLinkRequestObject{Body: &api.CompleteProviderLinkRequest{}})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.ListLinkedProviders(ctx, api.ListLinkedProvidersRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.UnlinkProvider(ctx, api.UnlinkProviderRequestObject{})
	assert.ErrorIs(t, err, errNoPrincipal)
}
