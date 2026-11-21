package security

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/osadapter"
	"tamestack/internal/tasks"
)

// ApplyHardeningFix applies a quick security fix.
func (s *Service) ApplyHardeningFix(ctx context.Context, serverID string, fixType string) (*db.TaskRecord, error) {
	titleMap := map[string]string{
		"ssh_root_login":    "Отключение SSH Root Login",
		"ssh_password_auth": "Отключение входа по SSH паролю",
		"security_updates":  "Установка обновлений безопасности",
		"firewall_active":   "Быстрая активация брандмауэра",
		"fail2ban_active":   "Установка и активация Fail2ban",
	}

	title, ok := titleMap[fixType]
	if !ok {
		return nil, fmt.Errorf("неизвестный тип исправления: %s", fixType)
	}

	payload, _ := json.Marshal(map[string]string{
		"server_id": serverID,
		"fix_type":  fixType,
	})

	return s.engine.Submit(serverID, "security_apply_hardening", title, string(payload))
}

func (s *Service) handleApplyHardeningTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID string `json:"server_id"`
		FixType  string `json:"fix_type"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, record, err := s.getSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	switch payload.FixType {
	case "ssh_root_login":
		logger.SetStep("Создание резервной копии sshd_config", 20)
		_, _ = s.execCmd(ctx, client, "cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak_TameStack")

		logger.SetStep("Применение PermitRootLogin no", 50)
		checkDir, _ := s.execCmd(ctx, client, "test -d /etc/ssh/sshd_config.d && echo 'yes'")
		if strings.TrimSpace(checkDir) == "yes" {
			if _, err := s.execCmd(ctx, client, "printf '%s\\n' 'PermitRootLogin no' | tee /etc/ssh/sshd_config.d/99-TameStack-root.conf >/dev/null"); err != nil {
				return "", fmt.Errorf("не удалось записать sshd drop-in: %w", err)
			}
		} else {
			if _, err := s.execCmd(ctx, client, "sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config"); err != nil {
				return "", fmt.Errorf("не удалось обновить sshd_config: %w", err)
			}
		}

		logger.SetStep("Проверка синтаксиса sshd и перезагрузка", 80)
		out, err := s.execCmd(ctx, client, "sshd -t && (systemctl reload sshd || systemctl reload ssh)")
		if err != nil {
			_, _ = s.execCmd(ctx, client, "cp /etc/ssh/sshd_config.bak_TameStack /etc/ssh/sshd_config")
			return "", fmt.Errorf("ошибка валидации sshd_config: %w: %s", err, out)
		}
		logger.Info("Вход под root по SSH успешно отключен.")
		return "PermitRootLogin установлен в no", nil

	case "ssh_password_auth":
		logger.SetStep("Создание резервной копии sshd_config", 20)
		_, _ = s.execCmd(ctx, client, "cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak_TameStack")

		logger.SetStep("Применение PasswordAuthentication no", 50)
		checkDir, _ := s.execCmd(ctx, client, "test -d /etc/ssh/sshd_config.d && echo 'yes'")
		if strings.TrimSpace(checkDir) == "yes" {
			if _, err := s.execCmd(ctx, client, "printf '%s\\n' 'PasswordAuthentication no' | tee /etc/ssh/sshd_config.d/99-TameStack-pwauth.conf >/dev/null"); err != nil {
				return "", fmt.Errorf("не удалось записать sshd drop-in: %w", err)
			}
		} else {
			if _, err := s.execCmd(ctx, client, "sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config"); err != nil {
				return "", fmt.Errorf("не удалось обновить sshd_config: %w", err)
			}
		}

		logger.SetStep("Проверка синтаксиса sshd и перезагрузка", 80)
		out, err := s.execCmd(ctx, client, "sshd -t && (systemctl reload sshd || systemctl reload ssh)")
		if err != nil {
			_, _ = s.execCmd(ctx, client, "cp /etc/ssh/sshd_config.bak_TameStack /etc/ssh/sshd_config")
			return "", fmt.Errorf("ошибка валидации sshd_config: %w: %s", err, out)
		}
		logger.Info("Вход по паролю отключен. Доступ разрешен только по ключам.")
		return "PasswordAuthentication установлен в no", nil

	case "security_updates":
		logger.SetStep("Установка обновлений безопасности", 30)
		adapter, _, err := osadapter.DetectAndCreateAdapter(ctx, client)
		if err != nil {
			return "", fmt.Errorf("ошибка определения ОС: %w", err)
		}
		out, err := adapter.SecurityUpgrade(ctx, client)
		if err != nil {
			return "", fmt.Errorf("ошибка обновления: %w: %s", err, out)
		}
		logger.Info("Обновления безопасности успешно применены.")
		return "Обновления безопасности установлены", nil

	case "firewall_active":
		logger.SetStep("Включение брандмауэра с сохранением SSH порта", 50)
		sshPort := record.Port
		if sshPort == 0 {
			sshPort = 22
		}
		_, _ = s.execCmd(ctx, client, fmt.Sprintf("ufw allow %d/tcp comment 'SSH' && ufw --force enable", sshPort))
		return "Брандмауэр включен", nil

	case "fail2ban_active":
		return s.handleFail2banInstallTask(ctx, task, logger)
	}

	return "", fmt.Errorf("неподдерживаемый тип исправления")
}
