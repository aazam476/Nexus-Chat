package contract

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMessagesTravelInTheirAudienceGroup(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	carol := server.newUser(t, admin.Token, "carol")
	call := func(token string, method string, path string, body string, status int) []byte {
		return server.authorizedCallWithBody(t, token, requestSpec{method: method, path: path, body: body}, status)
	}
	var created struct {
		Server struct {
			ID string `json:"id"`
		} `json:"server"`
		Roles []struct {
			ID string `json:"id"`
		} `json:"roles"`
	}
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, "/api/v1/servers", `{"name": "Book club"}`, http.StatusCreated), &created))
	base := "/api/v1/servers/" + created.Server.ID
	var invite struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodPost, base+"/invites", `{}`, http.StatusCreated), &invite))
	call(carol.Token, http.MethodPost, "/api/v1/invites/"+invite.Code+"/join", "", http.StatusOK)

	var details struct {
		Channels []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			AudienceID string `json:"audienceId"`
			MlsGroupID []byte `json:"mlsGroupId"`
		} `json:"channels"`
	}
	require.NoError(t, json.Unmarshal(call(carol.Token, http.MethodGet, base, "", http.StatusOK), &details))
	general := details.Channels[1]
	require.Equal(t, "general", general.Name)
	require.NotEmpty(t, general.AudienceID)
	group := "/api/v1/conversations/" + general.AudienceID

	// Carol joins the audience's group with an external commit, then Bob
	// writes in the channel. The audience is not a conversation to them.
	groupInfo := base64.StdEncoding.EncodeToString([]byte{0, 1, 0, 4, 1})
	call(carol.Token, http.MethodGet, group+"/mls", "", http.StatusOK)
	call(carol.Token, http.MethodPost, group+"/mls/commits", `{"commit": "`+mlsPrivateMessage(general.MlsGroupID, 0, 3)+`", "groupInfo": "`+groupInfo+`"}`, http.StatusOK)
	messageID := uuid.NewString()
	send := `{"message": "` + mlsPrivateMessage(general.MlsGroupID, 1, 1) + `", "archive": {"messageId": "` + messageID + `", "sealed": "c2VhbGVk"}}`
	call(bob.Token, http.MethodPost, base+"/channels/"+general.ID+"/messages", send, http.StatusCreated)
	call(bob.Token, http.MethodPost, group+"/mls/messages", `{"message": "`+mlsPrivateMessage(general.MlsGroupID, 1, 1)+`"}`, http.StatusForbidden)
	call(admin.Token, http.MethodPost, base+"/channels/"+general.ID+"/messages", send, http.StatusNotFound)
	call(carol.Token, http.MethodGet, group, "", http.StatusNotFound)
	assert.JSONEq(t, `[]`, string(call(carol.Token, http.MethodGet, "/api/v1/conversations", "", http.StatusOK)))

	var messages []struct {
		Kind      string `json:"kind"`
		ChannelID string `json:"channelId"`
	}
	require.NoError(t, json.Unmarshal(call(carol.Token, http.MethodGet, group+"/mls/messages", "", http.StatusOK), &messages))
	require.Len(t, messages, 2)
	assert.Equal(t, general.ID, messages[1].ChannelID)
	archive := call(carol.Token, http.MethodGet, base+"/channels/"+general.ID+"/archive", "", http.StatusOK)
	assert.Contains(t, string(archive), messageID)
	assert.Contains(t, string(archive), base64.StdEncoding.EncodeToString(general.MlsGroupID))

	// Hidden from Carol: she can no longer reach the channel, and the
	// group carries a directive for her devices' removal.
	call(bob.Token, http.MethodPut, base+"/channels/"+general.ID+"/overrides/member/"+carol.ID, `{"allow": [], "deny": ["VIEW_CHANNEL"]}`, http.StatusNoContent)
	call(carol.Token, http.MethodGet, base+"/channels/"+general.ID+"/archive", "", http.StatusNotFound)
	call(carol.Token, http.MethodGet, group+"/mls/messages", "", http.StatusNotFound)
	require.NoError(t, json.Unmarshal(call(bob.Token, http.MethodGet, group+"/mls/messages", "", http.StatusOK), &messages))
	assert.Equal(t, "directive", messages[len(messages)-1].Kind)
}
