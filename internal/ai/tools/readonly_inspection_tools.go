package tools

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
	"tamestack/internal/modules/php"
	"tamestack/internal/modules/security"
	"tamestack/internal/modules/site"
	"tamestack/internal/modules/webserver"
	"tamestack/internal/ssh"
)

// intArg reads a bounded integer argument coming from a model payload.
func intArg(args map[string]any, key string, def, limit int) int {
	value := def
	if number, ok := args[key].(float64); ok && number > 0 {
		value = int(number)
	} else if number, ok := args[key].(int); ok && number > 0 {
		value = number
	}
	if value > limit {
		value = limit
	}
	return value
}

// logTargetFor maps the free-text target a model passes to the code-owned log families.
func logTargetFor(serviceName string) security.LogTarget {
	switch lower := strings.ToLower(strings.TrimSpace(serviceName)); {
	case strings.Contains(lower, "apache") || strings.Contains(lower, "httpd"):
		return security.LogTargetApache
	case strings.Contains(lower, "php"):
		return security.LogTargetPHPFPM
	case strings.Contains(lower, "nginx"):
		return security.LogTargetNginx
	default:
		return security.LogTargetAuto
	}
}

// RegisterReadOnlyInspectionTools registers safe, non-mutating server inspection tools for the AI agent.
func RegisterReadOnlyInspectionTools(
	r *Registry,
	database *db.DB,
	wsService *webserver.Service,
	phpService *php.Service,
	siteService *site.Service,
	secService *security.Service,
) {
	r.Register(domain.ToolDefinition{
		Name:        "read_webserver_logs",
		Description: "Читает последние строки журналов веб-серверов (Nginx error.log/access.log, Apache error.log/access.log, PHP-FPM логи или логи конкретного сайта). Безопасная операция чтения.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("service", "Цель чтения: 'apache' (или apache2/httpd), 'nginx', 'php-fpm', или домен сайта (например, 'site.ru')").
			str("log_type", "Тип журнала: 'error' (журнал ошибок, по умолчанию) или 'access' (журнал запросов)").
			integer("lines", "Количество последних строк для чтения (по умолчанию 50, максимум 200)").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		serviceName, _ := args["service"].(string)
		logType, _ := args["log_type"].(string)
		if logType == "" {
			logType = "error"
		}
		if err := ssh.ValidateLogType(logType); err != nil {
			return nil, err
		}
		linesCount := intArg(args, "lines", 50, 200)

		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		// Check if serviceName matches a site domain or site ID.
		if siteService != nil && serviceName != "" {
			sites, _ := database.ListSites(ctx, srv.ID)
			for _, s := range sites {
				if strings.EqualFold(s.Domain, serviceName) || s.ID == serviceName {
					prefWS := "nginx"
					if strings.Contains(strings.ToLower(s.WebServer), "apache") {
						prefWS = "apache"
					}
					logs, sErr := siteService.GetSiteLogs(ctx, s.ID, logType, linesCount, prefWS)
					if sErr == nil && strings.TrimSpace(logs) != "" {
						return map[string]any{
							"source":   fmt.Sprintf("Сайт %s (%s)", s.Domain, prefWS),
							"log_type": logType,
							"lines":    linesCount,
							"content":  logs,
						}, nil
					}
				}
			}
		}

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		out, err := secService.ReadLogTail(ctx, srv.ID, logTargetFor(serviceName), logType, linesCount)
		if err != nil && strings.TrimSpace(out) == "" {
			return nil, fmt.Errorf("ошибка чтения логов: %w", err)
		}

		return map[string]any{
			"server_id": srv.ID,
			"target":    serviceName,
			"log_type":  logType,
			"lines":     linesCount,
			"content":   strings.TrimSpace(out),
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "get_service_status",
		Description: "Возвращает подробный статус системной службы (systemctl status <service>): активность, код завершения, PID, потребление ресурсов и последние строки journal. Безопасная операция чтения.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("service_name", "Имя службы: apache2, httpd, nginx, php8.1-fpm, php8.2-fpm, mysql, mariadb, postgresql, docker, fail2ban, ufw и т.д.").
			req("server_id", "service_name").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		serviceName, _ := args["service_name"].(string)
		serviceName = strings.TrimSpace(serviceName)

		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		out, err := secService.ReadServiceStatus(ctx, srv.ID, serviceName)
		if err != nil && strings.TrimSpace(out) == "" {
			return nil, fmt.Errorf("ошибка чтения статуса службы: %w", err)
		}

		isActive := strings.Contains(out, "Active: active (running)") || strings.Contains(out, "is running")
		isFailed := strings.Contains(out, "Active: failed") || strings.Contains(out, "failed (Result:")

		statusSummary := "неизвестно"
		if isActive {
			statusSummary = "активна (active/running)"
		} else if isFailed {
			statusSummary = "ошибка запуска / аварийно остановлена (failed)"
		} else if strings.Contains(out, "Active: inactive (dead)") {
			statusSummary = "остановлена (inactive/dead)"
		}

		return map[string]any{
			"service_name":   serviceName,
			"is_active":      isActive,
			"status_summary": statusSummary,
			"raw_output":     strings.TrimSpace(out),
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "get_service_logs",
		Description: "Читает системный журнал службы через journalctl (journalctl -u <service> -n <lines>). Позволяет увидеть сбои запуска, аварийные остановки и системные сообщения. Безопасная операция чтения.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("service_name", "Имя службы (например: nginx, apache2, php8.1-fpm, mysql, docker)").
			integer("lines", "Количество последних строк журнала (по умолчанию 50, максимум 200)").
			req("server_id", "service_name").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		serviceName, _ := args["service_name"].(string)
		serviceName = strings.TrimSpace(serviceName)
		linesCount := intArg(args, "lines", 50, 200)

		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		out, err := secService.ReadServiceJournal(ctx, srv.ID, serviceName, linesCount)
		if err != nil && strings.TrimSpace(out) == "" {
			return nil, fmt.Errorf("ошибка чтения журнала службы: %w", err)
		}

		return map[string]any{
			"service_name": serviceName,
			"lines":        linesCount,
			"journal":      strings.TrimSpace(out),
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "check_web_topology",
		Description: "Мгновенно определяет архитектуру веб-стека сервера: Nginx-only, Apache-only или Hybrid (Nginx фронтенд + Apache бэкенд), открытые порты, а также режим PHP (монолитный Apache mod_php или изолированные сокеты PHP-FPM в /run/php/*.sock). Безопасная операция чтения.",
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

		if wsService == nil {
			return nil, fmt.Errorf("сервис веб-серверов не инициализирован")
		}

		top, err := wsService.GetTopology(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка анализа топологии веб-сервера: %w", err)
		}

		// Human readable description.
		var desc string
		switch top.Topology {
		case webserver.TopologyHybrid:
			desc = fmt.Sprintf("Гибридная связка: Nginx на внешних портах (%d), Apache в роли бэкенда на порту %d", top.FrontendPort, top.BackendPort)
		case webserver.TopologyNginxOnly:
			desc = "Nginx-only: Nginx обрабатывает статику и напрямую проксирует PHP в сокеты PHP-FPM"
		case webserver.TopologyApacheOnly:
			desc = "Apache-only: Apache обслуживает внешние запросы на порту 80/443"
		default:
			desc = "Нестандартная или смешанная конфигурация"
		}

		phpMode := "Не обнаружен"
		if top.PHPRuntimes.HasApacheModPHP {
			phpMode = fmt.Sprintf("Apache mod_php (монолитный модуль %s)", top.PHPRuntimes.ModPHPVersion)
		} else if len(top.PHPRuntimes.ActiveFpmSockets) > 0 {
			phpMode = fmt.Sprintf("Изолированные сокеты PHP-FPM (%s)", strings.Join(top.PHPRuntimes.ActiveFpmSockets, ", "))
		}

		return map[string]any{
			"server_id":          srv.ID,
			"server_name":        srv.Name,
			"topology":           top.Topology,
			"topology_summary":   desc,
			"frontend_port":      top.FrontendPort,
			"backend_port":       top.BackendPort,
			"nginx_active":       top.NginxActive,
			"apache_active":      top.ApacheActive,
			"php_mode":           phpMode,
			"has_apache_mod_php": top.PHPRuntimes.HasApacheModPHP,
			"active_fpm_sockets": top.PHPRuntimes.ActiveFpmSockets,
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "inspect_php_environment",
		Description: "Инспектирует окружение PHP на сервере: установленные CLI-версии, активные пулы PHP-FPM, сокеты в /run/php/ и загруженные модули. Безопасная операция чтения.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("version", "Конкретная версия для проверки модулей (например: '8.1', '8.2', '7.4')").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		versionReq, _ := args["version"].(string)

		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		out, err := secService.ReadPHPEnvironment(ctx, srv.ID, versionReq)
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"server_id": srv.ID,
			"overview":  strings.TrimSpace(out),
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "read_config_file",
		Description: "Безопасно считывает содержимое конфигурационного файла на сервере (/etc/nginx/, /etc/apache2/, /etc/php/). Симлинки разрешаются на стороне сервера, чтение приватных ключей и паролей запрещено. Безопасная операция чтения.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("file_path", "Абсолютный путь к файлу конфигурации (например: /etc/nginx/nginx.conf, /etc/apache2/ports.conf, /etc/php/8.1/fpm/php.ini)").
			integer("max_lines", "Максимум строк для чтения (по умолчанию 150, максимум 300)").
			req("server_id", "file_path").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		filePath, _ := args["file_path"].(string)
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			return nil, fmt.Errorf("file_path обязателен")
		}
		maxLines := intArg(args, "max_lines", 150, 300)

		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		out, err := secService.ReadConfigHead(ctx, srv.ID, filePath, maxLines)
		if err != nil && strings.TrimSpace(out) == "" {
			return nil, fmt.Errorf("ошибка чтения файла %s: %w", filePath, err)
		}

		return map[string]any{
			"file_path": filePath,
			"content":   strings.TrimSpace(out),
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "get_listening_ports",
		Description: "Возвращает список всех открытых сетевых портов сервера (TCP/UDP) и процессов, слушающих их (ss -tulpn). Помогает понять, какие службы запущены и какие порты заняты (80, 443, 8080, 3306 и др.). Безопасная операция чтения.",
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

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		out, err := secService.ReadListeningSockets(ctx, srv.ID)
		if err != nil {
			return nil, err
		}

		lines := strings.Split(out, "\n")
		var filtered []string
		for _, l := range lines {
			lTrim := strings.TrimSpace(l)
			if strings.Contains(lTrim, "LISTEN") || strings.HasPrefix(lTrim, "Netid") || strings.HasPrefix(lTrim, "Proto") {
				filtered = append(filtered, lTrim)
			}
		}

		return map[string]any{
			"server_id": srv.ID,
			"ports_raw": strings.Join(filtered, "\n"),
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "test_webserver_syntax",
		Description: "Проверяет синтаксис конфигурационных файлов веб-серверов (nginx -t или apachectl configtest / httpd -t) без применения изменений. Безопасная операция чтения.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("webserver_type", "Тип веб-сервера для проверки: 'nginx', 'apache', или 'all' (по умолчанию: 'all')").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		wsType, _ := args["webserver_type"].(string)
		if wsType == "" {
			wsType = "all"
		}

		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		out, err := secService.CheckWebserverSyntax(ctx, srv.ID, wsType)
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"server_id":      srv.ID,
			"webserver_type": wsType,
			"output":         strings.TrimSpace(out),
		}, nil
	})
}
