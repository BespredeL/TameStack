package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
	"tamestack/internal/modules/site"
	"tamestack/internal/ssh"
)

// SiteDiagnosticsWorkflow handles automated troubleshooting of website outages.
type SiteDiagnosticsWorkflow struct {
	workflowBase
	siteService *site.Service
}

// NewSiteDiagnosticsWorkflow creates a new workflow runner.
func NewSiteDiagnosticsWorkflow(
	database *db.DB,
	siteService *site.Service,
	secService *security.Service,
	vault *crypto.Vault,
	pool *ssh.Pool,
	knownHosts string,
) *SiteDiagnosticsWorkflow {
	return &SiteDiagnosticsWorkflow{
		workflowBase: workflowBase{
			database:   database,
			secService: secService,
			vault:      vault,
			pool:       pool,
			knownHosts: knownHosts,
		},
		siteService: siteService,
	}
}

// Diagnose executes full multi-step diagnostic pipeline for a given site.
func (w *SiteDiagnosticsWorkflow) Diagnose(ctx context.Context, serverID string, siteIDOrDomain string) (*DiagnosticReport, error) {
	srv, err := w.lookupServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	report := w.newReport("site_outage", serverID, srv, siteIDOrDomain)

	var targetSite *db.SiteRecord
	sites, _ := w.database.ListSites(ctx, serverID)
	for _, s := range sites {
		if s.ID == siteIDOrDomain || strings.EqualFold(s.Domain, siteIDOrDomain) {
			targetSite = s
			break
		}
	}

	domainName := siteIDOrDomain
	if targetSite != nil {
		domainName = targetSite.Domain
		report.Target = domainName
		report.Metrics["site_id"] = targetSite.ID
		report.Metrics["webserver"] = targetSite.WebServer
		report.Metrics["php_version"] = targetSite.PHPVersion
	}

	checkURL := "https://" + domainName
	epCheck, err := w.secService.CheckEndpoint(checkURL)
	if err != nil || epCheck == nil || epCheck.StatusCode == 0 {
		// Try the HTTP fallback.
		checkURL = "http://" + domainName
		epCheck, _ = w.secService.CheckEndpoint(checkURL)
	}

	if epCheck != nil {
		report.Metrics["http_status"] = epCheck.StatusCode
		report.Metrics["latency_ms"] = epCheck.ResponseTimeMs
		report.Metrics["is_up"] = epCheck.IsUp
		report.Metrics["ssl_days_left"] = epCheck.SSLDaysRemaining

		if epCheck.StatusCode >= 500 {
			report.Issues = append(report.Issues, Issue{
				ID:             "http-server-error",
				Severity:       SeverityCritical,
				Component:      "webserver",
				Title:          fmt.Sprintf("Сайт возвращает ошибку %d", epCheck.StatusCode),
				Description:    fmt.Sprintf("Внешний запрос к %s завершился со статусом %d.", checkURL, epCheck.StatusCode),
				Recommendation: "Проверьте логи ошибок веб-сервера и статус бэкенда (PHP-FPM, Node.js или Docker).",
			})
		} else if epCheck.StatusCode == 0 {
			report.Issues = append(report.Issues, Issue{
				ID:             "http-unreachable",
				Severity:       SeverityCritical,
				Component:      "network",
				Title:          "Сайт недоступен из сети",
				Description:    fmt.Sprintf("Не удалось установить соединение с %s: %s", checkURL, epCheck.ErrorMessage),
				Recommendation: "Проверьте работу веб-сервера, правила брандмауэра и DNS-записи.",
			})
		}

		if epCheck.SSLDaysRemaining <= 0 && epCheck.ErrorMessage != "" && strings.Contains(strings.ToLower(epCheck.ErrorMessage), "certificate") {
			report.Issues = append(report.Issues, Issue{
				ID:             "ssl-invalid",
				Severity:       SeverityWarning,
				Component:      "ssl",
				Title:          "Проблема с SSL-сертификатом",
				Description:    epCheck.ErrorMessage,
				Recommendation: "Перевыпустите сертификат Let's Encrypt через раздел SSL.",
				ProposedAction: "issue_ssl",
				ActionParams:   map[string]any{"server_id": serverID, "domain": domainName},
			})
		}
	}

	dnsRes, dnsErr := w.siteService.CheckDNS(ctx, domainName, serverID)
	if dnsErr == nil && dnsRes != nil {
		report.Metrics["dns_ips"] = dnsRes.ResolvedIPs
		report.Metrics["server_ip"] = dnsRes.ServerIP
		if !dnsRes.Match && len(dnsRes.ResolvedIPs) > 0 {
			resolvedStr := strings.Join(dnsRes.ResolvedIPs, ", ")
			report.Issues = append(report.Issues, Issue{
				ID:             "dns-mismatch",
				Severity:       SeverityCritical,
				Component:      "dns",
				Title:          "DNS указывает на другой IP-адрес",
				Description:    fmt.Sprintf("Домен %s резолвится в [%s], тогда как IP сервера TameStack: %s.", domainName, resolvedStr, dnsRes.ServerIP),
				Recommendation: fmt.Sprintf("Обновите A-запись домена у DNS-провайдера на IP %s.", dnsRes.ServerIP),
			})
		}
	}

	opts, decErr := ssh.OptionsForServer(w.vault, srv, w.knownHosts, 15*time.Second)
	if decErr != nil {
		report.Issues = append(report.Issues, Issue{
			ID:          "ssh-decrypt-failed",
			Severity:    SeverityCritical,
			Component:   "security",
			Title:       "Не удалось расшифровать ключ SSH",
			Description: decErr.Error(),
		})
		report.Status = "error"
		report.Summary = "Диагностика прервана: нет доступа к учетным данным сервера."
		return report, nil
	}

	client, connErr := w.pool.Get(ctx, serverID, opts)
	if connErr != nil {
		report.Issues = append(report.Issues, Issue{
			ID:          "ssh-connection-failed",
			Severity:    SeverityCritical,
			Component:   "ssh",
			Title:       "Сервер недоступен по SSH",
			Description: connErr.Error(),
		})
		report.Status = "error"
		report.Summary = "Не удалось подключиться к серверу по SSH для выполнения диагностики."
		return report, nil
	}

	webserverType := "nginx"
	if targetSite != nil && targetSite.WebServer != "" {
		webserverType = strings.ToLower(targetSite.WebServer)
	}

	svcToCheck := "nginx"
	if strings.Contains(webserverType, "apache") {
		svcToCheck = "apache2"
	}

	stdout, _, _, _ := client.Exec(ctx, fmt.Sprintf("systemctl is-active %s || systemctl is-active httpd", svcToCheck))
	svcStatus := strings.TrimSpace(string(stdout))
	report.Metrics["service_"+svcToCheck] = svcStatus

	if svcStatus != "active" {
		report.Issues = append(report.Issues, Issue{
			ID:             "webserver-inactive",
			Severity:       SeverityCritical,
			Component:      "webserver",
			Title:          fmt.Sprintf("Служба веб-сервера %s остановлена", svcToCheck),
			Description:    fmt.Sprintf("systemctl is-active вернул статус: '%s'. Сайт не может обслуживать запросы.", svcStatus),
			Recommendation: fmt.Sprintf("Запустите службу командой 'systemctl start %s'.", svcToCheck),
			ProposedAction: "restart_service",
			ActionParams:   map[string]any{"server_id": serverID, "service_name": svcToCheck},
		})
	}

	var testCmd string
	if strings.Contains(svcToCheck, "apache") {
		testCmd = "apachectl configtest 2>&1 || httpd -t 2>&1"
	} else {
		testCmd = "nginx -t 2>&1"
	}
	testOut, _, exitCode, _ := client.Exec(ctx, testCmd)
	cfgOutput := string(testOut)
	if exitCode != 0 || strings.Contains(cfgOutput, "syntax is bad") || strings.Contains(cfgOutput, "test failed") {
		report.Issues = append(report.Issues, Issue{
			ID:             "webserver-syntax-error",
			Severity:       SeverityCritical,
			Component:      "webserver",
			Title:          "Синтаксическая ошибка в конфигурации веб-сервера",
			Description:    strings.TrimSpace(cfgOutput),
			Recommendation: "Исправьте синтаксис конфигурационного файла перед перезапуском службы.",
		})
	}

	if targetSite != nil && targetSite.PHPVersion != "" && targetSite.PHPVersion != "none" {
		phpSvc := fmt.Sprintf("php%s-fpm", targetSite.PHPVersion)
		phpOut, _, _, _ := client.Exec(ctx, fmt.Sprintf("systemctl is-active %s || systemctl is-active php-fpm", phpSvc))
		phpStatus := strings.TrimSpace(string(phpOut))
		report.Metrics["php_fpm_status"] = phpStatus

		if phpStatus != "active" {
			report.Issues = append(report.Issues, Issue{
				ID:             "php-fpm-inactive",
				Severity:       SeverityCritical,
				Component:      "php",
				Title:          fmt.Sprintf("Служба PHP-FPM (%s) не активна", phpSvc),
				Description:    fmt.Sprintf("Статус службы: '%s'. Запросы к PHP-скриптам будут возвращать ошибку 502 Bad Gateway.", phpStatus),
				Recommendation: fmt.Sprintf("Запустите службу 'systemctl start %s'.", phpSvc),
				ProposedAction: "restart_service",
				ActionParams:   map[string]any{"server_id": serverID, "service_name": phpSvc},
			})
		}
	}

	var errLogCmd string
	if targetSite != nil && targetSite.Domain != "" {
		errLogCmd = fmt.Sprintf("tail -n 25 /var/log/nginx/%s.error.log /var/log/apache2/%s.error.log /var/log/nginx/error.log 2>/dev/null | grep -E -i 'error|crit|alert|emerg|denied|failed|refused' | tail -n 10", targetSite.Domain, targetSite.Domain)
	} else {
		errLogCmd = "tail -n 25 /var/log/nginx/error.log /var/log/apache2/error.log 2>/dev/null | grep -E -i 'error|crit|alert|emerg|denied|failed|refused' | tail -n 10"
	}

	logOut, _, _, _ := client.Exec(ctx, errLogCmd)
	logErrors := strings.TrimSpace(string(logOut))
	if logErrors != "" {
		report.Metrics["recent_log_errors"] = logErrors
		if strings.Contains(strings.ToLower(logErrors), "permission denied") {
			report.Issues = append(report.Issues, Issue{
				ID:             "fs-permission-denied",
				Severity:       SeverityWarning,
				Component:      "filesystem",
				Title:          "Ошибки прав доступа к файлам сайта",
				Description:    "В логах веб-сервера обнаружены записи 'Permission denied'. Веб-сервер не может прочитать файлы или выполнить скрипты.",
				Recommendation: "Проверьте владельца директории сайта (обычно www-data:www-data) и права доступа (755 для директорий, 644 для файлов).",
			})
		}
		if strings.Contains(strings.ToLower(logErrors), "connection refused") {
			report.Issues = append(report.Issues, Issue{
				ID:             "upstream-refused",
				Severity:       SeverityCritical,
				Component:      "webserver",
				Title:          "Отказ в соединении с upstream-бэкендом",
				Description:    "В логах зафиксировано 'Connection refused' при обращении к бэкенду.",
				Recommendation: "Убедитесь, что upstream-приложение (PHP-FPM, Node, Python или Docker-контейнер) слушает назначенный порт/сокет.",
			})
		}
	}

	// Compute the overall status and build the summary.
	criticalCount := 0
	warningCount := 0
	for _, iss := range report.Issues {
		if iss.Severity == SeverityCritical {
			criticalCount++
		} else if iss.Severity == SeverityWarning {
			warningCount++
		}
	}

	if criticalCount > 0 {
		report.Status = "critical"
		report.Summary = fmt.Sprintf("Обнаружено %d критических проблем, приводящих к недоступности сайта %s.", criticalCount, domainName)
	} else if warningCount > 0 {
		report.Status = "warning"
		report.Summary = fmt.Sprintf("Сайт %s доступен, однако обнаружено %d потенциальных проблем.", domainName, warningCount)
	} else {
		report.Status = "healthy"
		report.Summary = fmt.Sprintf("Диагностика сайта %s завершена: веб-сервер, DNS и ключевые сервисы работают в штатном режиме.", domainName)
	}

	return report, nil
}
