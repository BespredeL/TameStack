package vaultops

import (
	"context"
	"os"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"

	"tamestack/internal/crypto"
	"tamestack/internal/db"
)

func TestSetMasterPasswordDoesNotWipeOnDecryptFailure(t *testing.T) {
	ctx := context.Background()
	dir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(dir, "hub.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	saltPath := filepath.Join(dir, "salt")
	if err := os.WriteFile(saltPath, []byte("01234567890123456789012345678901"), 0600); err != nil {
		t.Fatal(err)
	}
	vault, err := crypto.NewVault(saltPath, filepath.Join(dir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}

	enc, err := vault.Encrypt("ssh-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID:              "srv-1",
		Name:            "box",
		Host:            "10.0.0.1",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: enc,
		Status:          "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	vault.SetMasterPasswordConfigured(true)
	if err := SetMasterPassword(ctx, database, vault, "new-master"); err == nil {
		t.Fatal("expected error when vault is locked")
	}
	got, err := database.GetServer(ctx, "srv-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.EncryptedSecret != enc {
		t.Fatalf("ciphertext changed while vault was locked")
	}
	flag, _ := database.GetVaultSetting("has_master_password")
	if flag == "true" {
		t.Fatal("master password flag was set despite failure")
	}

	vault2, err := crypto.NewVault(saltPath, filepath.Join(dir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE servers SET encrypted_secret = ? WHERE id = ?`, "not-a-ciphertext", "srv-1"); err != nil {
		t.Fatal(err)
	}
	if err := SetMasterPassword(ctx, database, vault2, "new-master"); err == nil {
		t.Fatal("expected error for corrupt ciphertext")
	}
	got, err = database.GetServer(ctx, "srv-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.EncryptedSecret != "not-a-ciphertext" {
		t.Fatalf("corrupt ciphertext was overwritten: %s", got.EncryptedSecret)
	}
}

func TestSetMasterPasswordRoundTripAndRejectsSecondCall(t *testing.T) {
	ctx := context.Background()
	dir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(dir, "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	saltPath := filepath.Join(dir, "salt")
	if err := os.WriteFile(saltPath, []byte("01234567890123456789012345678901"), 0600); err != nil {
		t.Fatal(err)
	}
	vault, err := crypto.NewVault(saltPath, filepath.Join(dir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := vault.Encrypt("ssh-secret")
	if err != nil {
		t.Fatal(err)
	}
	sudo, err := vault.Encrypt("sudo-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID:                  "srv-1",
		Name:                "box",
		Host:                "10.0.0.1",
		Port:                22,
		Username:            "deploy",
		AuthType:            "key",
		EncryptedSecret:     enc,
		EncryptedSudoSecret: sudo,
		Status:              "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	if err := SetMasterPassword(ctx, database, vault, "master-one"); err != nil {
		t.Fatal(err)
	}
	got, err := database.GetServer(ctx, "srv-1")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := vault.Decrypt(got.EncryptedSecret)
	if err != nil || plain != "ssh-secret" {
		t.Fatalf("secret after rekey = %q err=%v", plain, err)
	}
	plainSudo, err := vault.Decrypt(got.EncryptedSudoSecret)
	if err != nil || plainSudo != "sudo-secret" {
		t.Fatalf("sudo after rekey = %q err=%v", plainSudo, err)
	}

	before := got.EncryptedSecret
	if err := SetMasterPassword(ctx, database, vault, "master-two"); err != crypto.ErrMasterPasswordAlreadySet {
		t.Fatalf("second setup err = %v", err)
	}
	got, _ = database.GetServer(ctx, "srv-1")
	if got.EncryptedSecret != before {
		t.Fatal("second setup rewrote ciphertext")
	}
}
