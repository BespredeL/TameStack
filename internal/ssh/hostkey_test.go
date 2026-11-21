package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestHostKeyStorageAndVerification(t *testing.T) {
	tmpDir := t.TempDir()
	knownHostsPath := filepath.Join(tmpDir, "known_hosts")

	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		t.Fatalf("failed to create ssh public key: %v", err)
	}

	addr := NormalizeAddr("192.168.1.50", 2222)
	if !strings.Contains(addr, "192.168.1.50") || !strings.Contains(addr, "2222") {
		t.Fatalf("unexpected normalized addr: %s", addr)
	}

	rawEntry := knownhosts.Line([]string{addr}, sshPubKey)
	if err := TrustAndSaveHostKey(knownHostsPath, rawEntry); err != nil {
		t.Fatalf("failed to trust and save host key: %v", err)
	}

	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("failed to read known_hosts: %v", err)
	}
	if !strings.Contains(string(content), "192.168.1.50") {
		t.Fatalf("known_hosts does not contain saved entry: %s", string(content))
	}

	fp := FingerprintFromRaw(sshPubKey)
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("unexpected fingerprint format: %s", fp)
	}
}
