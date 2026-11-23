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

func TestServerIcons(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	var created struct {
		Server struct {
			ID string `json:"id"`
		} `json:"server"`
	}
	require.NoError(t, json.Unmarshal(server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/servers", body: `{"name": "Club"}`}, http.StatusCreated), &created))
	base := "/api/v1/servers/" + created.Server.ID
	upload := func(token string, body string, want int) []byte {
		return server.callWithHeaders(t, requestSpec{method: http.MethodPut, path: base + "/icon", body: body,
			headers: map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/octet-stream"}}, want)
	}

	assert.Contains(t, string(upload(admin.Token, "not an image", http.StatusUnprocessableEntity)), "unsupported_image")
	upload(bob.Token, pngBytes(t, 64), http.StatusNotFound)
	upload(admin.Token, strings.Repeat("x", 8<<20+1), http.StatusRequestEntityTooLarge)
	upload(admin.Token, pngBytes(t, 64), http.StatusNoContent)

	var details struct {
		Server struct {
			IconURL string `json:"iconUrl"`
		} `json:"server"`
	}
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, admin.Token, http.MethodGet, base, http.StatusOK), &details))
	require.True(t, strings.HasPrefix(details.Server.IconURL, "/api/v1/server-icons/"))
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.baseURL+details.Server.IconURL, nil)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	assert.Equal(t, http.StatusOK, response.StatusCode, "public, for image tags and invite previews")
	assert.Contains(t, response.Header.Get("Cache-Control"), "immutable")
	picture, err := png.DecodeConfig(response.Body)
	require.NoError(t, err)
	assert.Equal(t, 512, picture.Width)

	server.authorizedCall(t, bob.Token, http.MethodDelete, base+"/icon", http.StatusNotFound)
	server.authorizedCall(t, admin.Token, http.MethodDelete, base+"/icon", http.StatusNoContent)
	server.call(t, http.MethodGet, details.Server.IconURL, "", http.StatusNotFound)
	server.call(t, http.MethodGet, "/api/v1/server-icons/"+uuid.NewString(), "", http.StatusNotFound)
}
