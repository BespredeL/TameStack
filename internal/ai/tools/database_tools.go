package tools

import (
	"context"
	"fmt"

	"tamestack/internal/ai/domain"
	"tamestack/internal/modules/database"
)

// RegisterDatabaseTools adds database inspection tools to the registry.
func RegisterDatabaseTools(r *Registry, dbService *database.Service) {
	r.Register(domain.ToolDefinition{
		Name:        "list_databases",
		Description: "Возвращает список баз данных (MySQL, MariaDB, PostgreSQL) и их размеры на сервере.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("engine", "Тип СУБД: mysql или postgresql (по умолчанию: mysql)").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, r.database, serverID)
		if err != nil {
			return nil, err
		}
		engineStr, _ := args["engine"].(string)
		if engineStr == "" {
			engineStr = "mysql"
		}

		if dbService == nil {
			return nil, fmt.Errorf("сервис баз данных не инициализирован")
		}

		dbs, err := dbService.ListDatabases(ctx, srv.ID, database.EngineType(engineStr))
		if err != nil {
			return nil, fmt.Errorf("ошибка получения списка баз данных: %w", err)
		}

		return dbs, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "inspect_databases",
		Description: "Возвращает статус запущенных СУБД на сервере, активные порты и версии.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, r.database, serverID)
		if err != nil {
			return nil, err
		}

		if dbService == nil {
			return nil, fmt.Errorf("сервис баз данных не инициализирован")
		}

		instances, err := dbService.InspectDatabases(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка инспекции СУБД: %w", err)
		}

		return instances, nil
	})
}
