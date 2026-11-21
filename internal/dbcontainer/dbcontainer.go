// Package dbcontainer stores the SQLite database file inside an encrypted
// container (AES-256-GCM chunks, PBKDF2-derived key) so a copied database
// file reveals nothing without the master password.
package dbcontainer

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"tamestack/internal/crypto"
)

const (
	magic           = "SHDBC01"
	saltOffset      = len(magic) + 4
	baseNonceOffset = saltOffset + saltBytes
	headerBytes     = baseNonceOffset + nonceBytes
	kdfIters        = 200000
	saltBytes       = 16
	nonceBytes      = 12
	keyBytes        = 32
	chunkPlainSize  = 64 * 1024
	maxChunkOnDisk  = chunkPlainSize + 64
)

var (
	ErrInvalidContainer         = errors.New("not a valid TameStack database container")
	ErrInvalidPasswordOrCorrupt = errors.New("invalid master password or corrupted database container")
)

// KeyFromPassword derives the container encryption key from the master password.
func KeyFromPassword(password string, salt []byte) []byte {
	return crypto.DeriveKeyPBKDF2(password, salt, kdfIters, keyBytes)
}

// Exists reports whether a container file is present at path.
func Exists(containerPath string) bool {
	st, err := os.Stat(containerPath)
	return err == nil && st.Size() > 0
}

func buildHeader(salt, baseNonce []byte) []byte {
	header := make([]byte, 0, headerBytes)
	header = append(header, []byte(magic)...)
	iters := make([]byte, 4)
	binary.BigEndian.PutUint32(iters, kdfIters)
	header = append(header, iters...)
	header = append(header, salt...)
	header = append(header, baseNonce...)
	return header
}

func parseHeader(header []byte) (salt, baseNonce []byte, err error) {
	if len(header) != headerBytes {
		return nil, nil, ErrInvalidContainer
	}
	if string(header[:len(magic)]) != magic {
		return nil, nil, ErrInvalidContainer
	}
	if binary.BigEndian.Uint32(header[len(magic):saltOffset]) != kdfIters {
		return nil, nil, ErrInvalidContainer
	}
	return header[saltOffset:baseNonceOffset], header[baseNonceOffset:], nil
}

func chunkNonce(base []byte, index uint64) []byte {
	nonce := make([]byte, nonceBytes)
	copy(nonce, base)
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], index)
	for i := 0; i < 8; i++ {
		nonce[4+i] ^= counter[i]
	}
	return nonce
}

func chunkAAD(header []byte, index uint64) []byte {
	aad := make([]byte, 0, len(header)+8)
	aad = append(aad, header...)
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], index)
	return append(aad, counter[:]...)
}

// writeContainer encrypts sourcePath into containerPath with a fresh header.
// The file is written to a temp sibling and atomically renamed.
func writeContainer(containerPath, sourcePath string, key, salt, baseNonce []byte) error {
	gcm, err := crypto.NewGCM(key)
	if err != nil {
		return err
	}
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open database source: %w", err)
	}
	defer src.Close()

	dir := filepath.Dir(containerPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create container directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".shdbc-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp container: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		tmp.Close()
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	header := buildHeader(salt, baseNonce)
	if _, err := tmp.Write(header); err != nil {
		return fmt.Errorf("failed to write container header: %w", err)
	}

	buf := make([]byte, chunkPlainSize)
	lenBuf := make([]byte, 4)
	var index uint64
	for {
		n, readErr := io.ReadFull(src, buf)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return fmt.Errorf("failed to read database source: %w", readErr)
		}
		ct := gcm.Seal(nil, chunkNonce(baseNonce, index), buf[:n], chunkAAD(header, index))
		binary.BigEndian.PutUint32(lenBuf, uint32(len(ct)))
		if _, err := tmp.Write(lenBuf); err != nil {
			return fmt.Errorf("failed to write container chunk frame: %w", err)
		}
		if _, err := tmp.Write(ct); err != nil {
			return fmt.Errorf("failed to write container chunk: %w", err)
		}
		index++
		if readErr == io.EOF || n == 0 {
			break
		}
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to sync container: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close container: %w", err)
	}
	if err := os.Rename(tmpName, containerPath); err != nil {
		return fmt.Errorf("failed to replace container: %w", err)
	}
	cleanup = false
	return nil
}

