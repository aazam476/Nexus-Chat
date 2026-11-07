package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerBody(username string, invite string) string {
	inviteField := ""
	if invite != "" {
		inviteField = fmt.Sprintf(`, "inviteCode": %q`, invite)
	}
	return fmt.Sprintf(`{"username": %q, "displayName": "New User", "email": "%s@example.com", "password": "correct horse battery staple", "deviceName": "Phone"%s}`,
		username, username, inviteField)
}

func TestInviteOnlyRegistration(t *testing.T) {
	server := setUpInstance(t) // setup chose registration mode invite_only
	admin := server.login(t, "Laptop")

	server.call(t, http.MethodPost, "/api/v1/auth/register", registerBody("bob", ""), http.StatusForbidden)

	var invite struct{ Code string }
	created := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/admin/invites", body: `{"maxUses": 1}`}, http.StatusCreated)
	require.NoError(t, json.Unmarshal(created, &invite))

	var registered loginResult
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodPost, "/api/v1/auth/register", registerBody("bob", invite.Code), http.StatusCreated), &registered))
	assert.Equal(t, "bob", registered.User.Username)
	me := server.authorizedCall(t, registered.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK)
	assert.Contains(t, string(me), `"emailVerified":false`)

	server.call(t, http.MethodPost, "/api/v1/auth/register", registerBody("carol", invite.Code), http.StatusForbidden)
	server.call(t, http.MethodPost, "/api/v1/auth/register", registerBody("bob", invite.Code), http.StatusForbidden)

	// Non-admins cannot manage invites.
	server.authorizedCall(t, registered.Token, http.MethodGet, "/api/v1/admin/invites", http.StatusForbidden)
	listed := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/admin/invites", http.StatusOK)
	assert.Contains(t, string(listed), `"uses":1`)
}

func TestEmailFlowsWithoutSMTP(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")

	// No SMTP is configured, so resending explains why it cannot.
	unavailable := server.authorizedCall(t, admin.Token, http.MethodPost, "/api/v1/auth/verify-email/resend", http.StatusServiceUnavailable)
	assert.Contains(t, string(unavailable), `"code":"email_unavailable"`)

	// Reset requests look the same for known and unknown addresses.
	server.call(t, http.MethodPost, "/api/v1/auth/password-reset", `{"email": "admin@example.com"}`, http.StatusAccepted)
	server.call(t, http.MethodPost, "/api/v1/auth/password-reset", `{"email": "nobody@example.com"}`, http.StatusAccepted)

	server.call(t, http.MethodPost, "/api/v1/auth/password-reset/complete", `{"token": "bogus", "password": "a brand new password"}`, http.StatusBadRequest)
	server.call(t, http.MethodPost, "/api/v1/auth/verify-email", `{"token": "bogus"}`, http.StatusBadRequest)
}

// authorizedCallWithBody is callWithHeaders with a session token.
func (server authenticatedServer) authorizedCallWithBody(t *testing.T, token string, spec requestSpec, wantStatus int) []byte {
	t.Helper()
	spec.headers = map[string]string{"Authorization": "Bearer " + token}
	return server.callWithHeaders(t, spec, wantStatus)
}
