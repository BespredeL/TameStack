package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/osadapter"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// --- Fail2ban ---

func formatSecondsToDuration(secStr string) string {
	s := strings.TrimSpace(secStr)
	if s == "" {
		return ""
	}
	sec, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	if sec <= 0 {
		return s
	}
	if sec%86400 == 0 {
		return fmt.Sprintf("%dd", sec/86400)
	}
	if sec%3600 == 0 {
		return fmt.Sprintf("%dh", sec/3600)
	}
	if sec%60 == 0 {
		return fmt.Sprintf("%dm", sec/60)
	}
	return fmt.Sprintf("%ds", sec)
}

type parsedJailConfig struct {
	Name       string
	Enabled    bool
	Port       string
	Filter     string
	LogPath    string
	MaxRetry   int
	FindTime   string
	BanTime    string
	Action     string
	IgnoreIP   string
	ConfigFile string
}

func parseFail2banConfigFiles(raw string) map[string]*parsedJailConfig {
	res := make(map[string]*parsedJailConfig)
	var currentFile string
	var currentCfg *parsedJailConfig

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "---FILE:") && strings.HasSuffix(line, "---") {
			currentFile = strings.TrimSuffix(strings.TrimPrefix(line, "---FILE:"), "---")
			currentCfg = nil
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.Trim(line, "[] \t")
			if strings.EqualFold(section, "DEFAULT") || strings.EqualFold(section, "INCLUDES") {
				currentCfg = nil
				continue
			}
			cfg, ok := res[section]
			if !ok {
				cfg = &parsedJailConfig{
					Name:       section,
					Enabled:    false,
					ConfigFile: currentFile,
				}
				res[section] = cfg
			} else {
				cfg.ConfigFile = currentFile
			}
			currentCfg = cfg
			continue
		}

		if currentCfg == nil {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch strings.ToLower(key) {
		case "enabled":
			currentCfg.Enabled = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		case "port":
			currentCfg.Port = val
		case "filter":
			currentCfg.Filter = val
		case "logpath":
			currentCfg.LogPath = val
		case "maxretry":
			if n, err := strconv.Atoi(val); err == nil {
				currentCfg.MaxRetry = n
			}
		case "findtime":
			currentCfg.FindTime = val
		case "bantime":
			currentCfg.BanTime = val
		case "action", "banaction":
			currentCfg.Action = val
		case "ignoreip":
			currentCfg.IgnoreIP = val
		}
	}
	return res
}

