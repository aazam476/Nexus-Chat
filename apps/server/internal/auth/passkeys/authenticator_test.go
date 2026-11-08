package passkeys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// softwareAuthenticator is a minimal FIDO2 authenticator for tests. It
// produces the same bytes a real one would (W3C WebAuthn §6.1 authenticator
// data, "none" attestation, ES256 signatures), so the library's real
// verification runs against it.
type softwareAuthenticator struct {
	key          *ecdsa.PrivateKey
	credentialID []byte
	signCount    uint32
	userHandle   []byte
}

const testOrigin = "https://chat.example.com"
const testRPID = "chat.example.com"

func newSoftwareAuthenticator(t *testing.T) *softwareAuthenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	credentialID := make([]byte, 16)
	_, _ = rand.Read(credentialID)
	return &softwareAuthenticator{key: key, credentialID: credentialID}
}

var encode = base64.RawURLEncoding.EncodeToString

// challengeFrom pulls publicKey.challenge out of the options the server sent.
func challengeFrom(t *testing.T, options []byte) string {
	t.Helper()
	var decoded struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
		} `json:"publicKey"`
	}
	require.NoError(t, json.Unmarshal(options, &decoded))
	require.NotEmpty(t, decoded.PublicKey.Challenge)
	return decoded.PublicKey.Challenge
}

func clientData(t *testing.T, kind string, challenge string, origin string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]string{"type": kind, "challenge": challenge, "origin": origin})
	require.NoError(t, err)
	return data
}

// authenticatorData: RP ID hash, flags, counter, and (when registering)
// the attested credential data with the public key in COSE form.
func (authenticator *softwareAuthenticator) authenticatorData(t *testing.T, rpID string, attested bool) []byte {
	t.Helper()
	rpHash := sha256.Sum256([]byte(rpID))
	flags := byte(0x01 | 0x04) // user present, user verified
	if attested {
		flags |= 0x40
	}
	authenticator.signCount++
	data := append(append([]byte{}, rpHash[:]...), flags)
	data = binary.BigEndian.AppendUint32(data, authenticator.signCount)
	if !attested {
		return data
	}
	data = append(data, make([]byte, 16)...)                                            // AAGUID: none
	data = binary.BigEndian.AppendUint16(data, uint16(len(authenticator.credentialID))) //nolint:gosec // the ID is always 16 bytes
	data = append(data, authenticator.credentialID...)
	publicKey, err := cbor.Marshal(map[int]any{
		1: 2, 3: -7, -1: 1, // kty EC2, alg ES256, crv P-256
		-2: authenticator.key.X.FillBytes(make([]byte, 32)),
		-3: authenticator.key.Y.FillBytes(make([]byte, 32)),
	})
	require.NoError(t, err)
	return append(data, publicKey...)
}

// register answers navigator.credentials.create().
func (authenticator *softwareAuthenticator) register(t *testing.T, options []byte, origin string) []byte {
	t.Helper()
	attestation, err := cbor.Marshal(map[string]any{
		"fmt": "none", "attStmt": map[string]any{}, "authData": authenticator.authenticatorData(t, testRPID, true),
	})
	require.NoError(t, err)
	return authenticator.credentialJSON(t, map[string]any{
		"clientDataJSON":    encode(clientData(t, "webauthn.create", challengeFrom(t, options), origin)),
		"attestationObject": encode(attestation),
	})
}

// assert answers navigator.credentials.get().
func (authenticator *softwareAuthenticator) assert(t *testing.T, options []byte) []byte {
	t.Helper()
	data := authenticator.authenticatorData(t, testRPID, false)
	client := clientData(t, "webauthn.get", challengeFrom(t, options), testOrigin)
	clientHash := sha256.Sum256(client)
	digest := sha256.Sum256(append(append([]byte{}, data...), clientHash[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, authenticator.key, digest[:])
	require.NoError(t, err)
	response := map[string]any{
		"clientDataJSON": encode(client), "authenticatorData": encode(data), "signature": encode(signature),
	}
	if authenticator.userHandle != nil {
		response["userHandle"] = encode(authenticator.userHandle)
	}
	return authenticator.credentialJSON(t, response)
}

func (authenticator *softwareAuthenticator) credentialJSON(t *testing.T, response map[string]any) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"id": encode(authenticator.credentialID), "rawId": encode(authenticator.credentialID),
		"type": "public-key", "response": response,
	})
	require.NoError(t, err)
	return encoded
}
