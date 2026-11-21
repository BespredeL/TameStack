package workflows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
	"tamestack/internal/ssh"
)

// SecurityAuditWorkflow automates deep server security scanning.
type SecurityAuditWorkflow struct {
	workflowBase
}

// NewSecurityAuditWorkflow creates a new security audit workflow runner.
func NewSecurityAuditWorkflow(
	database *db.DB,
	secService *security.Service,
	vault *crypto.Vault,
	pool *ssh.Pool,
	knownHosts string,
) *SecurityAuditWorkflow {
	return &SecurityAuditWorkflow{workflowBase{
		database:   database,
		secService: secService,
		vault:      vault,
		pool:       pool,
		knownHosts: knownHosts,
	}}
}

// Audit runs comprehensive security checks on a target server.
func (w *SecurityAuditWorkflow) Audit(ctx context.Context, serverID string) (*DiagnosticReport, error) {
	srv, err := w.lookupServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	report := w.newReport("security_audit", serverID, srv, srv.Host)

	score := 100

	client, err := w.dialSSH(ctx, serverID, srv, "секрета")
	if err != nil {
		return nil, err
	}

	auditData, auditErr := w.secService.RunSecurityAudit(ctx, serverID)
	if auditErr == nil && auditData != nil {
		report.Metrics["ssh_permit_root"] = auditData.SSHRuleRootLogin
		report.Metrics["ssh_password_auth"] = auditData.SSHRulePasswordAuth
		report.Metrics["ssh_port"] = auditData.SSHPort
		report.Metrics["security_updates"] = auditData.SecurityUpdatesCount
		report.Metrics["pending_updates"] = auditData.PendingUpdatesCount

		// Analyze root login.
		if strings.EqualFold(auditData.SSHRuleRootLogin, "yes") {
			score -= 20
			report.Issues = append(report.Issues, Issue{
				ID:             "sec-ssh-root-login",
				Severity:       SeverityCritical,
				Component:      "security",
				Title:          "Разрешен прямой вход root по SSH",
				Description:    "В файле sshd_config параметр PermitRootLogin установлен в 'yes'. Это создает угрозу брутфорс-атак суперпользователя.",
				Recommendation: "Создайте отдельного пользователя с правами sudo и установите 'PermitRootLogin no' или 'prohibit-password'.",
				ProposedAction: "apply_hardening",
				ActionParams:   map[string]any{"server_id": serverID, "fix_type": "ssh_root"},
			})
		}

		// Analyze password authentication.
		if strings.EqualFold(auditData.SSHRulePasswordAuth, "yes") {
			score -= 15
			report.Issues = append(report.Issues, Issue{
				ID:             "sec-ssh-password-auth",
				Severity:       SeverityWarning,
				Component:      "security",
				Title:          "Разрешена аутентификация по паролю в SSH",
				Description:    "Параметр PasswordAuthentication установлен в 'yes'. Рекомендуется использовать исключительно SSH-ключи (ed25519/rsa).",
				Recommendation: "Настройте вход по SSH-ключам и отключите PasswordAuthentication.",
				ProposedAction: "apply_hardening",
				ActionParams:   map[string]any{"server_id": serverID, "fix_type": "ssh_pass"},
			})
		}

		// Analyze security updates.
		if auditData.SecurityUpdatesCount > 0 {
			score -= 10
			report.Issues = append(report.Issues, Issue{
				ID:             "sec-pending-updates",
				Severity:       SeverityWarning,
				Component:      "system",
				Title:          fmt.Sprintf("Доступно %d обновлений безопасности", auditData.SecurityUpdatesCount),
				Description:    "В системе обнаружены не примененные критические обновления безопасности пакетов Linux.",
				Recommendation: "Выполните обновление пакетов безопасности ('apt-get update && apt-get upgrade -y').",
			})
		}
	}

	fwStatus, fwErr := w.secService.GetFirewallStatus(ctx, serverID)
	if fwErr == nil && fwStatus != nil {
		report.Metrics["firewall_type"] = fwStatus.Type
		report.Metrics["firewall_active"] = fwStatus.IsActive
		if !fwStatus.IsActive {
			score -= 25
			report.Issues = append(report.Issues, Issue{
				ID:             "sec-firewall-inactive",
				Severity:       SeverityCritical,
				Component:      "security",
				Title:          "Брандмауэр (Firewall) отключен",
				Description:    "На сервере отключен брандмауэр. Все открытые порты служб доступны из публичной сети без фильтрации.",
				Recommendation: "Включите брандмауэр и откройте только необходимые порты (22, 80, 443).",
				ProposedAction: "enable_firewall",
				ActionParams:   map[string]any{"server_id": serverID},
			})
		}
	}

	f2bStatus, f2bErr := w.secService.GetFail2banStatus(ctx, serverID)
	if f2bErr == nil && f2bStatus != nil {
		report.Metrics["fail2ban_active"] = f2bStatus.IsActive
		if !f2bStatus.IsActive {
			score -= 15
			report.Issues = append(report.Issues, Issue{
				ID:             "sec-fail2ban-inactive",
				Severity:       SeverityWarning,
				Component:      "security",
				Title:          "Служба Fail2ban не активна",
				Description:    "Fail2ban не запущен. Сервер не защищен от автоматического перебора паролей к SSH и веб-службам.",
				Recommendation: "Установите и включите Fail2ban для защиты от брутфорс-атак.",
				ProposedAction: "start_fail2ban",
				ActionParams:   map[string]any{"server_id": serverID},
			})
		}
	}

	portsOut, _, _, _ := client.Exec(ctx, "ss -tuln 2>/dev/null || netstat -tuln 2>/dev/null")
	portsText := string(portsOut)
	report.Metrics["open_ports_raw"] = portsText

	dangerousPorts := map[string]string{
		":3306 ":  "MySQL",
		":5432 ":  "PostgreSQL",
		":6379 ":  "Redis",
		":27017 ": "MongoDB",
		":11211 ": "Memcached",
	}

	for p, dbName := range dangerousPorts {
		if strings.Contains(portsText, "0.0.0.0"+p) || strings.Contains(portsText, "*"+p) || strings.Contains(portsText, "[::]"+p) {
			score -= 15
			report.Issues = append(report.Issues, Issue{
				ID:             "sec-exposed-db-" + strings.ToLower(dbName),
				Severity:       SeverityCritical,
				Component:      "database",
				Title:          fmt.Sprintf("База данных %s открыта во внешнюю сеть", dbName),
				Description:    fmt.Sprintf("Служба %s слушает порт %s на всех интерфейсах (0.0.0.0). Это создает критическую угрозу несанкционированного доступа.", dbName, strings.TrimSpace(p)),
				Recommendation: fmt.Sprintf("Привяжите %s к 127.0.0.1 (bind-address) или закройте порт в брандмауэре.", dbName),
			})
		}
	}

	bfOut, _, _, _ := client.Exec(ctx, "grep -c 'Failed password' /var/log/auth.log 2>/dev/null || journalctl -u ssh --since '24 hours ago' 2>/dev/null | grep -c 'Failed password' || echo '0'")
	bfCountStr := strings.TrimSpace(string(bfOut))
	bfCount, _ := strconv.Atoi(bfCountStr)
	report.Metrics["failed_ssh_logins_24h"] = bfCount

	if bfCount > 50 {
		report.Issues = append(report.Issues, Issue{
			ID:             "sec-ssh-bruteforce-detected",
			Severity:       SeverityWarning,
			Component:      "security",
			Title:          fmt.Sprintf("Зафиксировано %d неудачных попыток входа по SSH за сутки", bfCount),
			Description:    "Сервер подвергается активным попыткам перебора паролей.",
			Recommendation: "Убедитесь, что Fail2ban активен, смените стандартный порт SSH (22) и используйте авторизацию только по ключам.",
		})
	}

	if score < 0 {
		score = 0
	}
	report.Metrics["security_score"] = score

	if score < 60 {
		report.Status = "critical"
		report.Summary = fmt.Sprintf("Индекс безопасности сервера критический: %d/100. Обнаружено %d уязвимостей, требующих немедленного внимания.", score, len(report.Issues))
	} else if score < 85 {
		report.Status = "warning"
		report.Summary = fmt.Sprintf("Индекс безопасности сервера: %d/100. Обнаружены замечания по конфигурации SSH, брандмауэра или служб.", score)
	} else {
		report.Status = "healthy"
		report.Summary = fmt.Sprintf("Сервер защищен на высоком уровне. Индекс безопасности: %d/100.", score)
	}

	return report, nil
}
