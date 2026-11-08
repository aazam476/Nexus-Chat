package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// discoveryLifetime is how long a provider's published configuration is
// reused before it is fetched again. Signing keys are refreshed by go-oidc
// on their own whenever a token names an unknown key.
const discoveryLifetime = time.Hour

// requestTimeout bounds every call to a provider.
const requestTimeout = 15 * time.Second

var errNoIDToken = errors.New("the provider's token response has no ID token")

// Claims are the verified facts about the user from the ID token (and the
// UserInfo endpoint, when the ID token has no email).
type Claims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"emailVerified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferredUsername"`
	Nonce             string `json:"-"`
}

// relyingParty talks to one provider for one redirect address.
type relyingParty interface {
	// AuthorizationURL is where the user's browser goes to sign in.
	AuthorizationURL(state string, nonce string, verifier string) string
	// Exchange trades the authorization code for verified claims.
	Exchange(ctx context.Context, code string, verifier string) (Claims, error)
}

// Connector creates relying parties from provider settings, caching each
// issuer's discovery document.
type Connector struct {
	client *http.Client
	clock  clock.Clock
	mutex  sync.Mutex
	cache  map[string]discovered
}

type discovered struct {
	provider *oidc.Provider
	loadedAt time.Time
}

// NewConnector returns a Connector making requests with client (nil means
// a client with a 15-second timeout).
func NewConnector(client *http.Client, clock clock.Clock) *Connector {
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	return &Connector{client: client, clock: clock, cache: map[string]discovered{}}
}

// Check fetches issuer's discovery document now, so a mistyped issuer is
// caught when the admin saves the provider.
func (connector *Connector) Check(ctx context.Context, issuer string) error {
	_, err := connector.load(ctx, issuer)
	return err
}

// Connect returns a relying party for provider that asks the provider to
// redirect to redirectURL.
func (connector *Connector) Connect(ctx context.Context, provider Provider, redirectURL string) (relyingParty, error) {
	connector.mutex.Lock()
	cached, found := connector.cache[provider.Issuer]
	connector.mutex.Unlock()
	if !found || connector.clock.Now().Sub(cached.loadedAt) > discoveryLifetime {
		var err error
		if cached.provider, err = connector.load(ctx, provider.Issuer); err != nil {
			return nil, err
		}
	}
	return &party{
		client:   connector.client,
		provider: cached.provider,
		config: oauth2.Config{
			ClientID: provider.ClientID, ClientSecret: provider.ClientSecret,
			Endpoint: cached.provider.Endpoint(), RedirectURL: redirectURL, Scopes: provider.Scopes,
		},
		verifier: cached.provider.Verifier(&oidc.Config{ClientID: provider.ClientID}),
	}, nil
}

func (connector *Connector) load(ctx context.Context, issuer string) (*oidc.Provider, error) {
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, connector.client), issuer)
	if err != nil {
		return nil, fmt.Errorf("discovering %s: %w", issuer, err)
	}
	connector.mutex.Lock()
	connector.cache[issuer] = discovered{provider: provider, loadedAt: connector.clock.Now()}
	connector.mutex.Unlock()
	return provider, nil
}

type party struct {
	client   *http.Client
	provider *oidc.Provider
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// AuthorizationURL uses PKCE (RFC 7636, S256) and an OIDC nonce.
func (party *party) AuthorizationURL(state string, nonce string, verifier string) string {
	return party.config.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

// Exchange redeems the code and verifies the ID token's signature, issuer,
// audience and expiry (OIDC Core §3.1.3.7). The caller checks the nonce.
func (party *party) Exchange(ctx context.Context, code string, verifier string) (Claims, error) {
	ctx = oidc.ClientContext(ctx, party.client)
	token, err := party.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Claims{}, fmt.Errorf("exchanging the authorization code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return Claims{}, errNoIDToken
	}
	idToken, err := party.verifier.Verify(ctx, raw)
	if err != nil {
		return Claims{}, fmt.Errorf("verifying the ID token: %w", err)
	}
	var fields struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&fields); err != nil {
		return Claims{}, fmt.Errorf("reading the ID token: %w", err)
	}
	claims := Claims{
		Subject: idToken.Subject, Email: fields.Email, EmailVerified: fields.EmailVerified,
		Name: fields.Name, PreferredUsername: fields.PreferredUsername, Nonce: idToken.Nonce,
	}
	if claims.Email == "" {
		party.addUserInfoEmail(ctx, token, &claims)
	}
	return claims, nil
}

// addUserInfoEmail fills in the email from the UserInfo endpoint, which
// some providers use instead of the ID token. Its answer counts only if it
// is about the same subject (OIDC Core §5.3.2).
func (party *party) addUserInfoEmail(ctx context.Context, token *oauth2.Token, claims *Claims) {
	if party.provider.UserInfoEndpoint() == "" {
		return
	}
	info, err := party.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil || info.Subject != claims.Subject {
		return
	}
	claims.Email, claims.EmailVerified = info.Email, info.EmailVerified
}
