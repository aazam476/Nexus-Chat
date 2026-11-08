package secrets

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allFiles = []string{
	tokenSigningKeyFile, federationKeyFile, pushRelayKeyFile, vapidKeyFile,
	liveKitAPIKeyFile, liveKitSecretFile, turnSecretFile, totpKeyFile, settingsKeyFile,
}

func TestLoadOrCreateGeneratesEverySecretOnFirstBoot(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")

	secrets, err := LoadOrCreate(directory)

	require.NoError(t, err)
	assert.Len(t, secrets.TokenSigningKey, ed25519.PrivateKeySize)
	assert.Len(t, secrets.FederationKey, ed25519.PrivateKeySize)
	assert.Len(t, secrets.PushRelayKey, ed25519.PrivateKeySize)
	assert.Equal(t, elliptic.P256(), secrets.VAPIDKey.Curve)
	assert.Regexp(t, `^API[A-Z2-7]{12}$`, secrets.LiveKitAPIKey)
	assert.Regexp(t, `^[A-Z2-7]{52}$`, secrets.LiveKitAPISecret)
	assert.Regexp(t, `^[A-Z2-7]{52}$`, secrets.TURNSecret)
	assert.Len(t, secrets.TOTPKey, 32)
	assert.Len(t, secrets.SettingsKey, 32)
	assert.NotEqual(t, secrets.TOTPKey, secrets.SettingsKey)
	// Independent keys must never be the same key reused.
	assert.NotEqual(t, secrets.TokenSigningKey, secrets.FederationKey)
	assert.NotEqual(t, secrets.LiveKitAPISecret, secrets.TURNSecret)
}

func TestLoadOrCreateWritesOwnerOnlyFiles(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")

	_, err := LoadOrCreate(directory)
	require.NoError(t, err)

	assertMode(t, directory, 0o700)
	for _, name := range allFiles {
		assertMode(t, filepath.Join(directory, name), 0o600)
	}
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	assert.Len(t, entries, len(allFiles), "no temporary files may be left behind")
}

func TestLoadOrCreateReturnsTheSameSecretsOnLaterBoots(t *testing.T) {
	directory := t.TempDir()
	first, err := LoadOrCreate(directory)
	require.NoError(t, err)

	second, err := LoadOrCreate(directory)

	require.NoError(t, err)
	assert.Equal(t, first.TokenSigningKey, second.TokenSigningKey)
	assert.Equal(t, first.FederationKey, second.FederationKey)
	assert.Equal(t, first.PushRelayKey, second.PushRelayKey)
	assert.True(t, first.VAPIDKey.Equal(second.VAPIDKey))
	assert.Equal(t, first.LiveKitAPIKey, second.LiveKitAPIKey)
	assert.Equal(t, first.LiveKitAPISecret, second.LiveKitAPISecret)
	assert.Equal(t, first.TURNSecret, second.TURNSecret)
	assert.Equal(t, first.TOTPKey, second.TOTPKey)
	assert.Equal(t, first.SettingsKey, second.SettingsKey)
}

func TestLoadOrCreateGeneratesOnlyMissingSecrets(t *testing.T) {
	directory := t.TempDir()
	first, err := LoadOrCreate(directory)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(directory, turnSecretFile)))

	second, err := LoadOrCreate(directory)

	require.NoError(t, err)
	assert.Equal(t, first.FederationKey, second.FederationKey)
	assert.NotEqual(t, first.TURNSecret, second.TURNSecret)
}

func TestLoadOrCreateTightensLoosePermissions(t *testing.T) {
	directory := t.TempDir()
	_, err := LoadOrCreate(directory)
	require.NoError(t, err)
	path := filepath.Join(directory, federationKeyFile)
	require.NoError(t, os.Chmod(path, 0o644))
	require.NoError(t, os.Chmod(directory, 0o755))

	_, err = LoadOrCreate(directory)

	require.NoError(t, err)
	assertMode(t, path, 0o600)
	assertMode(t, directory, 0o700)
}

func TestLoadOrCreateRejectsDamagedFiles(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		contents  []byte
		wantError string
	}{
		{"not pem", federationKeyFile, []byte("garbage"), "not a PEM"},
		{"wrong pem label", federationKeyFile, pemBlock("RSA PRIVATE KEY", []byte{1}), "not a PEM"},
		{"invalid pkcs8", federationKeyFile, pemBlock(pemBlockType, []byte{1, 2, 3}), "parsing " + federationKeyFile},
		{"p256 key where ed25519 expected", tokenSigningKeyFile, p256PEM(t), "does not contain an Ed25519 key"},
		{"ed25519 key where p256 expected", vapidKeyFile, ed25519PEM(t), "does not contain a P-256 key"},
		{"empty token", turnSecretFile, []byte("  \n"), "is empty"},
		{"key not base64", totpKeyFile, []byte("!!!"), "base64-encoded 32-byte key"},
		{"key too short", totpKeyFile, []byte("c2hvcnQ="), "base64-encoded 32-byte key"},
		{"key file empty", totpKeyFile, []byte("\n"), "is empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(directory, test.file), test.contents, 0o600))

			_, err := LoadOrCreate(directory)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestLoadOrCreateRejectsWrongCurve(t *testing.T) {
	directory := t.TempDir()
	der := marshalPKCS8(t, mustGenerateECDSA(t, elliptic.P384()))
	require.NoError(t, os.WriteFile(filepath.Join(directory, vapidKeyFile), pemBlock(pemBlockType, der), 0o600))

	_, err := LoadOrCreate(directory)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain a P-256 key")
}

func TestLoadOrCreateFailsWhenDirectoryCannotBeCreated(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	_, err := LoadOrCreate(filepath.Join(blocker, "secrets"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating secrets directory")
}

func TestLoadOrCreateFailsWhenSecretPathIsADirectory(t *testing.T) {
	for _, name := range []string{federationKeyFile, turnSecretFile} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(directory, name), 0o700))

			_, err := LoadOrCreate(directory)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "reading "+name)
		})
	}
}

func TestDirectoryIn(t *testing.T) {
	assert.Equal(t, filepath.Join("/data", "secrets"), DirectoryIn("/data"))
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm(), path)
}

func pemBlock(label string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: label, Bytes: der})
}

func p256PEM(t *testing.T) []byte {
	return pemBlock(pemBlockType, marshalPKCS8(t, mustGenerateECDSA(t, elliptic.P256())))
}

func ed25519PEM(t *testing.T) []byte {
	_, key, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return pemBlock(pemBlockType, marshalPKCS8(t, key))
}

func marshalPKCS8(t *testing.T, key any) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return der
}

// mustGenerateECDSA is a small helper so table entries stay one line long.
func mustGenerateECDSA(t *testing.T, curve elliptic.Curve) any {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	return key
}

// The RSA import keeps the "unsupported type" case honest: an RSA key is a
// valid PKCS #8 key that is still the wrong kind for every secret file.
func TestLoadOrCreateRejectsRSAKey(t *testing.T) {
	directory := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, tokenSigningKeyFile), pemBlock(pemBlockType, marshalPKCS8(t, key)), 0o600))

	_, err = LoadOrCreate(directory)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain an Ed25519 key")
}