// Create seals plaintext database sourcePath into a brand-new container.
func Create(containerPath, sourcePath, password string) error {
	salt := make([]byte, saltBytes)
	baseNonce := make([]byte, nonceBytes)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate container salt: %w", err)
	}
	if _, err := rand.Read(baseNonce); err != nil {
		return fmt.Errorf("failed to generate container nonce: %w", err)
	}
	return writeContainer(containerPath, sourcePath, KeyFromPassword(password, salt), salt, baseNonce)
}

// readHeaderSalt loads the salt so Seal can reuse the original KDF parameters.
func readHeaderSalt(containerPath string) (salt []byte, err error) {
	f, err := os.Open(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open container: %w", err)
	}
	defer f.Close()
	header := make([]byte, headerBytes)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, ErrInvalidContainer
	}
	salt, _, err = parseHeader(header)
	return salt, err
}

// Seal re-encrypts an updated plaintext copy into the existing container,
// replacing it atomically with a fresh nonce base.
func Seal(containerPath, sourcePath, password string) error {
	salt, err := readHeaderSalt(containerPath)
	if err != nil {
		return err
	}
	baseNonce := make([]byte, nonceBytes)
	if _, err := rand.Read(baseNonce); err != nil {
		return fmt.Errorf("failed to generate container nonce: %w", err)
	}
	return writeContainer(containerPath, sourcePath, KeyFromPassword(password, salt), salt, baseNonce)
}

// VerifyPassword checks the master password against the first container chunk
// without extracting the database.
func VerifyPassword(containerPath, password string) error {
	f, err := os.Open(containerPath)
	if err != nil {
		return fmt.Errorf("failed to open container: %w", err)
	}
	defer f.Close()
	return verifyFromReader(f, password)
}

func verifyFromReader(f *os.File, password string) error {
	header := make([]byte, headerBytes)
	if _, err := io.ReadFull(f, header); err != nil {
		return ErrInvalidContainer
	}
	salt, baseNonce, err := parseHeader(header)
	if err != nil {
		return err
	}
	key := KeyFromPassword(password, salt)
	gcm, err := crypto.NewGCM(key)
	if err != nil {
		return err
	}
	return readFirstChunk(f, gcm, header, baseNonce)
}

func readFirstChunk(f *os.File, gcm cipher.AEAD, header, baseNonce []byte) error {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(f, lenBuf); err != nil {
		return ErrInvalidContainer
	}
	size := binary.BigEndian.Uint32(lenBuf)
	if size == 0 || size > maxChunkOnDisk {
		return ErrInvalidContainer
	}
	ct := make([]byte, size)
	if _, err := io.ReadFull(f, ct); err != nil {
		return ErrInvalidContainer
	}
	if _, err := gcm.Open(nil, chunkNonce(baseNonce, 0), ct, chunkAAD(header, 0)); err != nil {
		return ErrInvalidPasswordOrCorrupt
	}
	return nil
}

// Unlock decrypts the container into destPath (temp file + atomic rename).
func Unlock(containerPath, destPath, password string) error {
	f, err := os.Open(containerPath)
	if err != nil {
		return fmt.Errorf("failed to open container: %w", err)
	}
	defer f.Close()

	header := make([]byte, headerBytes)
	if _, err := io.ReadFull(f, header); err != nil {
		return ErrInvalidContainer
	}
	salt, baseNonce, err := parseHeader(header)
	if err != nil {
		return err
	}
	key := KeyFromPassword(password, salt)
	gcm, err := crypto.NewGCM(key)
	if err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".shdbw-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp working db: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		tmp.Close()
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	lenBuf := make([]byte, 4)
	var index uint64
	for {
		_, readErr := io.ReadFull(f, lenBuf)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return ErrInvalidContainer
		}
		size := binary.BigEndian.Uint32(lenBuf)
		if size == 0 || size > maxChunkOnDisk {
			return ErrInvalidContainer
		}
		ct := make([]byte, size)
		if _, err := io.ReadFull(f, ct); err != nil {
			return ErrInvalidContainer
		}
		plain, err := gcm.Open(nil, chunkNonce(baseNonce, index), ct, chunkAAD(header, index))
		if err != nil {
			return ErrInvalidPasswordOrCorrupt
		}
		if _, err := tmp.Write(plain); err != nil {
			return fmt.Errorf("failed to write working db: %w", err)
		}
		index++
	}
	if index == 0 {
		return ErrInvalidContainer
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to sync working db: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close working db: %w", err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("failed to place working db: %w", err)
	}
	cleanup = false
	return nil
}
