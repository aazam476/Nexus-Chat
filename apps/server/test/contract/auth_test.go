package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authenticatedServer is a running server whose setup is done, plus a
// helper for calls that carry a session token.
type authenticatedServer struct {
	runningServer
}

func setUpInstance(t *testing.T) authenticatedServer {
	t.Helper()
	server := startServer(t)
	server.call(t, http.MethodPost, "/api/v1/setup", setupRequest(server.setupToken(t), "admin"), http.StatusCreated)
	return authenticatedServer{server}
}

func loginBody(login string, password string, device string) string {
	return fmt.Sprintf(`{"login": %q, "password": %q, "deviceName": %q}`, login, password, device)
}

type loginResult struct {
	Token   string
	Session struct {
		ID      string `json:"id"`
		Current bool
	}
	User struct{ Username string }
}

func (server authenticatedServer) login(t *testing.T, device string) loginResult {
	t.Helper()
	var result loginResult
	body := server.call(t, http.MethodPost, "/api/v1/auth/login", loginBody("admin@example.com", "a long password", device), http.StatusOK)
	require.NoError(t, json.Unmarshal(body, &result))
	return result
}

func TestLoginSessionsAndLogout(t *testing.T) {
	server := setUpInstance(t)

	laptop := server.login(t, "Laptop")
	assert.True(t, strings.HasPrefix(laptop.Token, "nxs_"))
	assert.Equal(t, "admin", laptop.User.Username)
	assert.True(t, laptop.Session.Current)

	me := server.authorizedCall(t, laptop.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK)
	assert.Contains(t, string(me), `"isInstanceAdmin":true`)

	phone := server.login(t, "Phone")
	var sessions []struct {
		ID         string `json:"id"`
		DeviceName string
		Current    bool
	}
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, laptop.Token, http.MethodGet, "/api/v1/auth/sessions", http.StatusOK), &sessions))
	require.Len(t, sessions, 2)

	// The laptop revokes the phone; the phone's token stops working.
	server.authorizedCall(t, laptop.Token, http.MethodDelete, "/api/v1/auth/sessions/"+phone.Session.ID, http.StatusNoContent)
	server.authorizedCall(t, phone.Token, http.MethodGet, "/api/v1/users/me", http.StatusUnauthorized)
	server.authorizedCall(t, laptop.Token, http.MethodDelete, "/api/v1/auth/sessions/"+phone.Session.ID, http.StatusNotFound)

	server.authorizedCall(t, laptop.Token, http.MethodPost, "/api/v1/auth/logout", http.StatusNoContent)
	server.authorizedCall(t, laptop.Token, http.MethodGet, "/api/v1/users/me", http.StatusUnauthorized)
}

func TestProtectedEndpointsRejectMissingTokens(t *testing.T) {
	server := setUpInstance(t)

	server.call(t, http.MethodGet, "/api/v1/users/me", "", http.StatusUnauthorized)
	server.call(t, http.MethodGet, "/api/v1/auth/sessions", "", http.StatusUnauthorized)
}

func TestWrongPasswordsLeadToDelays(t *testing.T) {
	server := setUpInstance(t)
	wrong := loginBody("admin", "not the password", "Laptop")

	server.call(t, http.MethodPost, "/api/v1/auth/login", loginBody("nobody", "whatever", "x"), http.StatusUnauthorized)
	for range 5 {
		server.call(t, http.MethodPost, "/api/v1/auth/login", wrong, http.StatusUnauthorized)
	}
	// The fifth failure locked the account for a second.
	delayed := server.call(t, http.MethodPost, "/api/v1/auth/login", loginBody("admin", "a long password", "Laptop"), http.StatusTooManyRequests)
	assert.Contains(t, string(delayed), `"code":"login_delayed"`)
}

// authorizedCall is call with an Authorization header.
func (server authenticatedServer) authorizedCall(t *testing.T, token string, method string, path string, wantStatus int) []byte { //nolint:revive // test helper; same reasoning as call
	t.Helper()
	return server.callWithHeaders(t, requestSpec{method: method, path: path, headers: map[string]string{"Authorization": "Bearer " + token}}, wantStatus)
}
