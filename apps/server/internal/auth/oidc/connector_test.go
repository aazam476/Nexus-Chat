package oidc

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/oidctest"
)

const testRedirect = "https://chat.example.com" + CallbackPath

var alice = oidctest.User{Subject: "alice-1", Email: "Alice@Example.com", EmailVerified: true, Name: "Alice A", PreferredUsername: "alice"}

func providerFor(fake *oidctest.Provider) Provider {
	return Provider{ID: "test", Issuer: fake.Issuer(), ClientID: oidctest.ClientID, ClientSecret: oidctest.ClientSecret, Scopes: []string{"openid", "email"}}
}

// signInAt runs the browser part of a sign-in and returns the code and the
// relying party that started it.
func signInAt(t *testing.T, connector *Connector, fake *oidctest.Provider, nonce string) (relyingParty, string, string) {
	t.Helper()
	party, err := connector.Connect(t.Context(), providerFor(fake), testRedirect)
	require.NoError(t, err)
	verifier := "verifier-verifier-verifier-verifier-verifier-1234"
	query := fake.Authorize(t, party.AuthorizationURL("state-1", nonce, verifier))
	require.Equal(t, "state-1", query.Get("state"))
	return party, query.Get("code"), verifier
}

func TestExchangeReturnsVerifiedClaims(t *testing.T) {
	fake := oidctest.New(t, alice)
	party, code, verifier := signInAt(t, NewConnector(nil, &movableClock{}), fake, "nonce-1")

	claims, err := party.Exchange(t.Context(), code, verifier)

	require.NoError(t, err)
	assert.Equal(t, Claims{Subject: "alice-1", Email: "Alice@Example.com", EmailVerified: true, Name: "Alice A", PreferredUsername: "alice", Nonce: "nonce-1"}, claims)
	_, err = party.Exchange(t.Context(), code, verifier)
	require.ErrorContains(t, err, "exchanging the authorization code", "a code works once")
}

func TestExchangeNeedsTheRightPKCEVerifier(t *testing.T) {
	fake := oidctest.New(t, alice)
	party, code, _ := signInAt(t, NewConnector(nil, &movableClock{}), fake, "n")

	_, err := party.Exchange(t.Context(), code, "another-verifier-another-verifier-another-verifier")

	assert.ErrorContains(t, err, "exchanging the authorization code")
}

func TestExchangeRejectsBadProviderAnswers(t *testing.T) {
	tests := []struct {
		name     string
		behavior oidctest.Behavior
		want     string
	}{
		{"token error", oidctest.Behavior{FailToken: true}, "exchanging the authorization code"},
		{"no ID token", oidctest.Behavior{OmitIDToken: true}, errNoIDToken.Error()},
		{"forged signature", oidctest.Behavior{SignWithOtherKey: true}, "verifying the ID token"},
		{"malformed claims", oidctest.Behavior{MalformedClaims: true}, "reading the ID token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := oidctest.New(t, alice)
			party, code, verifier := signInAt(t, NewConnector(nil, &movableClock{}), fake, "n")
			fake.SetBehavior(test.behavior)

			_, err := party.Exchange(t.Context(), code, verifier)

			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestExchangeFallsBackToUserInfoForTheEmail(t *testing.T) {
	tests := []struct {
		name      string
		behavior  oidctest.Behavior
		wantEmail string
	}{
		{"from user info", oidctest.Behavior{EmailOnlyInUserInfo: true}, "Alice@Example.com"},
		{"no user info endpoint", oidctest.Behavior{EmailOnlyInUserInfo: true, NoUserInfo: true}, ""},
		{"user info fails", oidctest.Behavior{EmailOnlyInUserInfo: true, FailUserInfo: true}, ""},
		{"user info about someone else", oidctest.Behavior{EmailOnlyInUserInfo: true, UserInfoSubject: "mallory"}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := oidctest.New(t, alice)
			fake.SetBehavior(test.behavior) // before discovery, which reads NoUserInfo
			party, code, verifier := signInAt(t, NewConnector(nil, &movableClock{}), fake, "n")

			claims, err := party.Exchange(t.Context(), code, verifier)

			require.NoError(t, err)
			assert.Equal(t, test.wantEmail, claims.Email)
			assert.Equal(t, test.wantEmail != "", claims.EmailVerified)
		})
	}
}

func TestConnectCachesDiscoveryForAnHour(t *testing.T) {
	fake := oidctest.New(t, alice)
	clock := &movableClock{now: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}
	connector := NewConnector(&http.Client{Timeout: time.Second}, clock)
	require.NoError(t, connector.Check(t.Context(), fake.Issuer()))
	fake.Server.Close()

	_, err := connector.Connect(t.Context(), providerFor(fake), testRedirect)
	require.NoError(t, err, "served from the cache")

	clock.now = clock.now.Add(discoveryLifetime + time.Second)
	_, err = connector.Connect(t.Context(), providerFor(fake), testRedirect)
	assert.ErrorContains(t, err, "discovering "+fake.Issuer())
}

func TestCheckReportsUnreachableIssuers(t *testing.T) {
	connector := NewConnector(nil, &movableClock{})

	assert.ErrorContains(t, connector.Check(t.Context(), "http://127.0.0.1:1"), "discovering")
}

func TestAuthorizationURLAsksForPKCEAndNonce(t *testing.T) {
	fake := oidctest.New(t, alice)
	party, err := NewConnector(nil, &movableClock{}).Connect(t.Context(), providerFor(fake), testRedirect)
	require.NoError(t, err)

	address := party.AuthorizationURL("s", "the-nonce", "v")

	assert.Contains(t, address, fake.Issuer()+"/authorize?")
	assert.Contains(t, address, "code_challenge_method=S256")
	assert.Contains(t, address, "nonce=the-nonce")
	assert.Contains(t, address, "scope=openid+email")
}
