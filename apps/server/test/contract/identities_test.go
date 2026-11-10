package contract

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signedIdentity is a published identity as a client makes it
// (packages/crypto/src/identity/identity.ts), in the API's JSON.
func signedIdentity(t *testing.T) string {
	t.Helper()
	masterPublic, masterPrivate, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	selfSigning, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	const label = "nexus-chat self-signing key v1"
	statement := binary.BigEndian.AppendUint32(nil, uint32(len(label)))
	statement = append(statement, label...)
	statement = binary.BigEndian.AppendUint32(statement, ed25519.PublicKeySize)
	statement = append(statement, selfSigning...)
	encode := base64.StdEncoding.EncodeToString
	return fmt.Sprintf(`{"masterPublicKey": %q, "selfSigningPublicKey": %q, "selfSigningSignature": %q}`,
		encode(masterPublic), encode(selfSigning), encode(ed25519.Sign(masterPrivate, statement)))
}

func TestPublishedIdentities(t *testing.T) {
	server := setUpInstance(t)
	phone := server.login(t, "Phone")
	other := server.login(t, "Laptop")
	var me struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, phone.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK), &me))
	path := "/api/v1/users/" + me.ID + "/identity"

	server.authorizedCall(t, other.Token, http.MethodGet, path, http.StatusNotFound)
	identity := signedIdentity(t)
	server.authorizedCallWithBody(t, phone.Token, requestSpec{method: http.MethodPut, path: "/api/v1/identity", body: identity}, http.StatusNoContent)
	assert.JSONEq(t, identity, string(server.authorizedCall(t, other.Token, http.MethodGet, path, http.StatusOK)))

	// A signature over another key is refused, and the published one stays.
	var forged map[string]string
	require.NoError(t, json.Unmarshal([]byte(signedIdentity(t)), &forged))
	var published map[string]string
	require.NoError(t, json.Unmarshal([]byte(identity), &published))
	forged["selfSigningSignature"] = published["selfSigningSignature"]
	body, err := json.Marshal(forged)
	require.NoError(t, err)
	refused := server.authorizedCallWithBody(t, phone.Token, requestSpec{method: http.MethodPut, path: "/api/v1/identity", body: string(body)}, http.StatusUnprocessableEntity)
	assert.Contains(t, string(refused), "invalid_signature")
	assert.JSONEq(t, identity, string(server.authorizedCall(t, other.Token, http.MethodGet, path, http.StatusOK)))

	server.authorizedCall(t, other.Token, http.MethodGet, "/api/v1/users/"+uuid.NewString()+"/identity", http.StatusNotFound)
	server.call(t, http.MethodGet, path, "", http.StatusUnauthorized)
}
