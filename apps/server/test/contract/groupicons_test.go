package contract

import (
	"encoding/json"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupIcons(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	carol := server.newUser(t, admin.Token, "carol")
	dave := server.newUser(t, admin.Token, "dave")
	for _, user := range []account{bob, carol} {
		server.authorizedCallWithBody(t, user.Token, requestSpec{method: http.MethodPut, path: "/api/v1/privacy", body: `{"whoCanDirectMessage": "everyone", "whoCanFriendRequest": "everyone"}`}, http.StatusNoContent)
	}
	var group struct{ ID string }
	created := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/conversations/groups", body: `{"memberIds": ["` + bob.ID + `", "` + carol.ID + `"]}`}, http.StatusCreated)
	require.NoError(t, json.Unmarshal(created, &group))
	var dm struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/conversations/dms", body: `{"userId": "` + bob.ID + `"}`}, http.StatusCreated), &dm))
	upload := func(token string, conversationID string, body string, want int) []byte {
		return server.callWithHeaders(t, requestSpec{method: http.MethodPut, path: "/api/v1/conversations/" + conversationID + "/icon", body: body,
			headers: map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/octet-stream"}}, want)
	}

	assert.Contains(t, string(upload(bob.Token, group.ID, "not an image", http.StatusUnprocessableEntity)), "unsupported_image")
	assert.Contains(t, string(upload(bob.Token, dm.ID, pngBytes(t, 64), http.StatusUnprocessableEntity)), "not_a_group")
	upload(dave.Token, group.ID, pngBytes(t, 64), http.StatusNotFound)
	upload(bob.Token, group.ID, strings.Repeat("x", 8<<20+1), http.StatusRequestEntityTooLarge)

	var withIcon struct {
		IconURL string `json:"iconUrl"`
	}
	require.NoError(t, json.Unmarshal(upload(bob.Token, group.ID, pngBytes(t, 64), http.StatusOK), &withIcon))
	require.True(t, strings.HasPrefix(withIcon.IconURL, "/api/v1/conversation-icons/"))
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.baseURL+withIcon.IconURL, nil)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	assert.Equal(t, http.StatusOK, response.StatusCode, "public, for image tags")
	assert.Contains(t, response.Header.Get("Cache-Control"), "immutable")
	picture, err := png.DecodeConfig(response.Body)
	require.NoError(t, err)
	assert.Equal(t, 512, picture.Width)

	remove := func(token string, conversationID string, want int) []byte {
		return server.authorizedCall(t, token, http.MethodDelete, "/api/v1/conversations/"+conversationID+"/icon", want)
	}
	remove(dave.Token, group.ID, http.StatusNotFound)
	remove(bob.Token, dm.ID, http.StatusUnprocessableEntity)
	assert.NotContains(t, string(remove(carol.Token, group.ID, http.StatusOK)), "iconUrl")
	server.call(t, http.MethodGet, withIcon.IconURL, "", http.StatusNotFound)
	server.call(t, http.MethodGet, "/api/v1/conversation-icons/"+uuid.NewString(), "", http.StatusNotFound)
}