// GetFail2banStatus reads the fail2ban state, active runtime jails, and configured jails.
func (s *Service) GetFail2banStatus(ctx context.Context, serverID string) (*Fail2banStatus, error) {
	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	status := &Fail2banStatus{
		IsInstalled:      false,
		IsActive:         false,
		Version:          "",
		Jails:            []Fail2banJail{},
		AvailableFilters: []string{},
	}

	// Shell script to detect binary, active status, available filters, configured files, and runtime jails in one pass.
	inspectScript := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
F2B=""
for b in fail2ban-client /usr/bin/fail2ban-client /usr/local/bin/fail2ban-client; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then
        F2B="$b"
        break
    fi
done

INSTALLED="no"
if [ -n "$F2B" ]; then
    INSTALLED="yes"
fi

ACTIVE="no"
if systemctl is-active --quiet fail2ban 2>/dev/null; then
    ACTIVE="yes"
elif pgrep -f fail2ban-server >/dev/null 2>&1 || pidof fail2ban-server >/dev/null 2>&1; then
    ACTIVE="yes"
elif [ -n "$F2B" ]; then
    if $F2B ping 2>/dev/null | grep -qi "pong"; then
        ACTIVE="yes"
    elif sudo $F2B ping 2>/dev/null | grep -qi "pong"; then
        ACTIVE="yes"
    fi
fi

CLIENT_CMD="$F2B"
if [ -n "$F2B" ]; then
    if ! $F2B ping >/dev/null 2>&1; then
        if which sudo >/dev/null 2>&1 && sudo $F2B ping >/dev/null 2>&1; then
            CLIENT_CMD="sudo $F2B"
        fi
    fi
fi

VER=""
if [ -n "$CLIENT_CMD" ]; then
    VER=$($CLIENT_CMD --version 2>/dev/null | head -n 1 || true)
fi

echo "INSTALLED:$INSTALLED"
echo "ACTIVE:$ACTIVE"
echo "CMD:$CLIENT_CMD"
echo "VERSION:$VER"

echo "===FILTERS_START==="
ls -1 /etc/fail2ban/filter.d/*.conf 2>/dev/null | sed -e 's|.*/||' -e 's|\.conf$||' | sort -u
echo "===FILTERS_END==="

echo "===CONFIGS_START==="
for f in /etc/fail2ban/jail.conf /etc/fail2ban/jail.local /etc/fail2ban/jail.d/*.conf /etc/fail2ban/jail.d/*.local; do
    if [ -f "$f" ]; then
        echo "---FILE:$f---"
        cat "$f"
    fi
done
echo "===CONFIGS_END==="

echo "===RUNTIME_START==="
if [ -n "$CLIENT_CMD" ] && [ "$ACTIVE" = "yes" ]; then
    for j in $( $CLIENT_CMD status 2>/dev/null | grep "Jail list:" | sed 's/.*Jail list://;s/,/ /g' ); do
        echo "---JAIL:$j---"
        $CLIENT_CMD status "$j" 2>/dev/null
        echo "MAXRETRY:$($CLIENT_CMD get "$j" maxretry 2>/dev/null || true)"
        echo "FINDTIME:$($CLIENT_CMD get "$j" findtime 2>/dev/null || true)"
        echo "BANTIME:$($CLIENT_CMD get "$j" bantime 2>/dev/null || true)"
        F_LIST=$( $CLIENT_CMD status "$j" 2>/dev/null | grep -E "File list:" | sed -e 's/.*File list:[[:space:]]*//' )
        if [ -z "$F_LIST" ]; then
            F_LIST=$( $CLIENT_CMD get "$j" logpath 2>/dev/null | grep "/" | sed -e 's/^[^/]*//' | tr '\n' ' ' )
        fi
        echo "LOGPATH:$F_LIST"
    done
fi
echo "===RUNTIME_END==="
`

	initOut, _ := s.execCmd(ctx, client, inspectScript)
	lines := strings.Split(initOut, "\n")
	for _, l := range lines {
		lTrim := strings.TrimSpace(l)
		if strings.HasPrefix(lTrim, "INSTALLED:") {
			status.IsInstalled = strings.TrimPrefix(lTrim, "INSTALLED:") == "yes"
		} else if strings.HasPrefix(lTrim, "ACTIVE:") {
			status.IsActive = strings.TrimPrefix(lTrim, "ACTIVE:") == "yes"
		} else if strings.HasPrefix(lTrim, "VERSION:") {
			status.Version = strings.TrimPrefix(lTrim, "VERSION:")
		}
	}

	if !status.IsInstalled && !status.IsActive {
		return status, nil
	}

	// 1. Parse Available Filters
	if idxStart := strings.Index(initOut, "===FILTERS_START==="); idxStart != -1 {
		if idxEnd := strings.Index(initOut, "===FILTERS_END==="); idxEnd > idxStart {
			filtersPart := initOut[idxStart+len("===FILTERS_START===") : idxEnd]
			fScanner := bufio.NewScanner(strings.NewReader(filtersPart))
			for fScanner.Scan() {
				fName := strings.TrimSpace(fScanner.Text())
				if fName != "" {
					status.AvailableFilters = append(status.AvailableFilters, fName)
				}
			}
		}
	}

	// 2. Parse Config Files
	configs := make(map[string]*parsedJailConfig)
	if idxStart := strings.Index(initOut, "===CONFIGS_START==="); idxStart != -1 {
		if idxEnd := strings.Index(initOut, "===CONFIGS_END==="); idxEnd > idxStart {
			configsPart := initOut[idxStart+len("===CONFIGS_START===") : idxEnd]
			configs = parseFail2banConfigFiles(configsPart)
		}
	}

	// 3. Parse Runtime Jails
	processedJails := make(map[string]bool)
	if idxStart := strings.Index(initOut, "===RUNTIME_START==="); idxStart != -1 {
		if idxEnd := strings.Index(initOut, "===RUNTIME_END==="); idxEnd > idxStart {
			runtimePart := initOut[idxStart+len("===RUNTIME_START===") : idxEnd]
			rScanner := bufio.NewScanner(strings.NewReader(runtimePart))
			var currentJail *Fail2banJail

			for rScanner.Scan() {
				rLine := strings.TrimSpace(rScanner.Text())
				if strings.HasPrefix(rLine, "---JAIL:") && strings.HasSuffix(rLine, "---") {
					jName := strings.TrimSuffix(strings.TrimPrefix(rLine, "---JAIL:"), "---")
					if currentJail != nil {
						status.Jails = append(status.Jails, *currentJail)
					}
					status.IsActive = true
					processedJails[jName] = true
					currentJail = &Fail2banJail{
						Name:         jName,
						Enabled:      true,
						BannedIPList: []string{},
					}
					continue
				}
				if currentJail == nil {
					continue
				}

				if strings.Contains(rLine, "Currently failed:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "|- Currently failed:"))
					currentJail.CurrentlyFailed, _ = strconv.Atoi(valStr)
				} else if strings.Contains(rLine, "Total failed:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "`- Total failed:"))
					currentJail.TotalFailed, _ = strconv.Atoi(valStr)
				} else if strings.Contains(rLine, "Currently banned:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "|- Currently banned:"))
					currentJail.CurrentlyBanned, _ = strconv.Atoi(valStr)
				} else if strings.Contains(rLine, "Total banned:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "|- Total banned:"))
					currentJail.TotalBanned, _ = strconv.Atoi(valStr)
				} else if strings.Contains(rLine, "Banned IP list:") {
					parts := strings.SplitN(rLine, "Banned IP list:", 2)
					if len(parts) == 2 {
						ips := strings.Fields(parts[1])
						currentJail.BannedIPList = append(currentJail.BannedIPList, ips...)
					}
				} else if strings.Contains(rLine, "File list:") {
					parts := strings.SplitN(rLine, "File list:", 2)
					if len(parts) == 2 {
						files := strings.TrimSpace(parts[1])
						if files != "" && currentJail.LogPath == "" {
							currentJail.LogPath = files
						}
					}
				} else if strings.HasPrefix(rLine, "MAXRETRY:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "MAXRETRY:"))
					if !strings.Contains(valStr, "Invalid") && !strings.Contains(valStr, "ERROR") {
						if n, err := strconv.Atoi(valStr); err == nil && n > 0 {
							currentJail.MaxRetry = n
						}
					}
				} else if strings.HasPrefix(rLine, "FINDTIME:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "FINDTIME:"))
					if !strings.Contains(valStr, "Invalid") && !strings.Contains(valStr, "ERROR") {
						currentJail.FindTime = formatSecondsToDuration(valStr)
					}
				} else if strings.HasPrefix(rLine, "BANTIME:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "BANTIME:"))
					if !strings.Contains(valStr, "Invalid") && !strings.Contains(valStr, "ERROR") {
						currentJail.BanTime = formatSecondsToDuration(valStr)
					}
				} else if strings.HasPrefix(rLine, "LOGPATH:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(rLine, "LOGPATH:"))
					if !strings.Contains(valStr, "Invalid") && !strings.Contains(valStr, "ERROR") && !strings.Contains(valStr, "Current monitored") {
						if valStr != "" {
							currentJail.LogPath = valStr
						}
					}
				}
			}
			if currentJail != nil {
				status.Jails = append(status.Jails, *currentJail)
			}
		}
	}

	// Merge config file attributes into runtime jails
	for i := range status.Jails {
		j := &status.Jails[i]
		if cfg, ok := configs[j.Name]; ok {
			if j.Port == "" || strings.Contains(j.Port, "Invalid") {
				j.Port = cfg.Port
			}
			if j.Filter == "" || strings.Contains(j.Filter, "Invalid") {
				j.Filter = cfg.Filter
			}
			if j.LogPath == "" || strings.Contains(j.LogPath, "Current monitored") || strings.Contains(j.LogPath, "Invalid") {
				j.LogPath = cfg.LogPath
			}
			if j.MaxRetry == 0 && cfg.MaxRetry > 0 {
				j.MaxRetry = cfg.MaxRetry
			}
			if j.FindTime == "" {
				j.FindTime = cfg.FindTime
			}
			if j.BanTime == "" {
				j.BanTime = cfg.BanTime
			}
			j.ConfigFile = cfg.ConfigFile
			j.IgnoreIP = cfg.IgnoreIP
			j.Action = cfg.Action
			j.IsManaged = strings.Contains(cfg.ConfigFile, "TameStack-") || strings.HasPrefix(cfg.ConfigFile, "/etc/fail2ban/jail.d/")
		}

		// Clean up any remaining invalid commands in Port or Filter
		if j.Port == "" || strings.Contains(j.Port, "Invalid") {
			switch {
			case j.Name == "sshd":
				j.Port = "ssh"
			case strings.HasPrefix(j.Name, "nginx-") || strings.HasPrefix(j.Name, "apache-"):
				j.Port = "http,https"
			case strings.HasPrefix(j.Name, "mysqld-") || j.Name == "mysql":
				j.Port = "3306"
			default:
				j.Port = "ssh"
			}
		}

		if j.Filter == "" || strings.Contains(j.Filter, "Invalid") {
			j.Filter = j.Name
		}

		if j.LogPath == "" || strings.Contains(j.LogPath, "Current monitored") || strings.Contains(j.LogPath, "Invalid") {
			if j.Name == "sshd" {
				j.LogPath = "/var/log/auth.log"
			}
		}
	}

	// 4. Add configured jails that are currently not running (e.g. enabled = false or stopped in jail.local or jail.d)
	for name, cfg := range configs {
		if processedJails[name] {
			continue
		}
		// Skip reference template entries from stock jail.conf
		if cfg.ConfigFile == "/etc/fail2ban/jail.conf" {
			continue
		}
		// Only include if it is a user-managed jail (e.g. TameStack-*.local) or explicitly enabled in jail.local / jail.d
		isManaged := strings.Contains(cfg.ConfigFile, "TameStack-") || strings.HasPrefix(cfg.ConfigFile, "/etc/fail2ban/jail.d/")
		if !isManaged && !cfg.Enabled {
			continue
		}

		status.Jails = append(status.Jails, Fail2banJail{
			Name:            name,
			Enabled:         cfg.Enabled,
			Port:            cfg.Port,
			Filter:          cfg.Filter,
			LogPath:         cfg.LogPath,
			MaxRetry:        cfg.MaxRetry,
			FindTime:        cfg.FindTime,
			BanTime:         cfg.BanTime,
			Action:          cfg.Action,
			IgnoreIP:        cfg.IgnoreIP,
			CurrentlyFailed: 0,
			TotalFailed:     0,
			CurrentlyBanned: 0,
			TotalBanned:     0,
			BannedIPList:    []string{},
			ConfigFile:      cfg.ConfigFile,
			IsManaged:       isManaged,
		})
	}

	// Sort jails: active first, then alphabetically
	sort.Slice(status.Jails, func(i, j int) bool {
		if status.Jails[i].Enabled != status.Jails[j].Enabled {
			return status.Jails[i].Enabled
		}
		return status.Jails[i].Name < status.Jails[j].Name
	})

	return status, nil
}

