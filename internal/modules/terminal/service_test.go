package terminal

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"tamestack/internal/testutil"
	"testing"
	"unicode/utf8"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

func TestTerminalService_SessionNotFound(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "test_term.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	cfg := &config.AppConfig{KnownHostsPath: filepath.Join(tmpDir, "known_hosts")}
	saltPath := filepath.Join(tmpDir, "salt.bin")
	_ = os.WriteFile(saltPath, []byte("12345678901234567890123456789012"), 0600)

	vault, err := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}
	pool := ssh.NewPool()

	svc := NewService(cfg, database, vault, pool)

	// Writing to a non-existent session.
	err = svc.WriteInput("non-existent-session-id", "ls -la\n")
	if err == nil {
		t.Errorf("expected error writing to non-existent session, got nil")
	}

	// Resizing a non-existent session.
	err = svc.Resize("non-existent-session-id", 80, 24)
	if err == nil {
		t.Errorf("expected error resizing non-existent session, got nil")
	}

	// Closing a non-existent session must not panic.
	err = svc.CloseSession("non-existent-session-id")
	if err != nil {
		t.Errorf("expected nil error closing non-existent session, got %v", err)
	}
}

type chunkedReader struct {
	chunks [][]byte
	idx    int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.idx >= len(r.chunks) {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[r.idx])
	r.idx++
	return n, nil
}

func TestPumpUTF8Stream_SplitMultiByteRune(t *testing.T) {
	want := "Привет 🚀 мир!"
	raw := []byte(want)
	// Split right in the middle of multi-byte Cyrillic and emoji sequences.
	cr := &chunkedReader{
		chunks: [][]byte{
			raw[:3],
			raw[3:15],
			raw[15:],
		},
	}

	var out strings.Builder
	pumpUTF8Stream(context.Background(), cr, func(chunk string) {
		if !utf8.ValidString(chunk) {
			t.Errorf("emitted invalid UTF-8 chunk: %q", chunk)
		}
		out.WriteString(chunk)
	})

	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}
