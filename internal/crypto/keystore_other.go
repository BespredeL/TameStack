//go:build !windows

package crypto

import "fmt"

// Without an OS secret store the machine key is stored as raw random bytes;
// protection relies on the 0600 file mode. The key material is random and is
// not derivable from public machine data such as hostname or username.
func protectMachineKey(key []byte) ([]byte, error) {
	if len(key) != machineKeyBytes {
		return nil, fmt.Errorf("unexpected machine key size %d", len(key))
	}
	out := make([]byte, len(key))
	copy(out, key)
	return out, nil
}

func unprotectMachineKey(blob []byte) ([]byte, error) {
	if len(blob) != machineKeyBytes {
		return nil, fmt.Errorf("vault key file has unexpected size %d", len(blob))
	}
	return blob, nil
}
