package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateMachineKeyStableAcrossLoads(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "vault.key")

	first, err := LoadOrCreateMachineKey(keyPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(first) != machineKeyBytes {
		t.Fatalf("key size = %d, want %d", len(first), machineKeyBytes)
	}

	second, err := LoadOrCreateMachineKey(keyPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("machine key changed across reloads")
	}

	other, err := LoadOrCreateMachineKey(filepath.Join(dir, "other.key"))
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	if bytes.Equal(first, other) {
		t.Fatal("distinct key files share the same key material")
	}
}

func TestVaultKeyFileNotPlaintextOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI protection exists only on Windows")
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "vault.key")
	key, err := LoadOrCreateMachineKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(stored, key) {
		t.Fatal("DPAPI build stored the machine key in plaintext")
	}
}

func TestNewVaultKeysAreIndependentOfKeyPath(t *testing.T) {
	dir := t.TempDir()
	saltPath := filepath.Join(dir, "vault.salt")
	if err := os.WriteFile(saltPath, bytes.Repeat([]byte("s"), 32), 0600); err != nil {
		t.Fatal(err)
	}

	vaultA, err := NewVault(saltPath, filepath.Join(dir, "a.key"))
	if err != nil {
		t.Fatal(err)
	}
	vaultB, err := NewVault(saltPath, filepath.Join(dir, "b.key"))
	if err != nil {
		t.Fatal(err)
	}

	enc, err := vaultA.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vaultB.Decrypt(enc); err == nil {
		t.Fatal("vault with a different machine key decrypted the blob")
	}

	vaultA2, err := NewVault(saltPath, filepath.Join(dir, "a.key"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := vaultA2.Decrypt(enc)
	if err != nil || plain != "secret" {
		t.Fatalf("reopen with same key path: plain=%q err=%v", plain, err)
	}
}
