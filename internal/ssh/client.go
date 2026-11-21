package ssh

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHClient represents an authenticated SSH and SFTP session wrapper.
type SSHClient struct {
	mu            sync.Mutex
	client        *ssh.Client
	sftpClient    *sftp.Client
	host          string
	port          int
	user          string
	sudoPassword  string
	sudoAskpass   string
	askpassDir    string
	stopKeepAlive chan struct{}
}

// ClientOptions encapsulates connection parameters.
type ClientOptions struct {
	Host           string
	Port           int
	Username       string
	AuthType       string // "password", "key", "key_passphrase"
	Secret         string // Plaintext password or private key
	Passphrase     string
	SudoPassword   string // Plaintext password for sudo elevation
	KnownHostsPath string
	Timeout        time.Duration
}

// Connect establishes a secure SSH connection enforcing strict known_hosts checking.
func Connect(ctx context.Context, opts ClientOptions) (*SSHClient, error) {
	if opts.Port <= 0 {
		opts.Port = 22
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.Username == "" {
		opts.Username = "root"
	}

	var authMethods []ssh.AuthMethod
	switch opts.AuthType {
	case "password":
		authMethods = append(authMethods, ssh.Password(opts.Secret))
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(opts.Secret))
		if err != nil {
			return nil, fmt.Errorf("invalid SSH private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	case "key_passphrase":
		signer, err := ssh.ParsePrivateKeyWithPassphrase([]byte(opts.Secret), []byte(opts.Passphrase))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt SSH private key with passphrase: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", opts.AuthType)
	}

	// Strictly verify known_hosts - NEVER ignore host key!
	hostKeyCallback, err := knownhosts.New(opts.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            opts.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         opts.Timeout,
	}

	if opts.Port <= 0 {
		opts.Port = 22
	}

	dialAddr := net.JoinHostPort(opts.Host, fmt.Sprintf("%d", opts.Port))
	dialer := &net.Dialer{Timeout: opts.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", dialAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial TCP connection to %s: %w", dialAddr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, dialAddr, sshConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SSH handshake failed on %s: %w", dialAddr, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	sudoPass := opts.SudoPassword
	if sudoPass == "" && opts.AuthType == "password" {
		sudoPass = opts.Secret
	}
	sshClient := &SSHClient{
		client:        client,
		host:          opts.Host,
		port:          opts.Port,
		user:          opts.Username,
		sudoPassword:  sudoPass,
		stopKeepAlive: make(chan struct{}),
	}
	sshClient.startKeepAlive(25 * time.Second)
	return sshClient, nil
}

func (c *SSHClient) startKeepAlive(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		failCount := 0
		for {
			select {
			case <-c.stopKeepAlive:
				return
			case <-ticker.C:
				c.mu.Lock()
				cl := c.client
				c.mu.Unlock()
				if cl == nil {
					return
				}
				_, _, err := cl.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					failCount++
					if failCount >= 3 {
						return
					}
				} else {
					failCount = 0
				}
			}
		}
	}()
}

// WithLock wraps a command inside an advisory lock using remote flock
// to prevent concurrent package manager execution (e.g. apt/dpkg locks).
func (c *SSHClient) WithLock(lockName string, timeoutSeconds int, cmd string) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	safeName := regexp.MustCompile(`[^a-zA-Z0-9_\-]`).ReplaceAllString(lockName, "_")
	lockFile := fmt.Sprintf("/tmp/tamestack_%s.lock", safeName)

	return fmt.Sprintf(`
LOCKFILE=%q
if command -v flock >/dev/null 2>&1; then
    exec 200>"$LOCKFILE"
    if ! flock -w %d 200; then
        echo "TameStack Lock Error: unable to acquire exclusive lock for %s within %ds" >&2
        exit 75
    fi
fi
%s
`, lockFile, timeoutSeconds, safeName, timeoutSeconds, cmd)
}

// SetSudoPassword updates the cached sudo password.
func (c *SSHClient) SetSudoPassword(pass string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if pass == c.sudoPassword {
		return
	}
	c.sudoPassword = pass
	c.sudoAskpass = ""
	c.askpassDir = ""
}

// GetSudoPassword returns the cached sudo password.
func (c *SSHClient) GetSudoPassword() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sudoPassword
}

func (c *SSHClient) conn() (*ssh.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil, errors.New("ssh client is not connected")
	}
	return c.client, nil
}

