package packages

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// validPackageNameRegex matches standard Debian/Ubuntu package names.
	// Starts with alphanumeric, followed by alphanumeric, +, ., or -
	validPackageNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*$`)

	// instRegex matches:
	// "Inst package [old-ver] (new-ver distro [arch])"
	// "Inst package (new-ver distro [arch])"
	// "Inst package [old-ver] (new-ver)"
	instRegex = regexp.MustCompile(`^Inst\s+(\S+)(?:\s+\[([^\]]+)\])?\s+\((\S+)(?:\s+([^\[\)\s]+))?(?:\s+\[([^\]]+)\])?\)`)

	// remvRegex matches "Remv package [ver]" or "Remv package"
	remvRegex = regexp.MustCompile(`^Remv\s+(\S+)(?:\s+\[([^\]]+)\])?`)

	// confRegex matches "Conf package (ver ...)"
	confRegex = regexp.MustCompile(`^Conf\s+(\S+)\s+\((\S+)(?:[^\)]*)\)`)

	// summaryRegex matches "X upgraded, Y newly installed, Z to remove and W not upgraded."
	summaryRegex = regexp.MustCompile(`(\d+)\s+upgraded,\s+(\d+)\s+newly installed,\s+(\d+)\s+to remove and\s+(\d+)\s+not upgraded`)
)

// ValidatePackageNames checks each name in packages against strict debian package naming policy.
func ValidatePackageNames(packages []string) error {
	if len(packages) == 0 {
		return fmt.Errorf("список пакетов не может быть пустым")
	}
	for _, p := range packages {
		pkg := strings.TrimSpace(p)
		if pkg == "" {
			return fmt.Errorf("имя пакета не может быть пустым")
		}
		if len(pkg) > 128 {
			return fmt.Errorf("имя пакета слишком длинное: %s", pkg)
		}
		if !validPackageNameRegex.MatchString(pkg) {
			return fmt.Errorf("недопустимое имя пакета (содержит спецсимволы или неверный формат): %s", pkg)
		}
	}
	return nil
}

// ParseAptSimulation extracts package changes and summary counts from `apt-get -s` output.
func ParseAptSimulation(output string, command string) (*AptSimulationResult, error) {
	result := &AptSimulationResult{
		Success:   true,
		Command:   command,
		Changes:   make([]PackageChange, 0),
		RawOutput: output,
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if sm := summaryRegex.FindStringSubmatch(line); len(sm) == 5 {
			result.UpgradedCount, _ = strconv.Atoi(sm[1])
			result.InstalledCount, _ = strconv.Atoi(sm[2])
			result.RemovedCount, _ = strconv.Atoi(sm[3])
			result.HeldCount, _ = strconv.Atoi(sm[4])
			continue
		}

		if m := instRegex.FindStringSubmatch(line); len(m) > 1 {
			pkgName := m[1]
			oldVer := m[2]
			var newVer string
			var arch string

			if len(m) > 3 && m[3] != "" {
				newVer = m[3]
			}
			if len(m) > 5 && m[5] != "" {
				arch = m[5]
			}

			action := "install"
			if oldVer != "" {
				action = "upgrade"
			}

			result.Changes = append(result.Changes, PackageChange{
				Name:       pkgName,
				Action:     action,
				OldVersion: oldVer,
				NewVersion: newVer,
				Arch:       arch,
			})
			continue
		}

		if m := remvRegex.FindStringSubmatch(line); len(m) > 1 {
			pkgName := m[1]
			ver := ""
			if len(m) > 2 {
				ver = m[2]
			}
			result.Changes = append(result.Changes, PackageChange{
				Name:       pkgName,
				Action:     "remove",
				OldVersion: ver,
			})
			continue
		}

		if m := confRegex.FindStringSubmatch(line); len(m) > 1 {
			pkgName := m[1]
			ver := ""
			if len(m) > 2 {
				ver = m[2]
			}
			// Only append if not already in changes from Inst.
			alreadyPresent := false
			for _, c := range result.Changes {
				if c.Name == pkgName {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				result.Changes = append(result.Changes, PackageChange{
					Name:       pkgName,
					Action:     "configure",
					NewVersion: ver,
				})
			}
			continue
		}
	}

	return result, scanner.Err()
}

// ParseDpkgList parses the tab-separated output of:
// dpkg-query -W -f='${Package}\t${Version}\t${Architecture}\t${Status}\t${binary:Summary}\n'
func ParseDpkgList(output string) ([]PackageItem, error) {
	var items []PackageItem
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		ver := strings.TrimSpace(parts[1])
		arch := strings.TrimSpace(parts[2])
		status := strings.TrimSpace(parts[3])
		desc := ""
		if len(parts) >= 5 {
			desc = strings.TrimSpace(parts[4])
		}

		// Only include installed packages.
		if !strings.Contains(status, "installed") && !strings.Contains(status, "install ok installed") {
			continue
		}

		items = append(items, PackageItem{
			Name:         name,
			Version:      ver,
			Architecture: arch,
			Status:       "installed",
			Description:  desc,
		})
	}

	return items, scanner.Err()
}
