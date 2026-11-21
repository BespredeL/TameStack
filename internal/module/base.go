// Package module provides the shared scaffolding for backend service modules:
// dependency holder, SSH connector and privileged command helpers.
//
// Language policy: errors and messages that reach the user through task results
// are Russian; internal diagnostics and log records are English.
package module

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

// Base carries the dependencies every service module needs. Modules embed it
// to inherit GetSSHClient; the security module deliberately does not embed it
// so that no exec-style method is ever promoted onto its public surface.
type Base struct {
	Cfg      *config.AppConfig
	Database db.Store
	Vault    *crypto.Vault
	Pool     *ssh.Pool
}

// GetSSHClient resolves the server record and returns a pooled SSH connection.
func (b *Base) GetSSHClient(ctx context.Context, serverID string) (*ssh.SSHClient, *db.ServerRecord, error) {
	return GetSSHClient(ctx, b.Cfg, b.Database, b.Vault, b.Pool, serverID)
}

// GetSSHClient is the connector shared by all modules and the app facade.
func GetSSHClient(ctx context.Context, cfg *config.AppConfig, database db.ServerStore, vault *crypto.Vault, pool *ssh.Pool, serverID string) (*ssh.SSHClient, *db.ServerRecord, error) {
	record, err := database.GetServer(ctx, serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("сервер %s не найден: %w", serverID, err)
	}

	opts, err := ssh.OptionsForServer(vault, record, cfg.KnownHostsPath, 15*time.Second)
	if err != nil {
		return nil, nil, err
	}

	client, err := pool.Get(ctx, record.ID, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка SSH-подключения: %w", err)
	}

	return client, record, nil
}

// RunPrivileged runs cmd with sudo under the package-manager lock so concurrent
// installs cannot collide, and keeps apt from prompting for input.
func RunPrivileged(ctx context.Context, client *ssh.SSHClient, cmd string) error {
	_, stderr, code, err := client.NewCommand(cmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("команда %s завершилась с кодом %d: %s", cmd, code, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// Exec runs cmd optionally with sudo and folds the exit code into the error,
// returning stdout even on failure so callers can inspect partial output.
func Exec(ctx context.Context, client *ssh.SSHClient, cmd string, sudo bool) (string, error) {
	b := client.NewCommand(cmd)
	if sudo {
		b = b.WithSudo()
	}
	stdout, stderr, code, err := b.Exec(ctx)
	if err != nil {
		if len(stderr) > 0 {
			return string(stdout), fmt.Errorf("%w: %s", err, string(stderr))
		}
		return string(stdout), err
	}
	if code != 0 {
		return string(stdout), fmt.Errorf("команда завершилась с кодом %d: %s", code, string(stderr))
	}
	return string(stdout), nil
}

// SudoPreamble is the shell header for scripts that must work under root,
// passwordless sudo, interactive sudo or the run_sudo askpass helper.
const SudoPreamble = `export PATH=$PATH:/snap/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
do_sudo() {
    if command -v run_sudo >/dev/null 2>&1; then
        run_sudo "$@"
    elif [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1; then
        sudo -n "$@" 2>/dev/null || sudo "$@"
    else
        "$@"
    fi
}
`
