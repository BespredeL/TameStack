package packages

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

var dnfSummaryRegex = regexp.MustCompile(`(?i)^(Install|Upgrade|Remove|Downgrade|Reinstall)\s+(\d+)\s+Package`)

// dnfSectionAction maps a dnf transaction section header to a PackageChange action.
func dnfSectionAction(header string) string {
	h := strings.ToLower(strings.TrimSuffix(header, ":"))
	switch {
	case strings.HasPrefix(h, "installing"):
		return "install"
	case strings.HasPrefix(h, "upgrading"), strings.HasPrefix(h, "downgrading"), strings.HasPrefix(h, "reinstalling"):
		return "upgrade"
	case strings.HasPrefix(h, "removing"), strings.HasPrefix(h, "erasing"), strings.HasPrefix(h, "obsoleting"):
		return "remove"
	}
	return ""
}

// ParseDnfSimulation extracts package changes and summary counts from the
// transaction table printed by `dnf --assumeno` (install/upgrade/remove).
func ParseDnfSimulation(output string, command string) (*AptSimulationResult, error) {
	result := &AptSimulationResult{
		Success:   true,
		Command:   command,
		Changes:   make([]PackageChange, 0),
		RawOutput: output,
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	action := ""
	inSummary := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			action = ""
			continue
		}
		if strings.HasPrefix(line, "Transaction Summary") {
			inSummary = true
			action = ""
			continue
		}
		if inSummary {
			if m := dnfSummaryRegex.FindStringSubmatch(line); len(m) == 3 {
				n, _ := strconv.Atoi(m[2])
				switch strings.ToLower(m[1]) {
				case "install", "reinstall":
					result.InstalledCount += n
				case "upgrade", "downgrade":
					result.UpgradedCount += n
				case "remove":
					result.RemovedCount += n
				}
			}
			continue
		}
		if strings.HasSuffix(line, ":") {
			if a := dnfSectionAction(line); a != "" {
				action = a
			}
			continue
		}
		if action == "" || strings.HasPrefix(line, "=") || strings.HasPrefix(line, "Package ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		change := PackageChange{Name: fields[0], Arch: fields[1], Action: action}
		if action == "remove" {
			change.OldVersion = fields[2]
		} else {
			change.NewVersion = fields[2]
		}
		result.Changes = append(result.Changes, change)
	}

	return result, scanner.Err()
}

// ParseRpmList parses the tab-separated output of:
// rpm -qa --queryformat '%{NAME}\t%{VERSION}-%{RELEASE}\t%{ARCH}\t%{SUMMARY}\n'
func ParseRpmList(output string) ([]PackageItem, error) {
	var items []PackageItem
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		item := PackageItem{
			Name:         name,
			Version:      strings.TrimSpace(parts[1]),
			Architecture: strings.TrimSpace(parts[2]),
			Status:       "installed",
		}
		if len(parts) >= 4 {
			item.Description = strings.TrimSpace(parts[3])
		}
		items = append(items, item)
	}
	return items, scanner.Err()
}
