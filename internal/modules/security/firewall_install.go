package security

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/osadapter"
	"tamestack/internal/tasks"
)

// FirewallInstallPlan previews a firewall installation without changing anything.
type FirewallInstallPlan struct {
	FirewallType      string   `json:"firewall_type"`
	OSFamily          string   `json:"os_family"`
	OSDistro          string   `json:"os_distro"`
	OSCompatible      bool     `json:"os_compatible"`
	PackagesToInstall []string `json:"packages_to_install"`
	ServiceToStart    string   `json:"service_to_start"`
	AlreadyInstalled  bool     `json:"already_installed"`
	Message           string   `json:"message"`
}

// FirewallInstallPayload is the task payload for security_firewall_install.
type FirewallInstallPayload struct {
	ServerID     string `json:"server_id"`
	FirewallType string `json:"firewall_type"`
}

func normalizeFirewallType(fwType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(fwType)) {
	case "ufw":
		return "ufw", nil
	case "firewalld", "firewall-cmd":
		return "firewalld", nil
	default:
		return "", fmt.Errorf("неподдерживаемый брандмауэр: %s (доступны ufw и firewalld)", fwType)
	}
}

func firewallInstallSpec(fwType string) (packages []string, service string) {
	if fwType == "ufw" {
		return []string{"ufw"}, "ufw"
	}
	return []string{"firewalld"}, "firewalld"
}

// PlanFirewallInstall detects the remote OS and previews the firewall installation.
func (s *Service) PlanFirewallInstall(ctx context.Context, serverID, fwType string) (*FirewallInstallPlan, error) {
	fwType, err := normalizeFirewallType(fwType)
	if err != nil {
		return nil, err
	}

	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	_, osInfo, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("ошибка определения ОС: %w", err)
	}

	plan := &FirewallInstallPlan{
		FirewallType: fwType,
		OSFamily:     osInfo.Family,
		OSDistro:     osInfo.PrettyName,
		OSCompatible: true,
	}

	if osInfo.Family != osadapter.FamilyDebian && osInfo.Family != osadapter.FamilyRhel {
		plan.OSCompatible = false
		plan.Message = fmt.Sprintf("Неподдерживаемая ОС: %s. Установка доступна для Debian/Ubuntu и RHEL-совместимых дистрибутивов.", osInfo.PrettyName)
		return plan, nil
	}

	plan.PackagesToInstall, plan.ServiceToStart = firewallInstallSpec(fwType)

	binary := "ufw"
	if fwType == "firewalld" {
		binary = "firewall-cmd"
	}
	if out, _ := s.execCmd(ctx, client, "command -v "+binary); strings.TrimSpace(out) != "" {
		plan.AlreadyInstalled = true
		plan.Message = fmt.Sprintf("Брандмауэр %s уже установлен на этом сервере.", fwType)
		return plan, nil
	}

	plan.Message = fmt.Sprintf("Готов к установке: %s на %s", strings.Join(plan.PackagesToInstall, ", "), osInfo.PrettyName)
	if osInfo.Family == osadapter.FamilyRhel && fwType == "ufw" {
		plan.Message += ". UFW будет установлен из репозитория EPEL."
	}
	return plan, nil
}

// InstallFirewall queues an asynchronous firewall installation task.
func (s *Service) InstallFirewall(serverID, fwType string) (*db.TaskRecord, error) {
	fwType, err := normalizeFirewallType(fwType)
	if err != nil {
		return nil, err
	}
	record, err := s.database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("сервер %s не найден: %w", serverID, err)
	}

	payload, err := tasks.MarshalPayload(FirewallInstallPayload{ServerID: record.ID, FirewallType: fwType})
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("Установка брандмауэра %s на %s (%s)", fwType, record.Name, record.Host)
	return s.engine.Submit(record.ID, "security_firewall_install", title, payload)
}

