package ssl

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/osadapter"
	"tamestack/internal/tasks"
)

// CertbotStatus describes whether certbot is present and how it is kept up to date.
type CertbotStatus struct {
	Installed        bool   `json:"installed"`
	Version          string `json:"version"`
	Path             string `json:"path"`
	RenewTimerActive bool   `json:"renew_timer_active"`
	WebServer        string `json:"webserver"` // "nginx", "apache", "none"
}

// CertbotInstallPlan previews a certbot installation without changing anything.
type CertbotInstallPlan struct {
	OSFamily          string   `json:"os_family"`
	OSDistro          string   `json:"os_distro"`
	OSCompatible      bool     `json:"os_compatible"`
	PackagesToInstall []string `json:"packages_to_install"`
	ServiceToStart    string   `json:"service_to_start"`
	WebServer         string   `json:"webserver"`
	AlreadyInstalled  bool     `json:"already_installed"`
	Message           string   `json:"message"`
}

// CertbotInstallPayload is the task payload for ssl_install_certbot.
type CertbotInstallPayload struct {
	ServerID string `json:"server_id"`
}

// CertbotConfig represents global settings in /etc/letsencrypt/cli.ini and auto-renewal timer.
type CertbotConfig struct {
	Email         string `json:"email"`
	KeyType       string `json:"key_type"`       // "ecdsa" or "rsa"
	RSAKeySize    int    `json:"rsa_key_size"`   // 2048, 4096
	EllipticCurve string `json:"elliptic_curve"` // "secp256r1", "secp384r1"
	DeployHook    string `json:"deploy_hook"`    // reload web server command
	AutoRenew     bool   `json:"auto_renew"`     // systemd timer or cron active
	AgreeTOS      bool   `json:"agree_tos"`
	RawCLIIni     string `json:"raw_cli_ini"`
}

// CertbotRenewResult contains the execution result of certbot renew (--dry-run).
type CertbotRenewResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

// CertbotCertificateItem represents an individual certificate managed by Certbot.
type CertbotCertificateItem struct {
	Name       string   `json:"name"`
	KeyType    string   `json:"key_type"`
	Domains    []string `json:"domains"`
	ExpiryDate string   `json:"expiry_date"`
	DaysLeft   int      `json:"days_left"`
	IsExpired  bool     `json:"is_expired"`
	CertPath   string   `json:"cert_path"`
	KeyPath    string   `json:"key_path"`
}

const certbotDetectScript = `export PATH=$PATH:/snap/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
CB=""
for b in certbot /snap/bin/certbot /usr/bin/certbot /usr/bin/letsencrypt /usr/local/bin/certbot /opt/certbot/bin/certbot; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then CB="$b"; break; fi
done
if [ -n "$CB" ]; then
    echo "INSTALLED:yes"
    echo "PATH:$CB"
    echo "VERSION:$("$CB" --version 2>&1 | head -n1)"
else
    echo "INSTALLED:no"
fi
RENEW="no"
if systemctl is-active --quiet certbot.timer 2>/dev/null || systemctl is-active --quiet certbot-renew.timer 2>/dev/null; then
    RENEW="yes"
elif [ -f /etc/cron.d/certbot ] || [ -f /etc/cron.d/certbot-renew ]; then
    RENEW="yes"
fi
echo "RENEW:$RENEW"
WS="none"
if systemctl is-active --quiet nginx 2>/dev/null || pgrep -x nginx >/dev/null 2>&1; then
    WS="nginx"
elif systemctl is-active --quiet apache2 2>/dev/null || systemctl is-active --quiet httpd 2>/dev/null || pgrep -f 'apache2|httpd' >/dev/null 2>&1; then
    WS="apache"
fi
echo "WEBSERVER:$WS"
`

func parseCertbotDetect(out string) *CertbotStatus {
	status := &CertbotStatus{WebServer: "none"}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "INSTALLED:"):
			status.Installed = strings.TrimPrefix(line, "INSTALLED:") == "yes"
		case strings.HasPrefix(line, "PATH:"):
			status.Path = strings.TrimSpace(strings.TrimPrefix(line, "PATH:"))
		case strings.HasPrefix(line, "VERSION:"):
			status.Version = strings.TrimSpace(strings.TrimPrefix(line, "VERSION:"))
		case strings.HasPrefix(line, "RENEW:"):
			status.RenewTimerActive = strings.TrimPrefix(line, "RENEW:") == "yes"
		case strings.HasPrefix(line, "WEBSERVER:"):
			ws := strings.TrimSpace(strings.TrimPrefix(line, "WEBSERVER:"))
			if ws == "nginx" || ws == "apache" {
				status.WebServer = ws
			}
		}
	}
	return status
}

