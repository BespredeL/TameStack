package tools

import (
	"context"
	"fmt"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
)

// RegisterServerTools adds server inspection tools to the registry.
func RegisterServerTools(r *Registry, database *db.DB, secService *security.Service) {
	r.Register(domain.ToolDefinition{
		Name:        "get_server_status",
		Description: "Возвращает информацию о сервере, статус подключения, ОС и диагностические метрики.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера в TameStack").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, database, serverID)
		if err != nil {
			return nil, err
		}

		diag, _ := database.GetDiagnostics(ctx, srv.ID)

		return map[string]any{
			"server": map[string]any{
				"id":        srv.ID,
				"name":      srv.Name,
				"host":      srv.Host,
				"port":      srv.Port,
				"status":    srv.Status,
				"username":  srv.Username,
				"group":     srv.GroupName,
				"connected": srv.LastConnectedAt,
			},
			"diagnostics": diag,
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "get_system_metrics",
		Description: "Возвращает текущую загрузку CPU, оперативной памяти и диска сервера в реальном времени.",
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

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		metrics, err := secService.GetRealtimeSystemMetrics(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить метрики сервера %s (%s): %w", srv.Name, srv.Host, err)
		}

		summary := fmt.Sprintf("CPU: %.1f%% (LA: %.2f, %.2f, %.2f) | RAM: %d/%d MB (%.1f%%) | Диск: %.1f/%.1f GB (занято %.1f%%, свободно %.1f GB) | Uptime: %d сек",
			metrics.CPUPercent, metrics.Load1, metrics.Load5, metrics.Load15,
			metrics.MemoryUsedMB, metrics.MemoryTotalMB, metrics.MemoryUsedPercent,
			metrics.DiskUsedGB, metrics.DiskTotalGB, metrics.DiskUsedPercent, metrics.DiskFreeGB,
			metrics.UptimeSeconds,
		)

		return map[string]any{
			"summary": summary,
			"metrics": metrics,
		}, nil
	})
}
