package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/i18n"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

var allowedRecordTypes = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, "MX": true,
	"TXT": true, "NS": true, "SRV": true, "CAA": true,
}

// ValidateRecord validates a single DNS resource record before writing a zone file.
func ValidateRecord(r DNSRecord) error {
	name := strings.TrimSpace(r.Name)
	if name == "" || strings.ContainsAny(name, " \t\r\n;\"'") {
		return i18n.Err("dns.invalidRecordName", i18n.Args{"name": r.Name})
	}
	typ := strings.ToUpper(strings.TrimSpace(r.Type))
	if !allowedRecordTypes[typ] {
		return i18n.Err("dns.invalidRecordType", i18n.Args{"type": r.Type})
	}
	val := strings.TrimSpace(r.Value)
	if val == "" || strings.ContainsAny(val, "\r\n") {
		return i18n.Err("dns.invalidRecordValue", i18n.Args{"name": name})
	}
	if typ != "TXT" && strings.ContainsRune(val, ';') {
		return i18n.Err("dns.semicolonInValue", i18n.Args{"type": typ})
	}
	switch typ {
	case "A":
		ip := net.ParseIP(val)
		if ip == nil || ip.To4() == nil {
			return i18n.Err("dns.invalidIPv4", i18n.Args{"val": val})
		}
	case "AAAA":
		ip := net.ParseIP(val)
		if ip == nil || ip.To4() != nil {
			return i18n.Err("dns.invalidIPv6", i18n.Args{"val": val})
		}
	case "MX":
		if r.Priority < 0 || r.Priority > 65535 {
			return i18n.Err("dns.invalidMXPriority", i18n.Args{"priority": r.Priority})
		}
	}
	return nil
}

// Service manages BIND9 (named) DNS zones and records over SSH.
type Service struct {
	module.Base
	engine *tasks.Engine
}

// NewService constructs a BIND9 DNS service and registers task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}
	engine.RegisterHandler("dns_install_bind9", s.handleInstallBIND9Task)
	engine.RegisterHandler("dns_create_zone", s.handleCreateZoneTask)
	engine.RegisterHandler("dns_save_zone", s.handleSaveZoneTask)
	engine.RegisterHandler("dns_delete_zone", s.handleDeleteZoneTask)
	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string, useSudo bool) (string, error) {
	return module.Exec(ctx, client, cmd, useSudo)
}

type bindPaths struct {
	serviceName string
	mainConf    string
	tamestack   string
	zonesDir    string
}

func (s *Service) detectBindPaths(ctx context.Context, client *ssh.SSHClient) bindPaths {
	out, _ := s.execCmd(ctx, client, "if [ -d /etc/bind ]; then echo debian; else echo rhel; fi", false)
	if strings.TrimSpace(out) == "debian" {
		return bindPaths{
			serviceName: "bind9",
			mainConf:    "/etc/bind/named.conf.local",
			tamestack:   "/etc/bind/named.conf.tamestack",
			zonesDir:    "/etc/bind/zones",
		}
	}
	return bindPaths{
		serviceName: "named",
		mainConf:    "/etc/named.conf",
		tamestack:   "/etc/named.tamestack.conf",
		zonesDir:    "/var/named/tamestack",
	}
}

// GetDNSStatus inspects BIND9/named status and lists managed DNS zones.
func (s *Service) GetDNSStatus(ctx context.Context, serverID string) (*DNSStatus, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	bp := s.detectBindPaths(ctx, client)

	script := fmt.Sprintf(`command -v named >/dev/null 2>&1 && echo "INSTALLED=1" || echo "INSTALLED=0"
systemctl is-active %s 2>/dev/null || systemctl is-active named 2>/dev/null || echo "inactive"
echo "===ZONES==="
grep -E '^\s*zone\s+"' %s 2>/dev/null | sed -E 's/.*"([^"]+)".*/\1/' || true
`, ssh.ShellEscape(bp.serviceName), ssh.ShellEscape(bp.tamestack))

	out, err := s.execCmd(ctx, client, script, true)
	if err != nil {
		return nil, err
	}

	res := &DNSStatus{ServiceName: bp.serviceName}
	inZones := false
	lines := strings.Split(out, "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "===ZONES===" {
			inZones = true
			continue
		}
		if !inZones {
			if line == "INSTALLED=1" {
				res.Installed = true
			} else if i == 1 && line == "active" {
				res.Active = true
			}
			continue
		}
		if line != "" {
			zPath := fmt.Sprintf("%s/db.%s", bp.zonesDir, line)
			res.Zones = append(res.Zones, DNSZoneSummary{
				Domain:   line,
				FilePath: zPath,
			})
		}
	}
	return res, nil
}