// ControlFail2ban starts, stops, or reloads the fail2ban service.
func (s *Service) ControlFail2ban(ctx context.Context, serverID string, action string) (*db.TaskRecord, error) {
	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}
	if !validActions[action] {
		return nil, fmt.Errorf("недопустимое действие: %s", action)
	}

	payload, _ := json.Marshal(map[string]string{
		"server_id": serverID,
		"action":    action,
	})
	title := fmt.Sprintf("Управление Fail2ban: %s", strings.ToUpper(action))
	return s.engine.Submit(serverID, "security_fail2ban_control", title, string(payload))
}

func (s *Service) handleFail2banControlTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID string `json:"server_id"`
		Action   string `json:"action"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.getSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	logger.Info(fmt.Sprintf("Выполнение действия %s для службы Fail2ban...", payload.Action))
	logger.SetStep("Анализ состояния Fail2ban", 30)

	script := fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ACTION=%q

# Clean stale socket file if fail2ban-server process is dead
if ! pgrep -f fail2ban-server >/dev/null 2>&1 && ! systemctl is-active --quiet fail2ban 2>/dev/null; then
    rm -f /var/run/fail2ban/fail2ban.sock /run/fail2ban/fail2ban.sock 2>/dev/null
fi

OUT=""
case "$ACTION" in
    reload)
        if systemctl is-active --quiet fail2ban 2>/dev/null || fail2ban-client ping >/dev/null 2>&1; then
            OUT=$(fail2ban-client reload 2>&1 || systemctl reload fail2ban 2>&1 || systemctl restart fail2ban 2>&1)
        else
            rm -f /var/run/fail2ban/fail2ban.sock /run/fail2ban/fail2ban.sock 2>/dev/null
            OUT=$(systemctl restart fail2ban 2>&1 || systemctl start fail2ban 2>&1 || fail2ban-client start 2>&1)
        fi
        ;;
    restart)
        rm -f /var/run/fail2ban/fail2ban.sock /run/fail2ban/fail2ban.sock 2>/dev/null
        OUT=$(systemctl restart fail2ban 2>&1 || (fail2ban-client stop 2>/dev/null; sleep 1; fail2ban-client start 2>&1))
        ;;
    start)
        rm -f /var/run/fail2ban/fail2ban.sock /run/fail2ban/fail2ban.sock 2>/dev/null
        OUT=$(systemctl start fail2ban 2>&1 || systemctl restart fail2ban 2>&1 || fail2ban-client start 2>&1)
        ;;
    stop)
        OUT=$(systemctl stop fail2ban 2>&1 || fail2ban-client stop 2>&1)
        ;;
esac

echo "$OUT"

if [ "$ACTION" != "stop" ]; then
    sleep 1
    if systemctl is-active --quiet fail2ban 2>/dev/null || pgrep -f fail2ban-server >/dev/null 2>&1 || fail2ban-client ping >/dev/null 2>&1; then
        echo "RESULT:SUCCESS"
    else
        echo "RESULT:INACTIVE"
    fi
else
    echo "RESULT:STOPPED"
fi
`, payload.Action)

	logger.SetStep("Применение команды", 60)
	out, err := s.execCmd(ctx, client, script)
	if err != nil {
		logger.Warn(fmt.Sprintf("Предупреждение при выполнении (%v): %s", err, out))
	}

	logger.SetStep("Проверка статуса", 90)
	if strings.Contains(out, "RESULT:SUCCESS") || strings.Contains(out, "RESULT:STOPPED") {
		logger.Info(fmt.Sprintf("Действие %s для Fail2ban успешно завершено.", payload.Action))
		return fmt.Sprintf("Fail2ban: действие %s успешно выполнено", payload.Action), nil
	}

	if payload.Action != "stop" && strings.Contains(out, "RESULT:INACTIVE") {
		logger.Warn("Служба Fail2ban не смогла перейти в активное состояние. Проверьте синтаксис конфигурации")
		return fmt.Sprintf("Действие %s выполнено с замечаниями:\n%s", payload.Action, strings.TrimSpace(out)), nil
	}

	return fmt.Sprintf("Результат выполнения %s: %s", payload.Action, strings.TrimSpace(out)), nil
}

