package services

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// validUnitNameRegex matches standard systemd unit names (e.g. nginx.service, php8.2-fpm.service, certbot.timer).
	validUnitNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.@]+$`)

	// protectedUnits contains critical services that must never be stopped or disabled to prevent lockout.
	protectedUnits = map[string]bool{
		"ssh":                      true,
		"ssh.service":              true,
		"sshd":                     true,
		"sshd.service":             true,
		"systemd-networkd":         true,
		"systemd-networkd.service": true,
		"systemd-resolved":         true,
		"systemd-resolved.service": true,
		"networking":               true,
		"networking.service":       true,
		"network":                  true,
		"network.service":          true,
	}

	// ssProcessRegex extracts process name and pid from users:(("nginx",pid=1234,...)).
	ssProcessRegex = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)
)

// SanitizeUnitName ensures the unit name contains only valid characters and is non-empty.
func SanitizeUnitName(unitName string) (string, error) {
	u := strings.TrimSpace(unitName)
	if u == "" {
		return "", fmt.Errorf("имя службы не может быть пустым")
	}
	if len(u) > 128 {
		return "", fmt.Errorf("имя службы слишком длинное: %s", u)
	}
	if !validUnitNameRegex.MatchString(u) {
		return "", fmt.Errorf("недопустимое имя службы: %s (содержит недопустимые символы)", u)
	}
	return u, nil
}

// IsProtectedUnit checks whether stopping or disabling this unit would sever network/SSH connection.
func IsProtectedUnit(unitName, action string) error {
	u := strings.ToLower(strings.TrimSpace(unitName))
	act := strings.ToLower(strings.TrimSpace(action))

	if act == "stop" || act == "disable" || act == "mask" {
		if protectedUnits[u] || protectedUnits[strings.TrimSuffix(u, ".service")] {
			return fmt.Errorf("остановка или отключение службы '%s' запрещены политикой безопасности TameStack: это приведет к потере доступа к серверу по SSH", unitName)
		}
	}
	return nil
}

// ValidateProcessKill ensures PID 1 or negative PIDs are never targeted.
func ValidateProcessKill(pid int, signal string) error {
	if pid <= 1 {
		return fmt.Errorf("завершение процесса с PID %d (init/systemd/ядро) категорически запрещено", pid)
	}
	sig := strings.ToUpper(strings.TrimSpace(signal))
	allowedSignals := map[string]bool{
		"SIGTERM": true,
		"SIGKILL": true,
		"SIGHUP":  true,
		"SIGINT":  true,
		"TERM":    true,
		"KILL":    true,
		"HUP":     true,
	}
	if sig != "" && !allowedSignals[sig] {
		return fmt.Errorf("недопустимый сигнал: %s (разрешены: SIGTERM, SIGKILL, SIGHUP, SIGINT)", signal)
	}
	return nil
}

// ParseSystemctlUnits parses the plain output of `systemctl list-units --all --no-pager --no-legend --plain`.
func ParseSystemctlUnits(output string) ([]SystemUnit, error) {
	var units []SystemUnit
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		unitName := fields[0]
		load := fields[1]
		active := fields[2]
		sub := fields[3]
		desc := ""
		if len(fields) >= 5 {
			desc = strings.Join(fields[4:], " ")
		}

		unitType := "other"
		if strings.HasSuffix(unitName, ".service") {
			unitType = "service"
		} else if strings.HasSuffix(unitName, ".timer") {
			unitType = "timer"
		} else if strings.HasSuffix(unitName, ".socket") {
			unitType = "socket"
		} else if strings.HasSuffix(unitName, ".target") {
			unitType = "target"
		}

		units = append(units, SystemUnit{
			Unit:        unitName,
			Load:        load,
			Active:      active,
			Sub:         sub,
			Description: desc,
			UnitType:    unitType,
		})
	}

	return units, scanner.Err()
}

// ParsePsOutput parses the output of `ps -eo user:20,pid,%cpu,%mem,vsz,rss,stat,start,time,args --sort=-%cpu`.
func ParsePsOutput(output string) ([]ProcessItem, error) {
	var procs []ProcessItem
	scanner := bufio.NewScanner(strings.NewReader(output))

	lineNum := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++
		if lineNum == 1 && strings.Contains(line, "PID") {
			// Skip header
			continue
		}
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		user := fields[0]
		pid, _ := strconv.Atoi(fields[1])
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 64)
		vsz, _ := strconv.ParseInt(fields[4], 10, 64)
		rss, _ := strconv.ParseInt(fields[5], 10, 64)
		stat := fields[6]
		start := fields[7]
		timeStr := fields[8]
		cmd := strings.Join(fields[9:], " ")

		procs = append(procs, ProcessItem{
			User:    user,
			PID:     pid,
			CPU:     cpu,
			Mem:     mem,
			VSZ:     vsz,
			RSS:     rss,
			Stat:    stat,
			Start:   start,
			Time:    timeStr,
			Command: cmd,
		})
	}

	return procs, scanner.Err()
}

// ParseSsListening parses the output of `ss -tulpn`.
func ParseSsListening(output string) ([]ListeningPortItem, error) {
	var items []ListeningPortItem
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Netid") || strings.HasPrefix(line, "State") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		proto := fields[0]
		recvQ, _ := strconv.Atoi(fields[1])
		sendQ, _ := strconv.Atoi(fields[2])
		local := fields[4]
		peer := ""
		if len(fields) >= 6 {
			peer = fields[5]
		}

		// Split local addr and port.
		localAddr, localPort := splitHostPort(local)
		peerAddr, peerPort := splitHostPort(peer)

		processName := ""
		procPid := 0
		rawUsers := strings.Join(fields[5:], " ")
		if m := ssProcessRegex.FindStringSubmatch(rawUsers); len(m) == 3 {
			processName = m[1]
			procPid, _ = strconv.Atoi(m[2])
		}

		items = append(items, ListeningPortItem{
			Proto:       proto,
			RecvQ:       recvQ,
			SendQ:       sendQ,
			LocalAddr:   localAddr,
			LocalPort:   localPort,
			ForeignAddr: peerAddr,
			ForeignPort: peerPort,
			Process:     processName,
			PID:         procPid,
		})
	}

	return items, scanner.Err()
}

func splitHostPort(endpoint string) (string, int) {
	idx := strings.LastIndex(endpoint, ":")
	if idx == -1 {
		return endpoint, 0
	}
	host := endpoint[:idx]
	portStr := endpoint[idx+1:]
	port, _ := strconv.Atoi(portStr)
	return host, port
}
