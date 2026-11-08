package oidc

import (
	"context"
	"log/slog"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	registrationstore "github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

const testBase = "https://chat.example.com"

// fakeParty records the authorization request and answers Exchange with
// claims whose nonce is the one it was asked for, like a real provider.
type fakeParty struct {
	connector *fakeConnector
}

func (party *fakeParty) AuthorizationURL(state string, nonce string, verifier string) string {
	party.connector.state, party.connector.nonce, party.connector.verifier = state, nonce, verifier
	return "https://idp.example.com/authorize?state=" + url.QueryEscape(state)
}

func (party *fakeParty) Exchange(_ context.Context, code string, verifier string) (Claims, error) {
	if party.connector.exchangeErr != nil {
		return Claims{}, party.connector.exchangeErr
	}
	party.connector.exchanged = append(party.connector.exchanged, code+" "+verifier)
	claims := party.connector.claims
	if claims.Nonce == "" {
		claims.Nonce = party.connector.nonce
	}
	return claims, nil
}

type fakeConnector struct {
	connectErr  error
	exchangeErr error
	claims      Claims
	redirects   []string
	state       string
	nonce       string
	verifier    string
	exchanged   []string
}

func (fake *fakeConnector) Connect(_ context.Context, _ Provider, redirectURL string) (relyingParty, error) {
	if fake.connectErr != nil {
		return nil, fake.connectErr
	}
	fake.redirects = append(fake.redirects, redirectURL)
	return &fakeParty{connector: fake}, nil
}

type fakeSettings struct {
	base    string
	baseErr error
	mode    string
	modeErr error
}

func (fake *fakeSettings) BaseURL(context.Context) (string, error) {
	if fake.baseErr != nil {
		return "", fake.baseErr
	}
	if fake.base == "" {
		return "", instanceconfig.ErrNoDomain
	}
	return fake.base, nil
}

func (fake *fakeSettings) RegistrationMode(context.Context) (string, error) {
	return fake.mode, fake.modeErr
}

// fakeAccounts creates accounts by recording them and inserting the
// identity into the shared fake store.
type fakeAccounts struct {
	store   *fakeStore
	created []registrationstore.NewAccount
	err     error
}

func (fake *fakeAccounts) CreateWithIdentity(ctx context.Context, account registrationstore.NewAccount, identity store.InsertIdentityParams) error {
	if fake.err != nil {
		return fake.err
	}
	fake.created = append(fake.created, account)
	return fake.store.InsertIdentity(ctx, identity)
}

type fakeLogins struct {
	loggedIn []uuid.UUID
	devices  []session.Device
	result   login.Result
	err      error
}

func (fake *fakeLogins) ContinueLogin(_ context.Context, userID uuid.UUID, device session.Device) (login.Result, error) {
	if fake.err != nil {
		return login.Result{}, fake.err
	}
	fake.loggedIn = append(fake.loggedIn, userID)
	fake.devices = append(fake.devices, device)
	result := fake.result
	if result.Token == "" && result.MFAChallenge == "" {
		result = login.Result{Token: "nxs_token", User: accountsstore.User{ID: userID, Username: "someone"}}
	}
	return result, nil
}

type fakeLimiter struct {
	denied bool
	keys   []string
}

func (fake *fakeLimiter) Allow(key string) bool {
	fake.keys = append(fake.keys, key)
	return !fake.denied
}

type signInFixture struct {
	service   *SignIn
	store     *fakeStore
	connector *fakeConnector
	settings  *fakeSettings
	accounts  *fakeAccounts
	logins    *fakeLogins
	limiter   *fakeLimiter
	clock     *movableClock
	providers *Providers
}

// newSignInFixture has one provider, "company", with sign-up allowed, on
// an instance with open registration.
func newSignInFixture(t *testing.T) signInFixture {
	t.Helper()
	fake := newFakeStore()
	fixture := signInFixture{
		store: fake, connector: &fakeConnector{claims: Claims{Subject: "sub-1", Email: "Ada@Example.com", EmailVerified: true, Name: "Ada Lovelace", PreferredUsername: "ada"}},
		settings: &fakeSettings{base: testBase, mode: "open"}, accounts: &fakeAccounts{store: fake}, logins: &fakeLogins{},
		limiter: &fakeLimiter{}, clock: &movableClock{now: testNow},
	}
	fixture.providers = NewProviders(fake, testBox(t), &fakeChecker{}, fixture.clock)
	_, err := fixture.providers.Create(t.Context(), companyInput())
	require.NoError(t, err)
	fixture.service = NewSignIn(Dependencies{
		Store: fake, Providers: fixture.providers, Connector: fixture.connector, Settings: fixture.settings,
		Accounts: fixture.accounts, Logins: fixture.logins, Limiter: fixture.limiter, Clock: fixture.clock,
		Logger: slog.New(slog.DiscardHandler),
	})
	return fixture
}

// begin starts a sign-in (or a link for linkUser) returning to /login/oidc.
func (fixture signInFixture) begin(t *testing.T, linkUser *uuid.UUID, invite string) Started {
	t.Helper()
	started, err := fixture.service.Begin(t.Context(), BeginRequest{
		ProviderID: "company", ReturnTo: "/login/oidc", InviteCode: invite, LinkUserID: linkUser, ClientKey: "203.0.113.9",
	})
	require.NoError(t, err)
	return started
}

// callback plays the provider's redirect and returns the fragment values.
func (fixture signInFixture) callback(t *testing.T) url.Values {
	t.Helper()
	location, err := fixture.service.Callback(t.Context(), CallbackRequest{State: fixture.connector.state, Code: "the-code"})
	require.NoError(t, err)
	_, fragment, found := stringsCut(location, "#")
	require.True(t, found, location)
	values, err := url.ParseQuery(fragment)
	require.NoError(t, err)
	return values
}

// signIn runs begin and callback and returns what the client completes with.
func (fixture signInFixture) signIn(t *testing.T, linkUser *uuid.UUID, invite string) (uuid.UUID, string) {
	t.Helper()
	started := fixture.begin(t, linkUser, invite)
	values := fixture.callback(t)
	require.Equal(t, started.FlowID.String(), values.Get("flow"))
	return started.FlowID, values.Get("code")
}

func (fixture signInFixture) complete(t *testing.T, flowID uuid.UUID, code string) (login.Result, error) {
	t.Helper()
	return fixture.service.Complete(t.Context(), CompleteRequest{FlowID: flowID, Code: code, Device: session.Device{Name: "Phone"}, ClientKey: "203.0.113.9"})
}
