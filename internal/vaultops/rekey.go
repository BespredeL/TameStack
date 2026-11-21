package vaultops

import (
	"context"
	"fmt"

	"tamestack/internal/crypto"
	"tamestack/internal/db"
)

type serverPlain struct {
	secret string
	sudo   string
	pass   string
}

// SetMasterPassword re-encrypts stored secrets under a new master key.
// The active key and database rows change only after every ciphertext is written.
// A second call is rejected: rotation requires the current password and is not silent.
func SetMasterPassword(ctx context.Context, database *db.DB, vault *crypto.Vault, newPass string) error {
	if newPass == "" {
		return fmt.Errorf("master password cannot be empty")
	}
	if vault == nil || database == nil {
		return fmt.Errorf("vault is not initialized")
	}

	hasPass, err := database.GetVaultSetting("has_master_password")
	if err != nil {
		return fmt.Errorf("failed to read vault settings: %w", err)
	}
	if hasPass == "true" || vault.HasMasterPassword() {
		return crypto.ErrMasterPasswordAlreadySet
	}
	if vault.IsLocked() {
		return crypto.ErrVaultLocked
	}

	servers, err := database.ListServers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list servers for vault re-encryption: %w", err)
	}

	plainServers := make(map[string]serverPlain, len(servers))
	for _, s := range servers {
		secret, err := vault.Decrypt(s.EncryptedSecret)
		if err != nil {
			return fmt.Errorf("failed to decrypt credentials for server %s: %w", s.ID, err)
		}
		sudoSecret, err := vault.Decrypt(s.EncryptedSudoSecret)
		if err != nil {
			return fmt.Errorf("failed to decrypt sudo password for server %s: %w", s.ID, err)
		}
		passphrase, err := vault.Decrypt(s.EncryptedKeyPassphrase)
		if err != nil {
			return fmt.Errorf("failed to decrypt key passphrase for server %s: %w", s.ID, err)
		}
		plainServers[s.ID] = serverPlain{secret: secret, sudo: sudoSecret, pass: passphrase}
	}

	aiConfigs, err := database.ListAIProviderConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list AI provider keys: %w", err)
	}
	plainAI := make(map[string]string, len(aiConfigs))
	for _, c := range aiConfigs {
		if c.APIKeyEncrypted == "" {
			continue
		}
		plainKey, err := vault.Decrypt(c.APIKeyEncrypted)
		if err != nil {
			return fmt.Errorf("failed to decrypt API key %s: %w", c.ID, err)
		}
		plainAI[c.ID] = plainKey
	}

	verifier, derived, err := vault.PreviewMasterPassword(newPass)
	if err != nil {
		return fmt.Errorf("failed to prepare master password: %w", err)
	}

	type serverCipher struct {
		secret string
		sudo   string
		pass   string
	}
	cipherServers := make(map[string]serverCipher, len(plainServers))
	for id, plain := range plainServers {
		secret, err := vault.EncryptWithDerivedKey(derived, plain.secret)
		if err != nil {
			return fmt.Errorf("failed to encrypt credentials for server %s: %w", id, err)
		}
		sudoSecret, err := vault.EncryptWithDerivedKey(derived, plain.sudo)
		if err != nil {
			return fmt.Errorf("failed to encrypt sudo password for server %s: %w", id, err)
		}
		passphrase, err := vault.EncryptWithDerivedKey(derived, plain.pass)
		if err != nil {
			return fmt.Errorf("failed to encrypt key passphrase for server %s: %w", id, err)
		}
		cipherServers[id] = serverCipher{secret: secret, sudo: sudoSecret, pass: passphrase}
	}
	cipherAI := make(map[string]string, len(plainAI))
	for id, plain := range plainAI {
		enc, err := vault.EncryptWithDerivedKey(derived, plain)
		if err != nil {
			return fmt.Errorf("failed to encrypt API key %s: %w", id, err)
		}
		cipherAI[id] = enc
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin vault re-encryption: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for id, enc := range cipherServers {
		if _, err := tx.ExecContext(ctx, `
			UPDATE servers
			SET encrypted_secret = ?, encrypted_sudo_secret = ?, encrypted_key_passphrase = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			enc.secret, enc.sudo, enc.pass, id,
		); err != nil {
			return fmt.Errorf("failed to store re-encrypted credentials for server %s: %w", id, err)
		}
	}
	for id, enc := range cipherAI {
		if _, err := tx.ExecContext(ctx, `
			UPDATE ai_provider_configs
			SET api_key_encrypted = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			enc, id,
		); err != nil {
			return fmt.Errorf("failed to store re-encrypted API key %s: %w", id, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vault_settings (key, value, updated_at) VALUES ('has_master_password', 'true', CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`); err != nil {
		return fmt.Errorf("failed to store master password flag: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vault_settings (key, value, updated_at) VALUES ('master_password_verifier', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, verifier); err != nil {
		return fmt.Errorf("failed to store master password verifier: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit vault re-encryption: %w", err)
	}
	committed = true
	vault.ActivateMasterKey(derived)
	return nil
}
