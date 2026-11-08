package oidc

import (
	"crypto/sha256"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration"
)

func TestBeginStoresAFlowAndReturnsTheProviderAddress(t *testing.T) {
	fixture := newSignInFixture(t)
	expired := uuid.New()
	fixture.store.flows[expired] = store.OidcFlow{ID: expired, ExpiresAt: testNow.Add(-time.Second)}

	started := fixture.begin(t, nil, "")

	assert.Equal(t, "https://idp.example.com/authorize?state="+url.QueryEscape(fixture.connector.state), started.AuthorizationURL)
	assert.Equal(t, []string{testBase + CallbackPath}, fixture.connector.redirects)
	assert.Equal(t, []string{"203.0.113.9"}, fixture.limiter.keys)
	assert.NotContains(t, fixture.store.flows, expired, "expired flows are removed")
	flow := fixture.store.flows[started.FlowID]
	state := sha256.Sum256([]byte(fixture.connector.state))
	assert.Equal(t, state[:], flow.StateHash, "only the state's hash is stored")
	assert.Equal(t, fixture.connector.nonce, flow.Nonce)
	assert.Equal(t, fixture.connector.verifier, flow.CodeVerifier)
	assert.Equal(t, testBase+"/login/oidc", flow.ReturnTo)
	assert.Equal(t, testNow.Add(10*time.Minute), flow.ExpiresAt)
	assert.Nil(t, flow.UserID)
	assert.Nil(t, flow.InviteHash)
	assert.Len(t, fixture.connector.state, 43, "256 random bits")
	assert.NotEqual(t, fixture.connector.state, fixture.connector.nonce)
}

func TestBeginRemembersTheInviteAndTheUserToLink(t *testing.T) {
	fixture := newSignInFixture(t)
	user := uuid.New()

	started := fixture.begin(t, &user, "abcd-efgh")

	flow := fixture.store.flows[started.FlowID]
	invite := registration.HashInviteCode("ABCDEFGH")
	assert.Equal(t, invite[:], flow.InviteHash)
	assert.Equal(t, &user, flow.UserID)
}

func TestBeginErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(*signInFixture, *BeginRequest)
		check  func(*testing.T, error)
	}{
		{"rate limited", func(f *signInFixture, _ *BeginRequest) { f.limiter.denied = true }, is(ErrRateLimited)},
		{"unknown provider", func(_ *signInFixture, r *BeginRequest) { r.ProviderID = "nope" }, is(ErrProviderNotFound)},
		{"no domain", func(f *signInFixture, _ *BeginRequest) { f.settings.base = "" }, is(instanceconfig.ErrNoDomain)},
		{"bad return address", func(_ *signInFixture, r *BeginRequest) { r.ReturnTo = "https://evil.example" }, func(t *testing.T, err error) {
			var problems *validation.Error
			require.ErrorAs(t, err, &problems)
			assert.Equal(t, "returnTo", problems.Fields[0].Field)
		}},
		{"provider down", func(f *signInFixture, _ *BeginRequest) { f.connector.connectErr = errFailure }, func(t *testing.T, err error) {
			var unavailable *UnavailableError
			require.ErrorAs(t, err, &unavailable)
			require.ErrorIs(t, err, errFailure)
			assert.Contains(t, err.Error(), "not available right now")
		}},
		{"cleanup fails", func(f *signInFixture, _ *BeginRequest) { f.store.failOn["DeleteExpiredFlows"] = errFailure }, is(errFailure)},
		{"saving fails", func(f *signInFixture, _ *BeginRequest) { f.store.failOn["InsertFlow"] = errFailure }, is(errFailure)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSignInFixture(t)
			request := BeginRequest{ProviderID: "company", ReturnTo: "/login/oidc", ClientKey: "c"}
			test.change(&fixture, &request)

			_, err := fixture.service.Begin(t.Context(), request)

			test.check(t, err)
		})
	}
}

func is(target error) func(*testing.T, error) {
	return func(t *testing.T, err error) {
		t.Helper()
		assert.ErrorIs(t, err, target)
	}
}

