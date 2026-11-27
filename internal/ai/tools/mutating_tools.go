package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/policies"
	"tamestack/internal/db"
	"tamestack/internal/modules/webserver"
	"tamestack/internal/tasks"
)

// AllowedServicesToRestart defines the whitelist of services AI is permitted to restart upon approval.
var AllowedServicesToRestart = map[string]bool{
	"nginx":        true,
	"apache2":      true,
	"httpd":        true,
	"mysql":        true,
	"mariadb":      true,
	"postgresql":   true,
	"redis":        true,
	"redis-server": true,
	"fail2ban":     true,
	"docker":       true,
	"postfix":      true,
	"dovecot":      true,
	"opendkim":     true,
	"bind9":        true,
	"named":        true,
	"php-fpm":      true,
	"php8.0-fpm":   true,
	"php8.1-fpm":   true,
	"php8.2-fpm":   true,
	"php8.3-fpm":   true,
	"php8.4-fpm":   true,
}

// ConfigChangePayload is stored in the task record for applying approved config diffs.
type ConfigChangePayload struct {
	ServerID           string `json:"server_id"`
	FilePath           string `json:"file_path"`
	ProposedContent    string `json:"proposed_content"`
	BackupPath         string `json:"backup_path"`
	SyntaxCheckCommand string `json:"syntax_check_command"`
	ReloadCommand      string `json:"reload_command"`
}

// ServiceRestartPayload is stored in the task record for restarting an approved service.
type ServiceRestartPayload struct {
	ServerID    string `json:"server_id"`
	ServiceName string `json:"service_name"`
}

// RegisterMutatingTools adds mutating tools that require Human-in-the-Loop approval.
func RegisterMutatingTools(
	r *Registry,
	database *db.DB,
	wsService *webserver.Service,
	engine *tasks.Engine,
) {
	r.Register(domain.ToolDefinition{
		Name:            "prepare_config_diff",
		Description:     "Подготавливает план изменения конфигурационного файла (Nginx, Apache, PHP, MySQL) с расчетом Diff. Требует обязательного подтверждения пользователем.",
		RiskLevel:       domain.RiskHigh,
		IsMutating:      true,
		ApprovalViaTask: true,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("file_path", "Полный путь к конфигурационному файлу (например /etc/nginx/sites-available/site.conf)").
			str("proposed_content", "Новое предлагаемое содержимое файла").
			str("webserver_type", "Тип веб-сервера (nginx или apache)").
			req("server_id", "file_path", "proposed_content").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		filePath, _ := args["file_path"].(string)
		proposedContent, _ := args["proposed_content"].(string)
		wsType, _ := args["webserver_type"].(string)

		if filePath == "" || proposedContent == "" {
			return nil, fmt.Errorf("file_path и proposed_content обязательны")
		}

		srv, err := ResolveServer(ctx, r.database, serverID)
		if err != nil {
			return nil, err
		}
		serverID = srv.ID

		currentContent := ""
		if wsService != nil {
			cur, err := wsService.GetConfig(ctx, serverID, filePath)
			if err != nil {
				return nil, fmt.Errorf("не удалось прочитать текущий файл %s для расчёта diff: %w", filePath, err)
			}
			currentContent = cur
		}

		plan, err := policies.GenerateConfigChangePlan(serverID, filePath, currentContent, proposedContent, wsType)
		if err != nil {
			return nil, err
		}

		payload := ConfigChangePayload{
			ServerID:           serverID,
			FilePath:           filePath,
			ProposedContent:    proposedContent,
			BackupPath:         plan.BackupPath,
			SyntaxCheckCommand: plan.SyntaxCheckCommand,
		}
		if wsType == "nginx" || strings.Contains(filePath, "nginx") {
			payload.ReloadCommand = "systemctl reload nginx"
		} else if wsType == "apache" || strings.Contains(filePath, "apache") || strings.Contains(filePath, "httpd") {
			payload.ReloadCommand = "systemctl reload apache2 || systemctl reload httpd"
		}

		payloadBytes, _ := json.Marshal(payload)

		if engine == nil {
			return nil, fmt.Errorf("движок задач недоступен: подтверждение изменений невозможно")
		}
		taskRecord, err := engine.SubmitRequiresApproval(
			serverID,
			"ai_apply_config_diff",
			fmt.Sprintf("AI: Применение изменений в %s", filePath),
			string(payloadBytes),
		)
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"status":            "awaiting_approval",
			"task_id":           taskRecord.ID,
			"plan_id":           plan.ID,
			"title":             plan.Title,
			"diff":              plan.Diff,
			"backup_path":       plan.BackupPath,
			"syntax_check":      plan.SyntaxCheckCommand,
			"steps":             plan.Steps,
			"requires_approval": true,
		}, nil
	})

	r.Register(domain.ToolDefinition{
		Name:            "propose_service_restart",
		Description:     "Формирует запрос на перезапуск системной службы (nginx, apache2, php-fpm, mysql). Требует подтверждения пользователем.",
		RiskLevel:       domain.RiskModerate,
		IsMutating:      true,
		ApprovalViaTask: true,
		Parameters: newSchema().
			str("server_id", "Идентификатор сервера").
			str("service_name", "Имя службы: nginx, apache2, php8.2-fpm, mysql, mariadb, redis, fail2ban").
			req("server_id", "service_name").
			build(),
	}, func(ctx context.Context, args map[string]any) (any, error) {
		serverID, _ := args["server_id"].(string)
		srv, err := ResolveServer(ctx, r.database, serverID)
		if err != nil {
			return nil, err
		}
		serverID = srv.ID

		serviceName, _ := args["service_name"].(string)
		serviceName = strings.ToLower(strings.TrimSpace(serviceName))

		if !AllowedServicesToRestart[serviceName] {
			return nil, fmt.Errorf("служба '%s' не входит в список разрешенных для перезапуска через AI", serviceName)
		}

		payload := ServiceRestartPayload{
			ServerID:    serverID,
			ServiceName: serviceName,
		}
		payloadBytes, _ := json.Marshal(payload)

		if engine == nil {
			return nil, fmt.Errorf("движок задач недоступен: подтверждение перезапуска невозможно")
		}
		taskRecord, err := engine.SubmitRequiresApproval(
			serverID,
			"ai_restart_service",
			fmt.Sprintf("AI: Перезапуск службы %s", serviceName),
			string(payloadBytes),
		)
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"status":            "awaiting_approval",
			"task_id":           taskRecord.ID,
			"service_name":      serviceName,
			"requires_approval": true,
			"description":       fmt.Sprintf("Запрошен перезапуск службы %s на сервере %s", serviceName, serverID),
		}, nil
	})
}
