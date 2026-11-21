package tools

import (
	"context"
	"fmt"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/workflows"
)

// RegisterWorkflowTools registers high-level automated diagnostic and optimization workflows.
func RegisterWorkflowTools(
	r *Registry,
	siteWf *workflows.SiteDiagnosticsWorkflow,
	secWf *workflows.SecurityAuditWorkflow,
	optWf *workflows.ConfigOptimizerWorkflow,
) {
	r.Register(domain.ToolDefinition{
		Name:        "diagnose_site_outage",
		Description: "Выполняет полную автоматизированную диагностику сбоя сайта: проверка DNS, HTTP/HTTPS, SSL, веб-сервера (Nginx/Apache), сокетов PHP-FPM и анализ логов ошибок.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "ID сервера в TameStack").
			str("site", "Домен сайта или ID сайта").
			req("server_id", "site").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		siteTarget, _ := args["site"].(string)
		if siteTarget == "" {
			return nil, fmt.Errorf("site обязателен")
		}
		srv, err := ResolveServer(ctx, r.database, serverID)
		if err != nil {
			return nil, err
		}
		return siteWf.Diagnose(ctx, srv.ID, siteTarget)
	})

	r.Register(domain.ToolDefinition{
		Name:        "run_security_audit",
		Description: "Запускает комплексный аудит безопасности сервера: проверка SSH (root, пароли, порты), брандмауэра (UFW), Fail2ban, опасных открытых портов (БД наружу) и попыток брутфорса.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "ID сервера в TameStack").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, r.database, serverID)
		if err != nil {
			return nil, err
		}
		return secWf.Audit(ctx, srv.ID)
	})

	r.Register(domain.ToolDefinition{
		Name:        "optimize_server_configs",
		Description: "Анализирует объем оперативной памяти и ядер CPU сервера, рассчитывает оптимальные значения для MySQL/MariaDB (InnoDB Buffer Pool), PHP-FPM (pm.max_children) и Nginx (worker_processes/gzip), генерируя рекомендации.",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: newSchema().
			str("server_id", "ID сервера в TameStack").
			req("server_id").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, r.database, serverID)
		if err != nil {
			return nil, err
		}
		return optWf.Optimize(ctx, srv.ID)
	})
}