// InstallBIND9 submits a task to install and configure BIND9.
func (s *Service) InstallBIND9(ctx context.Context, serverID string) (*db.TaskRecord, error) {
	payload, _ := json.Marshal(map[string]string{"server_id": serverID})
	return s.engine.Submit(serverID, "dns_install_bind9", i18n.Tr("dns.taskInstallTitle"), string(payload))
}

// CreateZone submits a task to create a new BIND9 master zone.
func (s *Service) CreateZone(ctx context.Context, serverID, domain, serverIP string) (*db.TaskRecord, error) {
	d := strings.TrimSpace(strings.ToLower(domain))
	if err := ssh.ValidateHostToken(d); err != nil {
		return nil, i18n.Errf("dns.invalidDomain", nil, err)
	}
	ip := strings.TrimSpace(serverIP)
	if ip != "" && net.ParseIP(ip) == nil {
		return nil, i18n.Err("dns.invalidServerIP", i18n.Args{"ip": ip})
	}
	payload, _ := json.Marshal(createZoneRequest{ServerID: serverID, Domain: d, ServerIP: ip})
	return s.engine.Submit(serverID, "dns_create_zone", i18n.Tr("dns.taskCreateZoneTitle", i18n.Args{"domain": d}), string(payload))
}

// GetZone reads and parses a BIND9 zone file.
func (s *Service) GetZone(ctx context.Context, serverID, domain string) (*DNSZoneDetail, error) {
	d := strings.TrimSpace(strings.ToLower(domain))
	if err := ssh.ValidateHostToken(d); err != nil {
		return nil, err
	}
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	bp := s.detectBindPaths(ctx, client)
	zPath := fmt.Sprintf("%s/db.%s", bp.zonesDir, d)
	rawBytes, err := client.ReadFile(ctx, zPath)
	if err != nil {
		return nil, i18n.Errf("dns.readZoneError", i18n.Args{"path": zPath}, err)
	}
	raw := string(rawBytes)
	records := ParseZoneRecords(raw)
	return &DNSZoneDetail{
		Domain:     d,
		FilePath:   zPath,
		Records:    records,
		RawContent: raw,
	}, nil
}

// SaveZoneRecords submits a task to update DNS records in a BIND9 zone file with validation.
func (s *Service) SaveZoneRecords(ctx context.Context, serverID, domain string, records []DNSRecord) (*db.TaskRecord, error) {
	d := strings.TrimSpace(strings.ToLower(domain))
	if err := ssh.ValidateHostToken(d); err != nil {
		return nil, err
	}
	for _, r := range records {
		if err := ValidateRecord(r); err != nil {
			return nil, err
		}
	}
	payload, _ := json.Marshal(saveZoneRequest{ServerID: serverID, Domain: d, Records: records})
	return s.engine.Submit(serverID, "dns_save_zone", i18n.Tr("dns.taskSaveZoneTitle", i18n.Args{"domain": d}), string(payload))
}

// DeleteZone submits a task to remove a BIND9 DNS zone.
func (s *Service) DeleteZone(ctx context.Context, serverID, domain string) (*db.TaskRecord, error) {
	d := strings.TrimSpace(strings.ToLower(domain))
	if err := ssh.ValidateHostToken(d); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(deleteZoneRequest{ServerID: serverID, Domain: d})
	return s.engine.Submit(serverID, "dns_delete_zone", i18n.Tr("dns.taskDeleteZoneTitle", i18n.Args{"domain": d}), string(payload))
}

// ParseZoneRecords extracts resource records from a BIND zone file.
func ParseZoneRecords(raw string) []DNSRecord {
	var records []DNSRecord
	inSOA := false
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "$") {
			continue
		}
		if strings.Contains(line, "SOA") && strings.Contains(line, "(") {
			inSOA = true
			continue
		}
		if inSOA {
			if strings.Contains(line, ")") {
				inSOA = false
			}
			continue
		}
		if idx := strings.Index(line, ";"); idx != -1 && !strings.Contains(line, "\"") {
			line = strings.TrimSpace(line[:idx])
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		i := 1
		ttl := 3600
		if v, err := strconv.Atoi(fields[i]); err == nil {
			ttl = v
			i++
		}
		if i < len(fields) && strings.EqualFold(fields[i], "IN") {
			i++
		}
		if i+1 >= len(fields) {
			continue
		}
		recType := strings.ToUpper(fields[i])
		i++
		if !allowedRecordTypes[recType] {
			continue
		}
		prio := 0
		if recType == "MX" && i+1 < len(fields) {
			if p, err := strconv.Atoi(fields[i]); err == nil {
				prio = p
				i++
			}
		}
		val := strings.Join(fields[i:], " ")
		if recType == "TXT" {
			val = strings.Trim(val, "\"")
		}
		records = append(records, DNSRecord{
			Name:     name,
			TTL:      ttl,
			Type:     recType,
			Priority: prio,
			Value:    val,
		})
	}
	return records
}