func certbotPluginPackage(webServer string) string {
	if webServer == "apache" {
		return "python3-certbot-apache"
	}
	return "python3-certbot-nginx"
}

// GetCertbotStatus detects certbot presence, version, auto-renew timer and the active web server.
func (s *Service) GetCertbotStatus(ctx context.Context, serverID string) (*CertbotStatus, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out, _, _, _ := client.NewCommand(certbotDetectScript).Exec(ctx)
	return parseCertbotDetect(string(out)), nil
}

// PlanCertbotInstall detects the remote OS and previews the certbot installation.
func (s *Service) PlanCertbotInstall(ctx context.Context, serverID string) (*CertbotInstallPlan, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	_, osInfo, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("ошибка определения ОС: %w", err)
	}

	plan := &CertbotInstallPlan{
		OSFamily:       osInfo.Family,
		OSDistro:       osInfo.PrettyName,
		OSCompatible:   true,
		ServiceToStart: "certbot.timer",
	}

	if osInfo.Family != osadapter.FamilyDebian && osInfo.Family != osadapter.FamilyRhel {
		plan.OSCompatible = false
		plan.Message = fmt.Sprintf("Неподдерживаемая ОС: %s. Установка доступна для Debian/Ubuntu и RHEL-совместимых дистрибутивов.", osInfo.PrettyName)
		return plan, nil
	}

	out, _, _, _ := client.NewCommand(certbotDetectScript).Exec(ctx)
	status := parseCertbotDetect(string(out))
	plan.WebServer = status.WebServer

	if status.Installed {
		plan.AlreadyInstalled = true
		plan.Message = fmt.Sprintf("Certbot уже установлен (%s).", status.Version)
		return plan, nil
	}

	plugin := certbotPluginPackage(status.WebServer)
	if osInfo.Family == osadapter.FamilyRhel {
		plan.PackagesToInstall = []string{"epel-release", "certbot", plugin}
	} else {
		plan.PackagesToInstall = []string{"certbot", plugin}
	}
	plan.Message = fmt.Sprintf("Готов к установке: certbot + %s на %s", plugin, osInfo.PrettyName)
	return plan, nil
}

// InstallCertbot queues an asynchronous certbot installation task.
func (s *Service) InstallCertbot(serverID string) (*db.TaskRecord, error) {
	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("сервер %s не найден: %w", serverID, err)
	}
	payload, err := tasks.MarshalPayload(CertbotInstallPayload{ServerID: record.ID})
	if err != nil {
		return nil, err
	}
	title := fmt.Sprintf("Установка Certbot на %s (%s)", record.Name, record.Host)
	return s.engine.Submit(record.ID, "ssl_install_certbot", title, payload)
}

func (s *Service) handleInstallCertbotTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[CertbotInstallPayload](task.Payload)
	if err != nil {
		return "", err
	}

	logger.SetStep("Подключение к серверу по SSH", 10)
	client, _, err := s.GetSSHClient(ctx, p.ServerID)
	if err != nil {
		return "", err
	}
	adapter, osInfo, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return "", fmt.Errorf("ошибка определения ОС: %w", err)
	}
	if osInfo.Family != osadapter.FamilyDebian && osInfo.Family != osadapter.FamilyRhel {
		return "", fmt.Errorf("неподдерживаемая ОС: %s", osInfo.PrettyName)
	}
	logger.Info(fmt.Sprintf("ОС: %s (family: %s)", osInfo.PrettyName, osInfo.Family))

	outDetect, _, _, _ := client.NewCommand(certbotDetectScript).Exec(ctx)
	status := parseCertbotDetect(string(outDetect))
	plugin := certbotPluginPackage(status.WebServer)
	logger.Info(fmt.Sprintf("Обнаружен веб-сервер: %s, плагин certbot: %s", status.WebServer, plugin))

	logger.SetStep("Обновление кэша репозиториев", 25)
	if out, err := adapter.UpdatePackageIndex(ctx, client); err != nil {
		logger.Warn(fmt.Sprintf("Не удалось обновить кэш пакетов: %v", err))
	} else {
		logger.Info(strings.TrimSpace(out))
	}

	if osInfo.Family == osadapter.FamilyRhel {
		logger.SetStep("Подключение репозитория EPEL", 35)
		if _, err := adapter.InstallPackages(ctx, client, []string{"epel-release"}); err != nil {
			logger.Warn(fmt.Sprintf("Не удалось установить epel-release: %v", err))
		}
	}

	logger.SetStep(fmt.Sprintf("Установка пакетов: certbot %s", plugin), 55)
	out, err := adapter.InstallPackages(ctx, client, []string{"certbot", plugin})
	if err != nil {
		return "", fmt.Errorf("ошибка установки certbot: %w\nВывод:\n%s", err, out)
	}
	logger.Info(strings.TrimSpace(out))

	logger.SetStep("Включение автообновления (certbot.timer)", 80)
	enableCmd := "systemctl enable --now certbot.timer 2>/dev/null || systemctl enable --now certbot-renew.timer 2>/dev/null || true"
	_, _, _, _ = client.NewCommand(enableCmd).WithSudo().Exec(ctx)

	outVerify, _, _, _ := client.NewCommand(certbotDetectScript).Exec(ctx)
	verify := parseCertbotDetect(string(outVerify))
	if !verify.Installed {
		return "", fmt.Errorf("certbot не обнаружен после установки пакетов")
	}
	logger.Info(fmt.Sprintf("Certbot установлен: %s", verify.Version))
	if verify.RenewTimerActive {
		logger.Info("Автообновление сертификатов активно (systemd timer / cron).")
	} else {
		logger.Warn("Таймер автообновления не активен. Возможно, потребуется настроить cron для 'certbot renew' вручную.")
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Certbot установлен (%s), плагин %s", verify.Version, plugin), nil
}