// SaveFail2banJail creates or updates a fail2ban jail configuration.
func (s *Service) SaveFail2banJail(ctx context.Context, req SaveFail2banJailRequest) (*db.TaskRecord, error) {
	req.Name = strings.TrimSpace(req.Name)
	if err := ssh.ValidateToken(req.Name, "jail name"); err != nil {
		return nil, fmt.Errorf("недопустимое имя джейла: %w", err)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации: %w", err)
	}

	title := fmt.Sprintf("Настройка джейла Fail2ban: %s", req.Name)
	return s.engine.Submit(req.ServerID, "security_fail2ban_save_jail", title, string(payload))
}

func (s *Service) handleFail2banSaveJailTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req SaveFail2banJailRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	req.Name = strings.TrimSpace(req.Name)
	if err := ssh.ValidateToken(req.Name, "jail name"); err != nil {
		return "", fmt.Errorf("недопустимое имя джейла: %w", err)
	}
	if req.Filter != "" {
		req.Filter = strings.TrimSpace(req.Filter)
		if err := ssh.ValidateToken(req.Filter, "filter name"); err != nil {
			return "", fmt.Errorf("недопустимое имя фильтра: %w", err)
		}
	} else {
		req.Filter = req.Name
	}

	if req.MaxRetry <= 0 {
		req.MaxRetry = 5
	}
	if req.FindTime == "" {
		req.FindTime = "10m"
	}
	if req.BanTime == "" {
		req.BanTime = "1h"
	}
	if req.Port == "" {
		req.Port = "ssh"
	}

	cleanVal := func(s string) string {
		s = strings.ReplaceAll(s, "\n", "")
		s = strings.ReplaceAll(s, "\r", "")
		s = strings.ReplaceAll(s, "'", "")
		s = strings.ReplaceAll(s, `"`, "")
		return strings.TrimSpace(s)
	}
	req.Port = cleanVal(req.Port)
	req.FindTime = cleanVal(req.FindTime)
	req.BanTime = cleanVal(req.BanTime)
	req.LogPath = cleanVal(req.LogPath)
	req.IgnoreIP = cleanVal(req.IgnoreIP)
	req.Action = cleanVal(req.Action)

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Подготовка конфигурации джейла", 20)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Generated by TameStack\n[%s]\nenabled = %t\n", req.Name, req.Enabled))
	if req.Port != "" {
		sb.WriteString(fmt.Sprintf("port = %s\n", req.Port))
	}
	if req.Filter != "" {
		sb.WriteString(fmt.Sprintf("filter = %s\n", req.Filter))
	}
	if req.LogPath != "" {
		sb.WriteString(fmt.Sprintf("logpath = %s\n", req.LogPath))
	}
	sb.WriteString(fmt.Sprintf("maxretry = %d\n", req.MaxRetry))
	sb.WriteString(fmt.Sprintf("findtime = %s\n", req.FindTime))
	sb.WriteString(fmt.Sprintf("bantime = %s\n", req.BanTime))
	if req.IgnoreIP != "" {
		sb.WriteString(fmt.Sprintf("ignoreip = 127.0.0.1/8 ::1 %s\n", req.IgnoreIP))
	}
	if req.Action != "" {
		sb.WriteString(fmt.Sprintf("banaction = %s\n", req.Action))
	}
	configContent := sb.String()

	configFile := fmt.Sprintf("/etc/fail2ban/jail.d/tamestack-%s.local", req.Name)

	logger.SetStep("Запись файла конфигурации", 40)
	writeScript := fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
