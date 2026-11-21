package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

const machineKeyBytes = 32

// LoadOrCreateMachineKey returns the 32-byte machine key material stored at
// keyPath, creating it on first use. The stored blob is OS-protected where the
// platform supports it (see protectMachineKey).
func LoadOrCreateMachineKey(keyPath string) ([]byte, error) {
	blob, err := os.ReadFile(keyPath)
	if err == nil {
		key, uerr := unprotectMachineKey(blob)
		if uerr != nil {
			return nil, fmt.Errorf("failed to open vault key %s: %w", keyPath, uerr)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read vault key %s: %w", keyPath, err)
	}

	key := make([]byte, machineKeyBytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate machine key: %w", err)
	}
	blob, err = protectMachineKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, blob, 0600); err != nil {
		return nil, fmt.Errorf("failed to write vault key %s: %w", keyPath, err)
	}
	return key, nil
}
