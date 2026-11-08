package secretbox

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBox(t *testing.T, fill byte) *Box {
	t.Helper()
	box, err := New(bytes.Repeat([]byte{fill}, 32))
	require.NoError(t, err)
	return box
}

func TestSealThenOpen(t *testing.T) {
	box := newBox(t, 1)

	sealed := box.Seal("smtp.password", []byte("hunter2"))
	opened, err := box.Open("smtp.password", sealed)

	require.NoError(t, err)
	assert.Equal(t, []byte("hunter2"), opened)
	assert.NotContains(t, string(sealed), "hunter2")
	assert.NotEqual(t, sealed, box.Seal("smtp.password", []byte("hunter2")), "every seal uses a new nonce")
}

func TestOpenRejectsTamperingAndMisuse(t *testing.T) {
	box := newBox(t, 1)
	sealed := box.Seal("smtp.password", []byte("hunter2"))
	tampered := append([]byte{}, sealed...)
	tampered[len(tampered)-1] ^= 1

	for name, open := range map[string]func() ([]byte, error){
		"other context": func() ([]byte, error) { return box.Open("oidc.secret", sealed) },
		"other key":     func() ([]byte, error) { return newBox(t, 2).Open("smtp.password", sealed) },
		"tampered":      func() ([]byte, error) { return box.Open("smtp.password", tampered) },
		"too short":     func() ([]byte, error) { return box.Open("smtp.password", sealed[:5]) },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := open()
			assert.ErrorIs(t, err, ErrCannotOpen)
		})
	}
}

func TestNewRejectsWrongKeySize(t *testing.T) {
	_, err := New(make([]byte, 16))

	assert.ErrorContains(t, err, "must be 32 bytes, got 16")
}
