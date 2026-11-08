package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/oidctest"
)

type providerFlow struct {
	FlowID           string `json:"flowId"`
	AuthorizationURL string `json:"authorizationUrl"`
}

// providerSignIn begins a flow (as token's user when token is set), signs
// in at the fake provider and follows the callback. It returns the flow
// and the fragment of the address the browser was sent back to.
func (server authenticatedServer) providerSignIn(t *testing.T, fake *oidctest.Provider, spec requestSpec) (providerFlow, url.Values) {
	t.Helper()
	var flow providerFlow
	require.NoError(t, json.Unmarshal(server.callWithHeaders(t, spec, http.StatusOK), &flow))
	authorization, err := url.Parse(flow.AuthorizationURL)
	require.NoError(t, err)
	assert.Equal(t, passkeyOrigin+"/api/v1/auth/oidc/callback", authorization.Query().Get("redirect_uri"))
	query := fake.Authorize(t, flow.AuthorizationURL)
	var request *http.Request
	callback := "/api/v1/auth/oidc/callback?" + url.Values{"state": query["state"], "code": query["code"]}.Encode()
	request, err = http.NewRequestWithContext(t.Context(), http.MethodGet, server.baseURL+callback, nil)
	require.NoError(t, err)
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	require.NoError(t, err)
	_ = response.Body.Close()
	require.Equal(t, http.StatusFound, response.StatusCode)
	location := response.Header.Get("Location")
	address, fragment, found := strings.Cut(location, "#")
	require.True(t, found, location)
	assert.Equal(t, passkeyOrigin+"/login/oidc", address)
	values, err := url.ParseQuery(fragment)
	require.NoError(t, err)
	return flow, values
}

func completeBody(flow providerFlow, values url.Values) string {
	return fmt.Sprintf(`{"flowId": %q, "code": %q, "deviceName": "Browser"}`, flow.FlowID, values.Get("code"))
}

func TestSignInWithAProvider(t *testing.T) {
	server := setUpInstanceWithDomain(t) // invite-only
	admin := server.login(t, "Laptop")
	fake := oidctest.New(t, oidctest.User{Subject: "u-1", Email: "Grace@Example.com", EmailVerified: true, Name: "Grace Hopper", PreferredUsername: "grace"})
	provider := fmt.Sprintf(`{"id": "corp", "displayName": "Corp login", "issuer": %q, "clientId": %q, "clientSecret": %q, "allowSignup": true}`,
		fake.Issuer(), oidctest.ClientID, oidctest.ClientSecret)
	created := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/admin/oidc-providers", body: provider}, http.StatusCreated)
	assert.Contains(t, string(created), `"clientSecretSet":true`)
	assert.NotContains(t, string(created), oidctest.ClientSecret)
	listed := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/admin/oidc-providers", http.StatusOK)
	assert.Contains(t, string(listed), `"callbackUrl":"https://chat.example.com/api/v1/auth/oidc/callback"`)
	public := server.call(t, http.MethodGet, "/api/v1/auth/oidc/providers", "", http.StatusOK)
	assert.JSONEq(t, `[{"id":"corp","displayName":"Corp login"}]`, string(public))
	begin := requestSpec{method: http.MethodPost, path: "/api/v1/auth/oidc/providers/corp/begin", body: `{"returnTo": "/login/oidc"}`}

	// Invite-only: without an invite, no account is created.
	flow, values := server.providerSignIn(t, fake, begin)
	refused := server.call(t, http.MethodPost, "/api/v1/auth/oidc/complete", completeBody(flow, values), http.StatusForbidden)
	assert.Contains(t, string(refused), `"code":"invite_required"`)

	var invite struct{ Code string }
	require.NoError(t, json.Unmarshal(server.authorizedCallWithBody(t, admin.Token,
		requestSpec{method: http.MethodPost, path: "/api/v1/admin/invites", body: `{"maxUses": 1}`}, http.StatusCreated), &invite))
	invited := begin
	invited.body = fmt.Sprintf(`{"returnTo": "/login/oidc", "inviteCode": %q}`, invite.Code)
	flow, values = server.providerSignIn(t, fake, invited)
	server.call(t, http.MethodPost, "/api/v1/auth/oidc/complete", fmt.Sprintf(`{"flowId": %q, "code": "wrong", "deviceName": "x"}`, flow.FlowID), http.StatusUnauthorized)
	var first loginResult
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodPost, "/api/v1/auth/oidc/complete", completeBody(flow, values), http.StatusOK), &first))
	assert.Equal(t, "grace", first.User.Username)
	me := server.authorizedCall(t, first.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK)
	assert.Contains(t, string(me), `"emailVerified":true`)

	// The next sign-in finds the same account, without another invite.
	flow, values = server.providerSignIn(t, fake, begin)
	var second loginResult
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodPost, "/api/v1/auth/oidc/complete", completeBody(flow, values), http.StatusOK), &second))
	assert.Equal(t, "grace", second.User.Username)

	linked := server.authorizedCall(t, second.Token, http.MethodGet, "/api/v1/auth/oidc/identities", http.StatusOK)
	assert.Contains(t, string(linked), `"providerId":"corp"`)
	only := server.authorizedCall(t, second.Token, http.MethodDelete, "/api/v1/auth/oidc/identities/corp", http.StatusConflict)
	assert.Contains(t, string(only), `"code":"last_login_method"`)
}

