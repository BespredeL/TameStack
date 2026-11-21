package ssh

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveRemotePath resolves symlinks on the remote host so that policy checks run
// against the real target instead of the name the caller supplied.
func (c *SSHClient) ResolveRemotePath(ctx context.Context, remotePath string) (string, error) {
	if err := ValidateRemotePath(remotePath, false); err != nil {
		return "", err
	}

	cmd := fmt.Sprintf("readlink -f -- %s", ShellEscape(filepath.ToSlash(remotePath)))
	stdout, stderr, code, err := c.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return "", fmt.Errorf("failed to resolve %s: %s %s %w", remotePath, stdout, stderr, err)
	}

	resolved := strings.TrimSpace(string(stdout))
	if err := ValidateRemotePath(resolved, false); err != nil {
		return "", fmt.Errorf("resolved path of %s is not usable: %w", remotePath, err)
	}
	return resolved, nil
}
