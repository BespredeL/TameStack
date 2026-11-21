package tools

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/db"
)

// ResolveServer finds a server record by ID, Name, or Host.
// If query is empty and there is only 1 server in the database, it returns that server.
func ResolveServer(ctx context.Context, database *db.DB, query string) (*db.ServerRecord, error) {
	if database == nil {
		return nil, fmt.Errorf("база данных недоступна")
	}

	trimmed := strings.TrimSpace(query)

	// 1. If explicit ID provided, try direct lookup.
	if trimmed != "" {
		srv, err := database.GetServer(ctx, trimmed)
		if err == nil && srv != nil {
			return srv, nil
		}
	}

	// 2. Query all servers for name / host matching.
	servers, err := database.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка серверов: %w", err)
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("в TameStack не добавлено ни одного сервера")
	}

	// If query was empty and there is only 1 server, return it.
	if trimmed == "" {
		if len(servers) == 1 {
			return servers[0], nil
		}
		return nil, fmt.Errorf("не указан сервер (в системе %d серверов, укажите имя или ID)", len(servers))
	}

	// Exact name match (case-insensitive).
	for _, s := range servers {
		if strings.EqualFold(s.Name, trimmed) {
			return s, nil
		}
	}

	// Exact host / IP match.
	for _, s := range servers {
		if strings.EqualFold(s.Host, trimmed) {
			return s, nil
		}
	}

	return nil, fmt.Errorf("сервер '%s' не найден в TameStack", trimmed)
}