func TestLinkAProviderToAnExistingAccount(t *testing.T) {
	server := setUpInstanceWithDomain(t)
	admin := server.login(t, "Laptop")
	fake := oidctest.New(t, oidctest.User{Subject: "admin-at-corp", Email: "admin@example.com", EmailVerified: true})
	provider := fmt.Sprintf(`{"id": "corp", "displayName": "Corp", "issuer": %q, "clientId": %q, "clientSecret": %q, "allowSignup": false}`,
		fake.Issuer(), oidctest.ClientID, oidctest.ClientSecret)
	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/admin/oidc-providers", body: provider}, http.StatusCreated)

	// Signing in before linking finds no account, and sign-up is off.
	begin := requestSpec{method: http.MethodPost, path: "/api/v1/auth/oidc/providers/corp/begin", body: `{"returnTo": "/login/oidc"}`}
	flow, values := server.providerSignIn(t, fake, begin)
	refused := server.call(t, http.MethodPost, "/api/v1/auth/oidc/complete", completeBody(flow, values), http.StatusForbidden)
	assert.Contains(t, string(refused), `"code":"no_account"`)

	link := requestSpec{method: http.MethodPost, path: "/api/v1/auth/oidc/providers/corp/link", body: `{"returnTo": "/login/oidc"}`,
		headers: map[string]string{"Authorization": "Bearer " + admin.Token}}
	flow, values = server.providerSignIn(t, fake, link)
	// A link flow cannot be completed as a sign-in, so a victim tricked
	// into opening someone's link callback is not logged in.
	completeLink := fmt.Sprintf(`{"flowId": %q, "code": %q}`, flow.FlowID, values.Get("code"))
	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/oidc/link/complete", body: completeLink}, http.StatusNoContent)

	flow, values = server.providerSignIn(t, fake, begin)
	var result loginResult
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodPost, "/api/v1/auth/oidc/complete", completeBody(flow, values), http.StatusOK), &result))
	assert.Equal(t, "admin", result.User.Username)

	server.authorizedCall(t, admin.Token, http.MethodDelete, "/api/v1/auth/oidc/identities/corp", http.StatusNoContent)
	server.authorizedCall(t, admin.Token, http.MethodDelete, "/api/v1/admin/oidc-providers/corp", http.StatusNoContent)
}

func TestProviderSettingsAreValidated(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	unreachable := `{"id": "corp", "displayName": "Corp", "issuer": "http://127.0.0.1:1", "clientId": "c", "allowSignup": true}`

	rejected := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/admin/oidc-providers", body: unreachable}, http.StatusUnprocessableEntity)

	assert.Contains(t, string(rejected), `"field":"issuer"`)
	server.call(t, http.MethodGet, "/api/v1/auth/oidc/callback?state=unknown", "", http.StatusBadRequest)
	server.call(t, http.MethodPost, "/api/v1/auth/oidc/providers/none/begin", `{"returnTo": "/x"}`, http.StatusNotFound)
}
