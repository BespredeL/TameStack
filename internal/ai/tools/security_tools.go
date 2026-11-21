package tools

import (
	"context"
	"fmt"

	"tamestack/internal/ai/domain"
	"tamestack/internal/modules/security"
)

// RegisterSecurityTools adds firewall, fail2ban and audit inspection tools.
func RegisterSecurityTools(r *Registry, secService *security.Service) {
	r.Register(domain.ToolDefinition{
		Name:        "get_firewall_status",
		Description: "Возвращает состояние системного фаервола (UFW или Firewalld) и перечень открытых портов.",
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

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		status, err := secService.GetFirewallStatus(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения статуса фаервола: %w", err)
		}

		return status, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "get_fail2ban_status",
		Description: "Возвращает статус службы защиты от брутфорса Fail2ban, список активных тюрем (jails) и заблокированных IP.",
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

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		f2b, err := secService.GetFail2banStatus(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения статуса fail2ban: %w", err)
		}

		return f2b, nil
	})

	r.Register(domain.ToolDefinition{
		Name:        "run_security_audit",
		Description: "Запускает комплексный аудит безопасности: проверяет открытые порты, конфигурацию SSH, пакеты обновлений и права.",
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

		if secService == nil {
			return nil, fmt.Errorf("сервис безопасности не инициализирован")
		}

		report, err := secService.RunSecurityAudit(ctx, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка проведения аудита безопасности: %w", err)
		}

		return report, nil
	})
}
