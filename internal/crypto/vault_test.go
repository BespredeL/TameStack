package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultEncryptionDecryption(t *testing.T) {
	tmpDir := t.TempDir()
	saltPath := filepath.Join(tmpDir, "test.salt")
	if err := os.WriteFile(saltPath, []byte("01234567890123456789012345678901"), 0600); err != nil {
		t.Fatalf("failed to write salt: %v", err)
	}

	vault, err := NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	originalSecret := "SuperSecretPassword123!@#"
	encrypted, err := vault.Encrypt(originalSecret)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if encrypted == originalSecret {
		t.Fatalf("encrypted string should not match plaintext")
	}

	decrypted, err := vault.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != originalSecret {
		t.Fatalf("expected %q, got %q", originalSecret, decrypted)
	}
}

func TestScrubSecrets(t *testing.T) {
	logWithPassword := "Connecting to DB with password=MySecret123 host=localhost"
	scrubbed := ScrubSecrets(logWithPassword)
	if scrubbed != "Connecting to DB with password=[REDACTED_SECRET] host=localhost" {
		t.Fatalf("scrubbed output mismatch: %s", scrubbed)
	}
}

func TestMasterPasswordLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	saltPath := filepath.Join(tmpDir, "test.salt")
	if err := os.WriteFile(saltPath, []byte("01234567890123456789012345678901"), 0600); err != nil {
		t.Fatalf("failed to write salt: %v", err)
	}

	vault, err := NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	// 1. Initial state: unlocked with machine key.
	if vault.IsLocked() {
		t.Errorf("Expected initial vault to be unlocked")
	}
	if vault.HasMasterPassword() {
		t.Errorf("Expected initial vault to not have master password")
	}

	// 2. Setup Master Password.
	verifier, err := vault.SetupMasterPassword("MasterSecretPass2026!")
	if err != nil {
		t.Fatalf("Failed to setup master password: %v", err)
	}
	if verifier == "" {
		t.Fatalf("Verifier token should not be empty")
	}
	if !vault.HasMasterPassword() {
		t.Errorf("Expected HasMasterPassword to be true")
	}

	// Encrypt a secret under master key.
	secret := "DatabaseSuperPassword#99"
	enc, err := vault.Encrypt(secret)
	if err != nil {
		t.Fatalf("Failed to encrypt with master key: %v", err)
	}

	// 3. Lock vault.
	vault.Lock()
	if !vault.IsLocked() {
		t.Errorf("Expected vault to be locked")
	}

	// Encrypt/Decrypt should fail when locked.
	if _, err := vault.Decrypt(enc); err != ErrVaultLocked {
		t.Errorf("Expected ErrVaultLocked when decrypting, got: %v", err)
	}
	if _, err := vault.Encrypt("hello"); err != ErrVaultLocked {
		t.Errorf("Expected ErrVaultLocked when encrypting, got: %v", err)
	}

	// 4. Try unlock with wrong password.
	if err := vault.Unlock("WrongPassword123", verifier); err != ErrInvalidMasterPassword {
		t.Errorf("Expected ErrInvalidMasterPassword with bad password, got: %v", err)
	}
	if !vault.IsLocked() {
		t.Errorf("Vault should remain locked after failed attempt")
	}

	// 5. Unlock with correct password.
	if err := vault.Unlock("MasterSecretPass2026!", verifier); err != nil {
		t.Fatalf("Failed to unlock vault with correct password: %v", err)
	}
	if vault.IsLocked() {
		t.Errorf("Expected vault to be unlocked")
	}

	dec, err := vault.Decrypt(enc)
	if err != nil || dec != secret {
		t.Fatalf("Decryption after unlock failed: got=%s, want=%s, err=%v", dec, secret, err)
	}
}
