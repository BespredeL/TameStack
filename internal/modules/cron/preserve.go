package cron

import "strings"

// cronShortcuts are @-prefixed schedules accepted in cron files.
var cronShortcuts = map[string]bool{
	"@reboot": true, "@yearly": true, "@annually": true, "@monthly": true,
	"@weekly": true, "@daily": true, "@midnight": true, "@hourly": true,
}

// splitCrontab parses standard five-field and shortcut jobs and returns every other line unchanged in preserved.
func splitCrontab(raw, user string) (jobs []CronJob, preserved []string) {
	var pending string
	var pendingSet bool
	flushPending := func() {
		if pendingSet {
			preserved = append(preserved, pending)
			pending = ""
			pendingSet = false
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushPending()
			continue
		}
		if isManagedNoise(trimmed) {
			flushPending()
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			clean := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if strings.HasPrefix(clean, "DISABLED:") {
				clean = strings.TrimSpace(strings.TrimPrefix(clean, "DISABLED:"))
			}
			fields := strings.Fields(clean)
			// Shortcut: #DISABLED: @daily /path/to/cmd or # @reboot cmd
			if len(fields) >= 2 && cronShortcuts[strings.ToLower(fields[0])] {
				expr := fields[0]
				cmd := strings.Join(fields[1:], " ")
				comment := ""
				if pendingSet && strings.HasPrefix(strings.TrimSpace(pending), "#") && !strings.Contains(pending, "DISABLED:") {
					comment = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(pending), "#"))
					pendingSet = false
				} else {
					flushPending()
				}
				jobs = append(jobs, CronJob{
					ID: makeJobID(expr, cmd), Expression: expr, Command: cmd,
					Enabled: false, Comment: comment, User: user, Source: "crontab",
				})
				continue
			}
			// 5-field schedule: #DISABLED: 0 * * * * cmd
			if len(fields) >= 6 && isCronSchedule(fields[:5]) {
				expr := strings.Join(fields[:5], " ")
				cmd := strings.Join(fields[5:], " ")
				comment := ""
				if pendingSet && strings.HasPrefix(strings.TrimSpace(pending), "#") && !strings.Contains(pending, "DISABLED:") {
					comment = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(pending), "#"))
					pendingSet = false
				} else {
					flushPending()
				}
				jobs = append(jobs, CronJob{
					ID: makeJobID(expr, cmd), Expression: expr, Command: cmd,
					Enabled: false, Comment: comment, User: user, Source: "crontab",
				})
				continue
			}
			flushPending()
			pending = line
			pendingSet = true
			continue
		}

		fields := strings.Fields(trimmed)
		// Shortcut: @daily /path/to/cmd
		if len(fields) >= 2 && cronShortcuts[strings.ToLower(fields[0])] {
			expr := fields[0]
			cmd := strings.Join(fields[1:], " ")
			comment := ""
			if pendingSet && strings.HasPrefix(strings.TrimSpace(pending), "#") {
				comment = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(pending), "#"))
				pendingSet = false
			} else {
				flushPending()
			}
			jobs = append(jobs, CronJob{
				ID: makeJobID(expr, cmd), Expression: expr, Command: cmd,
				Enabled: true, Comment: comment, User: user, Source: "crontab",
			})
			continue
		}

		// 5-field schedule
		if len(fields) >= 6 && isCronSchedule(fields[:5]) {
			expr := strings.Join(fields[:5], " ")
			cmd := strings.Join(fields[5:], " ")
			comment := ""
			if pendingSet && strings.HasPrefix(strings.TrimSpace(pending), "#") {
				comment = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(pending), "#"))
				pendingSet = false
			} else {
				flushPending()
			}
			jobs = append(jobs, CronJob{
				ID: makeJobID(expr, cmd), Expression: expr, Command: cmd,
				Enabled: true, Comment: comment, User: user, Source: "crontab",
			})
			continue
		}
		flushPending()
		preserved = append(preserved, line)
	}
	flushPending()
	return jobs, preserved
}

// parseSystemCronFile parses /etc/crontab and /etc/cron.d/* style files where username is a column.
func parseSystemCronFile(raw, targetUser, source string) []CronJob {
	var jobs []CronJob
	var pendingComment string

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isManagedNoise(trimmed) {
			pendingComment = ""
			continue
		}

		enabled := true
		parseLine := trimmed
		if strings.HasPrefix(trimmed, "#") {
			clean := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if strings.HasPrefix(clean, "DISABLED:") {
				clean = strings.TrimSpace(strings.TrimPrefix(clean, "DISABLED:"))
			}
			testFields := strings.Fields(clean)
			isMatch := false
			if len(testFields) >= 3 && cronShortcuts[strings.ToLower(testFields[0])] {
				isMatch = true
			} else if len(testFields) >= 7 && isCronSchedule(testFields[:5]) {
				isMatch = true
			}
			if isMatch {
				enabled = false
				parseLine = clean
			} else {
				pendingComment = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
				continue
			}
		}

		// Skip environment assignments like SHELL=/bin/bash or PATH=...
		if strings.Contains(parseLine, "=") {
			idx := strings.Index(parseLine, "=")
			prefix := strings.TrimSpace(parseLine[:idx])
			if !strings.Contains(prefix, " ") && !strings.Contains(prefix, "*") {
				pendingComment = ""
				continue
			}
		}

		fields := strings.Fields(parseLine)
		// Shortcut format: @daily root cmd...
		if len(fields) >= 3 && cronShortcuts[strings.ToLower(fields[0])] {
			u := fields[1]
			if targetUser == "" || targetUser == "all" || strings.EqualFold(u, targetUser) {
				expr := fields[0]
				cmd := strings.Join(fields[2:], " ")
				jobs = append(jobs, CronJob{
					ID:         makeJobID(expr, cmd),
					Expression: expr,
					Command:    cmd,
					Enabled:    enabled,
					Comment:    pendingComment,
					User:       u,
					Source:     source,
				})
			}
			pendingComment = ""
			continue
		}

		// Standard 5-field format: min hour dom mon dow user cmd...
		if len(fields) >= 7 && isCronSchedule(fields[:5]) {
			u := fields[5]
			if targetUser == "" || targetUser == "all" || strings.EqualFold(u, targetUser) {
				expr := strings.Join(fields[:5], " ")
				cmd := strings.Join(fields[6:], " ")
				jobs = append(jobs, CronJob{
					ID:         makeJobID(expr, cmd),
					Expression: expr,
					Command:    cmd,
					Enabled:    enabled,
					Comment:    pendingComment,
					User:       u,
					Source:     source,
				})
			}
			pendingComment = ""
			continue
		}

		pendingComment = ""
	}

	return jobs
}

func isManagedNoise(line string) bool {
	switch line {
	case "# Managed by TameStack - Do not edit manually",
		"# Preserved by TameStack",
		"SHELL=/bin/bash",
		"PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin":
		return true
	default:
		return false
	}
}
