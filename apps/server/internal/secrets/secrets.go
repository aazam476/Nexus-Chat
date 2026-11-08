package secrets

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
)

// File names inside the secrets directory. They are part of the on-disk
// format: renaming one would make an upgraded server generate a new key.
//
//nolint:gosec // G101: these are file names, not credentials.
const (
	tokenSigningKeyFile = "token-signing.ed25519.pem"
	federationKeyFile   = "federation.ed25519.pem"
	pushRelayKeyFile    = "push-relay.ed25519.pem"
	vapidKeyFile        = "vapid.p256.pem"
	liveKitAPIKeyFile   = "livekit-api-key.txt"
	liveKitSecretFile   = "livekit-api-secret.txt"
	turnSecretFile      = "turn-secret.txt"
	totpKeyFile         = "totp-encryption.key"
)

// Secrets holds every key the instance generates for itself.
type Secrets struct {
	// TokenSigningKey signs the server's session and access tokens.
	TokenSigningKey ed25519.PrivateKey
	// FederationKey signs server-to-server requests (RFC 9421) and is
	// published in /.well-known/nexus-chat.
	FederationKey ed25519.PrivateKey
	// PushRelayKey identifies this instance to the push relay.
	PushRelayKey ed25519.PrivateKey
	// VAPIDKey signs Web Push requests (RFC 8292 requires P-256).
	VAPIDKey *ecdsa.PrivateKey
	// LiveKitAPIKey and LiveKitAPISecret are shared with the LiveKit
	// container so the server can mint room tokens.
	LiveKitAPIKey    string
	LiveKitAPISecret string
	// TURNSecret is the shared secret for time-limited TURN credentials.
	TURNSecret string
	// TOTPKey encrypts TOTP secrets stored in the database (AES-256-GCM).
	TOTPKey []byte
}

// LoadOrCreate returns the instance's secrets from directory, generating
// and saving any that are missing. Key material always comes from the
// operating system's secure random source (crypto/rand).
func LoadOrCreate(directory string) (Secrets, error) {
	if err := ensurePrivateDirectory(directory); err != nil {
		return Secrets{}, err
	}
	store := fileStore{directory: directory}
	var secrets Secrets
	var err error

	if secrets.TokenSigningKey, err = store.ed25519Key(tokenSigningKeyFile); err != nil {
		return Secrets{}, err
	}
	if secrets.FederationKey, err = store.ed25519Key(federationKeyFile); err != nil {
		return Secrets{}, err
	}
	if secrets.PushRelayKey, err = store.ed25519Key(pushRelayKeyFile); err != nil {
		return Secrets{}, err
	}
	if secrets.VAPIDKey, err = store.p256Key(vapidKeyFile); err != nil {
		return Secrets{}, err
	}
	// LiveKit expects an API key that looks like an identifier, and a
	// secret of at least 32 characters.
	if secrets.LiveKitAPIKey, err = store.token(liveKitAPIKeyFile, liveKitAPIKey); err != nil {
		return Secrets{}, err
	}
	if secrets.LiveKitAPISecret, err = store.token(liveKitSecretFile, longToken); err != nil {
		return Secrets{}, err
	}
	if secrets.TURNSecret, err = store.token(turnSecretFile, longToken); err != nil {
		return Secrets{}, err
	}
	if secrets.TOTPKey, err = store.symmetricKey(totpKeyFile); err != nil {
		return Secrets{}, err
	}
	return secrets, nil
}

// ensurePrivateDirectory creates the directory if needed and makes sure only
// the server's own user can list or read it.
func ensurePrivateDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("creating secrets directory %s: %w", directory, err)
	}
	// G302 expects 0600, but a directory needs the execute bit to be opened.
	if err := os.Chmod(directory, 0o700); err != nil { //nolint:gosec // see above
		return fmt.Errorf("restricting permissions on %s: %w", directory, err)
	}
	return nil
}

// DirectoryIn returns the secrets directory inside the data directory.
func DirectoryIn(dataDirectory string) string {
	return filepath.Join(dataDirectory, "secrets")
}
