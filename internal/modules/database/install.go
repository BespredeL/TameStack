package database

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/osadapter"
	"tamestack/internal/tasks"
)

// DBInstallPlan describes what a DBMS installation will do before it runs.
type DBInstallPlan struct {
	Engine            string   `json:"engine"`
	Variant           string   `json:"variant"`
	OSFamily          string   `json:"os_family"`
	OSDistro          string   `json:"os_distro"`
	OSCompatible      bool     `json:"os_compatible"`
	PackagesToInstall []string `json:"packages_to_install"`
	ServiceToStart    string   `json:"service_to_start"`
	Message           string   `json:"message"`
}

// DBInstallPayload is the task payload for db_install.
type DBInstallPayload struct {
	ServerID string `json:"server_id"`
	Engine   string `json:"engine"`
	Variant  string `json:"variant"`
}

// normalizeInstallVariant resolves the concrete product to install.
// The generic "mysql" card offers a choice between MariaDB and MySQL.
func normalizeInstallVariant(engine, variant string) (EngineType, string, error) {
	switch EngineType(engine) {
	case EngineMySQL, EngineMariaDB:
		switch strings.ToLower(strings.TrimSpace(variant)) {
		case "", "mariadb":
			return EngineMariaDB, "mariadb", nil
		case "mysql":
			return EngineMySQL, "mysql", nil
		default:
			return "", "", fmt.Errorf("неизвестный вариант MySQL: %s (доступны mariadb и mysql)", variant)
		}
	case EnginePostgreSQL:
		return EnginePostgreSQL, "postgres", nil
	case EngineRedis:
		return EngineRedis, "redis", nil
	default:
		return "", "", fmt.Errorf("неподдерживаемая СУБД: %s", engine)
	}
}

func dbInstallSpec(variant, osFamily string) (packages []string, service string) {
	switch variant {
	case "mariadb":
		return []string{"mariadb-server"}, "mariadb"
	case "mysql":
		return []string{"mysql-server"}, "mysql"
	case "postgres":
		if osFamily == osadapter.FamilyRhel {
			return []string{"postgresql-server", "postgresql-contrib"}, "postgresql"
		}
		return []string{"postgresql"}, "postgresql"
	case "redis":
		if osFamily == osadapter.FamilyRhel {
			return []string{"redis"}, "redis"
		}
		return []string{"redis-server"}, "redis-server"
	default:
		return nil, ""
	}
}

// PlanInstall detects the remote OS and previews the installation without changing anything.
func (s *Service) PlanInstall(ctx context.Context, serverID, engine, variant string) (*DBInstallPlan, error) {
	engineType, variant, err := normalizeInstallVariant(engine, variant)
	if err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	_, osInfo, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("ошибка определения ОС: %w", err)
	}

	plan := &DBInstallPlan{
		Engine:       string(engineType),
		Variant:      variant,
		OSFamily:     osInfo.Family,
		OSDistro:     osInfo.PrettyName,
		OSCompatible: true,
	}

	if osInfo.Family != osadapter.FamilyDebian && osInfo.Family != osadapter.FamilyRhel {
		plan.OSCompatible = false
		plan.Message = fmt.Sprintf("Неподдерживаемая ОС: %s. Установка доступна для Debian/Ubuntu и RHEL-совместимых дистрибутивов.", osInfo.PrettyName)
		return plan, nil
	}

	if variant == "mysql" && osInfo.Distro == "debian" {
		plan.OSCompatible = false
		plan.Message = "Официальные пакеты MySQL отсутствуют в репозиториях Debian. Выберите MariaDB — она полностью совместима с MySQL."
		return plan, nil
	}

	plan.PackagesToInstall, plan.ServiceToStart = dbInstallSpec(variant, osInfo.Family)
	plan.Message = fmt.Sprintf("Готов к установке: %s на %s", strings.Join(plan.PackagesToInstall, ", "), osInfo.PrettyName)
	return plan, nil
}

