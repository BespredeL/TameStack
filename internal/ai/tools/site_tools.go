package tools

import (
	"context"
	"fmt"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/policies"
	"tamestack/internal/db"
	"tamestack/internal/modules/site"
	"tamestack/internal/modules/webserver"
)

// RegisterSiteTools adds website and virtual host inspection tools to the registry.
func RegisterSiteTools(r *Registry, database *db.DB, siteService *site.Service, wsService *webserver.Service) {
	r.Register(domain.ToolDefinition{
		Name:        "list_sites",
		Description: "Возвращает список размещенных сайтов и виртуальных хостов на сервере (домены, корни документов, версии PHP, SSL).",
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

		sites, err := database.ListSites(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения сайтов: %w", err)
		}

		return sites, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "get_vhost_config",
		Description: "Читает содержимое конфигурационного файла VirtualHost (Nginx или Apache) для анализа и выявления ошибок синтаксиса.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("file_path", "Путь к файлу конфигурации (например, /etc/nginx/sites-available/example.com.conf)").
			req("server_id", "file_path").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		filePath, _ := args["file_path"].(string)
		if filePath == "" {
			return nil, fmt.Errorf("file_path обязателен")
		}
		if err := policies.NewSecurityGate().ValidatePath(filePath); err != nil {
			return nil, err
		}

		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		if wsService == nil {
			return nil, fmt.Errorf("сервис веб-серверов не инициализирован")
		}

		content, err := wsService.GetConfig(ctx, srv.ID, filePath)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения конфигурационного файла %s: %w", filePath, err)
		}

		return map[string]any{
			"file_path": filePath,
			"content":   content,
		}, nil
	})
}
