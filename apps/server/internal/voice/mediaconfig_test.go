package voice

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecrets = MediaSecrets{LiveKitAPIKey: "APIKEY123", LiveKitAPISecret: "SECRET456", TURNSecret: "TURN789"}

func TestWriteMediaConfig(t *testing.T) {
	directory := t.TempDir()

	require.NoError(t, WriteMediaConfig(directory, testSecrets))

	liveKit, err := os.ReadFile(filepath.Join(directory, "livekit", "livekit.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(liveKit), "  APIKEY123: SECRET456\n")
	assert.Contains(t, string(liveKit), "udp_port: 7882")
	coturn, err := os.ReadFile(filepath.Join(directory, "coturn", "turnserver.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(coturn), "static-auth-secret=TURN789\n")
	assert.Contains(t, string(coturn), "denied-peer-ip=10.0.0.0-10.255.255.255")
	assert.Contains(t, string(coturn), "listening-port=3479\n")
	for _, name := range []string{"livekit/livekit.yaml", "coturn/turnserver.conf"} {
		info, err := os.Stat(filepath.Join(directory, name))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), name)
	}
}

func TestWriteMediaConfigOverwritesOldFiles(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, WriteMediaConfig(directory, testSecrets))
	changed := testSecrets
	changed.TURNSecret = "NEWSECRET"

	require.NoError(t, WriteMediaConfig(directory, changed))

	coturn, err := os.ReadFile(filepath.Join(directory, "coturn", "turnserver.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(coturn), "static-auth-secret=NEWSECRET\n")
}

func TestWriteMediaConfigRejectsUnsafeSecrets(t *testing.T) {
	for name, secrets := range map[string]MediaSecrets{
		"empty key":         {LiveKitAPISecret: "a", TURNSecret: "b"},
		"newline in secret": {LiveKitAPIKey: "a", LiveKitAPISecret: "x\nkeys:", TURNSecret: "b"},
		"colon in key":      {LiveKitAPIKey: "a:b", LiveKitAPISecret: "c", TURNSecret: "d"},
		"space in turn":     {LiveKitAPIKey: "a", LiveKitAPISecret: "c", TURNSecret: "d e"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, WriteMediaConfig(t.TempDir(), secrets))
		})
	}
}

func TestWriteMediaConfigReportsUnwritableDirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))

	assert.ErrorContains(t, WriteMediaConfig(blocker, testSecrets), "creating")
}

func TestWriteMediaConfigReportsUnwritableFile(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(directory, "livekit", "livekit.yaml"), 0o700))

	assert.ErrorContains(t, WriteMediaConfig(directory, testSecrets), "writing")
}
