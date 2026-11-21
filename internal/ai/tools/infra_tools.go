package tools

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
	"tamestack/internal/modules/backup"
	"tamestack/internal/modules/cron"
	"tamestack/internal/modules/docker"
	"tamestack/internal/modules/files"
	"tamestack/internal/modules/packages"
	"tamestack/internal/modules/services"
	"tamestack/internal/modules/ssl"
)

// RegisterInfraTools registers read-only inspection tools for Docker, SSL/Certbot, Cron, Packages, Disk/Files, Services, and Backups.
func RegisterInfraTools(
	r *Registry,
	database *db.DB,
	dockerService *docker.Service,
	sslService *ssl.Service,
	cronService *cron.Service,
	packagesService *packages.Service,
	filesService *files.Service,
	servicesService *services.Service,
	backupService *backup.Service,
) {
	// 1. inspect_docker_containers
	r.Register(domain.ToolDefinition{
		Name:        "inspect_docker_containers",
		Description: "Возвращает статус демона Docker, версии Docker и Compose, а также список всех контейнеров (активных и остановленных), образов и Compose-проектов на сервере.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if dockerService == nil {
			return nil, fmt.Errorf("сервис Docker не инициализирован")
		}
		status, err := dockerService.GetDockerStatus(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения статуса docker: %w", err)
		}
		if !status.Installed {
			return map[string]any{
				"installed": false,
				"message":   "Docker не установлен или сервис docker не запущен на данном сервере",
			}, nil
		}
		containers, _ := dockerService.ListContainers(ctx, srv.ID, true)
		composeProjects, _ := dockerService.ListComposeProjects(ctx, srv.ID)
		return map[string]any{
			"installed":        true,
			"status":           status,
			"containers":       containers,
			"compose_projects": composeProjects,
		}, nil
	})

	// 2. get_docker_container_logs
	r.Register(domain.ToolDefinition{
		Name:        "get_docker_container_logs",
		Description: "Получает последние строки журнала (логов) конкретного Docker-контейнера по его имени или ID. Безопасная операция чтения.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("container_id", "Имя или хеш контейнера (например 'my-app' или 'a1b2c3d4')").
			integer("lines", "Количество последних строк журнала (по умолчанию 50, максимум 200)").
			req("server_id").
			req("container_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if dockerService == nil {
			return nil, fmt.Errorf("сервис Docker не инициализирован")
		}
		containerID, _ := args["container_id"].(string)
		if strings.TrimSpace(containerID) == "" {
			return nil, fmt.Errorf("параметр container_id обязателен")
		}
		linesCount := intArg(args, "lines", 50, 200)
		logs, err := dockerService.GetContainerLogs(ctx, srv.ID, containerID, linesCount)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить логи контейнера %s: %w", containerID, err)
		}
		return map[string]any{
			"container_id": containerID,
			"lines":        linesCount,
			"logs":         logs,
		}, nil
	})

	// 3. inspect_ssl_certificates
	r.Register(domain.ToolDefinition{
		Name:        "inspect_ssl_certificates",
		Description: "Возвращает информацию об установленных SSL/TLS сертификатах (Certbot / Let's Encrypt), включая защищенные домены, даты окончания действия, оставшиеся дни и статус продления.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if sslService == nil {
			return nil, fmt.Errorf("сервис SSL не инициализирован")
		}
		dbCerts, _ := sslService.ListCertificates(ctx, srv.ID)
		certbotCerts, _ := sslService.ListCertbotCertificates(ctx, srv.ID)
		metrics, _ := sslService.GetMetrics(ctx, srv.ID)
		return map[string]any{
			"metrics":              metrics,
			"certificates":         dbCerts,
			"certbot_certificates": certbotCerts,
		}, nil
	})

	// 4. list_cron_jobs
	r.Register(domain.ToolDefinition{
		Name:        "list_cron_jobs",
		Description: "Возвращает список запланированных задач cron (crontab) на сервере (пользовательские crontab, системный /etc/crontab и файлы /etc/cron.d).",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("user", "Имя пользователя (например 'root', 'www-data' или 'all' для системных и пользовательских, по умолчанию 'all')").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if cronService == nil {
			return nil, fmt.Errorf("сервис cron не инициализирован")
		}
		user, _ := args["user"].(string)
		if strings.TrimSpace(user) == "" {
			user = "all"
		}
		jobs, err := cronService.ListCronJobs(ctx, srv.ID, user)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить задачи cron: %w", err)
		}
		return map[string]any{
			"count": len(jobs),
			"user":  user,
			"jobs":  jobs,
		}, nil
	})

	// 5. check_package_updates
	r.Register(domain.ToolDefinition{
		Name:        "check_package_updates",
		Description: "Выполняет симуляцию обновления системных пакетов (APT/DNF) на сервере без внесения изменений. Возвращает количество доступных обновлений, обновления безопасности и необходимость перезагрузки.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if packagesService == nil {
			return nil, fmt.Errorf("сервис пакетов не инициализирован")
		}
		sim, err := packagesService.SimulateUpgrade(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка симуляции обновления пакетов: %w", err)
		}
		changes := sim.Changes
		if len(changes) > 50 {
			changes = changes[:50]
		}
		return map[string]any{
			"success":         sim.Success,
			"upgraded_count":  sim.UpgradedCount,
			"installed_count": sim.InstalledCount,
			"removed_count":   sim.RemovedCount,
			"held_count":      sim.HeldCount,
			"changes":         changes,
			"has_more":        len(sim.Changes) > 50,
		}, nil
	})

	// 6. analyze_disk_usage
	r.Register(domain.ToolDefinition{
		Name:        "analyze_disk_usage",
		Description: "Анализирует использование дискового пространства сервера: точки монтирования, самые объемные директории и файлы в указанном пути, а также рекомендации по безопасной очистке места.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("path", "Директория для анализа (по умолчанию '/')").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if filesService == nil {
			return nil, fmt.Errorf("файловый сервис не инициализирован")
		}
		targetPath, _ := args["path"].(string)
		if strings.TrimSpace(targetPath) == "" {
			targetPath = "/"
		}
		report, err := filesService.AnalyzeDiskUsage(ctx, srv.ID, targetPath)
		if err != nil {
			return nil, fmt.Errorf("ошибка анализа диска: %w", err)
		}
		return report, nil
	})

	// 7. list_system_processes
	r.Register(domain.ToolDefinition{
		Name:        "list_system_processes",
		Description: "Возвращает список запущенных процессов сервера с сортировкой по потреблению CPU и оперативной памяти (PID, пользователь, % CPU, % RAM, команда). Помогает выявлять ресурсоемкие или зависшие процессы.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			integer("limit", "Максимальное количество процессов в выдаче (по умолчанию 30, максимум 100)").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if servicesService == nil {
			return nil, fmt.Errorf("сервис управления службами не инициализирован")
		}
		limit := intArg(args, "limit", 30, 100)
		procs, err := servicesService.ListProcesses(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения списка процессов: %w", err)
		}
		if len(procs) > limit {
			procs = procs[:limit]
		}
		return map[string]any{
			"count":     len(procs),
			"processes": procs,
		}, nil
	})

	// 8. list_backups
	r.Register(domain.ToolDefinition{
		Name:        "list_backups",
		Description: "Возвращает список сохраненных резервных копий (бэкапов) сервера из базы данных TameStack.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if backupService == nil {
			return nil, fmt.Errorf("сервис бэкапов не инициализирован")
		}
		backups, err := backupService.ListBackups(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения списка бэкапов: %w", err)
		}
		return map[string]any{
			"count":   len(backups),
			"backups": backups,
		}, nil
	})
}