// RenderZoneFile builds a valid BIND9 zone file with SOA, NS, and resource records.
func RenderZoneFile(domain string, serial int64, records []DNSRecord) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("; Managed by TameStack for %s\n", domain))
	b.WriteString(fmt.Sprintf("$TTL 3600\n"))
	b.WriteString(fmt.Sprintf("@\tIN\tSOA\tns1.%s. hostmaster.%s. (\n", domain, domain))
	b.WriteString(fmt.Sprintf("\t\t%d\t; Serial\n", serial))
	b.WriteString("\t\t3600\t\t; Refresh\n")
	b.WriteString("\t\t900\t\t; Retry\n")
	b.WriteString("\t\t604800\t\t; Expire\n")
	b.WriteString("\t\t300 )\t\t; Negative Cache TTL\n\n")

	for _, r := range records {
		ttl := r.TTL
		if ttl <= 0 {
			ttl = 3600
		}
		typ := strings.ToUpper(strings.TrimSpace(r.Type))
		val := strings.TrimSpace(r.Value)
		if typ == "TXT" && !strings.HasPrefix(val, "\"") {
			val = fmt.Sprintf("\"%s\"", strings.ReplaceAll(val, "\"", "\\\""))
		}
		if typ == "MX" {
			b.WriteString(fmt.Sprintf("%s\t%d\tIN\tMX\t%d\t%s\n", r.Name, ttl, r.Priority, val))
		} else {
			b.WriteString(fmt.Sprintf("%s\t%d\tIN\t%s\t%s\n", r.Name, ttl, typ, val))
		}
	}
	return b.String()
}

