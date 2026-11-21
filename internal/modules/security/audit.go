package security

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/ssh"
)

// --- Security Audit ---

// RunSecurityAudit performs a deep security check of the server.
func (s *Service) RunSecurityAudit(ctx context.Context, serverID string) (*SecurityAuditReport, error) {
	client, record, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	report := &SecurityAuditReport{
		ServerID:       serverID,
		Score:          100,
		SSHPort:        record.Port,
		ListeningPorts: []ListeningPort{},
		Checks:         []SecurityCheckItem{},
		LastAuditedAt:  time.Now().Format("2006-01-02 15:04:05"),
	}

	sshOut, _ := s.execCmd(ctx, client, "sshd -T 2>/dev/null || cat /etc/ssh/sshd_config")
	sshText := strings.ToLower(sshOut)

	rootLogin := "yes"
	passAuth := "yes"

	scanner := bufio.NewScanner(strings.NewReader(sshText))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "permitrootlogin") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				rootLogin = parts[1]
			}
		} else if strings.HasPrefix(line, "passwordauthentication") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				passAuth = parts[1]
			}
		} else if strings.HasPrefix(line, "port ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				p, _ := strconv.Atoi(parts[1])
				if p > 0 {
					report.SSHPort = p
				}
			}
		}
	}
	report.SSHRuleRootLogin = rootLogin
	report.SSHRulePasswordAuth = passAuth

	// Check 1: root login.
	if rootLogin == "yes" {
		report.Score -= 20
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "ssh_root_login",
			Title:          "Вход по SSH под пользователем root разрешен",
			Category:       "ssh",
			Status:         "danger",
			Details:        "Параметр PermitRootLogin установлен в 'yes'. Прямой вход под учетной записью root создает высокий вектор угрозы.",
			Recommendation: "Отключите прямой вход root или разрешите вход только по SSH-ключам (prohibit-password).",
			CanAutoFix:     true,
		})
	} else {
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "ssh_root_login",
			Title:          "Прямой root-доступ по SSH ограничен",
			Category:       "ssh",
			Status:         "pass",
			Details:        fmt.Sprintf("PermitRootLogin установлен в '%s'.", rootLogin),
			Recommendation: "Настройка безопасна.",
			CanAutoFix:     false,
		})
	}

	// Check 2: password authentication.
	if passAuth == "yes" {
		report.Score -= 15
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "ssh_password_auth",
			Title:          "Вход по SSH с использованием пароля включен",
			Category:       "ssh",
			Status:         "warning",
			Details:        "Параметр PasswordAuthentication установлен в 'yes'. Сервер уязвим к атакам перебора паролей (Brute-force).",
			Recommendation: "Настройте аутентификацию по SSH-ключам и отключите PasswordAuthentication.",
			CanAutoFix:     true,
		})
	} else {
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "ssh_password_auth",
			Title:          "SSH аутентификация по паролям отключена",
			Category:       "ssh",
			Status:         "pass",
			Details:        "Разрешен вход исключительно по криптографическим SSH-ключам.",
			Recommendation: "Настройка безопасна.",
			CanAutoFix:     false,
		})
	}

	// Check 3: firewall.
	fwStatus, _ := s.GetFirewallStatus(ctx, serverID)
	if fwStatus == nil || !fwStatus.IsActive {
		report.Score -= 25
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "firewall_active",
			Title:          "Брандмауэр (Firewall) отключен",
			Category:       "firewall",
			Status:         "danger",
			Details:        "Служба UFW / Firewalld не активна. Все открытые сервисы доступны напрямую из Интернета.",
			Recommendation: "Включите брандмауэр и оставьте открытыми только необходимые порты (22, 80, 443).",
			CanAutoFix:     true,
		})
	} else {
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "firewall_active",
			Title:          fmt.Sprintf("Брандмауэр активен (%s)", fwStatus.Type),
			Category:       "firewall",
			Status:         "pass",
			Details:        fmt.Sprintf("Активно правил фильтрации: %d. Входящий трафик по умолчанию: %s.", len(fwStatus.Rules), fwStatus.DefaultIncoming),
			Recommendation: "Брандмауэр функционирует в штатном режиме.",
			CanAutoFix:     false,
		})
	}

	// Check 4: fail2ban.
	f2bStatus, _ := s.GetFail2banStatus(ctx, serverID)
	if f2bStatus == nil || !f2bStatus.IsActive {
		report.Score -= 15
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "fail2ban_active",
			Title:          "Fail2ban служба защиты от брутфорса не активна",
			Category:       "services",
			Status:         "warning",
			Details:        "Fail2ban не обнаруживает и не банит ботнет-сканеры, подбирающие учетные данные SSH и веб-сервера.",
			Recommendation: "Установите и запустите Fail2ban для автоматической блокировки вредоносных IP адресов.",
			CanAutoFix:     true,
		})
	} else {
		totalBanned := 0
		for _, j := range f2bStatus.Jails {
			totalBanned += j.CurrentlyBanned
		}
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "fail2ban_active",
			Title:          "Fail2ban активен и защищает порты",
			Category:       "services",
			Status:         "pass",
			Details:        fmt.Sprintf("Активных джейлов: %d. Заблокировано IP в данный момент: %d.", len(f2bStatus.Jails), totalBanned),
			Recommendation: "Система обнаружения вторжений работает корректно.",
			CanAutoFix:     false,
		})
	}

	ssOut, _ := s.execCmd(ctx, client, "ss -tulpn 2>/dev/null || netstat -tulpn 2>/dev/null")
	report.ListeningPorts = s.parseListeningPorts(ssOut)

	// Check for databases exposed to the external network (0.0.0.0 or ::).
	hasExposedDB := false
	var exposedDBList []string
	for _, lp := range report.ListeningPorts {
		if lp.IsPublic {
			if lp.Port == 3306 || lp.Port == 33060 {
				hasExposedDB = true
				exposedDBList = append(exposedDBList, fmt.Sprintf("MySQL (%d)", lp.Port))
			} else if lp.Port == 5432 {
				hasExposedDB = true
				exposedDBList = append(exposedDBList, fmt.Sprintf("PostgreSQL (%d)", lp.Port))
			} else if lp.Port == 6379 {
				hasExposedDB = true
				exposedDBList = append(exposedDBList, fmt.Sprintf("Redis (%d)", lp.Port))
			} else if lp.Port == 27017 {
				hasExposedDB = true
				exposedDBList = append(exposedDBList, fmt.Sprintf("MongoDB (%d)", lp.Port))
			}
		}
	}

	if hasExposedDB {
		report.Score -= 20
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "exposed_databases",
			Title:          "Базы данных выставлены в открытую сеть (0.0.0.0)",
			Category:       "network",
			Status:         "danger",
			Details:        fmt.Sprintf("Обнаружены службы СУБД, слушающие внешний интерфейс: %s.", strings.Join(exposedDBList, ", ")),
			Recommendation: "Привяжите СУБД к 127.0.0.1 (bind-address = 127.0.0.1) или ограничьте доступ через Firewall.",
			CanAutoFix:     false,
		})
	} else {
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "exposed_databases",
			Title:          "Базы данных изолированы от внешней сети",
			Category:       "network",
			Status:         "pass",
			Details:        "Все локальные СУБД слушают только локальный сокет или интерфейс 127.0.0.1.",
			Recommendation: "Конфигурация соответствует стандарту безопасности.",
			CanAutoFix:     false,
		})
	}

	pendingCount, secCount := s.checkSystemUpdates(ctx, client)
	report.PendingUpdatesCount = pendingCount
	report.SecurityUpdatesCount = secCount

	if secCount > 0 {
		report.Score -= 10
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "security_updates",
			Title:          fmt.Sprintf("Доступно критических обновлений безопасности: %d", secCount),
			Category:       "updates",
			Status:         "warning",
			Details:        fmt.Sprintf("Обнаружено %d пакетов с исправлениями уязвимостей безопасности (всего обновлений: %d).", secCount, pendingCount),
			Recommendation: "Установите обновления безопасности для закрытия известных CVE уязвимостей системы.",
			CanAutoFix:     true,
		})
	} else {
		report.Checks = append(report.Checks, SecurityCheckItem{
			Key:            "security_updates",
			Title:          "Критических обновлений безопасности не обнаружено",
			Category:       "updates",
			Status:         "pass",
			Details:        fmt.Sprintf("Все критические патчи установлены (всего ожидающих обновлений: %d).", pendingCount),
			Recommendation: "Система находится в актуальном состоянии.",
			CanAutoFix:     false,
		})
	}

	if report.Score < 0 {
		report.Score = 0
	}

	return report, nil
}