mkdir -p /etc/fail2ban/jail.d
if [ -f %s ]; then
    cp %s %s.bak 2>/dev/null || true
fi
`, ssh.ShellEscape(configFile), ssh.ShellEscape(configFile), ssh.ShellEscape(configFile))

	if req.LogPath != "" && !strings.Contains(req.LogPath, "*") {
		writeScript += fmt.Sprintf(`
LOG_DIR=$(dirname %s)
[ -d "$LOG_DIR" ] && touch %s 2>/dev/null || true
`, ssh.ShellEscape(req.LogPath), ssh.ShellEscape(req.LogPath))
	}

	writeScript += fmt.Sprintf(`
cat << 'EOF' > %s
%s
EOF
`, configFile, configContent)

	if out, err := s.execCmd(ctx, client, writeScript); err != nil {
		return "", fmt.Errorf("ошибка записи конфигурации (%v): %s", err, out)
	}

	logger.SetStep("Проверка синтаксиса Fail2ban", 70)
	testCmd := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
fail2ban-client -t 2>&1 || sudo fail2ban-client -t 2>&1
`
	testOut, testErr := s.execCmd(ctx, client, testCmd)
	if testErr != nil || strings.Contains(strings.ToLower(testOut), "error") || strings.Contains(strings.ToLower(testOut), "failed") {
		logger.Warn(fmt.Sprintf("Синтаксическая проверка вернула предупреждение или ошибку: %s", testOut))
		if strings.Contains(strings.ToLower(testOut), "error") {
			rollbackScript := fmt.Sprintf(`
if [ -f %s.bak ]; then
    mv %s.bak %s
else
    rm -f %s
fi
fail2ban-client reload 2>/dev/null || true
`, ssh.ShellEscape(configFile), ssh.ShellEscape(configFile), ssh.ShellEscape(configFile), ssh.ShellEscape(configFile))
			_, _ = s.execCmd(ctx, client, rollbackScript)
			return "", fmt.Errorf("ошибка валидации синтаксиса fail2ban: %s", strings.TrimSpace(testOut))
		}
	}

	logger.SetStep("Перезагрузка конфигурации Fail2ban", 90)
	reloadScript := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
