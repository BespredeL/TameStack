package osadapter

import (
	"strconv"
	"strings"
)

// parseHardwareOutput parses the ===TAG=== sectioned output of the hardware
// collection script shared by both adapters.
func parseHardwareOutput(out string) *HardwareInfo {
	hw := &HardwareInfo{}
	sections := strings.Split(out, "===")
	for i := 1; i+1 < len(sections); i += 2 {
		tag := sections[i]
		val := strings.TrimSpace(sections[i+1])

		switch tag {
		case "CPU_CORES":
			hw.CPUCores, _ = strconv.Atoi(val)
		case "CPU_MODEL":
			hw.CPUModel = val
		case "RAM_TOTAL":
			hw.RAMTotalBytes, _ = strconv.ParseInt(val, 10, 64)
		case "DISK_INFO":
			parts := strings.Fields(val)
			if len(parts) >= 2 {
				hw.DiskTotalBytes, _ = strconv.ParseInt(parts[0], 10, 64)
				hw.DiskUsedBytes, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		case "UPTIME":
			hw.UptimeSeconds, _ = strconv.ParseInt(val, 10, 64)
		case "SSH_VER":
			hw.SSHVersion = val
		}
	}
	return hw
}

// parseSoftwareOutput parses the ===TAG=== sectioned output of the software
// detection script. skipPHPGlob drops unresolved shell globs (Debian's
// /etc/php/* listing when the directory is empty).
func parseSoftwareOutput(out string, skipPHPGlob bool) *InstalledSoftware {
	sw := &InstalledSoftware{
		WebServers:  []string{},
		PHPVersions: []string{},
		DBVersions:  []string{},
	}

	sections := strings.Split(out, "===")
	for i := 1; i+1 < len(sections); i += 2 {
		tag := sections[i]
		val := strings.TrimSpace(sections[i+1])

		switch tag {
		case "WEBSERVERS":
			sw.WebServers = appendNonEmptyLines(sw.WebServers, val, false)
		case "PHP":
			sw.PHPVersions = appendNonEmptyLines(sw.PHPVersions, val, skipPHPGlob)
		case "DB":
			sw.DBVersions = appendNonEmptyLines(sw.DBVersions, val, false)
		case "DOCKER":
			sw.DockerVersion = strings.TrimSpace(val)
		}
	}
	return sw
}

func appendNonEmptyLines(dst []string, val string, skipGlob bool) []string {
	for _, line := range strings.Split(val, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if skipGlob && strings.Contains(line, "*") {
			continue
		}
		dst = append(dst, line)
	}
	return dst
}

// parseOSRelease interprets `cat /etc/os-release; uname -r; uname -m` output
// and classifies the distro family, defaulting to Debian.
func parseOSRelease(out string) *OSInfo {
	lines := strings.Split(out, "\n")
	kv := make(map[string]string)
	var kernel, arch string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"`)
			kv[key] = val
		}
	}

	// The last lines are uname -r and uname -m.
	if len(lines) >= 3 {
		arch = strings.TrimSpace(lines[len(lines)-1])
		if arch == "" && len(lines) >= 2 {
			arch = strings.TrimSpace(lines[len(lines)-2])
		}
		for i := len(lines) - 1; i >= 0; i-- {
			l := strings.TrimSpace(lines[i])
			if strings.Contains(l, ".") && !strings.Contains(l, "=") {
				kernel = l
				break
			}
		}
	}

	distroID := strings.ToLower(kv["ID"])
	idLike := strings.ToLower(kv["ID_LIKE"])
	prettyName := kv["PRETTY_NAME"]
	if prettyName == "" {
		prettyName = kv["NAME"] + " " + kv["VERSION_ID"]
	}

	osInfo := &OSInfo{
		Distro:     distroID,
		Version:    kv["VERSION_ID"],
		PrettyName: prettyName,
		Kernel:     kernel,
		Arch:       arch,
	}

	switch {
	case distroID == "ubuntu" || distroID == "debian" || strings.Contains(idLike, "debian") || strings.Contains(idLike, "ubuntu"):
		osInfo.Family = FamilyDebian
	case distroID == "almalinux" || distroID == "rocky" || distroID == "rhel" || distroID == "centos" || distroID == "fedora" || strings.Contains(idLike, "rhel") || strings.Contains(idLike, "fedora"):
		osInfo.Family = FamilyRhel
	default:
		osInfo.Family = FamilyDebian
	}
	return osInfo
}
