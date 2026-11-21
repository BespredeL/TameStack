package ssh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CommandBuilder provides a fluent, OOP-style interface for assembling and executing
// remote shell commands with safe escaping, sudo privilege handling, concurrency locks (flock),
// environment variables, and working directories.
type CommandBuilder struct {
	client      *SSHClient
	command     string
	useSudo     bool
	lockName    string
	lockTimeout int
	env         map[string]string
	workDir     string
}

// NewCommand initializes a CommandBuilder with a base command or script template.
func (c *SSHClient) NewCommand(baseCommand string) *CommandBuilder {
	return &CommandBuilder{
		client:  c,
		command: baseCommand,
		env:     make(map[string]string),
	}
}

// NewCommandWithArgs initializes a CommandBuilder with a binary and properly quoted arguments.
func (c *SSHClient) NewCommandWithArgs(binary string, args ...string) *CommandBuilder {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, binary)
	for _, arg := range args {
		parts = append(parts, ShellEscape(arg))
	}
	return c.NewCommand(strings.Join(parts, " "))
}

// WithSudo marks that the command should be run with elevated privileges (sudo).
func (b *CommandBuilder) WithSudo() *CommandBuilder {
	b.useSudo = true
	return b
}

// WithLock wraps the command in an advisory lock (flock) to prevent concurrent execution on the remote host.
func (b *CommandBuilder) WithLock(name string, timeoutSeconds int) *CommandBuilder {
	b.lockName = name
	b.lockTimeout = timeoutSeconds
	return b
}

// WithEnv sets an environment variable to be exported for the command.
func (b *CommandBuilder) WithEnv(key, value string) *CommandBuilder {
	b.env[key] = value
	return b
}

// WithWorkDir sets the directory in which the command should run.
func (b *CommandBuilder) WithWorkDir(dir string) *CommandBuilder {
	b.workDir = dir
	return b
}

// ShellEscape quotes a string safely for POSIX shell.
func ShellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// Build generates the complete POSIX shell script string.
func (b *CommandBuilder) Build() string {
	var sb strings.Builder

	if b.useSudo || strings.Contains(b.command, "$SUDO") || strings.Contains(b.command, "run_sudo") {
		if b.client != nil {
			sb.WriteString(b.client.SudoHeader())
			sb.WriteString("\n")
		}
	}

	if len(b.env) > 0 {
		keys := make([]string, 0, len(b.env))
		for k := range b.env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("export %s=%s\n", k, ShellEscape(b.env[k])))
		}
	}

	if b.workDir != "" {
		sb.WriteString(fmt.Sprintf("cd %s || exit 1\n", ShellEscape(b.workDir)))
	}

	// A lock requested together with sudo is taken inside the
	// elevated script so the SSH user does not have to create /var/run files.
	innerCmd := strings.TrimSpace(b.command)
	if b.lockName != "" && b.useSudo {
		innerCmd = lockScript(b.lockName, b.lockTimeout, innerCmd)
	}
	if b.useSudo && !strings.Contains(innerCmd, "$SUDO") && !strings.Contains(innerCmd, "run_sudo") {
		innerCmd = wrapRunSudo(innerCmd)
	}

	sb.WriteString(innerCmd)
	sb.WriteString("\n")

	fullScript := sb.String()

	// A lock without sudo uses /tmp, which the SSH user can create.
	if b.lockName != "" && !b.useSudo {
		return lockScript(b.lockName, b.lockTimeout, fullScript)
	}

	return fullScript
}

// wrapRunSudo keeps shell operators inside the elevated command.
// A bare "run_sudo apt-get update && apt-get install" elevates only the first segment.
func wrapRunSudo(inner string) string {
	if commandNeedsFullSudo(inner) {
		marker := "TameStack_SUDO_SCRIPT"
		for strings.Contains(inner, marker) {
			marker += "X"
		}
		return "run_sudo bash -s <<'" + marker + "'\n" + inner + "\n" + marker
	}
	return "run_sudo " + inner
}

func commandNeedsFullSudo(cmd string) bool {
	return strings.Contains(cmd, "\n") ||
		strings.Contains(cmd, "&&") ||
		strings.Contains(cmd, "||") ||
		strings.Contains(cmd, ";") ||
		strings.Contains(cmd, "|")
}

func lockScript(name string, timeoutSeconds int, body string) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	safeName := regexp.MustCompile(`[^a-zA-Z0-9_\-]`).ReplaceAllString(name, "_")
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
`, lockFile, timeoutSeconds, safeName, timeoutSeconds, body)
}

// Exec executes the built command on the remote host.
func (b *CommandBuilder) Exec(ctx context.Context) ([]byte, []byte, int, error) {
	if b.client == nil {
		return nil, nil, -1, fmt.Errorf("ssh client is nil")
	}
	return b.client.Exec(ctx, b.Build())
}

// ExecStream executes the built command on the remote host, streaming output lines.
func (b *CommandBuilder) ExecStream(ctx context.Context, onStdout, onStderr func(string)) (int, error) {
	if b.client == nil {
		return -1, fmt.Errorf("ssh client is nil")
	}
	return b.client.ExecStream(ctx, b.Build(), onStdout, onStderr)
}

// EnsureDirectory creates a remote directory if it doesn't exist, setting permissions and optional owner.
func (c *SSHClient) EnsureDirectory(ctx context.Context, path string, mode os.FileMode, owner string) error {
	sc, err := c.getSFTP()
	if err == nil {
		if err := sc.MkdirAll(filepath.ToSlash(path)); err == nil {
			if owner == "" {
				_ = sc.Chmod(filepath.ToSlash(path), mode)
				return nil
			}
		}
	}
	// Fallback to sudo execution if SFTP fails or owner needs to be set.
	var cmd string
	if owner != "" {
		cmd = fmt.Sprintf("mkdir -p %s && chmod %04o %s && chown %s %s",
			ShellEscape(path), mode, ShellEscape(path), ShellEscape(owner), ShellEscape(path))
	} else {
		cmd = fmt.Sprintf("mkdir -p %s && chmod %04o %s",
			ShellEscape(path), mode, ShellEscape(path))
	}
	out, stderr, code, execErr := c.NewCommand(cmd).WithSudo().Exec(ctx)
	if execErr != nil || code != 0 {
		return fmt.Errorf("failed to create directory %s: %s %s (%w)", path, string(out), string(stderr), execErr)
	}
	return nil
}

// FileExists checks whether a remote path exists using SFTP or sudo fallback.
func (c *SSHClient) FileExists(ctx context.Context, path string) bool {
	sc, err := c.getSFTP()
	if err == nil {
		if _, err := sc.Stat(filepath.ToSlash(path)); err == nil {
			return true
		}
	}
	_, _, code, _ := c.NewCommand(fmt.Sprintf("test -e %s", ShellEscape(path))).WithSudo().Exec(ctx)
	return code == 0
}

// RemovePath removes a remote file or directory, elevating via sudo if needed.
func (c *SSHClient) RemovePath(ctx context.Context, path string, recursive bool) error {
	sc, err := c.getSFTP()
	if err == nil && !recursive {
		if rmErr := sc.Remove(filepath.ToSlash(path)); rmErr == nil {
			return nil
		}
	}
	flag := "-f"
	if recursive {
		flag = "-rf"
	}
	cmd := fmt.Sprintf("rm %s %s", flag, ShellEscape(path))
	out, stderr, code, execErr := c.NewCommand(cmd).WithSudo().Exec(ctx)
	if execErr != nil || code != 0 {
		return fmt.Errorf("failed to remove %s: %s %s (%w)", path, string(out), string(stderr), execErr)
	}
	return nil
}
