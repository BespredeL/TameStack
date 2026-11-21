package security

import (
	"context"
	"fmt"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

type Service struct {
	cfg      *config.AppConfig
	database *db.DB
	vault    *crypto.Vault
	pool     *ssh.Pool
	engine   *tasks.Engine
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		cfg:      cfg,
		database: database,
		vault:    vault,
		pool:     pool,
		engine:   engine,
	}

	engine.RegisterHandler("security_firewall_toggle", s.handleFirewallToggleTask)
	engine.RegisterHandler("security_firewall_install", s.handleFirewallInstallTask)
	engine.RegisterHandler("security_firewall_add_rule", s.handleFirewallAddRuleTask)
	engine.RegisterHandler("security_firewall_update_rule", s.handleFirewallUpdateRuleTask)
	engine.RegisterHandler("security_firewall_del_rule", s.handleFirewallDelRuleTask)
	engine.RegisterHandler("security_fail2ban_unban", s.handleFail2banUnbanTask)
	engine.RegisterHandler("security_fail2ban_ban", s.handleFail2banBanTask)
	engine.RegisterHandler("security_fail2ban_control", s.handleFail2banControlTask)
	engine.RegisterHandler("security_fail2ban_install", s.handleFail2banInstallTask)
	engine.RegisterHandler("security_fail2ban_save_jail", s.handleFail2banSaveJailTask)
	engine.RegisterHandler("security_fail2ban_delete_jail", s.handleFail2banDeleteJailTask)
	engine.RegisterHandler("security_fail2ban_toggle_jail", s.handleFail2banToggleJailTask)
	engine.RegisterHandler("security_apply_hardening", s.handleApplyHardeningTask)
	engine.RegisterHandler("security_repair_web_access", s.handleRepairWebAccessTask)

	return s
}

func (s *Service) getSSHClient(ctx context.Context, serverID string) (*ssh.SSHClient, *db.ServerRecord, error) {
	return module.GetSSHClient(ctx, s.cfg, s.database, s.vault, s.pool, serverID)
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string) (string, error) {
	return module.Exec(ctx, client, cmd, true)
}

// execReadOnly runs a command script that this package owns. Callers outside security must
// go through the typed read-only methods in readonly.go, never through an arbitrary string.
func (s *Service) execReadOnly(ctx context.Context, serverID, cmd string) (string, error) {
	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}
	stdout, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		if len(stdout) > 0 {
			return string(stdout), nil
		}
		if len(stderr) > 0 {
			return string(stderr), nil
		}
		return "", fmt.Errorf("command exited with code %d: %w", code, err)
	}
	return string(stdout), nil
}