func (s *Service) parseListeningPorts(output string) []ListeningPort {
	var list []ListeningPort
	scanner := bufio.NewScanner(strings.NewReader(output))
	// ss line format:
	// tcp   LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=123,fd=3))
	// LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=123,fd=3))

	rePort := regexp.MustCompile(`(?:(tcp|udp)\s+)?(?:LISTEN|UNCONN)?\s*\S*\s*\S*\s*([0-9a-fA-F\.\:\[\]\*]+):(\d+)\s+.*users:\(\("([^"]+)",pid=(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := rePort.FindStringSubmatch(line)
		if len(matches) > 0 {
			proto := matches[1]
			if proto == "" {
				proto = "tcp"
			}
			addr := matches[2]
			portNum, _ := strconv.Atoi(matches[3])
			procName := matches[4]
			pid, _ := strconv.Atoi(matches[5])

			isPublic := (addr == "0.0.0.0" || addr == "*" || addr == "::" || addr == "[::]")

			list = append(list, ListeningPort{
				Protocol:     proto,
				LocalAddress: addr,
				Port:         portNum,
				Process:      procName,
				PID:          pid,
				IsPublic:     isPublic,
			})
		}
	}

	return list
}

func (s *Service) checkSystemUpdates(ctx context.Context, client *ssh.SSHClient) (total int, security int) {
	// Debian / Ubuntu.
	checkApt, _ := s.execCmd(ctx, client, "which apt-get")
	if strings.TrimSpace(checkApt) != "" {
		out, err := s.execCmd(ctx, client, "apt-get -s upgrade 2>/dev/null")
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(out))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "upgraded,") {
					parts := strings.Split(line, ",")
					if len(parts) > 0 {
						numStr := strings.Fields(parts[0])
						if len(numStr) > 0 {
							total, _ = strconv.Atoi(numStr[0])
						}
					}
				}
				if strings.Contains(strings.ToLower(line), "security") {
					security++
				}
			}
		}
		return total, security
	}

	// RHEL / AlmaLinux.
	checkDnf, _ := s.execCmd(ctx, client, "which dnf || which yum")
	if strings.TrimSpace(checkDnf) != "" {
		secOut, _ := s.execCmd(ctx, client, "dnf check-update --security -q 2>/dev/null | grep -v '^$' | wc -l")
		security, _ = strconv.Atoi(strings.TrimSpace(secOut))

		totOut, _ := s.execCmd(ctx, client, "dnf check-update -q 2>/dev/null | grep -v '^$' | wc -l")
		total, _ = strconv.Atoi(strings.TrimSpace(totOut))
		return total, security
	}

	return 0, 0
}
