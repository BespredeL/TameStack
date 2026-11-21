package ssh

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type HostKeyStatus string

const (
	HostKeyKnown    HostKeyStatus = "known"
	HostKeyNew      HostKeyStatus = "new"
	HostKeyMismatch HostKeyStatus = "mismatch"
)

// HostKeyInspection represents the result of scanning the remote SSH server public key.
type HostKeyInspection struct {
	Status      HostKeyStatus `json:"status"`
	Fingerprint string        `json:"fingerprint"`
	KeyType     string        `json:"key_type"`
	RawEntry    string        `json:"raw_entry"`
	Message     string        `json:"message"`
}

var knownHostsLock sync.Mutex

// errProbeCaptured aborts the probe handshake once the host key is captured.
var errProbeCaptured = errors.New("probe captured host key")

// NormalizeAddr returns formatted addr for known_hosts standard format using knownhosts.Normalize.
func NormalizeAddr(host string, port int) string {
	if port <= 0 {
		port = 22
	}
	return knownhosts.Normalize(net.JoinHostPort(host, fmt.Sprintf("%d", port)))
}

// InspectHostKey probes the remote SSH port to fetch and verify the server's public key.
func InspectHostKey(knownHostsPath, host string, port int, timeout time.Duration) (*HostKeyInspection, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if port <= 0 {
		port = 22
	}

	dialAddr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	normAddr := NormalizeAddr(host, port)
	var capturedKey ssh.PublicKey

	// Temporary callback to capture the public key.
	probeCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		capturedKey = key
		// Return an error to stop connection after key exchange.
		return errProbeCaptured
	}

	config := &ssh.ClientConfig{
		User:            "probe",
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: probeCallback,
		Timeout:         timeout,
	}

	conn, err := net.DialTimeout("tcp", dialAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to reach SSH port on %s: %w", dialAddr, err)
	}
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, dialAddr, config)
	if err == nil {
		defer sshConn.Close()
	}
	_ = chans
	_ = reqs

	if capturedKey == nil {
		return nil, errors.New("failed to capture remote SSH host key")
	}

	fingerprint := ssh.FingerprintSHA256(capturedKey)
	keyType := capturedKey.Type()
	rawEntry := knownhosts.Line([]string{normAddr}, capturedKey)

	// Verify against local known_hosts.
	checkCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		// If file is empty or missing, treat as new.
		return &HostKeyInspection{
			Status:      HostKeyNew,
			Fingerprint: fingerprint,
			KeyType:     keyType,
			RawEntry:    rawEntry,
			Message:     "Host key is not in known_hosts. User approval required.",
		}, nil
	}

	tcpAddr, _ := net.ResolveTCPAddr("tcp", dialAddr)
	err = checkCallback(dialAddr, tcpAddr, capturedKey)
	if err == nil {
		return &HostKeyInspection{
			Status:      HostKeyKnown,
			Fingerprint: fingerprint,
			KeyType:     keyType,
			RawEntry:    rawEntry,
			Message:     "Host key verified and trusted in known_hosts.",
		}, nil
	}

	var keyErr *knownhosts.KeyError
	if errors.As(err, &keyErr) {
		if len(keyErr.Want) > 0 {
			// Key mismatch! Potential MITM attack.
			return &HostKeyInspection{
				Status:      HostKeyMismatch,
				Fingerprint: fingerprint,
				KeyType:     keyType,
				RawEntry:    rawEntry,
				Message:     fmt.Sprintf("CRITICAL WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED! Known fingerprint does not match %s", fingerprint),
			}, nil
		}
	}

	// Not found in known_hosts -> new.
	return &HostKeyInspection{
		Status:      HostKeyNew,
		Fingerprint: fingerprint,
		KeyType:     keyType,
		RawEntry:    rawEntry,
		Message:     "Host key is new. User confirmation required before connecting.",
	}, nil
}

// ValidateHostKeyLine rejects blank, multi-line, wildcard, and unparsable known_hosts lines.
func ValidateHostKeyLine(rawEntry string) error {
	rawEntry = strings.TrimSpace(rawEntry)
	if rawEntry == "" {
		return errors.New("empty host key entry")
	}
	if strings.ContainsAny(rawEntry, "\r\n") {
		return errors.New("host key entry must be a single line")
	}
	fields := strings.Fields(rawEntry)
	if len(fields) < 3 {
		return errors.New("invalid host key entry")
	}
	host := fields[0]
	if strings.ContainsAny(host, "*?!") || strings.HasPrefix(host, "|") {
		return errors.New("wildcard host key entries are not allowed")
	}
	rest := strings.Join(fields[1:], " ")
	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(rest)); err != nil {
		return fmt.Errorf("host key entry does not contain a public key: %w", err)
	}
	return nil
}

// TrustAndSaveHostKey safely appends the verified host key entry into known_hosts.
func TrustAndSaveHostKey(knownHostsPath, rawEntry string) error {
	knownHostsLock.Lock()
	defer knownHostsLock.Unlock()

	if err := ValidateHostKeyLine(rawEntry); err != nil {
		return err
	}
	rawEntry = strings.TrimSpace(rawEntry)

	f, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(rawEntry + "\n"); err != nil {
		return fmt.Errorf("failed to write to known_hosts: %w", err)
	}

	return nil
}

// FingerprintFromRaw converts raw public key to SHA256 fingerprint string.
func FingerprintFromRaw(pubKey ssh.PublicKey) string {
	hash := sha256.Sum256(pubKey.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])
}
