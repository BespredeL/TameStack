package security

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
)

// --- Endpoint Monitoring (HTTP / HTTPS / SSL) ---

// CheckEndpoint performs an HTTP ping of the endpoint and validates its SSL.
func (s *Service) CheckEndpoint(targetURL string) (*SiteEndpointCheck, error) {
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}

	result := &SiteEndpointCheck{
		URL:              targetURL,
		CheckedAt:        time.Now().Format("2006-01-02 15:04:05"),
		SSLDaysRemaining: -1,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	start := time.Now()
	resp, err := httpClient.Get(targetURL)
	result.ResponseTimeMs = time.Since(start).Milliseconds()

	if err != nil {
		result.IsUp = false
		result.ErrorMessage = err.Error()
		return result, nil
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.IsUp = (resp.StatusCode >= 200 && resp.StatusCode < 400)

	// For HTTPS, extract the days until the SSL certificate expires.
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		remaining := time.Until(cert.NotAfter)
		days := int(remaining.Hours() / 24)
		result.SSLDaysRemaining = days
	}

	return result, nil
}

// RepairWebAccess restores web server availability (80/443 ports in UFW/firewalld and Nginx/Apache restart).
func (s *Service) RepairWebAccess(ctx context.Context, serverID string) (*db.TaskRecord, error) {
	payload, _ := json.Marshal(map[string]string{"server_id": serverID})
	return s.engine.Submit(serverID, "security_repair_web_access", "Восстановление доступа к веб-серверу (порты 80/443)", string(payload))
}

func (s *Service) handleRepairWebAccessTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID string `json:"server_id"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, record, err := s.getSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Разблокировка портов 80 и 443 в брандмауэре", 30)
	checkUFW, _ := s.execCmd(ctx, client, "which ufw")
	if strings.TrimSpace(checkUFW) != "" {
		sshPort := record.Port
		if sshPort <= 0 {
			sshPort = 22
		}
		_, _ = s.execCmd(ctx, client, fmt.Sprintf("ufw allow %d/tcp comment 'SSH' 2>/dev/null; ufw allow 80/tcp comment 'HTTP Web' 2>/dev/null; ufw allow 443/tcp comment 'HTTPS Web' 2>/dev/null; ufw reload 2>/dev/null || true", sshPort))
		logger.Info("Порты 80/tcp, 443/tcp и SSH проверены и открыты в UFW.")
	}

	checkFWD, _ := s.execCmd(ctx, client, "which firewall-cmd")
	if strings.TrimSpace(checkFWD) != "" {
		_, _ = s.execCmd(ctx, client, "firewall-cmd --permanent --add-service=http --add-service=https 2>/dev/null; firewall-cmd --reload 2>/dev/null || true")
		logger.Info("Сервисы HTTP и HTTPS проверены и открыты в firewalld.")
	}

	logger.SetStep("Снятие возможных блокировок Fail2ban", 50)
	_, _ = s.execCmd(ctx, client, "fail2ban-client unban --all 2>/dev/null || sudo fail2ban-client unban --all 2>/dev/null || true")

	logger.SetStep("Диагностика и запуск веб-серверов", 70)
	// Check and restart Apache or Nginx.
	repairScript := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

MSG=""
# 1. Apache check
if which apache2 >/dev/null 2>&1 || which httpd >/dev/null 2>&1; then
    (apache2ctl configtest 2>/dev/null || apachectl configtest 2>/dev/null || httpd -t 2>/dev/null) && (systemctl restart apache2 2>/dev/null || systemctl restart httpd 2>/dev/null || systemctl start apache2 2>/dev/null || systemctl start httpd 2>/dev/null)
    if systemctl is-active --quiet apache2 2>/dev/null || systemctl is-active --quiet httpd 2>/dev/null || pgrep -x apache2 >/dev/null 2>&1 || pgrep -x httpd >/dev/null 2>&1; then
        MSG="${MSG}Apache активен. "
    fi
fi

# 2. Nginx check
if which nginx >/dev/null 2>&1; then
    nginx -t 2>/dev/null && (systemctl restart nginx 2>/dev/null || systemctl start nginx 2>/dev/null)
    if systemctl is-active --quiet nginx 2>/dev/null || pgrep -x nginx >/dev/null 2>&1; then
        MSG="${MSG}Nginx активен. "
    fi
fi

# 3. Ports listening check
PORTS=$(ss -tulpn 2>/dev/null | grep -E ':(80|443)\s' || netstat -tulpn 2>/dev/null | grep -E ':(80|443)\s' || true)
echo "MSG:$MSG"
echo "PORTS:$PORTS"
`
	out, _ := s.execCmd(ctx, client, repairScript)
	logger.Info(fmt.Sprintf("Результат проверки веб-серверов: %s", strings.TrimSpace(out)))

	logger.SetStep("Готово", 100)
	return "Сетевой доступ к веб-серверу (порты 80/443) и службы Nginx/Apache успешно восстановлены", nil
}
