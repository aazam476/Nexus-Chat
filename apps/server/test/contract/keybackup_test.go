package contract

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyBackup(t *testing.T) {
	server := setUpInstance(t)
	phone := server.login(t, "Phone")
	laptop := server.login(t, "Laptop")
	encode := base64.StdEncoding.EncodeToString

	var backup struct {
		Salt        string
		WrappedKeys []struct{ Method, WrappedKey string }
		Blobs       []struct {
			Name string
			Size int
		}
	}
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, phone.Token, http.MethodGet, "/api/v1/key-backup", http.StatusOK), &backup))
	salt, err := base64.StdEncoding.DecodeString(backup.Salt)
	require.NoError(t, err)
	assert.Len(t, salt, 16)
	assert.Empty(t, backup.WrappedKeys)

	// Fresh logins count as recent, so turning on a method works straight away.
	put := func(path, body string, status int) []byte {
		return server.authorizedCallWithBody(t, phone.Token, requestSpec{method: http.MethodPut, path: path, body: body}, status)
	}
	put("/api/v1/key-backup/wrapped-keys/recovery-phrase", `{"wrappedKey": "`+encode([]byte("wrapped"))+`"}`, http.StatusNoContent)
	put("/api/v1/key-backup/wrapped-keys/email", `{"wrappedKey": "AQ=="}`, http.StatusBadRequest)
	put("/api/v1/key-backup/blobs/identity", `{"data": "`+encode([]byte("sealed identity"))+`"}`, http.StatusNoContent)
	put("/api/v1/key-backup/blobs/Bad.Name", `{"data": "AQ=="}`, http.StatusBadRequest)
	put("/api/v1/key-backup/blobs/empty", `{"data": ""}`, http.StatusUnprocessableEntity)

	// Another device of the same user sees the same salt and items.
	var seen struct {
		Salt        string
		WrappedKeys []struct{ Method string }
		Blobs       []struct {
			Name string
			Size int
		}
	}
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, laptop.Token, http.MethodGet, "/api/v1/key-backup", http.StatusOK), &seen))
	assert.Equal(t, backup.Salt, seen.Salt)
	require.Len(t, seen.WrappedKeys, 1)
	assert.Equal(t, "recovery-phrase", seen.WrappedKeys[0].Method)
	require.Len(t, seen.Blobs, 1)
	assert.Equal(t, "identity", seen.Blobs[0].Name)
	assert.Equal(t, len("sealed identity"), seen.Blobs[0].Size)
	blob := server.authorizedCall(t, laptop.Token, http.MethodGet, "/api/v1/key-backup/blobs/identity", http.StatusOK)
	assert.JSONEq(t, `{"data": "`+encode([]byte("sealed identity"))+`"}`, string(blob))

	server.authorizedCall(t, laptop.Token, http.MethodDelete, "/api/v1/key-backup/blobs/identity", http.StatusNoContent)
	server.authorizedCall(t, laptop.Token, http.MethodGet, "/api/v1/key-backup/blobs/identity", http.StatusNotFound)
	server.authorizedCall(t, laptop.Token, http.MethodDelete, "/api/v1/key-backup/wrapped-keys/recovery-phrase", http.StatusNoContent)
	server.authorizedCall(t, laptop.Token, http.MethodDelete, "/api/v1/key-backup/wrapped-keys/recovery-phrase", http.StatusNotFound)
	server.call(t, http.MethodGet, "/api/v1/key-backup", "", http.StatusUnauthorized)
}
