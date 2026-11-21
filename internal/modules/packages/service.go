package packages

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/osadapter"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

type Service struct {
	module.Base
	engine *tasks.Engine
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}

	engine.RegisterHandler("package_install", s.handlePackageInstallTask)
	engine.RegisterHandler("package_remove", s.handlePackageRemoveTask)
	engine.RegisterHandler("package_upgrade", s.handlePackageUpgradeTask)

	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string, sudo bool) (string, error) {
	return module.Exec(ctx, client, cmd, sudo)
}

func (s *Service) adapterFor(ctx context.Context, client *ssh.SSHClient) (osadapter.OSAdapter, error) {
	adapter, _, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("ошибка определения ОС: %w", err)
	}
	return adapter, nil
}

func parseSimulation(adapter osadapter.OSAdapter, out, cmd string) (*AptSimulationResult, error) {
	if adapter.GetOSInfo().Family == osadapter.FamilyRhel {
		return ParseDnfSimulation(out, cmd)
	}
	return ParseAptSimulation(out, cmd)
}

// SimulateInstall previews installing the given packages without modifying the system.
func (s *Service) SimulateInstall(ctx context.Context, serverID string, pkgs []string) (*AptSimulationResult, error) {
	if err := ValidatePackageNames(pkgs); err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.adapterFor(ctx, client)
	if err != nil {
		return nil, err
	}

	out, cmd, err := adapter.SimulateInstall(ctx, client, pkgs)
	if err != nil && out == "" {
		return nil, fmt.Errorf("ошибка симуляции установки: %w", err)
	}

	return parseSimulation(adapter, out, cmd)
}

// SimulateUpgrade previews available system updates.
func (s *Service) SimulateUpgrade(ctx context.Context, serverID string) (*AptSimulationResult, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.adapterFor(ctx, client)
	if err != nil {
		return nil, err
	}

	out, cmd, err := adapter.SimulateUpgrade(ctx, client)
	if err != nil && out == "" {
		return nil, fmt.Errorf("ошибка симуляции обновления: %w", err)
	}

	return parseSimulation(adapter, out, cmd)
}

// SimulateRemove previews removing the given packages.
func (s *Service) SimulateRemove(ctx context.Context, serverID string, pkgs []string) (*AptSimulationResult, error) {
	if err := ValidatePackageNames(pkgs); err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.adapterFor(ctx, client)
	if err != nil {
		return nil, err
	}

	out, cmd, err := adapter.SimulateRemove(ctx, client, pkgs)
	if err != nil && out == "" {
		return nil, fmt.Errorf("ошибка симуляции удаления: %w", err)
	}

	return parseSimulation(adapter, out, cmd)
}

// InstallPackages queues a package_install task.
func (s *Service) InstallPackages(ctx context.Context, serverID string, pkgs []string) (*db.TaskRecord, error) {
	if err := ValidatePackageNames(pkgs); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(PackageActionRequest{
		ServerID: serverID,
		Packages: pkgs,
		Action:   "install",
	})

	taskName := fmt.Sprintf("Установка пакетов: %s", strings.Join(pkgs, ", "))
	if len(taskName) > 80 {
		taskName = fmt.Sprintf("Установка %d пакетов", len(pkgs))
	}

	return s.engine.Submit(serverID, "package_install", taskName, string(payload))
}

// RemovePackages queues a package_remove task.
func (s *Service) RemovePackages(ctx context.Context, serverID string, pkgs []string) (*db.TaskRecord, error) {
	if err := ValidatePackageNames(pkgs); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(PackageActionRequest{
		ServerID: serverID,
		Packages: pkgs,
		Action:   "remove",
	})

	taskName := fmt.Sprintf("Удаление пакетов: %s", strings.Join(pkgs, ", "))
	if len(taskName) > 80 {
		taskName = fmt.Sprintf("Удаление %d пакетов", len(pkgs))
	}

	return s.engine.Submit(serverID, "package_remove", taskName, string(payload))
}

// UpgradePackages queues a package_upgrade task.
func (s *Service) UpgradePackages(ctx context.Context, serverID string, pkgs []string) (*db.TaskRecord, error) {
	if len(pkgs) > 0 {
		if err := ValidatePackageNames(pkgs); err != nil {
			return nil, err
		}
	}

	payload, _ := json.Marshal(PackageActionRequest{
		ServerID: serverID,
		Packages: pkgs,
		Action:   "upgrade",
	})

	taskName := "Обновление системы"
	if len(pkgs) > 0 {
		taskName = fmt.Sprintf("Обновление пакетов (%d): %s", len(pkgs), strings.Join(pkgs, ", "))
		if len(taskName) > 80 {
			taskName = fmt.Sprintf("Обновление %d выбранных пакетов", len(pkgs))
		}
	}

	return s.engine.Submit(serverID, "package_upgrade", taskName, string(payload))
}

