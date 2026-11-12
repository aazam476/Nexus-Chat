package contract

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadStateFollowsTheMemberAcrossDevices(t *testing.T) {
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
	send := `{"message": "` + mlsPrivateMessage(dm.MlsGroupID, 0, 1) + `", "archive": {"messageId": "` + uuid.NewString() + `", "sealed": "c2VhbGVk"}}`
	var sent struct{ ID int64 }
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, "/api/v1/conversations/"+dm.ID+"/mls/messages", send, http.StatusCreated), &sent))
	carolDevice := server.identified(t, carol.Token)

	assert.JSONEq(t, `[{"conversationId": "`+dm.ID+`", "lastReadId": 0, "unreadCount": 1}]`, string(call(carol.Token, http.MethodGet, "/api/v1/read-state", "", http.StatusOK)))
	read := call(carol.Token, http.MethodPut, "/api/v1/conversations/"+dm.ID+"/read", `{"lastReadId": `+strconv.FormatInt(sent.ID, 10)+`}`, http.StatusOK)
	assert.JSONEq(t, `{"conversationId": "`+dm.ID+`", "lastReadId": `+strconv.FormatInt(sent.ID, 10)+`, "unreadCount": 0}`, string(read))
	update := readFrameOfType(t, carolDevice, "read_update")
	assert.JSONEq(t, `{"conversationId": "`+dm.ID+`", "lastReadId": `+strconv.FormatInt(sent.ID, 10)+`}`, string(update.Data))

	call(admin.Token, http.MethodPut, "/api/v1/conversations/"+dm.ID+"/read", `{"lastReadId": 1}`, http.StatusNotFound)
	call(carol.Token, http.MethodPut, "/api/v1/conversations/"+dm.ID+"/read", `{"lastReadId": -1}`, http.StatusUnprocessableEntity)
	assert.JSONEq(t, `[]`, string(call(admin.Token, http.MethodGet, "/api/v1/read-state", "", http.StatusOK)))
}
