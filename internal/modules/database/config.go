package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tamestack/internal/ssh"
)

// serviceCandidates lists systemd unit names per engine in probe order.
func serviceCandidates(engine EngineType) []string {
	switch engine {
	case EngineMySQL:
		return []string{"mysql", "mysqld", "mariadb"}
	case EngineMariaDB:
		return []string{"mariadb", "mysql", "mysqld"}
	case EnginePostgreSQL:
		return []string{"postgresql"}
	case EngineRedis:
		return []string{"redis-server", "redis"}
	default:
		return nil
	}
}

func validateEngine(engine EngineType) error {
	if len(serviceCandidates(engine)) == 0 {
		return fmt.Errorf("неподдерживаемая СУБД: %s", engine)
	}
	return nil
}

// serviceControlScript builds a systemctl script that picks the first existing unit.
func serviceControlScript(engine EngineType, action string) (string, error) {
	if err := validateEngine(engine); err != nil {
		return "", err
	}
	if err := ssh.ValidateSystemctlAction(action); err != nil {
		return "", err
	}
	return fmt.Sprintf(`action=%s
for svc in %s; do
    if systemctl list-unit-files "$svc.service" 2>/dev/null | grep -q "$svc.service"; then
        systemctl "$action" "$svc"
        exit $?
    fi
done
echo "service unit not found for %s" >&2
exit 1
`, ssh.ShellEscape(action), strings.Join(serviceCandidates(engine), " "), ssh.ShellEscape(string(engine))), nil
}

func (s *Service) isServiceActive(ctx context.Context, client *ssh.SSHClient, engine EngineType) bool {
	script := fmt.Sprintf(`for svc in %s; do
    if systemctl is-active --quiet "$svc" 2>/dev/null; then
        echo active
        exit 0
    fi
done
echo inactive
`, strings.Join(serviceCandidates(engine), " "))
	out, _, _, _ := client.NewCommand(script).WithSudo().Exec(ctx)
	return strings.TrimSpace(string(out)) == "active"
}

// detectConfigPath finds the main configuration file of an installed engine.
func (s *Service) detectConfigPath(ctx context.Context, client *ssh.SSHClient, engine EngineType) string {
	switch engine {
	case EngineMySQL, EngineMariaDB:
		script := `for cf in /etc/mysql/mariadb.conf.d/50-server.cnf /etc/mysql/mysql.conf.d/mysqld.cnf /etc/my.cnf.d/mariadb-server.cnf /etc/my.cnf.d/mysql-server.cnf /etc/my.cnf /etc/mysql/my.cnf; do
    if [ -f "$cf" ]; then echo "$cf"; exit 0; fi
done`
		out, _, _, _ := s.execCmd(ctx, client, script)
		return strings.TrimSpace(out)
	case EnginePostgreSQL:
		out, _, code, _ := s.execCmd(ctx, client, "sudo -u postgres psql -t -A -c 'SHOW config_file;' 2>/dev/null")
		if code == 0 && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out)
		}
		script := `for cf in $(ls -1 /etc/postgresql/*/main/postgresql.conf 2>/dev/null | sort -V | tail -n 1) /var/lib/pgsql/data/postgresql.conf; do
    if [ -f "$cf" ]; then echo "$cf"; exit 0; fi
done`
		out, _, _, _ = s.execCmd(ctx, client, script)
		return strings.TrimSpace(out)
	case EngineRedis:
		script := `for cf in /etc/redis/redis.conf /etc/redis.conf /etc/redis/6379.conf; do
    if [ -f "$cf" ]; then echo "$cf"; exit 0; fi
done`
		out, _, _, _ := s.execCmd(ctx, client, script)
		return strings.TrimSpace(out)
	default:
		return ""
	}
}

// GetConfig reads an engine configuration file via SFTP.
func (s *Service) GetConfig(ctx context.Context, serverID, remotePath string) (string, error) {
	if err := ssh.ValidateRemotePath(remotePath, false); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}
	data, err := client.ReadFile(ctx, remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", remotePath, err)
	}
	return string(data), nil
}

// SaveConfig backs up the file, writes the new content and restarts the service.
// If the restart fails, the backup is restored and the service restarted again.
// When the service is not running, the file is saved without touching the service.
func (s *Service) SaveConfig(ctx context.Context, serverID string, engine EngineType, remotePath, content string) error {
	if err := validateEngine(engine); err != nil {
		return err
	}
	if err := ssh.ValidateRemotePath(remotePath, false); err != nil {
		return err
	}
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	wasActive := s.isServiceActive(ctx, client, engine)

	backupPath := fmt.Sprintf("%s.bak.%d", remotePath, time.Now().Unix())
	tmpPath := fmt.Sprintf("/tmp/TameStack_db_%d.conf", time.Now().UnixNano())

	if _, _, _, err := client.NewCommand(fmt.Sprintf("cp %s %s", ssh.ShellEscape(remotePath), ssh.ShellEscape(backupPath))).WithSudo().Exec(ctx); err != nil {
		return fmt.Errorf("не удалось создать резервную копию %s: %w", remotePath, err)
	}

	if err := client.WriteFile(ctx, tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("не удалось записать временный файл: %w", err)
	}
	defer client.RemovePath(ctx, tmpPath, false)

	if out, stderr, code, err := client.NewCommand(fmt.Sprintf("cp %s %s", ssh.ShellEscape(tmpPath), ssh.ShellEscape(remotePath))).WithSudo().Exec(ctx); err != nil || code != 0 {
		return fmt.Errorf("не удалось сохранить конфигурацию (code %d): %s %s", code, string(out), string(stderr))
	}

	if !wasActive {
		return nil
	}

	restartCmd, err := serviceControlScript(engine, "restart")
	if err != nil {
		return err
	}
	out, stderr, code, err := client.NewCommand(restartCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		rollbackCmd := fmt.Sprintf("cp %s %s", ssh.ShellEscape(backupPath), ssh.ShellEscape(remotePath))
		_, _, _, _ = client.NewCommand(rollbackCmd).WithSudo().Exec(ctx)
		_, _, _, _ = client.NewCommand(restartCmd).WithSudo().Exec(ctx)
		return fmt.Errorf("служба не запустилась с новой конфигурацией, изменения откатаны:\n%s %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(stderr)))
	}

	return nil
}
