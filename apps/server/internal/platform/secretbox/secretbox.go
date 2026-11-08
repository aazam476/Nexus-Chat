// Package secretbox encrypts small secrets the server must store and read
// back later, such as the SMTP password and OIDC client secrets.
//
// It uses AES-256-GCM (NIST SP 800-38D) from Go's standard library; nothing
// here is a new cryptographic construction. Each value is bound to a
// context string (the additional data), so a sealed value copied to another
// setting fails to open instead of being used in the wrong place.
//
// Start reading at [Box.Seal].
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// keySize is the AES-256 key length in bytes.
const keySize = 32

// ErrCannotOpen means a sealed value is damaged, was sealed for another
// context, or was sealed with another key.
var ErrCannotOpen = errors.New("stored secret cannot be decrypted; enter it again")

// Box seals and opens values with one key.
type Box struct {
	aead cipher.AEAD
}

// New returns a Box using a 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("secret box key must be %d bytes, got %d", keySize, len(key))
	}
	// A 32-byte key always makes a valid AES block, and AES's 128-bit block
	// is what GCM requires, so neither call below can fail.
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext for context and returns nonce || ciphertext. A
// fresh random 96-bit nonce per value is safe for far more values than an
// instance will ever store.
func (box *Box) Seal(context string, plaintext []byte) []byte {
	nonce := make([]byte, box.aead.NonceSize())
	_, _ = rand.Read(nonce) // crypto/rand.Read never fails (Go 1.24+)
	return box.aead.Seal(nonce, nonce, plaintext, []byte(context))
}

// Open decrypts a value sealed for context, or returns ErrCannotOpen.
func (box *Box) Open(context string, sealed []byte) ([]byte, error) {
	size := box.aead.NonceSize()
	if len(sealed) < size {
		return nil, ErrCannotOpen
	}
	plaintext, err := box.aead.Open(nil, sealed[:size], sealed[size:], []byte(context))
	if err != nil {
		return nil, ErrCannotOpen
	}
	return plaintext, nil
}
