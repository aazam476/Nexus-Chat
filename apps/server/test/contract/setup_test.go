package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupToken finds the token the server printed to its log on first boot.
func (server runningServer) setupToken(t *testing.T) string {
	t.Helper()
	match := regexp.MustCompile(`"setup_token":"([A-Z2-7-]+)"`).FindStringSubmatch(server.output.String())
	require.NotNil(t, match, "the server did not log a setup token:\n%s", server.output.String())
	return match[1]
}

func setupRequest(token string, username string) string {
	return fmt.Sprintf(`{
		"setupToken": %q,
		"admin": {"username": %q, "displayName": "Admin", "email": "admin@example.com", "password": "a long password"},
		"instance": {"name": "Contract Test", "registrationMode": "invite_only"}
	}`, token, username)
}

func TestSetupFlowMatchesTheContract(t *testing.T) {
	server := startServer(t)
	token := server.setupToken(t)

	var status struct{ SetupRequired, SecureConnection bool }
	require.NoError(t, json.Unmarshal(server.call(t, http.MethodGet, "/api/v1/setup", "", http.StatusOK), &status))
	assert.True(t, status.SetupRequired)
	assert.True(t, status.SecureConnection, "the test reaches the server via 127.0.0.1, which browsers treat as secure")

	server.call(t, http.MethodPost, "/api/v1/setup", "{not json", http.StatusBadRequest)
	server.call(t, http.MethodPost, "/api/v1/setup", setupRequest("WRONG-TOKEN", "admin"), http.StatusUnauthorized)
	invalid := server.call(t, http.MethodPost, "/api/v1/setup", setupRequest(token, "X"), http.StatusUnprocessableEntity)
	assert.Contains(t, string(invalid), `"field":"admin.username"`)

	server.call(t, http.MethodPost, "/api/v1/setup", setupRequest(token, "admin"), http.StatusCreated)
	server.call(t, http.MethodPost, "/api/v1/setup", setupRequest(token, "admin"), http.StatusConflict)

	require.NoError(t, json.Unmarshal(server.call(t, http.MethodGet, "/api/v1/setup", "", http.StatusOK), &status))
	assert.False(t, status.SetupRequired)
}

func TestSetupAttemptsAreRateLimited(t *testing.T) {
	server := startServer(t)

	// The limit allows a burst of 5 attempts per client.
	for range 5 {
		server.call(t, http.MethodPost, "/api/v1/setup", setupRequest("WRONG-TOKEN", "admin"), http.StatusUnauthorized)
	}
	server.call(t, http.MethodPost, "/api/v1/setup", setupRequest("WRONG-TOKEN", "admin"), http.StatusTooManyRequests)
}