// SudoHeader returns a shell preamble defining run_sudo and exporting $SUDO.
// The sudo password is never embedded in the script. Exec installs an askpass
// helper over stdin and exports SUDO_ASKPASS before the command runs.
func (c *SSHClient) SudoHeader() string {
	c.mu.Lock()
	user := c.user
	hasPass := c.sudoPassword != ""
	c.mu.Unlock()

	if user == "root" {
		return `
run_sudo() {
    "$@"
}
SUDO="run_sudo"
`
	}

	if hasPass {
		return `
run_sudo() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif [ -n "${SUDO_ASKPASS:-}" ] && command -v sudo >/dev/null 2>&1; then
        sudo -A -p '' "$@"
    else
        echo "TameStack: sudo askpass is not available" >&2
        return 77
    fi
}
SUDO="run_sudo"
`
	}

	return `
run_sudo() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
        sudo -n "$@"
    else
        echo "TameStack: sudo password is not set and passwordless sudo is unavailable" >&2
        return 77
    fi
}
SUDO="run_sudo"
`
}

func (c *SSHClient) withAskpassExport(cmd string) string {
	c.mu.Lock()
	ask := c.sudoAskpass
	user := c.user
	hasPass := c.sudoPassword != ""
	c.mu.Unlock()
	if user == "root" || !hasPass || ask == "" || strings.Contains(cmd, "SUDO_ASKPASS=") {
		return cmd
	}
	if !strings.Contains(cmd, "run_sudo") && !strings.Contains(cmd, "$SUDO") && !strings.Contains(cmd, "sudo") {
		return cmd
	}
	return "export SUDO_ASKPASS=" + ShellEscape(ask) + "\n" + cmd
}

func (c *SSHClient) ensureAskpass(ctx context.Context) error {
	c.mu.Lock()
	if c.user == "root" || c.sudoPassword == "" || c.sudoAskpass != "" {
		c.mu.Unlock()
		return nil
	}
	pass := c.sudoPassword
	c.mu.Unlock()

	if strings.ContainsAny(pass, "\r\n") {
		return errors.New("sudo password contains a newline")
	}
	body := "#!/bin/sh\nprintf '%s\\n' " + ShellEscape(pass) + "\n"
	script := `umask 077
d=$(mktemp -d /tmp/.sh_ask.XXXXXX) || exit 1
cat > "$d/askpass" || exit 1
chmod 700 "$d/askpass" || exit 1
printf '%s\n' "$d"
`
	out, stderr, code, err := c.execRaw(ctx, script, strings.NewReader(body))
	if err != nil || code != 0 {
		return fmt.Errorf("failed to install sudo askpass: %s %v", stderr, err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" || strings.ContainsAny(dir, "\r\n'\"$`;|&") {
		return fmt.Errorf("unexpected sudo askpass path")
	}
	c.mu.Lock()
	if c.sudoAskpass == "" {
		c.askpassDir = dir
		c.sudoAskpass = dir + "/askpass"
	}
	c.mu.Unlock()
	return nil
}

// InstallMySQLDefaults writes a mode-600 client defaults file over stdin.
// The password is not placed on the remote command line.
func (c *SSHClient) InstallMySQLDefaults(ctx context.Context, password string) (string, error) {
	if password == "" {
		return "", nil
	}
	if strings.ContainsAny(password, "\r\n") {
		return "", errors.New("mysql password contains a newline")
	}
	body := "[client]\nuser=root\npassword=" + password + "\n"
	script := `umask 077
f=$(mktemp /tmp/.sh_my.XXXXXX) || exit 1
cat > "$f" || exit 1
chmod 600 "$f" || exit 1
printf '%s\n' "$f"
`
	out, stderr, code, err := c.execRaw(ctx, script, strings.NewReader(body))
	if err != nil || code != 0 {
		return "", fmt.Errorf("failed to write mysql defaults file: %s %v", stderr, err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" || strings.ContainsAny(path, "\r\n'\"$`;|&") {
		return "", errors.New("unexpected mysql defaults path")
	}
	return path, nil
}

// ExecStdin runs a remote command with the provided standard input.
// Use it for SQL and other secrets that must not appear in argv.
func (c *SSHClient) ExecStdin(ctx context.Context, cmd string, stdin io.Reader) (stdout []byte, stderr []byte, exitCode int, err error) {
	if err := c.ensureAskpass(ctx); err != nil {
		return nil, nil, -1, err
	}
	return c.execRaw(ctx, c.withAskpassExport(cmd), stdin)
}

// Exec runs a remote shell command with context timeout and output capturing.
func (c *SSHClient) Exec(ctx context.Context, cmd string) (stdout []byte, stderr []byte, exitCode int, err error) {
	if err := c.ensureAskpass(ctx); err != nil {
		return nil, nil, -1, err
	}
	return c.execRaw(ctx, c.withAskpassExport(cmd), nil)
}

func (c *SSHClient) execRaw(ctx context.Context, cmd string, stdin io.Reader) (stdout []byte, stderr []byte, exitCode int, err error) {
	cl, err := c.conn()
	if err != nil {
		return nil, nil, -1, err
	}

	session, err := cl.NewSession()
	if err != nil {
		return nil, nil, -1, fmt.Errorf("failed to open SSH session: %w", err)
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	if stdin != nil {
		session.Stdin = stdin
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		abortSession(session)
		// The runner goroutine may still be writing into the buffers; wait for it
		// to finish before reading them, otherwise the caller races with session.Run.
		<-errChan
		c.removeAskpass()
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), -1, ctx.Err()
	case runErr := <-errChan:
		if runErr != nil {
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitStatusFromError(runErr), runErr
		}
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), 0, nil
	}
}

// abortSession terminates a running remote command and unblocks its I/O goroutines.
// SIGKILL alone is best-effort (OpenSSH usually ignores signals), so the session is
// also closed, which makes Run/Wait and the pipe readers return.
func abortSession(session *ssh.Session) {
	_ = session.Signal(ssh.SIGKILL)
	_ = session.Close()
}

func exitStatusFromError(err error) int {
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus()
	}
	return 1
}