if systemctl is-active --quiet fail2ban 2>/dev/null || fail2ban-client ping >/dev/null 2>&1; then
    fail2ban-client reload 2>&1 || systemctl reload fail2ban 2>&1 || systemctl restart fail2ban 2>&1
else
    systemctl start fail2ban 2>&1 || fail2ban-client start 2>&1 || true
fi
`
	reloadOut, _ := s.execCmd(ctx, client, reloadScript)

	logger.SetStep("Готово", 100)
	logger.Info(fmt.Sprintf("Джейл %s успешно настроен:\n%s", req.Name, reloadOut))
	return fmt.Sprintf("Джейл %s успешно сохранен и применен", req.Name), nil
}

// DeleteFail2banJail deletes a fail2ban jail.
func (s *Service) DeleteFail2banJail(ctx context.Context, req DeleteFail2banJailRequest) (*db.TaskRecord, error) {
	req.Name = strings.TrimSpace(req.Name)
	if err := ssh.ValidateToken(req.Name, "jail name"); err != nil {
		return nil, fmt.Errorf("недопустимое имя джейла: %w", err)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации: %w", err)
	}

	title := fmt.Sprintf("Удаление джейла Fail2ban: %s", req.Name)
	return s.engine.Submit(req.ServerID, "security_fail2ban_delete_jail", title, string(payload))
}

func (s *Service) handleFail2banDeleteJailTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req DeleteFail2banJailRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	req.Name = strings.TrimSpace(req.Name)
	if err := ssh.ValidateToken(req.Name, "jail name"); err != nil {
		return "", fmt.Errorf("недопустимое имя джейла: %w", err)
	}

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep(fmt.Sprintf("Удаление джейла %s", req.Name), 40)
	target1 := fmt.Sprintf("/etc/fail2ban/jail.d/tamestack-%s.local", req.Name)
	target2 := fmt.Sprintf("/etc/fail2ban/jail.d/%s.local", req.Name)
	target3 := fmt.Sprintf("/etc/fail2ban/jail.d/%s.conf", req.Name)

	script := fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
fail2ban-client stop %s 2>/dev/null || true
rm -f %s %s %s 2>/dev/null
if fail2ban-client status 2>/dev/null | grep -q %s; then
    printf '[%s]\nenabled = false\n' > %s
fi
fail2ban-client reload 2>&1 || systemctl reload fail2ban 2>&1 || true
`, ssh.ShellEscape(req.Name),
		ssh.ShellEscape(target1), ssh.ShellEscape(target2), ssh.ShellEscape(target3),
		ssh.ShellEscape(req.Name), req.Name, ssh.ShellEscape(target1))

	out, err := s.execCmd(ctx, client, script)
	if err != nil {
		logger.Warn(fmt.Sprintf("Замечание при удалении (%v): %s", err, out))
	}

	logger.SetStep("Готово", 100)
	logger.Info(fmt.Sprintf("Джейл %s удален.", req.Name))
	return fmt.Sprintf("Джейл %s успешно удален", req.Name), nil
}

