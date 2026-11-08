// Package passkeystest provides a software FIDO2 authenticator for tests.
// It produces the bytes a real authenticator would (W3C WebAuthn §6.1
// authenticator data, "none" attestation, ES256 signatures), so the real
// verification in go-webauthn runs against it.
package passkeystest

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

// Authenticator is one software security key or passkey.
type Authenticator struct {
	key          *ecdsa.PrivateKey
	credentialID []byte
	signCount    uint32
	// rpID is the domain the authenticator signs for.
	rpID string
	// UserHandle is sent with assertions when set, as a passkey does.
	UserHandle []byte
}

// New returns an authenticator with a fresh key for rpID.
func New(t testing.TB, rpID string) *Authenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	credentialID := make([]byte, 16)
	_, _ = rand.Read(credentialID)
	return &Authenticator{key: key, credentialID: credentialID, rpID: rpID}
}

// encode is the base64url form WebAuthn uses on the wire.
func encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// challengeFrom pulls publicKey.challenge out of the options the server sent.
func challengeFrom(t testing.TB, options []byte) string {
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

func clientData(t testing.TB, kind string, challenge string, origin string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]string{"type": kind, "challenge": challenge, "origin": origin})
	require.NoError(t, err)
	return data
}

// authenticatorData: RP ID hash, flags, counter, and (when registering)
// the attested credential data with the public key in COSE form.
func (authenticator *Authenticator) authenticatorData(t testing.TB, attested bool) []byte {
	t.Helper()
	rpHash := sha256.Sum256([]byte(authenticator.rpID))
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

// Register answers navigator.credentials.create() for options, as if the
// page were served from origin.
func (authenticator *Authenticator) Register(t testing.TB, options []byte, origin string) []byte {
	t.Helper()
	attestation, err := cbor.Marshal(map[string]any{
		"fmt": "none", "attStmt": map[string]any{}, "authData": authenticator.authenticatorData(t, true),
	})
	require.NoError(t, err)
	return authenticator.credentialJSON(t, map[string]any{
		"clientDataJSON":    encode(clientData(t, "webauthn.create", challengeFrom(t, options), origin)),
		"attestationObject": encode(attestation),
	})
}

// Assert answers navigator.credentials.get() for options from origin.
func (authenticator *Authenticator) Assert(t testing.TB, options []byte, origin string) []byte {
	t.Helper()
	data := authenticator.authenticatorData(t, false)
	client := clientData(t, "webauthn.get", challengeFrom(t, options), origin)
	clientHash := sha256.Sum256(client)
	digest := sha256.Sum256(append(append([]byte{}, data...), clientHash[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, authenticator.key, digest[:])
	require.NoError(t, err)
	response := map[string]any{
		"clientDataJSON": encode(client), "authenticatorData": encode(data), "signature": encode(signature),
	}
	if authenticator.UserHandle != nil {
		response["userHandle"] = encode(authenticator.UserHandle)
	}
	return authenticator.credentialJSON(t, response)
}

func (authenticator *Authenticator) credentialJSON(t testing.TB, response map[string]any) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"id": encode(authenticator.credentialID), "rawId": encode(authenticator.credentialID),
		"type": "public-key", "response": response,
	})
	require.NoError(t, err)
	return encoded
}
