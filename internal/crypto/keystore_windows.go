//go:build windows

package crypto

import (
	"bytes"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// vaultKeyEntropy binds DPAPI protection to TameStack so blobs protected by
// other applications under the same user cannot be reused as a vault key.
var vaultKeyEntropy = []byte("TameStack/vault-key/v2")

func protectMachineKey(key []byte) ([]byte, error) {
	in := windows.DataBlob{Size: uint32(len(key)), Data: &key[0]}
	ent := windows.DataBlob{Size: uint32(len(vaultKeyEntropy)), Data: &vaultKeyEntropy[0]}
	var out windows.DataBlob

	if err := windows.CryptProtectData(&in, nil, &ent, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("DPAPI CryptProtectData failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return bytes.Clone(unsafe.Slice(out.Data, out.Size)), nil
}

func unprotectMachineKey(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("empty vault key blob")
	}
	in := windows.DataBlob{Size: uint32(len(blob)), Data: &blob[0]}
	ent := windows.DataBlob{Size: uint32(len(vaultKeyEntropy)), Data: &vaultKeyEntropy[0]}
	var out windows.DataBlob

	if err := windows.CryptUnprotectData(&in, nil, &ent, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("DPAPI CryptUnprotectData failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return bytes.Clone(unsafe.Slice(out.Data, out.Size)), nil
}