// ToggleFail2banJail enables or disables a fail2ban jail.
func (s *Service) ToggleFail2banJail(ctx context.Context, req ToggleFail2banJailRequest) (*db.TaskRecord, error) {
	req.Name = strings.TrimSpace(req.Name)
	if err := ssh.ValidateToken(req.Name, "jail name"); err != nil {
		return nil, fmt.Errorf("недопустимое имя джейла: %w", err)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации: %w", err)
	}

	actionStr := "Включение"
	if !req.Enabled {
		actionStr = "Отключение"
	}
	title := fmt.Sprintf("%s джейла Fail2ban: %s", actionStr, req.Name)
	return s.engine.Submit(req.ServerID, "security_fail2ban_toggle_jail", title, string(payload))
}

func (s *Service) handleFail2banToggleJailTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req ToggleFail2banJailRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	req.Name = strings.TrimSpace(req.Name)
	if err := ssh.ValidateToken(req.Name, "jail name"); err != nil {
		return "", fmt.Errorf("недопустимое имя джейла: %w", err)
	}

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	actionStr := "Включение"
	if !req.Enabled {
		actionStr = "Отключение"
	}
	logger.SetStep(fmt.Sprintf("%s джейла %s", actionStr, req.Name), 40)
	target := fmt.Sprintf("/etc/fail2ban/jail.d/tamestack-%s.local", req.Name)

	var script string
	if req.Enabled {
		script = fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
mkdir -p /etc/fail2ban/jail.d
if [ -f %s ]; then
    sed -i 's/^enabled\s*=.*/enabled = true/' %s
else
    printf '[%s]\nenabled = true\n' > %s
fi
fail2ban-client reload 2>&1 || fail2ban-client start %s 2>&1 || systemctl reload fail2ban 2>&1 || true
`, ssh.ShellEscape(target), ssh.ShellEscape(target), req.Name, ssh.ShellEscape(target), ssh.ShellEscape(req.Name))
	} else {
		script = fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
mkdir -p /etc/fail2ban/jail.d
if [ -f %s ]; then
    sed -i 's/^enabled\s*=.*/enabled = false/' %s
else
    printf '[%s]\nenabled = false\n' > %s
fi
fail2ban-client stop %s 2>/dev/null || true
fail2ban-client reload 2>&1 || systemctl reload fail2ban 2>&1 || true
`, ssh.ShellEscape(target), ssh.ShellEscape(target), req.Name, ssh.ShellEscape(target), ssh.ShellEscape(req.Name))
	}

	out, err := s.execCmd(ctx, client, script)
	if err != nil {
		logger.Warn(fmt.Sprintf("Замечание при переключении (%v): %s", err, out))
	}

	logger.SetStep("Готово", 100)
	logger.Info(fmt.Sprintf("Статус джейла %s изменен: enabled=%t", req.Name, req.Enabled))
	return fmt.Sprintf("Джейл %s: статус изменен на %t", req.Name, req.Enabled), nil
}

// BanIP blocks an IP address in the specified jail.
func (s *Service) BanIP(ctx context.Context, req BanIPRequest) (*db.TaskRecord, error) {
	if req.IP == "" || req.Jail == "" {
		return nil, fmt.Errorf("не указан IP или джейл для блокировки")
	}

	payload, _ := json.Marshal(req)
	title := fmt.Sprintf("Блокировка IP %s в Fail2ban (%s)", req.IP, req.Jail)
	return s.engine.Submit(req.ServerID, "security_fail2ban_ban", title, string(payload))
}

