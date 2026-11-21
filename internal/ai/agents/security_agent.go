package agents

import (
	"context"
	"fmt"
	"time"

	"tamestack/internal/ai/workflows"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
	"tamestack/internal/modules/ssl"
)

// SecurityAgent periodically verifies firewall integrity, fail2ban service, and SSL expirations.
type SecurityAgent struct {
	database   *db.DB
	secService *security.Service
	sslService *ssl.Service
	interval   time.Duration
}

// NewSecurityAgent initializes the security monitoring agent.
func NewSecurityAgent(
	database *db.DB,
	secService *security.Service,
	sslService *ssl.Service,
	interval time.Duration,
) *SecurityAgent {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &SecurityAgent{
		database:   database,
		secService: secService,
		sslService: sslService,
		interval:   interval,
	}
}

func (a *SecurityAgent) ID() string {
	return "agent_security_watchdog"
}

func (a *SecurityAgent) Name() string {
	return "Агент контроля безопасности и сертификатов"
}

func (a *SecurityAgent) Description() string {
	return "Отслеживает активность брандмауэра (UFW), службу защиты от брутфорса Fail2ban и предупреждает об истекающих SSL-сертификатах."
}

func (a *SecurityAgent) DefaultInterval() time.Duration {
	return a.interval
}

func (a *SecurityAgent) Run(ctx context.Context) ([]AgentFinding, error) {
	servers, err := a.database.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения серверов: %w", err)
	}

	findings := make([]AgentFinding, 0)

	for _, srv := range servers {
		if srv.Status == "offline" {
			continue
		}

		fwStatus, fwErr := a.secService.GetFirewallStatus(ctx, srv.ID)
		if fwErr == nil && fwStatus != nil && !fwStatus.IsActive {
			findings = append(findings, AgentFinding{
				ID:        fmt.Sprintf("fw-crit-%s-%d", srv.ID, time.Now().Unix()),
				AgentID:   a.ID(),
				ServerID:  srv.ID,
				Severity:  workflows.SeverityCritical,
				Title:     fmt.Sprintf("Брандмауэр отключен на сервере %s", srv.Name),
				Details:   "Служба UFW/Firewalld не активна. Сервер открыт для входящих подключений на всех портах.",
				CreatedAt: time.Now().UTC(),
			})
		}

		f2bStatus, f2bErr := a.secService.GetFail2banStatus(ctx, srv.ID)
		if f2bErr == nil && f2bStatus != nil && !f2bStatus.IsActive {
			findings = append(findings, AgentFinding{
				ID:        fmt.Sprintf("f2b-warn-%s-%d", srv.ID, time.Now().Unix()),
				AgentID:   a.ID(),
				ServerID:  srv.ID,
				Severity:  workflows.SeverityWarning,
				Title:     fmt.Sprintf("Fail2ban не активен на сервере %s", srv.Name),
				Details:   "Служба Fail2ban остановлена. Отсутствует автоматическая блокировка атакующих IP при брутфорсе SSH.",
				CreatedAt: time.Now().UTC(),
			})
		}

		certs, certErr := a.database.ListSSLCertificates(ctx, srv.ID)
		if certErr == nil {
			for _, cert := range certs {
				daysLeft := cert.DaysRemaining
				validToStr := ""
				if cert.ValidTo != nil {
					validToStr = cert.ValidTo.Format("02.01.2006")
				}

				if daysLeft <= 3 && daysLeft >= 0 {
					findings = append(findings, AgentFinding{
						ID:        fmt.Sprintf("ssl-crit-%s-%s-%d", srv.ID, cert.PrimaryDomain, time.Now().Unix()),
						AgentID:   a.ID(),
						ServerID:  srv.ID,
						Severity:  workflows.SeverityCritical,
						Title:     fmt.Sprintf("SSL-сертификат для %s истекает через %d дн.!", cert.PrimaryDomain, daysLeft),
						Details:   fmt.Sprintf("Срок действия сертификата истекает %s. Необходим срочный перевыпуск Let's Encrypt.", validToStr),
						CreatedAt: time.Now().UTC(),
					})
				} else if daysLeft <= 14 && daysLeft > 3 {
					findings = append(findings, AgentFinding{
						ID:        fmt.Sprintf("ssl-warn-%s-%s-%d", srv.ID, cert.PrimaryDomain, time.Now().Unix()),
						AgentID:   a.ID(),
						ServerID:  srv.ID,
						Severity:  workflows.SeverityWarning,
						Title:     fmt.Sprintf("SSL-сертификат для %s истекает через %d дн.", cert.PrimaryDomain, daysLeft),
						Details:   fmt.Sprintf("Действителен до %s. Запланируйте продление сертификата.", validToStr),
						CreatedAt: time.Now().UTC(),
					})
				}
			}
		}
	}

	return findings, nil
}