// ExecStream runs a remote command and streams stdout and stderr line-by-line via callbacks.
func (c *SSHClient) ExecStream(ctx context.Context, cmd string, onStdout, onStderr func(string)) (exitCode int, err error) {
	if err := c.ensureAskpass(ctx); err != nil {
		return -1, err
	}
	cmd = c.withAskpassExport(cmd)
	cl, err := c.conn()
	if err != nil {
		return -1, err
	}

	session, err := cl.NewSession()
	if err != nil {
		return -1, fmt.Errorf("failed to open SSH session: %w", err)
	}
	defer session.Close()

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return -1, fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return -1, fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		return -1, fmt.Errorf("failed to start remote command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			if onStdout != nil {
				onStdout(scanner.Text())
			}
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			if onStderr != nil {
				onStderr(scanner.Text())
			}
		}
	}()

	waitChan := make(chan error, 1)
	go func() {
		wg.Wait()
		waitChan <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		abortSession(session)
		// Wait for the scanner goroutines and session.Wait to finish so no
		// stdout/stderr callback can fire after ExecStream has returned.
		<-waitChan
		return -1, ctx.Err()
	case waitErr := <-waitChan:
		if waitErr != nil {
			return exitStatusFromError(waitErr), waitErr
		}
		return 0, nil
	}
}

// SFTP initializes or returns the active SFTP client.
func (c *SSHClient) SFTP() (*sftp.Client, error) {
	return c.getSFTP()
}