// GetCertbotConfig reads /etc/letsencrypt/cli.ini and inspects the auto-renewal timer.
func (s *Service) GetCertbotConfig(ctx context.Context, serverID string) (*CertbotConfig, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	readCmd := "cat /etc/letsencrypt/cli.ini 2>/dev/null || true"
	out, _, _, _ := client.NewCommand(readCmd).WithSudo().Exec(ctx)

	cfg := parseCLIIni(string(out))

	timerCmd := "if systemctl is-active --quiet certbot.timer 2>/dev/null || systemctl is-active --quiet certbot-renew.timer 2>/dev/null || [ -f /etc/cron.d/certbot ] || [ -f /etc/cron.d/certbot-renew ]; then echo 'ACTIVE:yes'; else echo 'ACTIVE:no'; fi"
	timerOut, _, _, _ := client.NewCommand(timerCmd).Exec(ctx)
	cfg.AutoRenew = strings.Contains(string(timerOut), "ACTIVE:yes")

	return cfg, nil
}

// SaveCertbotConfig writes /etc/letsencrypt/cli.ini and enables/disables the renewal timer.
func (s *Service) SaveCertbotConfig(ctx context.Context, serverID string, cfg CertbotConfig) error {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	iniContent := cfg.RawCLIIni
	if strings.TrimSpace(iniContent) == "" {
		iniContent = renderCLIIni(cfg)
	}

	script := fmt.Sprintf(`mkdir -p /etc/letsencrypt
cat << 'EOF' > /etc/letsencrypt/cli.ini
%s
EOF
chmod 644 /etc/letsencrypt/cli.ini
`, iniContent)

	_, stderr, code, err := client.NewCommand(script).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("ошибка сохранения /etc/letsencrypt/cli.ini: %s", string(stderr))
	}

	var timerCmd string
	if cfg.AutoRenew {
		timerCmd = "systemctl unmask certbot.timer certbot-renew.timer 2>/dev/null; systemctl enable --now certbot.timer 2>/dev/null || systemctl enable --now certbot-renew.timer 2>/dev/null || true"
	} else {
		timerCmd = "systemctl stop certbot.timer certbot-renew.timer 2>/dev/null; systemctl disable certbot.timer certbot-renew.timer 2>/dev/null || true"
	}
	_, _, _, _ = client.NewCommand(timerCmd).WithSudo().Exec(ctx)

	return nil
}

// RunCertbotRenew executes certbot renew (optionally with --dry-run) and returns the output.
func (s *Service) RunCertbotRenew(ctx context.Context, serverID string, dryRun bool) (*CertbotRenewResult, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	var cmd string
	if dryRun {
		cmd = "certbot renew --dry-run 2>&1"
	} else {
		cmd = "certbot renew 2>&1"
	}

	stdout, _, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	outStr := string(stdout)

	return &CertbotRenewResult{
		Success: (err == nil && code == 0),
		Output:  outStr,
	}, nil
}

// ListCertbotCertificates executes certbot certificates and parses the output.
func (s *Service) ListCertbotCertificates(ctx context.Context, serverID string) ([]CertbotCertificateItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	out, _, _, err := client.NewCommand("certbot certificates 2>&1").WithSudo().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка вызова certbot certificates: %w", err)
	}

	return parseCertbotCertificates(string(out)), nil
}

