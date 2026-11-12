package contract

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identified connects a gateway for token and waits for ready.
func (server authenticatedServer) identified(t *testing.T, token string) *websocket.Conn {
	t.Helper()
	conn := server.connectGateway(t)
	readFrameOfType(t, conn, "hello")
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(`{"type":"identify","data":{"token":"`+token+`"}}`)))
	readFrameOfType(t, conn, "ready")
	return conn
}

func TestTypingReachesTheOtherMemberOnly(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	carol := server.newUser(t, admin.Token, "carol")
	dave := server.newUser(t, admin.Token, "dave")
	server.authorizedCallWithBody(t, carol.Token, requestSpec{method: http.MethodPut, path: "/api/v1/privacy", body: `{"whoCanDirectMessage": "everyone", "whoCanFriendRequest": "everyone"}`}, http.StatusNoContent)
	var dm struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCallWithBody(t, bob.Token, requestSpec{method: http.MethodPost, path: "/api/v1/conversations/dms", body: `{"userId": "` + carol.ID + `"}`}, http.StatusCreated), &dm))
	bobConn, carolConn := server.identified(t, bob.Token), server.identified(t, carol.Token)
	daveConn := server.identified(t, dave.Token)
	typing := []byte(`{"type":"typing","data":{"conversationId":"` + dm.ID + `"}}`)

	// Dave is not in the DM: nobody hears him, and he learns nothing.
	require.NoError(t, daveConn.Write(t.Context(), websocket.MessageText, typing))
	require.NoError(t, bobConn.Write(t.Context(), websocket.MessageText, typing))

	frame := readFrameOfType(t, carolConn, "typing_start")
	assert.JSONEq(t, `{"conversationId": "`+dm.ID+`", "userId": "`+bob.ID+`"}`, string(frame.Data))
	require.NoError(t, daveConn.Write(t.Context(), websocket.MessageText, []byte(`{"type":"heartbeat","data":{}}`)))
	readFrameOfType(t, daveConn, "heartbeat_ack")
}