// ListInstalledPackages queries the remote package database with optional text filter.
func (s *Service) ListInstalledPackages(ctx context.Context, serverID string, query string) ([]PackageItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.adapterFor(ctx, client)
	if err != nil {
		return nil, err
	}

	out, err := adapter.ListInstalledPackages(ctx, client, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка пакетов: %w", err)
	}

	if adapter.GetOSInfo().Family == osadapter.FamilyRhel {
		return ParseRpmList(out)
	}
	return ParseDpkgList(out)
}

// ListPackageHistory returns past package tasks from the database.
func (s *Service) ListPackageHistory(ctx context.Context, serverID string) ([]PackageTaskHistoryItem, error) {
	tasksList, err := s.Database.ListTasks(ctx, 100)
	if err != nil {
		return nil, err
	}

	var history []PackageTaskHistoryItem
	for _, t := range tasksList {
		if t.ServerID == serverID && strings.HasPrefix(t.OperationType, "package_") {
			var pkgs []string
			var req PackageActionRequest
			if err := json.Unmarshal([]byte(t.Payload), &req); err == nil {
				pkgs = req.Packages
			}

			history = append(history, PackageTaskHistoryItem{
				ID:            t.ID,
				ServerID:      t.ServerID,
				OperationType: t.OperationType,
				Status:        string(t.Status),
				Packages:      pkgs,
				CreatedAt:     t.CreatedAt,
				CompletedAt:   t.CompletedAt,
				ErrorMessage:  t.ErrorMessage,
			})
		}
	}

	return history, nil
}

// checkDiskSpace ensures at least 200MB free disk space before package modifications.
func (s *Service) checkDiskSpace(ctx context.Context, client *ssh.SSHClient, logger tasks.TaskLogger) error {
	logger.SetStep("Проверка свободного места на диске", 10)
	out, err := s.execCmd(ctx, client, "df -k /var/cache/apt/archives 2>/dev/null || df -k /", false)
	if err != nil {
		logger.Warn("Не удалось прочитать df, продолжаем")
		return nil
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) >= 4 {
		availKb, parseErr := strconv.ParseInt(fields[3], 10, 64)
		if parseErr == nil && availKb < 204800 { // less than 200MB
			return fmt.Errorf("недостаточно свободного места на диске: доступно %d МБ, требуется минимум 200 МБ", availKb/1024)
		}
	}
	return nil
}

func (s *Service) handlePackageInstallTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req PackageActionRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	if err := ValidatePackageNames(req.Packages); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	if err := s.checkDiskSpace(ctx, client, logger); err != nil {
		return "", err
	}

	adapter, err := s.adapterFor(ctx, client)
	if err != nil {
		return "", err
	}

	logger.SetStep("Обновление кэша репозиториев", 25)
	_, _ = adapter.UpdatePackageIndex(ctx, client)

	logger.SetStep(fmt.Sprintf("Установка пакетов: %s", strings.Join(req.Packages, " ")), 60)
	out, err := adapter.InstallPackages(ctx, client, req.Packages)
	if err != nil {
		return "", fmt.Errorf("ошибка установки пакетов: %w\nВывод:\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Пакеты успешно установлены: %s", strings.Join(req.Packages, ", ")), nil
}

func (s *Service) handlePackageRemoveTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req PackageActionRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	if err := ValidatePackageNames(req.Packages); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	adapter, err := s.adapterFor(ctx, client)
	if err != nil {
		return "", err
	}

	logger.SetStep(fmt.Sprintf("Удаление пакетов: %s", strings.Join(req.Packages, " ")), 50)
	out, err := adapter.RemovePackages(ctx, client, req.Packages)
	if err != nil {
		return "", fmt.Errorf("ошибка удаления пакетов: %w\nВывод:\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Пакеты успешно удалены: %s", strings.Join(req.Packages, ", ")), nil
}

func (s *Service) handlePackageUpgradeTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req PackageActionRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	if err := s.checkDiskSpace(ctx, client, logger); err != nil {
		return "", err
	}

	adapter, err := s.adapterFor(ctx, client)
	if err != nil {
		return "", err
	}

	logger.SetStep("Обновление списков пакетов", 20)
	_, _ = adapter.UpdatePackageIndex(ctx, client)

	if len(req.Packages) > 0 {
		if err := ValidatePackageNames(req.Packages); err != nil {
			return "", err
		}
		logger.SetStep(fmt.Sprintf("Обновление выбранных пакетов (%d)", len(req.Packages)), 60)
	} else {
		logger.SetStep("Полное обновление системы", 60)
	}

	out, err := adapter.UpgradePackages(ctx, client, req.Packages)
	if err != nil {
		return "", fmt.Errorf("ошибка обновления пакетов: %w\nВывод:\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return "Обновление пакетов успешно завершено", nil
}
