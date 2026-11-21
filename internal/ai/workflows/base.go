package workflows

import (
	"context"
	"fmt"
	"time"

	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
	"tamestack/internal/ssh"
)

// workflowBase carries the dependencies and preamble shared by the diagnostic
// workflows: server lookup, report scaffolding and SSH dialing.
type workflowBase struct {
	database   *db.DB
	secService *security.Service
	vault      *crypto.Vault
	pool       *ssh.Pool
	knownHosts string
}

// lookupServer resolves the server record or fails with a user-facing message.
func (b *workflowBase) lookupServer(ctx context.Context, serverID string) (*db.ServerRecord, error) {
	srv, err := b.database.GetServer(ctx, serverID)
	if err != nil || srv == nil {
		return nil, fmt.Errorf("сервер %s не найден", serverID)
	}
	return srv, nil
}

// newReport builds the common DiagnosticReport skeleton for a workflow run.
func (b *workflowBase) newReport(workflowType, serverID string, srv *db.ServerRecord, target string) *DiagnosticReport {
	return &DiagnosticReport{
		WorkflowType: workflowType,
		ServerID:     serverID,
		ServerName:   srv.Name,
		Target:       target,
		Timestamp:    time.Now().UTC(),
		Status:       "healthy",
		Issues:       make([]Issue, 0),
		Metrics:      make(map[string]any),
	}
}

// dialSSH decrypts the server credentials and returns a pooled SSH connection.
// secretNoun names the decrypted secret in the error message ("секрета"/"ключа").
func (b *workflowBase) dialSSH(ctx context.Context, serverID string, srv *db.ServerRecord, secretNoun string) (*ssh.SSHClient, error) {
	opts, err := ssh.OptionsForServer(b.vault, srv, b.knownHosts, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("ошибка расшифровки SSH %s: %w", secretNoun, err)
	}
	client, err := b.pool.Get(ctx, serverID, opts)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к SSH: %w", err)
	}
	return client, nil
}
