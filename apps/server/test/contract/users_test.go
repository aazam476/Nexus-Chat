package contract

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// account is a user logged in on one device.
type account struct {
	ID    string
	Token string
}

// newUser invites and registers another user (the test instance is
// invite-only) and returns them logged in.
func (server authenticatedServer) newUser(t *testing.T, adminToken string, username string) account {
	t.Helper()
	var invite struct{ Code string }
	created := server.authorizedCallWithBody(t, adminToken, requestSpec{method: http.MethodPost, path: "/api/v1/admin/invites", body: `{"maxUses": 1}`}, http.StatusCreated)
	require.NoError(t, json.Unmarshal(created, &invite))
	var registered loginResult
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodPost, "/api/v1/auth/register", registerBody(username, invite.Code), http.StatusCreated), &registered))
	var me struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, registered.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK), &me))
	return account{ID: me.ID, Token: registered.Token}
}
