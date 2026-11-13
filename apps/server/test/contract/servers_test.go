package contract

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serverDetails struct {
	Server struct {
		ID string `json:"id"`
	} `json:"server"`
	Roles []struct {
		ID       string `json:"id"`
		Everyone bool   `json:"everyone"`
	} `json:"roles"`
	Channels []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"channels"`
	Permissions []string `json:"permissions"`
}

func TestServersChannelsRolesAndInvites(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	carol := server.newUser(t, admin.Token, "carol")
	call := func(token string, method string, path string, body string, status int) []byte {
		return server.authorizedCallWithBody(t, token, requestSpec{method: method, path: path, body: body}, status)
	}
	// Carol only takes DMs from friends and people she shares a server with.
	call(bob.Token, http.MethodPost, "/api/v1/conversations/dms", `{"userId": "`+carol.ID+`"}`, http.StatusForbidden)

	var details serverDetails
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, "/api/v1/servers", `{"name": "Book club", "description": "We read."}`, http.StatusCreated), &details))
	call(bob.Token, http.MethodPost, "/api/v1/servers", `{"name": ""}`, http.StatusUnprocessableEntity)
	require.Len(t, details.Channels, 4)
	base := "/api/v1/servers/" + details.Server.ID
	call(carol.Token, http.MethodGet, base, "", http.StatusNotFound)

	var invite struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, base+"/invites", `{"maxUses": 5, "expiresInSeconds": 3600}`, http.StatusCreated), &invite))
	call(bob.Token, http.MethodPost, base+"/invites", `{"maxUses": 0}`, http.StatusUnprocessableEntity)
	preview := call(carol.Token, http.MethodGet, "/api/v1/invites/"+invite.Code, "", http.StatusOK)
	assert.Contains(t, string(preview), `"members":1`)
	call(carol.Token, http.MethodGet, "/api/v1/invites/nope", "", http.StatusNotFound)
	call(carol.Token, http.MethodPost, "/api/v1/invites/nope/join", "", http.StatusNotFound)
	joined := call(carol.Token, http.MethodPost, "/api/v1/invites/"+invite.Code+"/join", "", http.StatusOK)
	assert.Contains(t, string(joined), details.Server.ID)
	call(bob.Token, http.MethodPost, "/api/v1/conversations/dms", `{"userId": "`+carol.ID+`"}`, http.StatusCreated)

	// Carol is a plain member: she sees the server but cannot manage it.
	call(carol.Token, http.MethodGet, base, "", http.StatusOK)
	call(carol.Token, http.MethodPost, base+"/channels", `{"kind": "text", "name": "nope"}`, http.StatusForbidden)
	call(carol.Token, http.MethodGet, base+"/invites", "", http.StatusForbidden)
	members := call(carol.Token, http.MethodGet, base+"/members", "", http.StatusOK)
	assert.Contains(t, string(members), `"username":"carol"`)

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, base+"/channels", `{"kind": "text", "name": "secret", "topic": "Shh"}`, http.StatusCreated), &created))
	staffChannel := created.ID
	call(bob.Token, http.MethodPatch, base+"/channels/"+created.ID, `{"name": "staff", "topic": "Staff only"}`, http.StatusNoContent)
	call(bob.Token, http.MethodPatch, base+"/channels/"+details.Server.ID, `{"name": "x"}`, http.StatusNotFound)
	everyone := details.Roles[0].ID
	call(bob.Token, http.MethodPut, base+"/channels/"+created.ID+"/overrides/role/"+everyone, `{"allow": [], "deny": ["VIEW_CHANNEL"]}`, http.StatusNoContent)
	assert.NotContains(t, string(call(carol.Token, http.MethodGet, base, "", http.StatusOK)), "staff")

	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, base+"/roles", `{"name": "Staff", "color": 3381759, "permissions": ["MANAGE_CHANNELS"]}`, http.StatusCreated), &created))
	staff := created.ID
	call(bob.Token, http.MethodPut, base+"/members/"+carol.ID+"/roles/"+staff, "", http.StatusNoContent)
	call(carol.Token, http.MethodPatch, base+"/roles/"+staff, `{"name": "Bosses", "permissions": []}`, http.StatusForbidden)
	call(bob.Token, http.MethodPatch, base+"/roles/"+staff, `{"name": "Staff", "permissions": ["MANAGE_CHANNELS", "CREATE_INVITES"]}`, http.StatusNoContent)
	call(bob.Token, http.MethodPut, base+"/role-order", `{"roleIds": ["`+staff+`"]}`, http.StatusNoContent)
	call(bob.Token, http.MethodPut, base+"/role-order", `{"roleIds": []}`, http.StatusUnprocessableEntity)

	// Staff may now manage channels: Carol changes and deletes the ones she sees.
	var seen serverDetails
	require.NoError(t, json.Unmarshal(call(carol.Token, http.MethodGet, base, "", http.StatusOK), &seen))
	assert.Contains(t, seen.Permissions, "MANAGE_CHANNELS")
	order := "["
	for i, channel := range seen.Channels {
		if i > 0 {
			order += ","
		}
		order += `{"id": "` + channel.ID + `"}`
	}
	// Rearranging needs seeing every channel, and "staff" is hidden from her.
	call(carol.Token, http.MethodPut, base+"/channel-order", order+"]", http.StatusForbidden)
	call(bob.Token, http.MethodPut, base+"/channel-order", order+"]", http.StatusUnprocessableEntity)
	order += `,{"id": "` + staffChannel + `"}]`
	call(bob.Token, http.MethodPut, base+"/channel-order", order, http.StatusNoContent)
	call(carol.Token, http.MethodDelete, base+"/channels/"+seen.Channels[0].ID+"/overrides/member/"+carol.ID, "", http.StatusNoContent)
	call(carol.Token, http.MethodDelete, base+"/channels/"+seen.Channels[len(seen.Channels)-1].ID, "", http.StatusNoContent)

	call(bob.Token, http.MethodDelete, base+"/members/"+carol.ID+"/roles/"+staff, "", http.StatusNoContent)
	call(bob.Token, http.MethodDelete, base+"/roles/"+staff, "", http.StatusNoContent)
	invites := call(bob.Token, http.MethodGet, base+"/invites", "", http.StatusOK)
	assert.Contains(t, string(invites), invite.Code)
	call(bob.Token, http.MethodDelete, base+"/invites/"+invite.Code, "", http.StatusNoContent)
	call(bob.Token, http.MethodDelete, base+"/invites/"+invite.Code, "", http.StatusNotFound)

	call(bob.Token, http.MethodPatch, base, `{"name": "Readers", "description": "", "compressMedia": true, "attachmentMaxBytes": 1048576}`, http.StatusNoContent)
	call(carol.Token, http.MethodPatch, base, `{"name": "Mine", "description": "", "compressMedia": false}`, http.StatusForbidden)
	list := call(carol.Token, http.MethodGet, "/api/v1/servers", "", http.StatusOK)
	assert.Contains(t, string(list), "Readers")
	call(bob.Token, http.MethodDelete, base+"/members/me", "", http.StatusUnprocessableEntity)
	call(carol.Token, http.MethodDelete, base+"/members/me", "", http.StatusNoContent)
	call(carol.Token, http.MethodDelete, base, "", http.StatusNotFound)
	call(bob.Token, http.MethodDelete, base, "", http.StatusNoContent)
	call(bob.Token, http.MethodGet, base, "", http.StatusNotFound)
}
