package contract

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mlsPrivateMessage is the cleartext start of an MLS PrivateMessage, which
// is all the server reads; the rest stands in for ciphertext.
func mlsPrivateMessage(groupID []byte, epoch uint64, contentType byte) string {
	message := append([]byte{0, 1, 0, 2, 16}, groupID...) // group IDs are 16 bytes
	message = binary.BigEndian.AppendUint64(message, epoch)
	return base64.StdEncoding.EncodeToString(append(message, contentType, 0, 1, 2, 3))
}

func TestDirectMessagesThroughTheDeliveryService(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	carol := server.newUser(t, admin.Token, "carol")
	call := func(token string, method string, path string, body string, status int) []byte {
		return server.authorizedCallWithBody(t, token, requestSpec{method: method, path: path, body: body}, status)
	}
	call(carol.Token, http.MethodPut, "/api/v1/privacy", `{"whoCanDirectMessage": "everyone", "whoCanFriendRequest": "everyone"}`, http.StatusNoContent)

	var dm struct {
		ID         string `json:"id"`
		MlsGroupID []byte `json:"mlsGroupId"`
	}
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, "/api/v1/conversations/dms", `{"userId": "`+carol.ID+`"}`, http.StatusCreated), &dm))
	call(carol.Token, http.MethodPost, "/api/v1/conversations/dms", `{"userId": "`+bob.ID+`"}`, http.StatusOK)
	call(bob.Token, http.MethodPost, "/api/v1/conversations/dms", `{"userId": "`+bob.ID+`"}`, http.StatusUnprocessableEntity)
	base := "/api/v1/conversations/" + dm.ID

	state := server.authorizedCall(t, carol.Token, http.MethodGet, base+"/mls", http.StatusOK)
	assert.JSONEq(t, `{"groupId": "`+base64.StdEncoding.EncodeToString(dm.MlsGroupID)+`", "epoch": 0}`, string(state))

	groupInfo := base64.StdEncoding.EncodeToString([]byte{0, 1, 0, 4, 1})
	commit := `{"commit": "` + mlsPrivateMessage(dm.MlsGroupID, 0, 3) + `", "groupInfo": "` + groupInfo + `"}`
	call(bob.Token, http.MethodPost, base+"/mls/commits", commit, http.StatusOK)
	conflict := call(carol.Token, http.MethodPost, base+"/mls/commits", commit, http.StatusConflict)
	assert.Contains(t, string(conflict), "wrong_epoch")

	messageID := uuid.NewString()
	send := `{"message": "` + mlsPrivateMessage(dm.MlsGroupID, 1, 1) + `", "archive": {"messageId": "` + messageID + `", "sealed": "c2VhbGVk"}}`
	call(carol.Token, http.MethodPost, base+"/mls/messages", send, http.StatusCreated)
	call(carol.Token, http.MethodPost, base+"/mls/messages", send, http.StatusConflict)
	call(carol.Token, http.MethodPost, base+"/mls/messages", `{"message": "AAEC"}`, http.StatusUnprocessableEntity)

	var messages []struct{ Kind string }
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, bob.Token, http.MethodGet, base+"/mls/messages?after=0", http.StatusOK), &messages))
	require.Len(t, messages, 2)
	assert.Equal(t, "commit", messages[0].Kind)
	assert.Equal(t, "application", messages[1].Kind)
	archive := server.authorizedCall(t, bob.Token, http.MethodGet, base+"/archive", http.StatusOK)
	assert.Contains(t, string(archive), messageID)

	// An outsider sees nothing, and a block stops sending but not reading.
	server.authorizedCall(t, admin.Token, http.MethodGet, base, http.StatusNotFound)
	server.authorizedCall(t, admin.Token, http.MethodGet, base+"/mls/messages", http.StatusNotFound)
	call(carol.Token, http.MethodPut, "/api/v1/blocks/"+bob.ID, "", http.StatusNoContent)
	call(bob.Token, http.MethodPost, base+"/mls/messages", `{"message": "`+mlsPrivateMessage(dm.MlsGroupID, 1, 1)+`"}`, http.StatusForbidden)
	server.authorizedCall(t, bob.Token, http.MethodGet, base+"/mls/messages", http.StatusOK)
}

func TestGroupDirectMessages(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	users := map[string]account{}
	for _, name := range []string{"bob", "carol", "dave", "erin"} {
		users[name] = server.newUser(t, admin.Token, name)
		server.authorizedCallWithBody(t, users[name].Token, requestSpec{method: http.MethodPut, path: "/api/v1/privacy", body: `{"whoCanDirectMessage": "everyone", "whoCanFriendRequest": "everyone"}`}, http.StatusNoContent)
	}
	bob := users["bob"]
	call := func(token string, method string, path string, body string, status int) []byte {
		return server.authorizedCallWithBody(t, token, requestSpec{method: method, path: path, body: body}, status)
	}

	var group struct{ ID string }
	body := `{"name": "Compilers", "memberIds": ["` + users["carol"].ID + `", "` + users["dave"].ID + `"]}`
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, "/api/v1/conversations/groups", body, http.StatusCreated), &group))
	call(bob.Token, http.MethodPost, "/api/v1/conversations/groups", `{"memberIds": ["`+users["carol"].ID+`"]}`, http.StatusUnprocessableEntity)
	base := "/api/v1/conversations/" + group.ID
	call(bob.Token, http.MethodPost, base+"/members", `{"userId": "`+users["erin"].ID+`"}`, http.StatusNoContent)
	call(bob.Token, http.MethodPost, base+"/members", `{"userId": "`+users["erin"].ID+`"}`, http.StatusConflict)
	call(users["erin"].Token, http.MethodPatch, base, `{"name": "Compilers II"}`, http.StatusNoContent)
	assert.Contains(t, string(server.authorizedCall(t, users["carol"].Token, http.MethodGet, base, http.StatusOK)), `"name":"Compilers II"`)

	server.authorizedCall(t, users["dave"].Token, http.MethodDelete, base+"/members/me", http.StatusNoContent)
	server.authorizedCall(t, users["dave"].Token, http.MethodGet, base, http.StatusNotFound)
	directives := server.authorizedCall(t, bob.Token, http.MethodGet, base+"/mls/messages", http.StatusOK)
	assert.Contains(t, string(directives), `"kind":"directive"`)
	listed := server.authorizedCall(t, users["carol"].Token, http.MethodGet, "/api/v1/conversations", http.StatusOK)
	assert.Contains(t, string(listed), group.ID)
}