func (s *Service) handleInstallBIND9Task(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req struct {
		ServerID string `json:"server_id"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Установка пакетов BIND9", 30)
	installScript := `if command -v apt-get >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq bind9 bind9utils
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q bind bind-utils
else
    echo "Unsupported package manager" >&2; exit 1
fi`
	if out, err := s.execCmd(ctx, client, installScript, true); err != nil {
		return "", fmt.Errorf("ошибка установки BIND9: %w\n%s", err, out)
	}

	bp := s.detectBindPaths(ctx, client)
	logger.SetStep("Настройка named.conf и каталога зон", 70)
	setupScript := fmt.Sprintf(`mkdir -p %s
touch %s
if ! grep -qF %s %s 2>/dev/null; then
    printf '\ninclude "%s";\n' >> %s
fi
named-checkconf
systemctl enable %s 2>/dev/null || true
systemctl restart %s`,
		ssh.ShellEscape(bp.zonesDir),
		ssh.ShellEscape(bp.tamestack),
		bp.tamestack, bp.mainConf,
		bp.tamestack, bp.mainConf,
		ssh.ShellEscape(bp.serviceName), ssh.ShellEscape(bp.serviceName))
	if out, err := s.execCmd(ctx, client, setupScript, true); err != nil {
		return "", fmt.Errorf("ошибка настройки BIND9: %w\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return "DNS-сервер BIND9 установлен и запущен", nil
}

func (s *Service) handleCreateZoneTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req createZoneRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ssh.ValidateHostToken(req.Domain); err != nil {
		return "", err
	}
	client, record, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}
	ip := req.ServerIP
	if ip == "" && record != nil {
		ip = record.Host
	}
	if net.ParseIP(ip) == nil {
		ip = "127.0.0.1"
	}

	bp := s.detectBindPaths(ctx, client)
	zPath := fmt.Sprintf("%s/db.%s", bp.zonesDir, req.Domain)
	serial, _ := strconv.ParseInt(time.Now().Format("20060102")+"01", 10, 64)
	records := []DNSRecord{
		{Name: "@", TTL: 3600, Type: "NS", Value: fmt.Sprintf("ns1.%s.", req.Domain)},
		{Name: "@", TTL: 3600, Type: "A", Value: ip},
		{Name: "ns1", TTL: 3600, Type: "A", Value: ip},
		{Name: "www", TTL: 3600, Type: "A", Value: ip},
	}
	content := RenderZoneFile(req.Domain, serial, records)

	logger.SetStep("Запись файла зоны и проверка named-checkzone", 50)
	if err := client.EnsureDirectory(ctx, bp.zonesDir, 0755, "root:root"); err != nil {
		return "", err
	}
	if err := client.WriteFile(ctx, zPath, []byte(content), 0644); err != nil {
		return "", err
	}

	script := fmt.Sprintf(`named-checkzone %s %s
touch %s
if ! grep -q 'zone "%s"' %s; then
    cat >> %s <<'ZONE_EOF'
zone "%s" {
    type master;
    file "%s";
};
ZONE_EOF
fi
named-checkconf
systemctl reload %s 2>/dev/null || systemctl restart %s`,
		ssh.ShellEscape(req.Domain), ssh.ShellEscape(zPath),
		ssh.ShellEscape(bp.tamestack),
		req.Domain, ssh.ShellEscape(bp.tamestack),
		ssh.ShellEscape(bp.tamestack),
		req.Domain, zPath,
		ssh.ShellEscape(bp.serviceName), ssh.ShellEscape(bp.serviceName))

	if out, err := s.execCmd(ctx, client, script, true); err != nil {
		return "", fmt.Errorf("ошибка создания DNS-зоны %s: %w\n%s", req.Domain, err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("DNS-зона %s создана", req.Domain), nil
}

func (s *Service) handleSaveZoneTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req saveZoneRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ssh.ValidateHostToken(req.Domain); err != nil {
		return "", err
	}
	for _, r := range req.Records {
		if err := ValidateRecord(r); err != nil {
			return "", err
		}
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}
	bp := s.detectBindPaths(ctx, client)
	zPath := fmt.Sprintf("%s/db.%s", bp.zonesDir, req.Domain)
	bakPath := zPath + ".bak"
	_, _ = s.execCmd(ctx, client, fmt.Sprintf("[ -f %s ] && cp %s %s || true", ssh.ShellEscape(zPath), ssh.ShellEscape(zPath), ssh.ShellEscape(bakPath)), true)

	serial, _ := strconv.ParseInt(time.Now().Format("20060102")+"01", 10, 64)
	content := RenderZoneFile(req.Domain, serial, req.Records)
	if err := client.WriteFile(ctx, zPath, []byte(content), 0644); err != nil {
		return "", err
	}

	logger.SetStep("Проверка named-checkzone и перезагрузка BIND9", 70)
	checkCmd := fmt.Sprintf(`if ! named-checkzone %s %s; then
    [ -f %s ] && mv %s %s
    exit 1
fi
rm -f %s
systemctl reload %s 2>/dev/null || systemctl restart %s`,
		ssh.ShellEscape(req.Domain), ssh.ShellEscape(zPath),
		ssh.ShellEscape(bakPath), ssh.ShellEscape(bakPath), ssh.ShellEscape(zPath),
		ssh.ShellEscape(bakPath),
		ssh.ShellEscape(bp.serviceName), ssh.ShellEscape(bp.serviceName))

	if out, err := s.execCmd(ctx, client, checkCmd, true); err != nil {
		return "", fmt.Errorf("ошибка проверки зоны (изменения откатаны): %w\n%s", err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Записи DNS-зоны %s обновлены", req.Domain), nil
}

func (s *Service) handleDeleteZoneTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req deleteZoneRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ssh.ValidateHostToken(req.Domain); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}
	bp := s.detectBindPaths(ctx, client)
	zPath := fmt.Sprintf("%s/db.%s", bp.zonesDir, req.Domain)

	logger.SetStep("Удаление зоны и перезагрузка BIND9", 60)
	script := fmt.Sprintf(`if [ -f %s ]; then
    awk -v dom="%s" 'BEGIN{skip=0} $0 ~ "zone \"" dom "\"" {skip=1; next} skip && /\};/ {skip=0; next} !skip {print}' %s > %s.tmp && mv %s.tmp %s
fi
rm -f %s
named-checkconf
systemctl reload %s 2>/dev/null || systemctl restart %s`,
		ssh.ShellEscape(bp.tamestack), req.Domain,
		ssh.ShellEscape(bp.tamestack), ssh.ShellEscape(bp.tamestack), ssh.ShellEscape(bp.tamestack), ssh.ShellEscape(bp.tamestack),
		ssh.ShellEscape(zPath),
		ssh.ShellEscape(bp.serviceName), ssh.ShellEscape(bp.serviceName))
	if out, err := s.execCmd(ctx, client, script, true); err != nil {
		return "", fmt.Errorf("ошибка удаления DNS-зоны %s: %w\n%s", req.Domain, err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("DNS-зона %s удалена", req.Domain), nil
}
