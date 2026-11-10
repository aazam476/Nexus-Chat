package contract

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFriendsBlocksAndPrivacy(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	carol := server.newUser(t, admin.Token, "carol")
	request := func(token string, method string, path string, body string, status int) []byte {
		return server.authorizedCallWithBody(t, token, requestSpec{method: method, path: path, body: body}, status)
	}

	sent := request(bob.Token, http.MethodPost, "/api/v1/friends/requests", `{"username": "carol"}`, http.StatusOK)
	assert.JSONEq(t, `{"userId": "`+carol.ID+`", "relationship": "outgoing"}`, string(sent))
	request(bob.Token, http.MethodPost, "/api/v1/friends/requests", `{"username": "carol"}`, http.StatusConflict)
	request(bob.Token, http.MethodPost, "/api/v1/friends/requests", `{"username": "nobody"}`, http.StatusNotFound)
	request(bob.Token, http.MethodPost, "/api/v1/friends/requests", `{"username": "bob"}`, http.StatusUnprocessableEntity)

	var pending struct {
		Incoming []struct {
			User struct{ Username string }
		}
	}
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, carol.Token, http.MethodGet, "/api/v1/friends", http.StatusOK), &pending))
	require.Len(t, pending.Incoming, 1)
	assert.Equal(t, "bob", pending.Incoming[0].User.Username)

	server.authorizedCall(t, carol.Token, http.MethodPost, "/api/v1/friends/requests/"+bob.ID+"/accept", http.StatusNoContent)
	server.authorizedCall(t, carol.Token, http.MethodPost, "/api/v1/friends/requests/"+bob.ID+"/accept", http.StatusNotFound)
	server.authorizedCall(t, bob.Token, http.MethodDelete, "/api/v1/friends/"+carol.ID, http.StatusNoContent)
	server.authorizedCall(t, bob.Token, http.MethodDelete, "/api/v1/friends/"+carol.ID, http.StatusNotFound)

	// Carol blocks Bob: his request is refused without saying why.
	server.authorizedCall(t, carol.Token, http.MethodPut, "/api/v1/blocks/"+bob.ID, http.StatusNoContent)
	request(bob.Token, http.MethodPost, "/api/v1/friends/requests", `{"username": "carol"}`, http.StatusForbidden)
	blocked := server.authorizedCall(t, carol.Token, http.MethodGet, "/api/v1/blocks", http.StatusOK)
	assert.Contains(t, string(blocked), `"username":"bob"`)
	server.authorizedCall(t, carol.Token, http.MethodDelete, "/api/v1/blocks/"+bob.ID, http.StatusNoContent)
	server.authorizedCall(t, carol.Token, http.MethodDelete, "/api/v1/blocks/"+bob.ID, http.StatusNotFound)
	server.authorizedCall(t, carol.Token, http.MethodPut, "/api/v1/blocks/"+carol.ID, http.StatusUnprocessableEntity)

	defaults := server.authorizedCall(t, carol.Token, http.MethodGet, "/api/v1/privacy", http.StatusOK)
	assert.JSONEq(t, `{"whoCanDirectMessage": "friends_and_server_members", "whoCanFriendRequest": "everyone"}`, string(defaults))
	request(carol.Token, http.MethodPut, "/api/v1/privacy", `{"whoCanDirectMessage": "everyone", "whoCanFriendRequest": "nobody"}`, http.StatusNoContent)
	request(bob.Token, http.MethodPost, "/api/v1/friends/requests", `{"username": "carol"}`, http.StatusForbidden)
	server.call(t, http.MethodGet, "/api/v1/friends", "", http.StatusUnauthorized)
}