func TestReturnAddresses(t *testing.T) {
	allowed := map[string]string{
		"/login/oidc":              testBase + "/login/oidc",
		"/settings?tab=logins":     testBase + "/settings?tab=logins",
		"nexuschat://auth/oidc":    "nexuschat://auth/oidc",
		"nexuschat:/auth/callback": "nexuschat:/auth/callback",
	}
	for returnTo, want := range allowed {
		resolved, err := resolveReturnTo(testBase, returnTo)
		require.NoError(t, err, returnTo)
		assert.Equal(t, want, resolved)
	}
	for _, returnTo := range []string{
		"", "//evil.example/path", "https://evil.example", "javascript:alert(1)", "/a#b", `/a\b`,
		"/a b", "/a\x00", "/a\x7f", "nexuschat://%zz", "/" + strings.Repeat("a", 512), "login",
	} {
		_, err := resolveReturnTo(testBase, returnTo)
		assert.Error(t, err, returnTo)
	}
}

func TestCallbackRecordsVerifiedClaims(t *testing.T) {
	fixture := newSignInFixture(t)
	started := fixture.begin(t, nil, "")

	values := fixture.callback(t)

	assert.Equal(t, started.FlowID.String(), values.Get("flow"))
	code := values.Get("code")
	assert.Len(t, code, 43)
	flow := fixture.store.flows[started.FlowID]
	completion := sha256.Sum256([]byte(code))
	assert.Equal(t, completion[:], flow.CompletionHash)
	assert.JSONEq(t, `{"sub":"sub-1","email":"Ada@Example.com","emailVerified":true,"name":"Ada Lovelace","preferredUsername":"ada"}`, string(flow.Claims))
	assert.Equal(t, []string{"the-code " + fixture.connector.verifier}, fixture.connector.exchanged, "the PKCE verifier goes with the code")

	_, err := fixture.service.Callback(t.Context(), CallbackRequest{State: fixture.connector.state, Code: "the-code"})
	assert.ErrorIs(t, err, ErrInvalidState, "a callback works once")
}

func TestCallbackSendsProviderErrorsBackToTheClient(t *testing.T) {
	for name, request := range map[string]CallbackRequest{
		"error":   {Error: "access_denied", Code: "x"},
		"no code": {},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newSignInFixture(t)
			fixture.begin(t, nil, "")
			request.State = fixture.connector.state

			location, err := fixture.service.Callback(t.Context(), request)

			require.NoError(t, err)
			assert.Equal(t, testBase+"/login/oidc#error=provider_error", location)
		})
	}
}

func TestCallbackReportsFailedSignInsToTheClient(t *testing.T) {
	tests := map[string]func(*signInFixture){
		"provider gone":   func(f *signInFixture) { f.store.failOn["GetProvider"] = errFailure },
		"domain removed":  func(f *signInFixture) { f.settings.base = "" },
		"provider down":   func(f *signInFixture) { f.connector.connectErr = errFailure },
		"exchange fails":  func(f *signInFixture) { f.connector.exchangeErr = errFailure },
		"wrong nonce":     func(f *signInFixture) { f.connector.claims.Nonce = "replayed" },
		"missing subject": func(f *signInFixture) { f.connector.claims.Subject = "" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newSignInFixture(t)
			started := fixture.begin(t, nil, "")
			change(&fixture)

			location, err := fixture.service.Callback(t.Context(), CallbackRequest{State: fixture.connector.state, Code: "c"})

			require.NoError(t, err)
			assert.Equal(t, testBase+"/login/oidc#error=sign_in_failed", location)
			assert.Nil(t, fixture.store.flows[started.FlowID].CompletionHash)
		})
	}
}

func TestCallbackErrors(t *testing.T) {
	fixture := newSignInFixture(t)
	_, err := fixture.service.Callback(t.Context(), CallbackRequest{State: "unknown", Code: "c"})
	require.ErrorIs(t, err, ErrInvalidState)

	fixture.begin(t, nil, "")
	fixture.clock.now = testNow.Add(11 * time.Minute)
	_, err = fixture.service.Callback(t.Context(), CallbackRequest{State: fixture.connector.state, Code: "c"})
	require.ErrorIs(t, err, ErrInvalidState, "expired")

	for method, want := range map[string]error{"GetFlowByState": errFailure, "RecordCallback": errFailure} {
		fixture := newSignInFixture(t)
		fixture.begin(t, nil, "")
		fixture.store.failOn[method] = errFailure
		_, err := fixture.service.Callback(t.Context(), CallbackRequest{State: fixture.connector.state, Code: "c"})
		require.ErrorIs(t, err, want, method)
	}

	fixture = newSignInFixture(t)
	fixture.begin(t, nil, "")
	fixture.store.recordNothing = true
	_, err = fixture.service.Callback(t.Context(), CallbackRequest{State: fixture.connector.state, Code: "c"})
	assert.ErrorIs(t, err, ErrInvalidState, "lost a race with another callback")
}