// DeleteCertbotCertificate removes a certificate by name from certbot's management.
func (s *Service) DeleteCertbotCertificate(ctx context.Context, serverID, certName string) error {
	certName = strings.TrimSpace(certName)
	if certName == "" {
		return fmt.Errorf("certificate name is required")
	}
	if strings.ContainsAny(certName, ";|&$`\"'<> \t\r\n") {
		return fmt.Errorf("invalid certificate name: %s", certName)
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("certbot delete --cert-name %s --non-interactive", certName)
	stdout, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("ошибка удаления сертификата %s (code %d): %s %s", certName, code, string(stdout), string(stderr))
	}
	return nil
}

func parseCLIIni(content string) *CertbotConfig {
	cfg := &CertbotConfig{
		KeyType:       "ecdsa",
		RSAKeySize:    4096,
		EllipticCurve: "secp256r1",
		AgreeTOS:      true,
		DeployHook:    "systemctl reload nginx 2>/dev/null || systemctl reload apache2 2>/dev/null || true",
		RawCLIIni:     content,
	}

	lines := strings.Split(content, "\n")
	hasCustomSettings := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		hasCustomSettings = true

		switch key {
		case "email":
			cfg.Email = val
		case "key-type":
			cfg.KeyType = strings.ToLower(val)
		case "rsa-key-size":
			var sz int
			if _, err := fmt.Sscanf(val, "%d", &sz); err == nil && sz > 0 {
				cfg.RSAKeySize = sz
			}
		case "elliptic-curve":
			cfg.EllipticCurve = strings.ToLower(val)
		case "deploy-hook":
			cfg.DeployHook = val
		case "agree-tos":
			cfg.AgreeTOS = strings.ToLower(val) == "true" || val == "1"
		}
	}

	if !hasCustomSettings {
		cfg.RawCLIIni = renderCLIIni(*cfg)
	}

	return cfg
}

func renderCLIIni(cfg CertbotConfig) string {
	var sb strings.Builder
	sb.WriteString("# Managed by TameStack\n")
	if cfg.Email != "" {
		sb.WriteString(fmt.Sprintf("email = %s\n", cfg.Email))
	}
	if cfg.KeyType != "" {
		sb.WriteString(fmt.Sprintf("key-type = %s\n", cfg.KeyType))
	}
	if cfg.KeyType == "rsa" && cfg.RSAKeySize > 0 {
		sb.WriteString(fmt.Sprintf("rsa-key-size = %d\n", cfg.RSAKeySize))
	} else if cfg.KeyType == "ecdsa" && cfg.EllipticCurve != "" {
		sb.WriteString(fmt.Sprintf("elliptic-curve = %s\n", cfg.EllipticCurve))
	}
	if cfg.AgreeTOS {
		sb.WriteString("agree-tos = true\n")
	}
	if cfg.DeployHook != "" {
		sb.WriteString(fmt.Sprintf("deploy-hook = %s\n", cfg.DeployHook))
	}
	return sb.String()
}

func parseCertbotCertificates(out string) []CertbotCertificateItem {
	var certs []CertbotCertificateItem
	var current *CertbotCertificateItem

	lines := strings.Split(out, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "Certificate Name:") {
			if current != nil && current.Name != "" {
				certs = append(certs, *current)
			}
			current = &CertbotCertificateItem{
				Name: strings.TrimSpace(strings.TrimPrefix(line, "Certificate Name:")),
			}
			continue
		}

		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "Domains:"):
			domainsStr := strings.TrimSpace(strings.TrimPrefix(line, "Domains:"))
			current.Domains = strings.Fields(domainsStr)
		case strings.HasPrefix(line, "Key Type:"):
			current.KeyType = strings.TrimSpace(strings.TrimPrefix(line, "Key Type:"))
		case strings.HasPrefix(line, "Expiry Date:"):
			expStr := strings.TrimSpace(strings.TrimPrefix(line, "Expiry Date:"))
			current.ExpiryDate = expStr
			if strings.Contains(expStr, "(VALID:") {
				var days int
				idx := strings.Index(expStr, "(VALID:")
				sub := expStr[idx+len("(VALID:"): ]
				fmt.Sscanf(sub, "%d", &days)
				current.DaysLeft = days
				current.IsExpired = false
			} else if strings.Contains(expStr, "EXPIRED") {
				current.DaysLeft = 0
				current.IsExpired = true
			}
		case strings.HasPrefix(line, "Certificate Path:"):
			current.CertPath = strings.TrimSpace(strings.TrimPrefix(line, "Certificate Path:"))
		case strings.HasPrefix(line, "Private Key Path:"):
			current.KeyPath = strings.TrimSpace(strings.TrimPrefix(line, "Private Key Path:"))
		}
	}

	if current != nil && current.Name != "" {
		certs = append(certs, *current)
	}

	return certs
}