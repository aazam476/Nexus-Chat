// Package oidctest runs a small OpenID Connect provider for tests. It
// publishes discovery and keys, signs real ID tokens, checks PKCE, and has
// switches for the ways real providers misbehave.
//
// Start reading at [New] and [Provider.Authorize].
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
)

// ClientID and ClientSecret are what the fake provider accepts.
const (
	ClientID     = "nexus-chat"
	ClientSecret = "client-secret"
)

// User is who signs in at the fake provider.
type User struct {
	Subject           string
	Email             string
	EmailVerified     bool
	Name              string
	PreferredUsername string
}

// Behavior switches; all false means a well-behaved provider.
type Behavior struct {
	// FailToken makes the token endpoint answer with an error.
	FailToken bool
	// OmitIDToken leaves the ID token out of the token response.
	OmitIDToken bool
	// SignWithOtherKey signs ID tokens with a key the provider does not publish.
	SignWithOtherKey bool
	// WrongNonce puts a different nonce in the ID token.
	WrongNonce bool
	// EmailOnlyInUserInfo leaves email out of the ID token, as some
	// providers do, and serves it from the UserInfo endpoint.
	EmailOnlyInUserInfo bool
	// NoUserInfo leaves the UserInfo endpoint out of discovery.
	NoUserInfo bool
	// FailUserInfo makes the UserInfo endpoint answer with an error.
	FailUserInfo bool
	// UserInfoSubject, if set, is the subject UserInfo reports.
	UserInfoSubject string
	// MalformedClaims sends email_verified as a string, which a strict
	// reader rejects.
	MalformedClaims bool
}

type grant struct {
	nonce       string
	challenge   string
	redirectURI string
}

// Provider is a running fake provider.
type Provider struct {
	Server *httptest.Server
	key    *rsa.PrivateKey
	other  *rsa.PrivateKey

	mutex    sync.Mutex
	user     User
	behavior Behavior
	grants   map[string]grant
}

// New starts a fake provider that signs in user. It stops when the test ends.
func New(t *testing.T, user User) *Provider {
	t.Helper()
	provider := &Provider{user: user, grants: map[string]grant{}}
	var err error
	provider.key, err = rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	provider.other, err = rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", provider.discovery)
	mux.HandleFunc("GET /keys", provider.keys)
	mux.HandleFunc("POST /token", provider.token)
	mux.HandleFunc("GET /userinfo", provider.userInfo)
	provider.Server = httptest.NewServer(mux)
	t.Cleanup(provider.Server.Close)
	return provider
}

// Issuer is the provider's issuer URL.
func (provider *Provider) Issuer() string {
	return provider.Server.URL
}

// SetUser changes who signs in next.
func (provider *Provider) SetUser(user User) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	provider.user = user
}

// SetBehavior changes how the provider misbehaves.
func (provider *Provider) SetBehavior(behavior Behavior) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	provider.behavior = behavior
}

// Authorize plays the user signing in at authorizationURL and returns the
// query the provider would redirect back with (state and code).
func (provider *Provider) Authorize(t *testing.T, authorizationURL string) url.Values {
	t.Helper()
	parsed, err := url.Parse(authorizationURL)
	require.NoError(t, err)
	query := parsed.Query()
	require.Equal(t, ClientID, query.Get("client_id"))
	require.Equal(t, "code", query.Get("response_type"))
	require.Equal(t, "S256", query.Get("code_challenge_method"))
	code := base64.RawURLEncoding.EncodeToString([]byte(query.Get("state") + time.Now().String()))
	provider.mutex.Lock()
	provider.grants[code] = grant{nonce: query.Get("nonce"), challenge: query.Get("code_challenge"), redirectURI: query.Get("redirect_uri")}
	provider.mutex.Unlock()
	return url.Values{"state": {query.Get("state")}, "code": {code}}
}

func (provider *Provider) discovery(writer http.ResponseWriter, _ *http.Request) {
	provider.mutex.Lock()
	noUserInfo := provider.behavior.NoUserInfo
	provider.mutex.Unlock()
	document := map[string]any{
		"issuer":                                provider.Issuer(),
		"authorization_endpoint":                provider.Issuer() + "/authorize",
		"token_endpoint":                        provider.Issuer() + "/token",
		"jwks_uri":                              provider.Issuer() + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	if !noUserInfo {
		document["userinfo_endpoint"] = provider.Issuer() + "/userinfo"
	}
	writeJSON(writer, http.StatusOK, document)
}

func (provider *Provider) keys(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &provider.key.PublicKey, KeyID: "main", Algorithm: "RS256", Use: "sig"},
	}})
}

// token implements the authorization code grant with PKCE (RFC 7636 §4.6).
func (provider *Provider) token(writer http.ResponseWriter, request *http.Request) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	clientID, secret, ok := request.BasicAuth()
	if !ok {
		clientID, secret = request.PostFormValue("client_id"), request.PostFormValue("client_secret")
	}
	code := request.PostFormValue("code")
	granted, known := provider.grants[code]
	delete(provider.grants, code)
	challenge := sha256.Sum256([]byte(request.PostFormValue("code_verifier")))
	if provider.behavior.FailToken || !known || clientID != ClientID || secret != ClientSecret ||
		base64.RawURLEncoding.EncodeToString(challenge[:]) != granted.challenge ||
		request.PostFormValue("redirect_uri") != granted.redirectURI {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	response := map[string]any{"access_token": "access-" + code, "token_type": "Bearer", "expires_in": 300}
	if !provider.behavior.OmitIDToken {
		response["id_token"] = provider.idToken(granted.nonce)
	}
	writeJSON(writer, http.StatusOK, response)
}

func (provider *Provider) idToken(nonce string) string {
	if provider.behavior.WrongNonce {
		nonce = "not-the-nonce"
	}
	now := time.Now()
	claims := map[string]any{
		"iss": provider.Issuer(), "sub": provider.user.Subject, "aud": ClientID, "nonce": nonce,
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
		"name": provider.user.Name, "preferred_username": provider.user.PreferredUsername,
	}
	if !provider.behavior.EmailOnlyInUserInfo {
		claims["email"], claims["email_verified"] = provider.user.Email, provider.user.EmailVerified
	}
	if provider.behavior.MalformedClaims {
		claims["email_verified"] = "yes"
	}
	key := provider.key
	if provider.behavior.SignWithOtherKey {
		key = provider.other
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "main"))
	if err != nil {
		panic(err) // a fixed RSA key and algorithm always make a signer
	}
	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		panic(err) // plain JSON claims always serialize
	}
	return token
}

func (provider *Provider) userInfo(writer http.ResponseWriter, _ *http.Request) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	if provider.behavior.FailUserInfo {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	subject := provider.user.Subject
	if provider.behavior.UserInfoSubject != "" {
		subject = provider.behavior.UserInfoSubject
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"sub": subject, "email": provider.user.Email, "email_verified": provider.user.EmailVerified,
	})
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}
