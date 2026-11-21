package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

// DeriveKeyPBKDF2 derives a key from password and salt using PBKDF2-HMAC-SHA256.
// Iterations and key length are supplied by the caller because they are part of
// each subsystem's on-disk format and must not change.
func DeriveKeyPBKDF2(password string, salt []byte, iterations, keyLen int) []byte {
	return pbkdf2.Key([]byte(password), salt, iterations, keyLen, sha256.New)
}

// NewGCM constructs an AES-GCM AEAD from a raw AES key.
func NewGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return gcm, nil
}
