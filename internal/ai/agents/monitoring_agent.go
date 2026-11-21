package agents

import (
	"context"
	"fmt"
	"time"

	"tamestack/internal/ai/workflows"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
)

// MonitoringAgent periodically inspects CPU, RAM, and Disk metrics across all connected servers.
type MonitoringAgent struct {
	database   *db.DB
	secService *security.Service
	interval   time.Duration
}

// NewMonitoringAgent creates a resource monitoring autonomous agent.
func NewMonitoringAgent(database *db.DB, secService *security.Service, interval time.Duration) *MonitoringAgent {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &MonitoringAgent{
		database:   database,
		secService: secService,
		interval:   interval,
	}
}

func (a *MonitoringAgent) ID() string {
	return "agent_monitoring_resources"
}

func (a *MonitoringAgent) Name() string {
	return "Агент мониторинга ресурсов и аномалий"
}

func (a *MonitoringAgent) Description() string {
	return "Непрерывно отслеживает критическое потребление диска (>90%), перегрузку памяти (>95%) и всплески CPU на серверах."
}

func (a *MonitoringAgent) DefaultInterval() time.Duration {
	return a.interval
}

func (a *MonitoringAgent) Run(ctx context.Context) ([]AgentFinding, error) {
	servers, err := a.database.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения серверов: %w", err)
	}

	findings := make([]AgentFinding, 0)

	for _, srv := range servers {
		if srv.Status == "offline" {
			continue
		}

		metrics, err := a.secService.GetRealtimeSystemMetrics(ctx, srv.ID)
		if err != nil {
			// Skip the server while it is temporarily unavailable.
			continue
		}

		if metrics.DiskUsedPercent >= 90.0 {
			findings = append(findings, AgentFinding{
				ID:        fmt.Sprintf("disk-crit-%s-%d", srv.ID, time.Now().Unix()),
				AgentID:   a.ID(),
				ServerID:  srv.ID,
				Severity:  workflows.SeverityCritical,
				Title:     fmt.Sprintf("Критическое переполнение диска на %s: %.1f%%", srv.Name, metrics.DiskUsedPercent),
				Details:   fmt.Sprintf("Занято %.1f ГБ из %.1f ГБ (осталось всего %.1f ГБ). Существует риск аварийной остановки MySQL и сервисов.", metrics.DiskUsedGB, metrics.DiskTotalGB, metrics.DiskFreeGB),
				CreatedAt: time.Now().UTC(),
			})
		} else if metrics.DiskUsedPercent >= 80.0 {
			findings = append(findings, AgentFinding{
				ID:        fmt.Sprintf("disk-warn-%s-%d", srv.ID, time.Now().Unix()),
				AgentID:   a.ID(),
				ServerID:  srv.ID,
				Severity:  workflows.SeverityWarning,
				Title:     fmt.Sprintf("Высокая занятость диска на %s: %.1f%%", srv.Name, metrics.DiskUsedPercent),
				Details:   fmt.Sprintf("Занято %.1f ГБ из %.1f ГБ. Рекомендуется очистить старые логи или временные файлы.", metrics.DiskUsedGB, metrics.DiskTotalGB),
				CreatedAt: time.Now().UTC(),
			})
		}

		if metrics.MemoryUsedPercent >= 95.0 {
			findings = append(findings, AgentFinding{
				ID:        fmt.Sprintf("ram-crit-%s-%d", srv.ID, time.Now().Unix()),
				AgentID:   a.ID(),
				ServerID:  srv.ID,
				Severity:  workflows.SeverityCritical,
				Title:     fmt.Sprintf("Критическая нехватка RAM на %s: %.1f%%", srv.Name, metrics.MemoryUsedPercent),
				Details:   fmt.Sprintf("Занято %d МБ из %d МБ. Ядро Linux может принудительно остановить процессы (OOM Killer).", metrics.MemoryUsedMB, metrics.MemoryTotalMB),
				CreatedAt: time.Now().UTC(),
			})
		} else if metrics.MemoryUsedPercent >= 85.0 {
			findings = append(findings, AgentFinding{
				ID:        fmt.Sprintf("ram-warn-%s-%d", srv.ID, time.Now().Unix()),
				AgentID:   a.ID(),
				ServerID:  srv.ID,
				Severity:  workflows.SeverityWarning,
				Title:     fmt.Sprintf("Повышенная утилизация RAM на %s: %.1f%%", srv.Name, metrics.MemoryUsedPercent),
				Details:   fmt.Sprintf("Свободно всего %d МБ памяти.", metrics.MemoryFreeMB),
				CreatedAt: time.Now().UTC(),
			})
		}

		if metrics.CPUPercent >= 90.0 {
			findings = append(findings, AgentFinding{
				ID:        fmt.Sprintf("cpu-warn-%s-%d", srv.ID, time.Now().Unix()),
				AgentID:   a.ID(),
				ServerID:  srv.ID,
				Severity:  workflows.SeverityWarning,
				Title:     fmt.Sprintf("Высокая загрузка процессора на %s: %.1f%%", srv.Name, metrics.CPUPercent),
				Details:   fmt.Sprintf("Load Average: %.2f, %.2f, %.2f.", metrics.Load1, metrics.Load5, metrics.Load15),
				CreatedAt: time.Now().UTC(),
			})
		}
	}

	return findings, nil
}
