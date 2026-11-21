package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"strings"

	"tamestack/internal/ai/policies"
	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

func allowedReloadCommand(cmd string) bool {
	switch strings.TrimSpace(cmd) {
	case "nginx -t", "apache2ctl -t", "httpd -t", "apache2ctl -t || httpd -t",
		"systemctl reload nginx", "systemctl reload apache2", "systemctl reload httpd",
		"systemctl reload apache2 || systemctl reload httpd":
		return true
	default:
		return false
	}
}

// TaskHandlerManager registers AI execution handlers on tasks.Engine.
type TaskHandlerManager struct {
	module.Base
}

// NewTaskHandlerManager creates a manager for AI task executions.
func NewTaskHandlerManager(
	cfg *config.AppConfig,
	database *db.DB,
	vault *crypto.Vault,
	pool *ssh.Pool,
) *TaskHandlerManager {
	return &TaskHandlerManager{
		Base: module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
	}
}

// RegisterHandlers registers AI handlers onto the task engine.
func (m *TaskHandlerManager) RegisterHandlers(engine *tasks.Engine) {
	engine.RegisterHandler("ai_apply_config_diff", m.handleApplyConfigDiff)
	engine.RegisterHandler("ai_restart_service", m.handleRestartService)
}

func (m *TaskHandlerManager) handleApplyConfigDiff(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload ConfigChangePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("ошибка разбора параметров задачи: %w", err)
	}

	gate := policies.NewSecurityGate()
	if err := gate.ValidatePath(payload.FilePath); err != nil {
		return "", err
	}
	if payload.BackupPath != "" {
		if err := gate.ValidatePath(payload.BackupPath); err != nil {
			return "", err
		}
	}

	logger.SetStep("Подключение к удаленному серверу по SSH", 15)
	client, _, err := m.GetSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	// The stored policy prefixes are only trustworthy against the real target, so every
	// path is resolved on the server and re-checked before anything is written.
	logger.SetStep("Разрешение символьных ссылок и повторная проверка политикой", 25)
	targetPath, err := resolveManagedPath(ctx, client, gate, payload.FilePath)
	if err != nil {
		return "", err
	}
	backupPath := ""
	if payload.BackupPath != "" {
		if backupPath, err = resolveManagedPath(ctx, client, gate, payload.BackupPath); err != nil {
			return "", err
		}
	}

	logger.SetStep(fmt.Sprintf("Создание резервной копии в %s", backupPath), 30)
	currentData, err := client.ReadFile(ctx, targetPath)
	if err == nil && len(currentData) > 0 {
		if bErr := client.WriteFile(ctx, backupPath, currentData, 0644); bErr != nil {
			logger.Warn(fmt.Sprintf("Не удалось создать бэкап: %v", bErr))
		} else {
			logger.Info(fmt.Sprintf("Резервная копия сохранена в %s", backupPath))
		}
	}

	logger.SetStep(fmt.Sprintf("Запись обновленной конфигурации в %s", targetPath), 55)
	if err := client.WriteFile(ctx, targetPath, []byte(payload.ProposedContent), 0644); err != nil {
		return "", fmt.Errorf("ошибка записи файла конфигурации: %w", err)
	}

	// Syntax check if configured.
	if payload.SyntaxCheckCommand != "" {
		if !allowedReloadCommand(payload.SyntaxCheckCommand) {
			return "", fmt.Errorf("команда проверки синтаксиса не входит в разрешенный список")
		}
		logger.SetStep(fmt.Sprintf("Проверка синтаксиса: %s", payload.SyntaxCheckCommand), 75)
		stdout, stderr, exitCode, err := client.NewCommand(payload.SyntaxCheckCommand).WithSudo().Exec(ctx)
		if err != nil || exitCode != 0 {
			logger.Error(fmt.Sprintf("Синтаксическая проверка не пройдена (код %d):\n%s\n%s", exitCode, stdout, stderr))
			logger.Warn("Выполняется автоматический откат к резервной копии...")
			if len(currentData) > 0 {
				_ = client.WriteFile(ctx, targetPath, currentData, 0644)
				logger.Info("Конфигурация успешно восстановлена из резервной копии.")
			}
			return "", fmt.Errorf("ошибка синтаксиса конфигурации, выполнен откат: %s %s", stdout, stderr)
		}
		logger.Info("Синтаксическая проверка успешно пройдена.")
	}

	// Service reload if configured.
	if payload.ReloadCommand != "" {
		if !allowedReloadCommand(payload.ReloadCommand) {
			return "", fmt.Errorf("команда перезагрузки не входит в разрешенный список")
		}
		logger.SetStep(fmt.Sprintf("Перезагрузка службы: %s", payload.ReloadCommand), 90)
		stdout, stderr, exitCode, err := client.NewCommand(payload.ReloadCommand).WithSudo().Exec(ctx)
		if err != nil || exitCode != 0 {
			return "", fmt.Errorf("перезагрузка службы не удалась (код %d): %s %s", exitCode, stdout, stderr)
		}
		logger.Info("Служба успешно перезагружена.")
	}

	logger.SetStep("Завершено", 100)
	return fmt.Sprintf("Конфигурация %s успешно обновлена и проверена.", targetPath), nil
}

// resolveManagedPath turns a caller-supplied path into the real target on the server and
// re-runs the security policy on it, so a symlink cannot point a managed write outside the
// allowed configuration directories.
func resolveManagedPath(ctx context.Context, client *ssh.SSHClient, gate *policies.SecurityGate, targetPath string) (string, error) {
	resolved, err := client.ResolveRemotePath(ctx, targetPath)
	if err != nil {
		return "", fmt.Errorf("не удалось разрешить путь %s на сервере: %w", targetPath, err)
	}
	if err := gate.ValidatePath(resolved); err != nil {
		return "", fmt.Errorf("путь %s указывает на %s: %w", targetPath, resolved, err)
	}
	return resolved, nil
}

func (m *TaskHandlerManager) handleRestartService(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload ServiceRestartPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("ошибка разбора параметров задачи: %w", err)
	}

	logger.SetStep("Подключение по SSH", 20)
	client, _, err := m.GetSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	if err := ssh.ValidateServiceName(payload.ServiceName); err != nil {
		return "", err
	}
	unit := ssh.ShellEscape(payload.ServiceName)

	logger.SetStep(fmt.Sprintf("Выполнение перезапуска службы %s", payload.ServiceName), 50)
	restartCmd := fmt.Sprintf("systemctl restart %s", unit)
	stdout, stderr, exitCode, err := client.NewCommand(restartCmd).WithSudo().Exec(ctx)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("ошибка перезапуска службы %s (код %d): %s %s", payload.ServiceName, exitCode, stdout, stderr)
	}

	logger.SetStep("Проверка статуса службы", 85)
	checkCmd := fmt.Sprintf("systemctl is-active %s", unit)
	out, _, _, _ := client.NewCommand(checkCmd).WithSudo().Exec(ctx)
	logger.Info(fmt.Sprintf("Служба %s активна (статус: %s)", payload.ServiceName, out))

	logger.SetStep("Завершено", 100)
	return fmt.Sprintf("Служба %s успешно перезапущена.", payload.ServiceName), nil
}