func (s *Service) handleFirewallInstallTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[FirewallInstallPayload](task.Payload)
	if err != nil {
		return "", err
	}
	fwType, err := normalizeFirewallType(p.FirewallType)
	if err != nil {
		return "", err
	}

	logger.SetStep("Подключение к серверу по SSH", 10)
	client, record, err := s.getSSHClient(ctx, p.ServerID)
	if err != nil {
		return "", err
	}
	adapter, osInfo, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return "", fmt.Errorf("ошибка определения ОС: %w", err)
	}
	if osInfo.Family != osadapter.FamilyDebian && osInfo.Family != osadapter.FamilyRhel {
		return "", fmt.Errorf("неподдерживаемая ОС: %s", osInfo.PrettyName)
	}
	logger.Info(fmt.Sprintf("ОС: %s (family: %s)", osInfo.PrettyName, osInfo.Family))

	packages, _ := firewallInstallSpec(fwType)

	logger.SetStep("Обновление кэша репозиториев", 25)
	if out, err := adapter.UpdatePackageIndex(ctx, client); err != nil {
		logger.Warn(fmt.Sprintf("Не удалось обновить кэш пакетов: %v", err))
	} else {
		logger.Info(strings.TrimSpace(out))
	}

	if osInfo.Family == osadapter.FamilyRhel && fwType == "ufw" {
		logger.SetStep("Подключение репозитория EPEL", 35)
		if _, err := adapter.InstallPackages(ctx, client, []string{"epel-release"}); err != nil {
			logger.Warn(fmt.Sprintf("Не удалось установить epel-release: %v", err))
		}
	}

	logger.SetStep(fmt.Sprintf("Установка пакетов: %s", strings.Join(packages, " ")), 50)
	out, err := adapter.InstallPackages(ctx, client, packages)
	if err != nil {
		return "", fmt.Errorf("ошибка установки пакетов: %w\nВывод:\n%s", err, out)
	}
	logger.Info(strings.TrimSpace(out))

	sshPort := record.Port
	if sshPort == 0 {
		sshPort = 22
	}

	if fwType == "ufw" {
		// CRITICAL: open SSH and web ports before enabling, otherwise the SSH session is locked out.
		logger.SetStep(fmt.Sprintf("Открытие портов SSH (%d/tcp), HTTP (80/tcp), HTTPS (443/tcp)", sshPort), 70)
		allowCmd := fmt.Sprintf("ufw allow %d/tcp comment 'TameStack SSH' && ufw allow 80/tcp comment 'HTTP Web' && ufw allow 443/tcp comment 'HTTPS Web'", sshPort)
		if allowOut, err := s.execCmd(ctx, client, allowCmd); err != nil {
			logger.Warn(fmt.Sprintf("Не удалось открыть порты доступа: %v: %s", err, allowOut))
		}

		logger.SetStep("Включение UFW", 85)
		if out, err := s.execCmd(ctx, client, "ufw --force enable"); err != nil {
			return "", fmt.Errorf("ошибка включения ufw: %w: %s", err, out)
		}
		statusOut, _ := s.execCmd(ctx, client, "env LC_ALL=C ufw status 2>/dev/null || ufw status")
		lowerStatus := strings.ToLower(statusOut)
		if !strings.Contains(lowerStatus, "status: active") && !strings.Contains(lowerStatus, "активен") {
			outUnit, _ := s.execCmd(ctx, client, "systemctl is-active ufw 2>/dev/null || true")
			if strings.TrimSpace(outUnit) != "active" {
				return "", fmt.Errorf("ufw не перешёл в активное состояние:\n%s", statusOut)
			}
		}
		logger.Info("UFW включён, порты SSH, 80 и 443 открыты.")
	} else {
		logger.SetStep("Запуск службы firewalld", 70)
		if out, err := s.execCmd(ctx, client, "systemctl enable --now firewalld"); err != nil {
			return "", fmt.Errorf("ошибка запуска firewalld: %w: %s", err, out)
		}
		logger.SetStep(fmt.Sprintf("Открытие портов SSH (%d/tcp), HTTP, HTTPS", sshPort), 85)
		allowCmd := fmt.Sprintf("firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --permanent --add-service=http && firewall-cmd --permanent --add-service=https && firewall-cmd --reload", sshPort)
		if allowOut, err := s.execCmd(ctx, client, allowCmd); err != nil {
			logger.Warn(fmt.Sprintf("Не удалось открыть порты доступа: %v: %s", err, allowOut))
		}
		stateOut, _ := s.execCmd(ctx, client, "firewall-cmd --state")
		if strings.TrimSpace(stateOut) != "running" {
			return "", fmt.Errorf("firewalld не перешёл в активное состояние: %s", strings.TrimSpace(stateOut))
		}
		logger.Info("firewalld запущен, порты SSH, HTTP и HTTPS открыты.")
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Брандмауэр %s установлен и активирован", fwType), nil
}
