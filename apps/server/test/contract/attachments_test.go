package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptedAttachments(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	carol := server.newUser(t, admin.Token, "carol")
	server.authorizedCallWithBody(t, bob.Token, requestSpec{method: http.MethodPut, path: "/api/v1/privacy", body: `{"whoCanDirectMessage": "everyone", "whoCanFriendRequest": "everyone"}`}, http.StatusNoContent)
	var dm struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/conversations/dms", body: `{"userId": "` + bob.ID + `"}`}, http.StatusCreated), &dm))
	base := "/api/v1/conversations/" + dm.ID + "/attachments"
	upload := func(token string, size int, body string, want int) []byte {
		return server.callWithHeaders(t, requestSpec{method: http.MethodPost, path: base + "?size=" + strconv.Itoa(size), body: body,
			headers: map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/octet-stream"}}, want)
	}

	var stored struct {
		ID   string `json:"id"`
		Size int    `json:"size"`
	}
	require.NoError(t, json.Unmarshal(upload(admin.Token, 10, "ciphertext", http.StatusCreated), &stored))
	assert.Equal(t, 10, stored.Size)
	assert.Contains(t, string(upload(carol.Token, 10, "ciphertext", http.StatusForbidden)), "not_allowed")
	assert.Contains(t, string(upload(admin.Token, 8, "ciphertext", http.StatusUnprocessableEntity)), "invalid_size")
	assert.Contains(t, string(upload(admin.Token, 30<<20, "x", http.StatusRequestEntityTooLarge)), "attachment_too_large")

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.baseURL+base+"/"+stored.ID, nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+bob.Token)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "ciphertext", string(content))
	assert.True(t, strings.HasPrefix(response.Header.Get("Cache-Control"), "private"))
	server.authorizedCall(t, carol.Token, http.MethodGet, base+"/"+stored.ID, http.StatusNotFound)
	server.authorizedCall(t, bob.Token, http.MethodGet, base+"/"+uuid.NewString(), http.StatusNotFound)
}

func TestTheAdminSetsTheAttachmentLimit(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	call := func(token string, method string, body string, want int) []byte {
		return server.authorizedCallWithBody(t, token, requestSpec{method: method, path: "/api/v1/admin/uploads", body: body}, want)
	}

	assert.JSONEq(t, `{"attachmentMaxBytes": 26214400}`, string(call(admin.Token, http.MethodGet, "", http.StatusOK)))
	assert.JSONEq(t, `{"attachmentMaxBytes": 104857600}`, string(call(admin.Token, http.MethodPut, `{"attachmentMaxBytes": 104857600}`, http.StatusOK)))
	call(admin.Token, http.MethodPut, `{"attachmentMaxBytes": 5}`, http.StatusUnprocessableEntity)
	call(bob.Token, http.MethodGet, "", http.StatusForbidden)
	assert.Contains(t, string(server.call(t, http.MethodGet, "/api/v1/instance", "", http.StatusOK)), `"attachmentMaxBytes":104857600`)
}
