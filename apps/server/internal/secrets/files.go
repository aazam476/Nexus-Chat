package secrets

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// pemBlockType is the standard label for PKCS #8 private keys (RFC 7468 §10).
const pemBlockType = "PRIVATE KEY"

// fileStore reads and writes individual secret files. Each method follows the
// same pattern: if the file exists, parse it; otherwise generate a value,
// save it, and return it.
type fileStore struct {
	directory string
}

// ed25519Key loads or creates an Ed25519 key stored as PKCS #8 PEM.
func (store fileStore) ed25519Key(name string) (ed25519.PrivateKey, error) {
	parsed, err := store.privateKey(name, func() (any, error) {
		// A nil reader means "use the secure system source" (Go 1.26+).
		_, key, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, fmt.Errorf("generating Ed25519 key: %w", err)
		}
		return key, nil
	})
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s does not contain an Ed25519 key", name)
	}
	return key, nil
}

// p256Key loads or creates an ECDSA P-256 key stored as PKCS #8 PEM.
func (store fileStore) p256Key(name string) (*ecdsa.PrivateKey, error) {
	parsed, err := store.privateKey(name, func() (any, error) {
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	})
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%s does not contain a P-256 key", name)
	}
	return key, nil
}

// privateKey implements load-or-create for any PKCS #8 key type.
func (store fileStore) privateKey(name string, generate func() (any, error)) (any, error) {
	contents, err := store.read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return store.createPrivateKey(name, generate)
	}
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(contents)
	if block == nil || block.Type != pemBlockType {
		return nil, fmt.Errorf("%s is not a PEM %q block", name, pemBlockType)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	return key, nil
}

func (store fileStore) createPrivateKey(name string, generate func() (any, error)) (any, error) {
	key, err := generate()
	if err != nil {
		return nil, fmt.Errorf("generating %s: %w", name, err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("encoding %s: %w", name, err)
	}
	contents := pem.EncodeToMemory(&pem.Block{Type: pemBlockType, Bytes: der})
	if err := store.write(name, contents); err != nil {
		return nil, err
	}
	return key, nil
}

// liveKitAPIKey returns an identifier-like key such as "APIJBSWY3DPEHPK".
func liveKitAPIKey() string {
	return "API" + rand.Text()[:12]
}

// longToken returns 256 bits of randomness as 52 base32 characters.
// rand.Text alone gives 128 bits; two calls double that.
func longToken() string {
	return rand.Text() + rand.Text()
}

// token loads or creates a random string produced by generate.
func (store fileStore) token(name string, generate func() string) (string, error) {
	contents, err := store.read(name)
	if errors.Is(err, fs.ErrNotExist) {
		value := generate()
		if err := store.write(name, []byte(value+"\n")); err != nil {
			return "", err
		}
		return value, nil
	}
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", fmt.Errorf("%s is empty; delete it to generate a new value", name)
	}
	return value, nil
}

// read returns a file's contents, first tightening its permissions to
// owner-only in case an admin restored it from a backup with looser ones.
func (store fileStore) read(name string) ([]byte, error) {
	path := filepath.Join(store.directory, name)
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("restricting permissions on %s: %w", name, err)
	}
	// G304: path is our own directory joined with a constant file name.
	contents, err := os.ReadFile(path) //nolint:gosec // see above
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return contents, nil
}

// write saves a file atomically: it writes a temporary file and renames it,
// so a crash mid-write can never leave a half-written key behind.
func (store fileStore) write(name string, contents []byte) error {
	temporary, err := os.CreateTemp(store.directory, name+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file for %s: %w", name, err)
	}
	// Remove is a no-op after a successful rename; it only cleans up when
	// a step below fails.
	defer func() { _ = os.Remove(temporary.Name()) }()

	_, writeError := temporary.Write(contents)
	syncError := temporary.Sync()
	closeError := temporary.Close()
	if err := errors.Join(writeError, syncError, closeError); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if err := os.Rename(temporary.Name(), filepath.Join(store.directory, name)); err != nil {
		return fmt.Errorf("saving %s: %w", name, err)
	}
	return nil
}
