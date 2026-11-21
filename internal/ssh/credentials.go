package ssh

import (
	"fmt"
	"time"

	"tamestack/internal/crypto"
	"tamestack/internal/db"
)

// OptionsForServer decrypts stored credentials and builds connection options.
// Every module should use this so passphrase and sudo password are not dropped.
func OptionsForServer(vault *crypto.Vault, record *db.ServerRecord, knownHosts string, timeout time.Duration) (ClientOptions, error) {
	if vault == nil {
		return ClientOptions{}, fmt.Errorf("vault is not initialized")
	}
	if record == nil {
		return ClientOptions{}, fmt.Errorf("server record is missing")
	}

	secret, err := vault.Decrypt(record.EncryptedSecret)
	if err != nil {
		return ClientOptions{}, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	var passphrase string
	if record.AuthType == "key_passphrase" {
		if record.EncryptedKeyPassphrase == "" {
			return ClientOptions{}, fmt.Errorf("key passphrase is not stored for this server")
		}
		passphrase, err = vault.Decrypt(record.EncryptedKeyPassphrase)
		if err != nil {
			return ClientOptions{}, fmt.Errorf("failed to decrypt key passphrase: %w", err)
		}
		if passphrase == "" {
			return ClientOptions{}, fmt.Errorf("key passphrase is empty")
		}
	}

	sudoSecret := ""
	if record.EncryptedSudoSecret != "" {
		sudoSecret, err = vault.Decrypt(record.EncryptedSudoSecret)
		if err != nil {
			return ClientOptions{}, fmt.Errorf("failed to decrypt sudo password: %w", err)
		}
	} else if record.AuthType == "password" {
		sudoSecret = secret
	}

	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return ClientOptions{
		Host:           record.Host,
		Port:           record.Port,
		Username:       record.Username,
		AuthType:       record.AuthType,
		Secret:         secret,
		Passphrase:     passphrase,
		SudoPassword:   sudoSecret,
		KnownHostsPath: knownHosts,
		Timeout:        timeout,
	}, nil
}
