package tools

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
	"tamestack/internal/modules/apps"
	"tamestack/internal/modules/dns"
	"tamestack/internal/modules/mail"
	"tamestack/internal/modules/users"
)

func resolveSiteID(ctx context.Context, database *db.DB, serverID, siteOrDomain string) (string, error) {
	target := strings.TrimSpace(siteOrDomain)
	if target == "" {
		return "", fmt.Errorf("параметр site обязателен")
	}
	if database == nil {
		return "", fmt.Errorf("база данных недоступна")
	}
	if s, err := database.GetSite(ctx, target); err == nil && s != nil {
		return s.ID, nil
	}
	sites, err := database.ListSites(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("ошибка получения списка сайтов: %w", err)
	}
	for _, s := range sites {
		if strings.EqualFold(s.Domain, target) {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("сайт '%s' не найден на сервере", target)
}

// RegisterPlatformTools registers read-only AI inspection tools for app runtimes, users/SFTP, mail, and DNS.
func RegisterPlatformTools(
	r *Registry,
	database *db.DB,
	appsService *apps.Service,
	usersService *users.Service,
	mailService *mail.Service,
	dnsService *dns.Service,
) {
	// 1. inspect_site_app
	r.Register(domain.ToolDefinition{
		Name:        "inspect_site_app",
		Description: "Инспектирует веб-приложение сайта: определяет фреймворк (Laravel, WordPress, Node.js, Python, Go), версию PHP, наличие .env, статус queue worker и scheduler.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор или имя сервера").
			str("site", "Домен сайта или ID сайта в TameStack").
			req("server_id", "site").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if appsService == nil {
			return nil, fmt.Errorf("сервис приложений не инициализирован")
		}
		siteTarget, _ := args["site"].(string)
		siteID, err := resolveSiteID(ctx, database, srv.ID, siteTarget)
		if err != nil {
			return nil, err
		}
		res, err := appsService.InspectSiteApp(ctx, siteID)
		if err != nil {
			return nil, fmt.Errorf("ошибка инспекции приложения: %w", err)
		}
		return res, nil
	})

	// 2. get_app_process_status
	r.Register(domain.ToolDefinition{
		Name:        "get_app_process_status",
		Description: "Возвращает статус systemd-процесса приложения (Node.js / Python / Go): имя юнита, активность, PID, память, команду запуска и последние логи.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор или имя сервера").
			str("site", "Домен сайта или ID сайта в TameStack").
			req("server_id", "site").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if appsService == nil {
			return nil, fmt.Errorf("сервис приложений не инициализирован")
		}
		siteTarget, _ := args["site"].(string)
		siteID, err := resolveSiteID(ctx, database, srv.ID, siteTarget)
		if err != nil {
			return nil, err
		}
		st, err := appsService.GetAppProcessStatus(ctx, siteID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения статуса процесса приложения: %w", err)
		}
		return st, nil
	})

	// 3. list_system_users
	r.Register(domain.ToolDefinition{
		Name:        "list_system_users",
		Description: "Возвращает список системных и SFTP-пользователей сервера: UID/GID, домашние каталоги, shell и статус SFTP Chroot-изоляции.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор или имя сервера").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if usersService == nil {
			return nil, fmt.Errorf("сервис пользователей не инициализирован")
		}
		list, err := usersService.ListUsers(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения списка пользователей: %w", err)
		}
		return map[string]any{
			"count": len(list),
			"users": list,
		}, nil
	})

	// 4. list_user_ssh_keys
	r.Register(domain.ToolDefinition{
		Name:        "list_user_ssh_keys",
		Description: "Возвращает список авторизованных публичных SSH-ключей (authorized_keys) указанного пользователя на сервере. Безопасная операция чтения (только открытые ключи).",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор или имя сервера").
			str("username", "Имя системного пользователя Linux").
			req("server_id", "username").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if usersService == nil {
			return nil, fmt.Errorf("сервис пользователей не инициализирован")
		}
		username, _ := args["username"].(string)
		keys, err := usersService.GetAuthorizedKeys(ctx, srv.ID, username)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения SSH-ключей: %w", err)
		}
		return map[string]any{
			"username": username,
			"count":    len(keys),
			"keys":     keys,
		}, nil
	})

	// 5. inspect_mail_server
	r.Register(domain.ToolDefinition{
		Name:        "inspect_mail_server",
		Description: "Инспектирует почтовый стек сервера (Postfix, Dovecot, OpenDKIM): статус служб, почтовые домены, DNS-записи SPF/DMARC/DKIM, почтовые ящики и алиасы переадресации.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор или имя сервера").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if mailService == nil {
			return nil, fmt.Errorf("почтовый сервис не инициализирован")
		}
		status, err := mailService.GetMailStatus(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка инспекции почтового сервера: %w", err)
		}
		return status, nil
	})

	// 6. inspect_dns_zones
	r.Register(domain.ToolDefinition{
		Name:        "inspect_dns_zones",
		Description: "Возвращает статус DNS-сервера BIND9 и список локальных DNS-зон. Если указан параметр domain, также возвращает записи (A, AAAA, MX, TXT, CNAME, NS) этой зоны.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор или имя сервера").
			str("domain", "Необязательное имя домена для чтения записей конкретной зоны").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}
		if dnsService == nil {
			return nil, fmt.Errorf("DNS-сервис не инициализирован")
		}
		status, err := dnsService.GetDNSStatus(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка инспекции DNS: %w", err)
		}
		domainName, _ := args["domain"].(string)
		domainName = strings.TrimSpace(domainName)
		if domainName == "" {
			return status, nil
		}
		zone, err := dnsService.GetZone(ctx, srv.ID, domainName)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения зоны %s: %w", domainName, err)
		}
		return map[string]any{
			"status": status,
			"zone":   zone,
		}, nil
	})
}
