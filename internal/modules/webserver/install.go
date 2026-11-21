package webserver

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/osadapter"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// InstallPlan describes what a web server installation will do before it runs.
type InstallPlan struct {
	WebServerType       string   `json:"webserver_type"`
	OSFamily            string   `json:"os_family"`
	OSDistro            string   `json:"os_distro"`
	OSCompatible        bool     `json:"os_compatible"`
	PackagesToInstall   []string `json:"packages_to_install"`
	ServiceToStart      string   `json:"service_to_start"`
	PortConflictWarning string   `json:"port_conflict_warning,omitempty"`
	Message             string   `json:"message"`
}

// InstallPayload is the task payload for webserver_install.
type InstallPayload struct {
	ServerID      string `json:"server_id"`
	WebServerType string `json:"webserver_type"`
}

func normalizeWebServerType(wsType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(wsType)) {
	case "nginx":
		return "nginx", nil
	case "apache", "apache2", "httpd":
		return "apache2", nil
	default:
		return "", fmt.Errorf("неподдерживаемый веб-сервер: %s (доступны nginx и apache)", wsType)
	}
}

func installSpec(wsType, osFamily string) (packages []string, service string) {
	if wsType == "nginx" {
		return []string{"nginx"}, "nginx"
	}
	if osFamily == osadapter.FamilyRhel {
		return []string{"httpd"}, "httpd"
	}
	return []string{"apache2"}, "apache2"
}

// PlanInstall detects the remote OS and previews the installation without changing anything.
func (s *Service) PlanInstall(ctx context.Context, serverID, wsType string) (*InstallPlan, error) {
	wsType, err := normalizeWebServerType(wsType)
	if err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	adapter, osInfo, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("ошибка определения ОС: %w", err)
	}

	plan := &InstallPlan{
		WebServerType: wsType,
		OSFamily:      osInfo.Family,
		OSDistro:      osInfo.PrettyName,
		OSCompatible:  true,
	}

	if osInfo.Family != osadapter.FamilyDebian && osInfo.Family != osadapter.FamilyRhel {
		plan.OSCompatible = false
		plan.Message = fmt.Sprintf("Неподдерживаемая ОС: %s. Установка доступна для Debian/Ubuntu и RHEL-совместимых дистрибутивов.", osInfo.PrettyName)
		return plan, nil
	}

	plan.PackagesToInstall, plan.ServiceToStart = installSpec(wsType, adapter.GetOSInfo().Family)
	plan.Message = fmt.Sprintf("Готов к установке: %s на %s", strings.Join(plan.PackagesToInstall, ", "), osInfo.PrettyName)

	if topo, err := DetectTopology(ctx, client); err == nil {
		if wsType == "nginx" && topo.ApacheActive {
			plan.PortConflictWarning = "На сервере уже активен Apache. Если он занимает порт 80, Nginx не сможет запуститься с конфигурацией по умолчанию — потребуется освободить порт или настроить гибридный стек."
		} else if wsType == "apache2" && topo.NginxActive {
			plan.PortConflictWarning = "На сервере уже активен Nginx. Если он занимает порт 80, Apache не сможет запуститься с конфигурацией по умолчанию — потребуется освободить порт или настроить гибридный стек."
		}
	}

	return plan, nil
}

// InstallWebServer queues an asynchronous installation task.
func (s *Service) InstallWebServer(serverID, wsType string) (*db.TaskRecord, error) {
	wsType, err := normalizeWebServerType(wsType)
	if err != nil {
		return nil, err
	}
	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("сервер %s не найден: %w", serverID, err)
	}

	payload, err := tasks.MarshalPayload(InstallPayload{ServerID: record.ID, WebServerType: wsType})
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("Установка %s на %s (%s)", wsType, record.Name, record.Host)
	return s.engine.Submit(record.ID, "webserver_install", title, payload)
}

func (s *Service) handleInstallTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[InstallPayload](task.Payload)
	if err != nil {
		return "", err
	}
	wsType, err := normalizeWebServerType(p.WebServerType)
	if err != nil {
		return "", err
	}

	logger.SetStep("Подключение к серверу по SSH", 10)
	client, _, err := s.GetSSHClient(ctx, p.ServerID)
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

	packages, service := installSpec(wsType, osInfo.Family)

	logger.SetStep("Обновление кэша репозиториев", 25)
	if out, err := adapter.UpdatePackageIndex(ctx, client); err != nil {
		logger.Warn(fmt.Sprintf("Не удалось обновить кэш пакетов: %v", err))
	} else {
		logger.Info(strings.TrimSpace(out))
	}

	logger.SetStep(fmt.Sprintf("Установка пакетов: %s", strings.Join(packages, " ")), 45)
	out, err := adapter.InstallPackages(ctx, client, packages)
	if err != nil {
		return "", fmt.Errorf("ошибка установки пакетов: %w\nВывод:\n%s", err, out)
	}
	logger.Info(strings.TrimSpace(out))

	logger.SetStep(fmt.Sprintf("Включение и запуск службы %s", service), 80)
	srvCmd := fmt.Sprintf("systemctl enable --now %s && systemctl is-active %s", ssh.ShellEscape(service), ssh.ShellEscape(service))
	outSrv, _, codeSrv, _ := client.NewCommand(srvCmd).WithSudo().Exec(ctx)
	if codeSrv != 0 || strings.TrimSpace(string(outSrv)) != "active" {
		logger.Warn(fmt.Sprintf("Служба %s не активна после запуска, пробую перезапуск...", service))
		restartCmd := fmt.Sprintf("systemctl restart %s && systemctl is-active %s", ssh.ShellEscape(service), ssh.ShellEscape(service))
		outRestart, stderrRestart, codeRestart, errRestart := client.NewCommand(restartCmd).WithSudo().Exec(ctx)
		if errRestart != nil || codeRestart != 0 || strings.TrimSpace(string(outRestart)) != "active" {
			return "", fmt.Errorf("служба %s не запустилась: %s", service, strings.TrimSpace(string(stderrRestart)))
		}
	}
	logger.Info(fmt.Sprintf("Служба %s активна и включена в автозагрузку.", service))

	logger.SetStep("Проверка конфигурации", 90)
	passed, testOut, err := s.TestConfig(ctx, p.ServerID, wsType)
	if err != nil {
		return "", err
	}
	if !passed {
		return "", fmt.Errorf("конфигурация установленного %s не прошла проверку:\n%s", wsType, testOut)
	}
	logger.Info("Синтаксис конфигурации корректен.")

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Веб-сервер %s установлен и запущен", wsType), nil
}