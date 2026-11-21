package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/hkdf"

	"tamestack/internal/secretredact"
)

var (
	ErrVaultLocked              = errors.New("vault is locked: master password required")
	ErrInvalidMasterPassword    = errors.New("invalid master password")
	ErrMasterPasswordNotSet     = errors.New("master password is not configured")
	ErrMasterPasswordAlreadySet = errors.New("master password is already configured")
)

const (
	// VerifierConstant is encrypted and stored to verify master password correctness without storing the password.
	VerifierConstant = "TameStack_Vault_Verified_V1"
)

// Vault handles AES-256-GCM encryption and decryption for sensitive secrets.
type Vault struct {
	mu                sync.RWMutex
	salt              []byte
	key               []byte
	isLocked          bool
	hasMasterPassword bool
	lastActivityNano  atomic.Int64
	onLockedAccess    func()
}

// NewVault initializes a Vault with a key derived from a random OS-protected
// machine key and the per-install salt.
func NewVault(saltPath, keyPath string) (*Vault, error) {
	salt, err := os.ReadFile(saltPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault salt: %w", err)
	}

	machineKey, err := LoadOrCreateMachineKey(keyPath)
	if err != nil {
		return nil, err
	}
	derivedKey, err := deriveVaultKey(machineKey, salt)
	if err != nil {
		return nil, err
	}

	v := &Vault{
		salt:              salt,
		key:               derivedKey,
		isLocked:          false,
		hasMasterPassword: false,
	}
	v.lastActivityNano.Store(time.Now().UnixNano())
	return v, nil
}

func deriveVaultKey(machineKey, salt []byte) ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, machineKey, salt, []byte("TameStack/vault/v2")), key); err != nil {
		return nil, fmt.Errorf("failed to derive vault key: %w", err)
	}
	return key, nil
}

// SetOnLockedAccess registers a callback invoked when encrypted access is attempted while vault is locked.
func (v *Vault) SetOnLockedAccess(fn func()) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onLockedAccess = fn
}

// Touch updates the last activity timestamp.
func (v *Vault) Touch() {
	v.lastActivityNano.Store(time.Now().UnixNano())
}

// LastActivity returns the timestamp of last successful vault operation.
func (v *Vault) LastActivity() time.Time {
	nano := v.lastActivityNano.Load()
	if nano == 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
}

// DeriveMasterKey derives a 256-bit AES key from a master password and salt with 200,000 PBKDF2 iterations.
func (v *Vault) DeriveMasterKey(masterPassword string) []byte {
	return DeriveKeyPBKDF2(masterPassword, v.salt, 200000, 32)
}

// IsLocked reports whether the vault is locked.
func (v *Vault) IsLocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.isLocked
}

// HasMasterPassword reports whether a master password is configured.
func (v *Vault) HasMasterPassword() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.hasMasterPassword
}

// SetMasterPasswordConfigured marks whether master password mode is active.
func (v *Vault) SetMasterPasswordConfigured(configured bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hasMasterPassword = configured
	if configured {
		v.isLocked = true // Locked until unlocked with master password
	}
}

// Lock clears the active encryption key from memory and marks vault as locked.
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.hasMasterPassword {
		// Zero the key buffer for security.
		for i := range v.key {
			v.key[i] = 0
		}
		v.key = nil
		v.isLocked = true
	}
}

// Unlock verifies the provided master password against verifierToken and unlocks vault.
func (v *Vault) Unlock(password string, verifierToken string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	derived := v.DeriveMasterKey(password)

	// Verify using the token.
	decrypted, err := decryptWithKey(derived, verifierToken)
	if err != nil || decrypted != VerifierConstant {
		return ErrInvalidMasterPassword
	}

	v.key = derived
	v.isLocked = false
	v.lastActivityNano.Store(time.Now().UnixNano())
	return nil
}

// SetupMasterPassword initializes master password and returns a verifier token.
// It refuses to rotate an existing master password.
func (v *Vault) SetupMasterPassword(masterPassword string) (string, error) {
	verifier, derived, err := v.PreviewMasterPassword(masterPassword)
	if err != nil {
		return "", err
	}
	v.ActivateMasterKey(derived)
	return verifier, nil
}

// PreviewMasterPassword derives a key and verifier without switching the active key.
func (v *Vault) PreviewMasterPassword(masterPassword string) (string, []byte, error) {
	if masterPassword == "" {
		return "", nil, errors.New("master password cannot be empty")
	}

	v.mu.RLock()
	already := v.hasMasterPassword
	v.mu.RUnlock()
	if already {
		return "", nil, ErrMasterPasswordAlreadySet
	}

	derived := v.DeriveMasterKey(masterPassword)
	verifierToken, err := encryptWithKey(derived, VerifierConstant)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate verifier token: %w", err)
	}
	return verifierToken, derived, nil
}

// EncryptWithDerivedKey encrypts plaintext with a key that is not yet active.
func (v *Vault) EncryptWithDerivedKey(key []byte, plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	if len(key) == 0 {
		return "", errors.New("derived key is empty")
	}
	return encryptWithKey(key, plainText)
}

// ActivateMasterKey switches the vault to a previously prepared master key.
func (v *Vault) ActivateMasterKey(key []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.key = append([]byte(nil), key...)
	v.hasMasterPassword = true
	v.isLocked = false
	v.lastActivityNano.Store(time.Now().UnixNano())
}

// Encrypt encrypts plain text into a base64 encoded AES-256-GCM ciphertext.
func (v *Vault) Encrypt(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.isLocked || len(v.key) == 0 {
		if v.onLockedAccess != nil {
			go v.onLockedAccess()
		}
		return "", ErrVaultLocked
	}

	v.lastActivityNano.Store(time.Now().UnixNano())
	return encryptWithKey(v.key, plainText)
}

// Decrypt decrypts a base64 encoded AES-256-GCM ciphertext.
func (v *Vault) Decrypt(cipherTextB64 string) (string, error) {
	if cipherTextB64 == "" {
		return "", nil
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.isLocked || len(v.key) == 0 {
		if v.onLockedAccess != nil {
			go v.onLockedAccess()
		}
		return "", ErrVaultLocked
	}

	v.lastActivityNano.Store(time.Now().UnixNano())
	return decryptWithKey(v.key, cipherTextB64)
}

func encryptWithKey(key []byte, plainText string) (string, error) {
	gcm, err := NewGCM(key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptWithKey(key []byte, cipherTextB64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cipherTextB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 ciphertext: %w", err)
	}

	gcm, err := NewGCM(key)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext is too short")
	}

	nonce, actualCiphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (bad key or corrupted data): %w", err)
	}

	return string(plaintext), nil
}

// ScrubSecrets masks passwords, private keys and tokens from a string.
func ScrubSecrets(input string) string {
	return secretredact.ScrubSecrets(input)
}
