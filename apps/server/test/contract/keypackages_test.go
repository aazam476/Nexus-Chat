package contract

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeKeyPackage has the header of an MLS key package for ciphersuite
// 0x0001; the server checks nothing else (ADR 0002).
func fakeKeyPackage(index byte) string {
	return base64.StdEncoding.EncodeToString([]byte{0, 1, 0, 5, 0, 1, 0, 1, index, 7, 7, 7})
}

func TestKeyPackageDirectory(t *testing.T) {
	server := setUpInstance(t)
	phone := server.login(t, "Phone")
	other := server.login(t, "Laptop")
	var me struct{ ID string }
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, phone.Token, http.MethodGet, "/api/v1/users/me", http.StatusOK), &me))

	body := fmt.Sprintf(`{"keyPackages": [%q, %q], "lastResort": %q}`, fakeKeyPackage(1), fakeKeyPackage(2), fakeKeyPackage(3))
	stored := server.authorizedCallWithBody(t, phone.Token, requestSpec{method: http.MethodPost, path: "/api/v1/key-packages", body: body}, http.StatusOK)
	assert.JSONEq(t, `{"oneTime": 2, "hasLastResort": true, "refill": true}`, string(stored))
	server.authorizedCallWithBody(t, phone.Token, requestSpec{
		method: http.MethodPost, path: "/api/v1/key-packages", body: `{"keyPackages": ["AAECAw=="]}`,
	}, http.StatusUnprocessableEntity)

	var claimed []struct {
		DeviceID   string `json:"deviceId"`
		KeyPackage string `json:"keyPackage"`
		LastResort bool   `json:"lastResort"`
	}
	claim := "/api/v1/users/" + me.ID + "/key-packages/claim"
	require.NoError(t, json.Unmarshal(server.authorizedCall(t, other.Token, http.MethodPost, claim, http.StatusOK), &claimed))
	require.Len(t, claimed, 1, "only the phone has key packages")
	assert.Equal(t, phone.Session.ID, claimed[0].DeviceID)
	assert.Equal(t, fakeKeyPackage(1), claimed[0].KeyPackage)

	stock := server.authorizedCall(t, phone.Token, http.MethodGet, "/api/v1/key-packages", http.StatusOK)
	assert.JSONEq(t, `{"oneTime": 1, "hasLastResort": true, "refill": true}`, string(stock))
}
