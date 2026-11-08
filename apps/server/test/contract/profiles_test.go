package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditProfileAndStatus(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")

	updated := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPatch, path: "/api/v1/users/me", body: `{"bio": "Runs this place"}`}, http.StatusOK)
	assert.Contains(t, string(updated), `"bio":"Runs this place"`)
	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPatch, path: "/api/v1/users/me", body: `{"displayName": ""}`}, http.StatusUnprocessableEntity)

	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	status := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPut, path: "/api/v1/users/me/status",
		body: `{"status": "dnd", "customStatus": {"text": "Upgrading", "expiresAt": "` + expires + `"}}`}, http.StatusOK)
	assert.Contains(t, string(status), `"status":"dnd"`)
	assert.Contains(t, string(status), `"text":"Upgrading"`)

	var me struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK), &me))
	profile := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/users/"+me.ID, http.StatusOK)
	assert.Contains(t, string(profile), `"status":"offline"`, "no gateway connection yet")
	assert.NotContains(t, string(profile), "admin@example.com", "profiles never show the email")
}

func pngBytes(t *testing.T, size int) string {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, size, size))
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, picture))
	return buffer.String()
}

func TestProfilePictures(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	upload := func(query string, body string, want int) []byte {
		return server.callWithHeaders(t, requestSpec{method: http.MethodPut, path: "/api/v1/users/me/avatar" + query, body: body,
			headers: map[string]string{"Authorization": "Bearer " + admin.Token, "Content-Type": "application/octet-stream"}}, want)
	}

	rejected := upload("", "not an image", http.StatusUnprocessableEntity)
	assert.Contains(t, string(rejected), `"code":"unsupported_image"`)
	upload("?cropX=90&cropY=0&cropSize=20", pngBytes(t, 100), http.StatusUnprocessableEntity)

	var user struct {
		AvatarURL string `json:"avatarUrl"`
	}
	require.NoError(t, json.Unmarshal(upload("?cropX=10&cropY=10&cropSize=50", pngBytes(t, 100), http.StatusOK), &user))
	require.True(t, strings.HasPrefix(user.AvatarURL, "/api/v1/avatars/"))

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.baseURL+user.AvatarURL, nil)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "image/png", response.Header.Get("Content-Type"))
	assert.Contains(t, response.Header.Get("Cache-Control"), "immutable")
	picture, err := png.DecodeConfig(response.Body)
	require.NoError(t, err)
	assert.Equal(t, 512, picture.Width)

	server.authorizedCall(t, admin.Token, http.MethodDelete, "/api/v1/users/me/avatar", http.StatusNoContent)
	server.call(t, http.MethodGet, user.AvatarURL, "", http.StatusNotFound)
}

type gatewayFrame struct {
	Type string          `json:"type"`
	Seq  uint64          `json:"seq"`
	Data json.RawMessage `json:"data"`
}

func (server authenticatedServer) connectGateway(t *testing.T) *websocket.Conn {
	t.Helper()
	address := "ws" + strings.TrimPrefix(server.baseURL, "http") + "/api/v1/gateway"
	conn, response, err := websocket.Dial(t.Context(), address, &websocket.DialOptions{Subprotocols: []string{"nexus-chat.v1"}})
	require.NoError(t, err)
	if response.Body != nil {
		_ = response.Body.Close()
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func readFrameOfType(t *testing.T, conn *websocket.Conn, wanted string) gatewayFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		_, message, err := conn.Read(ctx)
		require.NoError(t, err)
		var frame gatewayFrame
		require.NoError(t, json.Unmarshal(message, &frame))
		if frame.Type == wanted {
			return frame
		}
	}
}

func TestGatewayPresence(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	conn := server.connectGateway(t)
	readFrameOfType(t, conn, "hello")
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(`{"type":"identify","data":{"token":"`+admin.Token+`"}}`)))
	ready := readFrameOfType(t, conn, "ready")
	assert.Contains(t, string(ready.Data), `"username":"admin"`)

	var me struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK), &me))
	online := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/users/"+me.ID, http.StatusOK)
	assert.Contains(t, string(online), `"status":"online"`)

	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPut, path: "/api/v1/users/me/status", body: `{"status": "dnd"}`}, http.StatusOK)
	readFrameOfType(t, conn, "user_update")
	presence := readFrameOfType(t, conn, "presence_update")
	assert.Contains(t, string(presence.Data), `"status":"dnd"`)

	// Logging out disconnects the gateway at the next heartbeat.
	server.authorizedCall(t, admin.Token, http.MethodPost, "/api/v1/auth/logout", http.StatusNoContent)
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(`{"type":"heartbeat","data":{}}`)))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			assert.Equal(t, websocket.StatusCode(4004), websocket.CloseStatus(err))
			return
		}
	}
}