func (s *Service) handleFail2banBanTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req BanIPRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Блокировка IP", 50)
	cmd := fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
fail2ban-client set %s banip %s 2>/dev/null || sudo fail2ban-client set %s banip %s 2>/dev/null
`, req.Jail, req.IP, req.Jail, req.IP)

	out, err := s.execCmd(ctx, client, cmd)
	if err != nil {
		return "", fmt.Errorf("ошибка блокировки: %w: %s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("IP %s успешно заблокирован в джейле %s", req.IP, req.Jail), nil
}

// UnbanIP unblocks an IP address in the specified jail.
func (s *Service) UnbanIP(ctx context.Context, req UnbanIPRequest) (*db.TaskRecord, error) {
	if req.IP == "" || req.Jail == "" {
		return nil, fmt.Errorf("не указан IP или джейл для разблокировки")
	}

	payload, _ := json.Marshal(req)
	title := fmt.Sprintf("Разблокировка IP %s в Fail2ban (%s)", req.IP, req.Jail)
	return s.engine.Submit(req.ServerID, "security_fail2ban_unban", title, string(payload))
}

func (s *Service) handleFail2banUnbanTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req UnbanIPRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Разблокировка IP", 50)
	cmd := fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
fail2ban-client set %s unbanip %s 2>/dev/null || sudo fail2ban-client set %s unbanip %s 2>/dev/null
`, req.Jail, req.IP, req.Jail, req.IP)
	out, err := s.execCmd(ctx, client, cmd)
	if err != nil {
		return "", fmt.Errorf("ошибка разблокировки: %w: %s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("IP %s успешно разблокирован в джейле %s", req.IP, req.Jail), nil
}

// InstallFail2ban runs installation and basic activation of fail2ban.
func (s *Service) InstallFail2ban(ctx context.Context, serverID string) (*db.TaskRecord, error) {
	payload, _ := json.Marshal(map[string]string{"server_id": serverID})
	return s.engine.Submit(serverID, "security_fail2ban_install", "Установка и настройка Fail2ban", string(payload))
}

func (s *Service) handleFail2banInstallTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID string `json:"server_id"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.getSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Определение пакетного менеджера", 20)
	adapter, _, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return "", fmt.Errorf("ошибка определения ОС: %w", err)
	}

	if adapter.GetOSInfo().Family != osadapter.FamilyRhel {
		logger.SetStep("Установка Fail2ban через APT", 40)
		if _, err := adapter.UpdatePackageIndex(ctx, client); err != nil {
			return "", fmt.Errorf("ошибка установки fail2ban: %w", err)
		}
		if _, err := adapter.InstallPackages(ctx, client, []string{"fail2ban"}); err != nil {
			return "", fmt.Errorf("ошибка установки fail2ban: %w", err)
		}
	} else {
		logger.SetStep("Установка Fail2ban через DNF/YUM", 40)
		_, _ = adapter.InstallPackages(ctx, client, []string{"epel-release"})
		if _, err := adapter.InstallPackages(ctx, client, []string{"fail2ban"}); err != nil {
			return "", fmt.Errorf("ошибка установки fail2ban: %w", err)
		}
	}

	logger.SetStep("Создание базовой конфигурации jail.local", 70)
	_, _ = s.execCmd(ctx, client, "test -f /etc/fail2ban/jail.local && cp /etc/fail2ban/jail.local /etc/fail2ban/jail.local.bak_tamestack")

	localConfig := `[DEFAULT]
ignoreip = 127.0.0.1/8 ::1
bantime = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
port = ssh
filter = sshd
`
	_, _ = s.execCmd(ctx, client, fmt.Sprintf("cat << 'EOF' > /etc/fail2ban/jail.local\n%s\nEOF", localConfig))

	logger.SetStep("Запуск и включение службы Fail2ban", 90)
	_, _ = s.execCmd(ctx, client, "rm -f /var/run/fail2ban/fail2ban.sock /run/fail2ban/fail2ban.sock 2>/dev/null")
	_, _ = s.execCmd(ctx, client, "systemctl enable --now fail2ban 2>/dev/null || true")
	_, _ = s.execCmd(ctx, client, "systemctl restart fail2ban 2>/dev/null || fail2ban-client restart 2>/dev/null || fail2ban-client start 2>/dev/null || true")

	logger.SetStep("Установка завершена", 100)
	return "Fail2ban успешно установлен и активирован для защиты SSH", nil
}
