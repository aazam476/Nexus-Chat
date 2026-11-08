package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys/passkeystest"
)

const (
	passkeyDomain = "chat.example.com"
	passkeyOrigin = "https://chat.example.com"
)

// setUpInstanceWithDomain completes setup with a public domain, which
// WebAuthn needs as its relying-party ID.
func setUpInstanceWithDomain(t *testing.T) authenticatedServer {
	t.Helper()
	server := startServer(t)
	body := fmt.Sprintf(`{
		"setupToken": %q,
		"admin": {"username": "admin", "displayName": "Admin", "email": "admin@example.com", "password": "a long password"},
		"instance": {"name": "Passkeys", "domain": %q, "registrationMode": "invite_only"}
	}`, server.setupToken(t), passkeyDomain)
	server.call(t, http.MethodPost, "/api/v1/setup", body, http.StatusCreated)
	return authenticatedServer{server}
}

type ceremonyResponse struct {
	CeremonyID string         `json:"ceremonyId"`
	Options    map[string]any `json:"options"`
}

func (response ceremonyResponse) optionsJSON(t *testing.T) []byte {
	t.Helper()
	encoded, err := json.Marshal(response.Options)
	require.NoError(t, err)
	return encoded
}

func answer(ceremonyID string, credential []byte, extra string) string {
	return fmt.Sprintf(`{"ceremonyId": %q, "credential": %s%s}`, ceremonyID, credential, extra)
}

func TestPasskeyRegistrationAndLogin(t *testing.T) {
	server := setUpInstanceWithDomain(t)
	admin := server.login(t, "Laptop")
	me := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK)
	var user struct{ ID string }
	require.NoError(t, json.Unmarshal(me, &user))
	authenticator := passkeystest.New(t, passkeyDomain)

	var registration ceremonyResponse
	body := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/webauthn/register/begin",
		body: `{"kind": "passkey", "name": "Phone"}`}, http.StatusOK)
	require.NoError(t, json.Unmarshal(body, &registration))
	credential := authenticator.Register(t, registration.optionsJSON(t), passkeyOrigin)
	body = server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/webauthn/register/finish",
		body: answer(registration.CeremonyID, credential, `, "kind": "passkey"`)}, http.StatusCreated)
	assert.Contains(t, string(body), "backupCodes", "the first second factor brings backup codes")

	// With a passkey on the account, a password alone needs a second step,
	// and the passkey can serve as that step.
	var challenge struct {
		ChallengeToken string
		Methods        []string
	}
	body = server.call(t, http.MethodPost, "/api/v1/auth/login", loginBody("admin", "a long password", "Laptop"), http.StatusAccepted)
	require.NoError(t, json.Unmarshal(body, &challenge))
	assert.Equal(t, []string{"security_key", "backup_code"}, challenge.Methods)
	var second ceremonyResponse
	body = server.call(t, http.MethodPost, "/api/v1/auth/login/mfa/security-key/begin", fmt.Sprintf(`{"challengeToken": %q}`, challenge.ChallengeToken), http.StatusOK)
	require.NoError(t, json.Unmarshal(body, &second))
	server.call(t, http.MethodPost, "/api/v1/auth/login/mfa/security-key/finish",
		answer(second.CeremonyID, authenticator.Assert(t, second.optionsJSON(t), passkeyOrigin), `, "deviceName": "Laptop"`), http.StatusOK)

	// And on its own, with no username or password.
	userID := uuid.MustParse(user.ID)
	authenticator.UserHandle = userID[:]
	var passkey ceremonyResponse
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodPost, "/api/v1/auth/passkey/begin", "", http.StatusOK), &passkey))
	server.call(t, http.MethodPost, "/api/v1/auth/passkey/finish",
		answer(passkey.CeremonyID, authenticator.Assert(t, passkey.optionsJSON(t), passkeyOrigin), `, "deviceName": "Phone"`), http.StatusOK)

	// A response made for another site is refused.
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodPost, "/api/v1/auth/passkey/begin", "", http.StatusOK), &passkey))
	server.call(t, http.MethodPost, "/api/v1/auth/passkey/finish",
		answer(passkey.CeremonyID, authenticator.Assert(t, passkey.optionsJSON(t), "https://phishing.example.net"), `, "deviceName": "Phone"`), http.StatusUnauthorized)

	listed := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/auth/webauthn", http.StatusOK)
	assert.Contains(t, string(listed), `"kind":"passkey"`)
}

func TestWebAuthnNeedsADomain(t *testing.T) {
	server := setUpInstance(t) // no domain set
	admin := server.login(t, "Laptop")

	server.call(t, http.MethodPost, "/api/v1/auth/passkey/begin", "", http.StatusConflict)
	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/webauthn/register/begin",
		body: `{"kind": "security_key", "name": "Key"}`}, http.StatusConflict)
}