// InstallDatabase queues an asynchronous DBMS installation task.
func (s *Service) InstallDatabase(serverID, engine, variant string) (*db.TaskRecord, error) {
	engineType, variant, err := normalizeInstallVariant(engine, variant)
	if err != nil {
		return nil, err
	}
	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("сервер %s не найден: %w", serverID, err)
	}

	payload, err := tasks.MarshalPayload(DBInstallPayload{ServerID: record.ID, Engine: string(engineType), Variant: variant})
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("Установка %s на %s (%s)", variant, record.Name, record.Host)
	return s.engine.Submit(record.ID, "db_install", title, payload)
}

func (s *Service) handleDBInstallTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[DBInstallPayload](task.Payload)
	if err != nil {
		return "", err
	}
	engineType, variant, err := normalizeInstallVariant(p.Engine, p.Variant)
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
	if variant == "mysql" && osInfo.Distro == "debian" {
		return "", fmt.Errorf("пакеты MySQL недоступны в репозиториях Debian, выберите MariaDB")
	}
	logger.Info(fmt.Sprintf("ОС: %s (family: %s)", osInfo.PrettyName, osInfo.Family))

	packages, service := dbInstallSpec(variant, osInfo.Family)

	logger.SetStep("Обновление кэша репозиториев", 20)
	if out, err := adapter.UpdatePackageIndex(ctx, client); err != nil {
		logger.Warn(fmt.Sprintf("Не удалось обновить кэш пакетов: %v", err))
	} else {
		logger.Info(strings.TrimSpace(out))
	}

	logger.SetStep(fmt.Sprintf("Установка пакетов: %s", strings.Join(packages, " ")), 40)
	out, err := adapter.InstallPackages(ctx, client, packages)
	if err != nil {
		return "", fmt.Errorf("ошибка установки пакетов: %w\nВывод:\n%s", err, out)
	}
	logger.Info(strings.TrimSpace(out))

	if variant == "postgres" && osInfo.Family == osadapter.FamilyRhel {
		logger.SetStep("Инициализация кластера PostgreSQL", 65)
		initCmd := `if [ ! -f /var/lib/pgsql/data/PG_VERSION ]; then
    if command -v postgresql-setup >/dev/null 2>&1; then
        postgresql-setup --initdb
    else
        runuser -u postgres -- /usr/bin/initdb -D /var/lib/pgsql/data
    fi
fi`
		outInit, stderrInit, codeInit, errInit := client.NewCommand(initCmd).WithSudo().Exec(ctx)
		if errInit != nil || codeInit != 0 {
			return "", fmt.Errorf("инициализация PostgreSQL не удалась (code %d): %s", codeInit, strings.TrimSpace(string(stderrInit)))
		}
		logger.Info(strings.TrimSpace(string(outInit)))
	}

	logger.SetStep(fmt.Sprintf("Включение и запуск службы %s", service), 80)
	enableCmd, err := serviceControlScript(engineType, "enable")
	if err != nil {
		return "", err
	}
	if _, stderr, code, err := client.NewCommand(enableCmd).WithSudo().Exec(ctx); err != nil || code != 0 {
		return "", fmt.Errorf("не удалось включить службу %s (code %d): %s", service, code, strings.TrimSpace(string(stderr)))
	}
	startCmd, err := serviceControlScript(engineType, "start")
	if err != nil {
		return "", err
	}
	if _, stderr, code, err := client.NewCommand(startCmd).WithSudo().Exec(ctx); err != nil || code != 0 {
		return "", fmt.Errorf("служба %s не запустилась (code %d): %s", service, code, strings.TrimSpace(string(stderr)))
	}
	if !s.isServiceActive(ctx, client, engineType) {
		return "", fmt.Errorf("служба %s установлена, но не активна; проверьте journalctl -u %s", service, service)
	}
	logger.Info(fmt.Sprintf("Служба %s активна и включена в автозагрузку.", service))

	logger.SetStep("Проверка установки", 90)
	var verCmd string
	switch variant {
	case "mariadb", "mysql":
		verCmd = "mysql --version 2>/dev/null || mariadb --version 2>/dev/null"
	case "postgres":
		verCmd = "psql --version 2>/dev/null"
	case "redis":
		verCmd = "redis-server --version 2>/dev/null"
	}
	if verCmd != "" {
		outVer, _, _, _ := client.Exec(ctx, verCmd)
		logger.Info(fmt.Sprintf("Установлено: %s", strings.TrimSpace(string(outVer))))
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("СУБД %s установлена и запущена", variant), nil
}