package password

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testParams are the cheapest valid parameters, so tests stay fast. The
// algorithm is identical; only the cost differs.
var testParams = Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1}

func newTestHasher(t *testing.T) *Hasher {
	t.Helper()
	hasher, err := NewHasher(testParams, 2)
	require.NoError(t, err)
	return hasher
}

func TestHashThenVerify(t *testing.T) {
	hasher := newTestHasher(t)

	encoded, err := hasher.Hash(t.Context(), "correct horse battery")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(encoded, "$argon2id$v=19$m=64,t=1,p=1$"))
	match, err := hasher.Verify(t.Context(), "correct horse battery", encoded)
	require.NoError(t, err)
	assert.True(t, match)
	match, err = hasher.Verify(t.Context(), "correct horse batterY", encoded)
	require.NoError(t, err)
	assert.False(t, match)
}

func TestHashUsesFreshSalt(t *testing.T) {
	hasher := newTestHasher(t)

	first, err := hasher.Hash(t.Context(), "same password")
	require.NoError(t, err)
	second, err := hasher.Hash(t.Context(), "same password")
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

// TestKnownAnswer checks our hashing against an independent implementation:
// the expected value was computed with OpenSSL's Argon2id (through Node.js
// crypto.argon2Sync) for password "password", salt "somesaltsomesalt",
// m=64 KiB, t=1, p=1, 32-byte output.
func TestKnownAnswer(t *testing.T) {
	const expected = "$argon2id$v=19$m=64,t=1,p=1$c29tZXNhbHRzb21lc2FsdA$55PWTvddWPUD1GMbKxSff4ASfF85k9ibHJt4HlHQtBM"
	hasher := newTestHasher(t)

	key, err := hasher.derive(t.Context(), "password", []byte("somesaltsomesalt"), testParams)
	require.NoError(t, err)
	assert.Equal(t, expected, encode(testParams, []byte("somesaltsomesalt"), key))

	match, err := hasher.Verify(t.Context(), "password", expected)
	require.NoError(t, err)
	assert.True(t, match)
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	hasher := newTestHasher(t)
	valid, err := hasher.Hash(t.Context(), "a valid password")
	require.NoError(t, err)
	parts := strings.Split(valid, "$") // "", argon2id, v=19, params, salt, key
	salt, key := parts[4], parts[5]

	tests := map[string]string{
		"empty":               "",
		"bcrypt":              "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz012",
		"argon2i":             "$argon2i$v=19$m=64,t=1,p=1$" + salt + "$" + key,
		"old version":         "$argon2id$v=16$m=64,t=1,p=1$" + salt + "$" + key,
		"missing part":        "$argon2id$v=19$m=64,t=1,p=1$" + salt,
		"extra part":          valid + "$extra",
		"params not numbers":  "$argon2id$v=19$m=x,t=1,p=1$" + salt + "$" + key,
		"params trailing":     "$argon2id$v=19$m=64,t=1,p=1x$" + salt + "$" + key,
		"params leading zero": "$argon2id$v=19$m=064,t=1,p=1$" + salt + "$" + key,
		"memory too large":    "$argon2id$v=19$m=99999999,t=1,p=1$" + salt + "$" + key,
		"memory too small":    "$argon2id$v=19$m=4,t=1,p=1$" + salt + "$" + key,
		"zero iterations":     "$argon2id$v=19$m=64,t=0,p=1$" + salt + "$" + key,
		"too many iterations": "$argon2id$v=19$m=64,t=65,p=1$" + salt + "$" + key,
		"zero parallelism":    "$argon2id$v=19$m=64,t=1,p=0$" + salt + "$" + key,
		"parallelism too big": "$argon2id$v=19$m=1024,t=1,p=17$" + salt + "$" + key,
		"parallelism > uint8": "$argon2id$v=19$m=64,t=1,p=300$" + salt + "$" + key,
		"salt not base64":     "$argon2id$v=19$m=64,t=1,p=1$!!!$" + key,
		"salt too short":      "$argon2id$v=19$m=64,t=1,p=1$c2FsdA$" + key,
		"key not base64":      "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$!!!",
		"key wrong length":    "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$c2hvcnQ",
	}
	for name, encoded := range tests {
		t.Run(name, func(t *testing.T) {
			match, err := hasher.Verify(t.Context(), "a valid password", encoded)

			require.Error(t, err)
			assert.False(t, match)
		})
	}
}

func TestNeedsRehash(t *testing.T) {
	hasher := newTestHasher(t)
	current, err := hasher.Hash(t.Context(), "a valid password")
	require.NoError(t, err)
	stronger, err := NewHasher(Params{MemoryKiB: 128, Iterations: 2, Parallelism: 1}, 1)
	require.NoError(t, err)

	assert.False(t, hasher.NeedsRehash(current))
	assert.True(t, stronger.NeedsRehash(current))
	assert.True(t, hasher.NeedsRehash("garbage"))
}

func TestNewHasherRejectsBadSettings(t *testing.T) {
	_, err := NewHasher(Params{MemoryKiB: 0, Iterations: 1, Parallelism: 1}, 1)
	require.ErrorIs(t, err, errInvalidParams)

	_, err = NewHasher(testParams, 0)
	assert.ErrorContains(t, err, "concurrency")
}

func TestDefaultParamsAreValid(t *testing.T) {
	hasher, err := NewHasher(DefaultParams(), 1)

	require.NoError(t, err)
	assert.Equal(t, "m=65536,t=3,p=1", hasher.params.String())
}

func TestHashWaitsForFreeSlotAndHonorsCancellation(t *testing.T) {
	hasher, err := NewHasher(testParams, 1)
	require.NoError(t, err)
	hasher.slots <- struct{}{} // occupy the only slot
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	_, err = hasher.Hash(ctx, "a valid password")
	require.ErrorIs(t, err, context.DeadlineExceeded)

	encoded := encode(testParams, []byte("somesaltsomesalt"), make([]byte, keyLength))
	_, err = hasher.Verify(ctx, "a valid password", encoded)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCheckPolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     error
	}{
		{"ten ascii characters", "abcdefghij", nil},
		{"nine characters", "abcdefghi", ErrTooShort},
		{"ten multi-byte characters", "üüüüüüüüüü", nil},
		{"nine multi-byte characters", "üüüüüüüüü", ErrTooShort},
		{"1024 bytes", strings.Repeat("a", 1024), nil},
		{"1025 bytes", strings.Repeat("a", 1025), ErrTooLong},
		{"invalid utf-8", "abcdefghij\xff", ErrInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, CheckPolicy(test.password))
		})
	}
}
