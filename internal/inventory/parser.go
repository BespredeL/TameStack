package inventory

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reSocketProcess = regexp.MustCompile(`users:\(\("([^"]+)",pid=([0-9]+)`)
)

// ParseUnits parses `systemctl list-units --no-legend --no-pager --plain`.
func ParseUnits(output string) []SystemUnit {
	var units []SystemUnit
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "●") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 4 {
			desc := ""
			if len(fields) >= 5 {
				desc = strings.Join(fields[4:], " ")
			}
			units = append(units, SystemUnit{
				Unit:        fields[0],
				Load:        fields[1],
				Active:      fields[2],
				Sub:         fields[3],
				Description: desc,
			})
		}
	}
	return units
}

// ParseSockets parses `ss -tulpn` or `ss -tlpn`.
func ParseSockets(output string) []ListeningSocket {
	var sockets []ListeningSocket
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Netid") || strings.HasPrefix(trimmed, "State") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 5 {
			continue
		}
		proto := strings.ToLower(fields[0])
		local := fields[4]
		port := 0
		if idx := strings.LastIndex(local, ":"); idx != -1 {
			p, _ := strconv.Atoi(local[idx+1:])
			port = p
		}

		proc := ""
		pid := 0
		if len(fields) >= 6 {
			lineEnd := strings.Join(fields[5:], " ")
			if m := reSocketProcess.FindStringSubmatch(lineEnd); len(m) > 2 {
				proc = m[1]
				pid, _ = strconv.Atoi(m[2])
			}
		}

		sockets = append(sockets, ListeningSocket{
			Proto:   proto,
			Local:   local,
			Port:    port,
			Process: proc,
			Pid:     pid,
		})
	}
	return sockets
}

// ParsePasswd parses `/etc/passwd`.
func ParsePasswd(output string) []OSUser {
	var users []OSUser
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			uid, _ := strconv.Atoi(parts[2])
			gid, _ := strconv.Atoi(parts[3])
			isSystem := uid < 1000
			users = append(users, OSUser{
				Username: parts[0],
				UID:      uid,
				GID:      gid,
				Home:     parts[5],
				Shell:    parts[6],
				IsSystem: isSystem,
			})
		}
	}
	return users
}

// ParseDF parses `df -B1 -x tmpfs -x devtmpfs -x squashfs`.
func ParseDF(output string) []MountPoint {
	var mounts []MountPoint
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Filesystem") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 6 {
			total, _ := strconv.ParseInt(fields[1], 10, 64)
			free, _ := strconv.ParseInt(fields[3], 10, 64)
			mounts = append(mounts, MountPoint{
				Filesystem: fields[0],
				TotalBytes: total,
				FreeBytes:  free,
				Mount:      fields[5],
			})
		}
	}
	return mounts
}

// ParseCron parses crontab or cron file lines.
func ParseCron(output, user, source string) []CronEntry {
	var entries []CronEntry
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, "=") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 6 {
			sched := strings.Join(fields[0:5], " ")
			cmd := strings.Join(fields[5:], " ")
			entries = append(entries, CronEntry{
				Schedule: sched,
				Command:  cmd,
				User:     user,
				Source:   source,
			})
		}
	}
	return entries
}

// cronShortcuts are @-prefixed schedules accepted in cron files.
var cronShortcuts = map[string]bool{
	"@reboot": true, "@yearly": true, "@annually": true, "@monthly": true,
	"@weekly": true, "@daily": true, "@midnight": true, "@hourly": true,
}

// ParseCronSystem parses /etc/crontab and /etc/cron.d/* style lines, where a
// user column sits between the schedule and the command: `<sched...> user cmd`.
func ParseCronSystem(output, source string) []CronEntry {
	var entries []CronEntry
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, "=") {
			continue
		}
		fields := strings.Fields(trimmed)
		if strings.HasPrefix(fields[0], "@") {
			if !cronShortcuts[fields[0]] || len(fields) < 3 {
				continue
			}
			entries = append(entries, CronEntry{
				Schedule: fields[0],
				Command:  strings.Join(fields[2:], " "),
				User:     fields[1],
				Source:   source,
			})
			continue
		}
		if len(fields) < 8 {
			continue
		}
		entries = append(entries, CronEntry{
			Schedule: strings.Join(fields[0:5], " "),
			Command:  strings.Join(fields[6:], " "),
			User:     fields[5],
			Source:   source,
		})
	}
	return entries
}

var reVersionToken = regexp.MustCompile(`\d+\.\d+\.\d+|\d+\.\d+`)

// ShortVersion extracts a comparable version number from DBMS `--version`
// output. MariaDB prints the constant client protocol version "15.1" before
// the real server version ("Distrib 10.6.22"), so search after that marker;
// otherwise the first version token is the server's own.
func ShortVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "Distrib"); i >= 0 {
		raw = raw[i+len("Distrib"):]
	}
	if m := reVersionToken.FindString(raw); m != "" {
		return "v" + m
	}
	if len(raw) > 40 {
		raw = raw[:40]
	}
	return raw
}

// ParseContainers parses `docker ps -a --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'`.
func ParseContainers(output string) []ContainerInfo {
	var containers []ContainerInfo
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(trimmed, "\t")
		if len(parts) >= 4 {
			ports := ""
			if len(parts) >= 5 {
				ports = parts[4]
			}
			containers = append(containers, ContainerInfo{
				ID:     parts[0],
				Names:  parts[1],
				Image:  parts[2],
				Status: parts[3],
				Ports:  ports,
			})
		}
	}
	return containers
}
