package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// phcPrefix starts every hash this package produces. Version 19 (0x13) is
// the current Argon2 version (RFC 9106 §3.1).
const phcPrefix = "$argon2id$v=19$"

// Hasher creates and checks password hashes.
type Hasher struct {
	params Params
	// slots limits how many hashes run at once. Each one uses
	// params.MemoryKiB of memory, so without a limit a burst of logins could
	// exhaust the server's RAM.
	slots chan struct{}
}

// NewHasher returns a Hasher that creates hashes with params and runs at
// most concurrency hashes at a time.
func NewHasher(params Params, concurrency int) (*Hasher, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}
	if concurrency < 1 {
		return nil, fmt.Errorf("concurrency must be at least 1, got %d", concurrency)
	}
	return &Hasher{params: params, slots: make(chan struct{}, concurrency)}, nil
}

// Hash returns the PHC-encoded Argon2id hash of password with a new random
// salt. It waits for a free slot, and gives up if ctx is cancelled first.
func (hasher *Hasher) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, saltLength)
	// crypto/rand.Read never fails on supported platforms (Go 1.24+).
	_, _ = rand.Read(salt)
	key, err := hasher.derive(ctx, password, salt, hasher.params)
	if err != nil {
		return "", err
	}
	return encode(hasher.params, salt, key), nil
}

// Verify reports whether password matches encoded. A malformed hash is an
// error, never a match.
func (hasher *Hasher) Verify(ctx context.Context, password string, encoded string) (bool, error) {
	params, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}
	got, err := hasher.derive(ctx, password, salt, params)
	if err != nil {
		return false, err
	}
	// Constant-time comparison so timing reveals nothing about the hash.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NeedsRehash reports whether encoded was made with parameters other than
// the Hasher's current ones, so it should be replaced after the next
// successful login.
func (hasher *Hasher) NeedsRehash(encoded string) bool {
	params, _, _, err := decode(encoded)
	return err != nil || params != hasher.params
}

func (hasher *Hasher) derive(ctx context.Context, password string, salt []byte, params Params) ([]byte, error) {
	select {
	case hasher.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting to hash password: %w", ctx.Err())
	}
	defer func() { <-hasher.slots }()
	return argon2.IDKey([]byte(password), salt, params.Iterations, params.MemoryKiB, params.Parallelism, keyLength), nil
}

func encode(params Params, salt []byte, key []byte) string {
	return phcPrefix + params.String() + "$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key)
}

// decode parses a PHC string produced by encode. It accepts nothing else.
func decode(encoded string) (Params, []byte, []byte, error) {
	rest, found := strings.CutPrefix(encoded, phcPrefix)
	parts := strings.Split(rest, "$")
	if !found || len(parts) != 3 {
		return Params{}, nil, nil, fmt.Errorf("not an argon2id v19 PHC hash")
	}
	var params Params
	_, err := fmt.Sscanf(parts[0], "m=%d,t=%d,p=%d", &params.MemoryKiB, &params.Iterations, &params.Parallelism)
	// Sscanf ignores trailing text and accepts leading zeros, so also require
	// the canonical spelling.
	if err != nil || params.String() != parts[0] {
		return Params{}, nil, nil, fmt.Errorf("malformed argon2 parameters %q", parts[0])
	}
	if err := params.validate(); err != nil {
		return Params{}, nil, nil, err
	}
	salt, saltError := base64.RawStdEncoding.DecodeString(parts[1])
	key, keyError := base64.RawStdEncoding.DecodeString(parts[2])
	if saltError != nil || keyError != nil || len(salt) < 8 || len(key) != keyLength {
		return Params{}, nil, nil, fmt.Errorf("malformed argon2 salt or hash")
	}
	return params, salt, key, nil
}