// getSFTP initializes or returns active SFTP client.
func (c *SSHClient) getSFTP() (*sftp.Client, error) {
	c.mu.Lock()
	if c.sftpClient != nil {
		sc := c.sftpClient
		c.mu.Unlock()
		return sc, nil
	}
	cl := c.client
	c.mu.Unlock()
	if cl == nil {
		return nil, errors.New("ssh client is not connected")
	}

	sc, err := sftp.NewClient(cl)
	if err != nil {
		return nil, fmt.Errorf("failed to start sftp subsystem: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		_ = sc.Close()
		return nil, errors.New("ssh client is not connected")
	}
	if c.sftpClient != nil {
		_ = sc.Close()
		return c.sftpClient, nil
	}
	c.sftpClient = sc
	return sc, nil
}

// sudoWriteScript builds the elevated copy-in script used when SFTP cannot write directly.
// Every remote path is single-quoted so that $(), backticks or $VAR inside a filename stay
// literal instead of being expanded by the remote shell.
func (c *SSHClient) sudoWriteScript(remoteDir, remotePath, tmpName string, mode os.FileMode) string {
	return fmt.Sprintf(`%s
$SUDO mkdir -p %s 2>/dev/null
$SUDO cp %s %s
$SUDO chmod %04o %s
rm -f %s
`, c.SudoHeader(),
		ShellEscape(filepath.ToSlash(remoteDir)),
		ShellEscape(tmpName),
		ShellEscape(filepath.ToSlash(remotePath)),
		mode,
		ShellEscape(filepath.ToSlash(remotePath)),
		ShellEscape(tmpName))
}

// sudoReadScript builds the elevated head-read script for one absolute path.
func (c *SSHClient) sudoReadScript(remotePath string) string {
	quoted := ShellEscape(filepath.ToSlash(remotePath))
	return fmt.Sprintf(`%s
if $SUDO test -f %s; then
    $SUDO head -c 10485760 %s 2>/dev/null
fi
`, c.SudoHeader(), quoted, quoted)
}

// WriteFile writes content to a remote file path with specified permissions.
// If direct SFTP write fails with permission denied, it automatically attempts elevation via sudo.
func (c *SSHClient) WriteFile(ctx context.Context, remotePath string, content []byte, mode os.FileMode) error {
	if err := ValidateRemotePath(remotePath, false); err != nil {
		return err
	}

	sc, err := c.getSFTP()
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(remotePath)
	mkdirErr := sc.MkdirAll(filepath.ToSlash(remoteDir))

	f, openErr := sc.OpenFile(filepath.ToSlash(remotePath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if openErr != nil || mkdirErr != nil {
		// Fallback for permission-denied directories or root-owned files:
		// Write to a temporary file in /tmp and copy it into place through the sudo wrapper
		tmpName := fmt.Sprintf("/tmp/.sh_write_%d_%d", time.Now().UnixNano(), os.Getpid())
		tf, tmpErr := sc.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if tmpErr == nil {
			_, writeErr := tf.Write(content)
			tf.Close()
			if writeErr == nil {
				_ = sc.Chmod(tmpName, mode)
				cmd := c.sudoWriteScript(remoteDir, remotePath, tmpName, mode)
				out, stderr, code, execErr := c.Exec(ctx, cmd)
				if execErr == nil && code == 0 {
					return nil
				}
				return fmt.Errorf("failed to write %s: %s %s", remotePath, string(out), string(stderr))
			}
			_ = sc.Remove(tmpName)
		}
		if openErr != nil {
			return fmt.Errorf("failed to open remote file %s: %w", remotePath, openErr)
		}
		return fmt.Errorf("failed to create directory %s: %w", remoteDir, mkdirErr)
	}
	defer f.Close()

	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("failed to write content to %s: %w", remotePath, err)
	}

	return sc.Chmod(filepath.ToSlash(remotePath), mode)
}

// ReadFile reads bytes from a remote file up to a maximum limit (e.g. 10MB).
// If direct SFTP read fails with permission denied, it automatically attempts elevation via sudo.
func (c *SSHClient) ReadFile(ctx context.Context, remotePath string) ([]byte, error) {
	if err := ValidateRemotePath(remotePath, false); err != nil {
		return nil, err
	}

	sc, err := c.getSFTP()
	if err != nil {
		return nil, err
	}

	f, openErr := sc.Open(filepath.ToSlash(remotePath))
	if openErr != nil {
		// Fallback for permission-denied files: read via sudo.
		out, _, code, execErr := c.Exec(ctx, c.sudoReadScript(remotePath))
		if execErr == nil && code == 0 {
			return out, nil
		}
		return nil, fmt.Errorf("failed to open remote file %s: %w", remotePath, openErr)
	}
	defer f.Close()

	// Limit read size to 10MB to protect memory.
	limitedReader := io.LimitReader(f, 10*1024*1024)
	return io.ReadAll(limitedReader)
}

// IsAlive checks if the SSH transport is currently responding.
func (c *SSHClient) IsAlive() bool {
	cl, err := c.conn()
	if err != nil {
		return false
	}
	_, _, err = cl.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

// NewSession creates an interactive SSH session for PTY and interactive commands.
func (c *SSHClient) NewSession() (*ssh.Session, error) {
	cl, err := c.conn()
	if err != nil {
		return nil, err
	}
	return cl.NewSession()
}

// Close gracefully terminates SFTP and SSH connections.
func (c *SSHClient) removeAskpass() {
	c.mu.Lock()
	dir := c.askpassDir
	cl := c.client
	c.askpassDir = ""
	c.sudoAskpass = ""
	c.mu.Unlock()
	if dir == "" || cl == nil || strings.ContainsAny(dir, "\r\n") {
		return
	}
	_, _, _, _ = c.execRaw(context.Background(), "rm -rf "+ShellEscape(dir), nil)
}

func (c *SSHClient) Close() error {
	c.removeAskpass()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopKeepAlive != nil {
		select {
		case <-c.stopKeepAlive:
		default:
			close(c.stopKeepAlive)
		}
	}

	var errs []error
	if c.sftpClient != nil {
		if err := c.sftpClient.Close(); err != nil {
			errs = append(errs, err)
		}
		c.sftpClient = nil
	}
	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, err)
		}
		c.client = nil
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
