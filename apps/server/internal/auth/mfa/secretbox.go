package mfa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// sealer encrypts TOTP secrets with AES-256-GCM (NIST SP 800-38D) from the
// standard library. The user's ID is authenticated as additional data, so
// a secret copied to another user's row fails to decrypt.
type sealer struct {
	aead cipher.AEAD
}

func newSealer(key []byte) (sealer, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return sealer{}, fmt.Errorf("TOTP encryption key: %w", err)
	}
	// NewGCM fails only for block ciphers whose block size is not 128 bits;
	// AES's always is, so there is no error to handle here.
	aead, _ := cipher.NewGCM(block)
	return sealer{aead: aead}, nil
}

// seal returns nonce || ciphertext. A fresh random 96-bit nonce per secret
// is safe for far more secrets than one instance will ever store.
func (box sealer) seal(userID uuid.UUID, plaintext []byte) []byte {
	nonce := make([]byte, box.aead.NonceSize())
	_, _ = rand.Read(nonce) // crypto/rand.Read never fails (Go 1.24+)
	return box.aead.Seal(nonce, nonce, plaintext, userID[:])
}

func (box sealer) open(userID uuid.UUID, sealed []byte) ([]byte, error) {
	if len(sealed) < box.aead.NonceSize() {
		return nil, errors.New("stored TOTP secret is too short")
	}
	nonce, ciphertext := sealed[:box.aead.NonceSize()], sealed[box.aead.NonceSize():]
	plaintext, err := box.aead.Open(nil, nonce, ciphertext, userID[:])
	if err != nil {
		return nil, fmt.Errorf("decrypting stored TOTP secret: %w", err)
	}
	return plaintext, nil
}
