package inventory

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

var (
	reNginxVersion   = regexp.MustCompile(`nginx version:\s*nginx/([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	reNginxPrefix    = regexp.MustCompile(`--prefix=([^\s]+)`)
	reNginxConfPath  = regexp.MustCompile(`--conf-path=([^\s]+)`)
	reApacheVersion  = regexp.MustCompile(`Server version:\s*Apache/([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	reApacheRoot     = regexp.MustCompile(`-D\s+HTTPD_ROOT="([^"]+)"`)
	reApacheConf     = regexp.MustCompile(`-D\s+SERVER_CONFIG_FILE="([^"]+)"`)
	reApacheModPHP   = regexp.MustCompile(`(?:php(\d+\.\d+)_module|mod_php)`)
	reNoPubkey       = regexp.MustCompile(`NO_PUBKEY\s+([A-Fa-f0-9]+)`)
	reAptDebLine     = regexp.MustCompile(`^(deb(?:-src)?)\s+(?:\[[^\]]*\]\s+)?(https?://[^\s]+)\s+([^\s]+)\s+(.+)$`)
	reProxyPass      = regexp.MustCompile(`proxy_pass\s+https?://(127\.0\.0\.1|localhost):([0-9]+)`)
	reFastcgiPass    = regexp.MustCompile(`fastcgi_pass\s+(?:unix:)?([^\s;]+);`)
	reSetHandlerPHP  = regexp.MustCompile(`SetHandler\s+"?proxy:fcgi://(127\.0\.0\.1:[0-9]+|unix:[^\s\|]+)`)
	rePhpVersionNum  = regexp.MustCompile(`(\d+\.\d+)`)
	reStrictPhpVer   = regexp.MustCompile(`^([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
)

// ParseOSInfo parses /etc/os-release, uname, and user identity commands into OSInfo.
func ParseOSInfo(osReleaseOut, unameOut, userDetailOut string) OSInfo {
	info := OSInfo{
		InitSystem:     "systemd",
		PackageManager: "apt",
	}

	for _, line := range strings.Split(osReleaseOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), `"`)

		switch k {
		case "NAME":
			info.Distro = v
		case "VERSION_ID":
			info.Version = v
		case "VERSION_CODENAME", "UBUNTU_CODENAME":
			if info.Codename == "" || k == "VERSION_CODENAME" {
				info.Codename = v
			}
		case "ID":
			if info.Distro == "" {
				info.Distro = v
			}
			if v == "rhel" || v == "centos" || v == "almalinux" || v == "rocky" || v == "fedora" {
				info.PackageManager = "dnf"
			}
		}
	}

	var unameTokens []string
	for _, line := range strings.Split(unameOut, "\n") {
		for _, field := range strings.Fields(line) {
			unameTokens = append(unameTokens, field)
		}
	}
	if len(unameTokens) >= 2 {
		info.Kernel = unameTokens[0]
		info.Architecture = unameTokens[1]
	} else if len(unameTokens) == 1 {
		info.Kernel = unameTokens[0]
	}

	lines := strings.Split(userDetailOut, "\n")
	if len(lines) >= 3 {
		uid, _ := strconv.Atoi(strings.TrimSpace(lines[0]))
		username := strings.TrimSpace(lines[1])
		groups := strings.Fields(strings.TrimSpace(lines[2]))

		hasSudo := uid == 0
		for _, g := range groups {
			if g == "sudo" || g == "wheel" || g == "root" || g == "admin" {
				hasSudo = true
				break
			}
		}

		info.CurrentUser = CurrentUserDetail{
			Username: username,
			UID:      uid,
			GID:      uid,
			Groups:   groups,
			HasSudo:  hasSudo,
		}
	}

	return info
}

// ParseNginxVOutput parses the output of `nginx -V`.
func ParseNginxVOutput(output, binPath string) (version, prefix, confPath string) {
	if m := reNginxVersion.FindStringSubmatch(output); len(m) > 1 {
		version = m[1]
	}
	if m := reNginxPrefix.FindStringSubmatch(output); len(m) > 1 {
		prefix = m[1]
	} else {
		prefix = "/etc/nginx"
	}
	if m := reNginxConfPath.FindStringSubmatch(output); len(m) > 1 {
		confPath = m[1]
	} else {
		confPath = path.Join(prefix, "conf", "nginx.conf")
	}
	return version, prefix, confPath
}

// ParseApacheVOutput parses the output of `apache2 -V` or `httpd -V`.
func ParseApacheVOutput(output, binPath string) (version, root, confPath string) {
	if m := reApacheVersion.FindStringSubmatch(output); len(m) > 1 {
		version = m[1]
	}
	if m := reApacheRoot.FindStringSubmatch(output); len(m) > 1 {
		root = m[1]
	} else {
		root = "/etc/apache2"
	}
	if m := reApacheConf.FindStringSubmatch(output); len(m) > 1 {
		if strings.HasPrefix(m[1], "/") {
			confPath = m[1]
		} else {
			confPath = path.Join(root, m[1])
		}
	} else {
		confPath = path.Join(root, "apache2.conf")
	}
	return version, root, confPath
}

// DiscoverWebServers finds all installed or running web server instances (Nginx, Apache, Caddy, etc.).
func DiscoverWebServers(ctx context.Context, client *ssh.SSHClient, sockets []ListeningSocket, runningProcesses []ProcessInfo) ([]WebServerInstance, error) {
	execWithTimeout := func(cmd string, timeout time.Duration) (string, int) {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		out, _, code, _ := client.NewCommand(cmd).WithSudo().Exec(cmdCtx)
		return string(out), code
	}

	// 1. Gather all potential binaries from running processes and known search directories.
	candidateBinaries := make(map[string]bool)

	// Check running processes
	for _, proc := range runningProcesses {
		procName := strings.ToLower(proc.Name)
		if strings.Contains(procName, "nginx") || strings.Contains(procName, "apache") || strings.Contains(procName, "httpd") || strings.Contains(procName, "caddy") {
			if proc.Exe != "" && strings.HasPrefix(proc.Exe, "/") {
				candidateBinaries[proc.Exe] = true
			}
		}
	}

	// Probe filesystem for binaries
	probeBinScript := `
for b in /usr/sbin/nginx /usr/bin/nginx /usr/local/nginx/sbin/nginx /usr/local/sbin/nginx /usr/local/bin/nginx /opt/*/sbin/nginx /opt/*/bin/nginx /snap/bin/nginx /usr/sbin/apache2 /usr/sbin/httpd /usr/bin/apache2 /usr/bin/httpd /usr/local/apache2/bin/httpd /opt/*/bin/httpd /usr/bin/caddy; do
    if [ -x "$b" ]; then
        echo "BIN:$b"
    fi
done
`
	binOut, _ := execWithTimeout(probeBinScript, 4*time.Second)
	for _, line := range strings.Split(binOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "BIN:") {
			candidateBinaries[strings.TrimPrefix(line, "BIN:")] = true
		}
	}

	var instances []WebServerInstance

	for bin := range candidateBinaries {
		if strings.Contains(bin, "nginx") {
			inst := probeNginxInstance(bin, runningProcesses, sockets, execWithTimeout)
			instances = append(instances, inst)
		} else if strings.Contains(bin, "apache") || strings.Contains(bin, "httpd") {
			inst := probeApacheInstance(bin, runningProcesses, sockets, execWithTimeout)
			instances = append(instances, inst)
		}
	}

	// Sort instances for deterministic output
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].BinaryPath < instances[j].BinaryPath
	})

	return instances, nil
}

func probeNginxInstance(bin string, procs []ProcessInfo, sockets []ListeningSocket, execFn func(string, time.Duration) (string, int)) WebServerInstance {
	inst := WebServerInstance{
		Engine:       "nginx",
		BinaryPath:   bin,
		Status:       "inactive",
		IsManageable: true,
	}

	// ID and Install Type
	if strings.HasPrefix(bin, "/opt/") || strings.HasPrefix(bin, "/usr/local/") {
		inst.InstallType = "source"
		inst.ID = fmt.Sprintf("nginx-%s", sanitizeID(bin))
		cleanDir := strings.ReplaceAll(path.Dir(path.Dir(bin)), "\\", "/")
		inst.Name = fmt.Sprintf("Nginx (%s)", cleanDir)
	} else if strings.HasPrefix(bin, "/snap/") {
		inst.InstallType = "docker"
		inst.ID = fmt.Sprintf("nginx-%s", sanitizeID(bin))
		inst.Name = "Nginx (Snap)"
	} else {
		inst.InstallType = "apt"
		inst.ID = "nginx-system"
		inst.Name = "Nginx (System Package)"
	}

	// Inspect version and prefix with -V
	vOut, _ := execFn(fmt.Sprintf("%s -V 2>&1", ssh.ShellEscape(bin)), 3*time.Second)
	version, prefix, confPath := ParseNginxVOutput(vOut, bin)
	inst.Version = version
	inst.PrefixDir = prefix
	inst.ConfigPath = confPath

	// Check running PIDs and status
	for _, p := range procs {
		if p.Exe == bin || strings.Contains(p.Cmdline, bin) {
			inst.PIDs = append(inst.PIDs, p.PID)
			inst.Status = "active"
		}
	}

	if inst.Status == "inactive" {
		unitExec, _ := execFn("systemctl cat nginx.service 2>/dev/null | grep -E '^ExecStart=' || true", 2*time.Second)
		if strings.Contains(unitExec, bin) {
			activeCheck, _ := execFn("systemctl is-active nginx.service 2>/dev/null || true", 2*time.Second)
			if strings.TrimSpace(activeCheck) == "active" {
				inst.Status = "active"
			}
		}
	}

	// Check listening ports & sockets (only for the active instance)
	if inst.Status == "active" {
		for _, s := range sockets {
			if strings.Contains(strings.ToLower(s.Process), "nginx") {
				if s.Port > 0 && !containsInt(inst.ListeningPorts, s.Port) {
					inst.ListeningPorts = append(inst.ListeningPorts, s.Port)
				}
			}
		}
	}

	// Formulate test & reload commands
	inst.TestCommand = fmt.Sprintf("%s -t -c %s -p %s", ssh.ShellEscape(bin), ssh.ShellEscape(confPath), ssh.ShellEscape(prefix))

	// Check systemd unit
	unitOut, _ := execFn("systemctl list-units 'nginx*' --plain --no-legend --no-pager 2>/dev/null", 2*time.Second)
	if strings.Contains(unitOut, "nginx.service") {
		inst.SystemdUnit = "nginx.service"
		inst.ReloadCommand = "systemctl reload nginx"
	} else {
		inst.ReloadCommand = fmt.Sprintf("%s -s reload -c %s -p %s", ssh.ShellEscape(bin), ssh.ShellEscape(confPath), ssh.ShellEscape(prefix))
	}

	// Test config execution
	testOut, code := execFn(inst.TestCommand+" 2>&1", 3*time.Second)
	if code != 0 || !strings.Contains(testOut, "syntax is ok") {
		inst.Limitations = append(inst.Limitations, fmt.Sprintf("Config syntax test warning: %s", strings.TrimSpace(testOut)))
	}

	return inst
}

func probeApacheInstance(bin string, procs []ProcessInfo, sockets []ListeningSocket, execFn func(string, time.Duration) (string, int)) WebServerInstance {
	inst := WebServerInstance{
		Engine:       "apache2",
		BinaryPath:   bin,
		Status:       "inactive",
		IsManageable: true,
	}

	if strings.HasPrefix(bin, "/opt/") || strings.HasPrefix(bin, "/usr/local/") {
		inst.InstallType = "source"
		inst.ID = fmt.Sprintf("apache-%s", sanitizeID(bin))
		cleanDir := strings.ReplaceAll(path.Dir(path.Dir(bin)), "\\", "/")
		inst.Name = fmt.Sprintf("Apache HTTP Server (%s)", cleanDir)
	} else {
		inst.InstallType = "apt"
		inst.ID = "apache2-system"
		inst.Name = "Apache HTTP Server (System Package)"
	}

	// Inspect version and paths with -V
	vOut, _ := execFn(fmt.Sprintf("%s -V 2>&1", ssh.ShellEscape(bin)), 3*time.Second)
	version, root, confPath := ParseApacheVOutput(vOut, bin)
	inst.Version = version
	inst.PrefixDir = root
	inst.ConfigPath = confPath

	// Inspect modules with -M
	mOut, _ := execFn(fmt.Sprintf("%s -M 2>&1", ssh.ShellEscape(bin)), 3*time.Second)
	for _, line := range strings.Split(mOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "(shared)") || strings.HasSuffix(line, "(static)") {
			mod := strings.Fields(line)[0]
			inst.Modules = append(inst.Modules, mod)
		}
	}

	// Check running PIDs
	for _, p := range procs {
		if p.Exe == bin || strings.Contains(p.Cmdline, bin) || strings.Contains(p.Name, "apache2") || strings.Contains(p.Name, "httpd") {
			inst.PIDs = append(inst.PIDs, p.PID)
			inst.Status = "active"
		}
	}

	// Check listening ports & sockets (only for active instance)
	if inst.Status == "active" {
		for _, s := range sockets {
			procLower := strings.ToLower(s.Process)
			if strings.Contains(procLower, "apache2") || strings.Contains(procLower, "httpd") {
				if s.Port > 0 && !containsInt(inst.ListeningPorts, s.Port) {
					inst.ListeningPorts = append(inst.ListeningPorts, s.Port)
				}
			}
		}
	}

	ctlBin := "apache2ctl"
	if strings.Contains(bin, "httpd") {
		ctlBin = "apachectl"
	}
	inst.TestCommand = fmt.Sprintf("%s configtest 2>/dev/null || %s -t 2>/dev/null", ctlBin, ssh.ShellEscape(bin))

	unitOut, _ := execFn("systemctl list-units 'apache2*' 'httpd*' --plain --no-legend --no-pager 2>/dev/null", 2*time.Second)
	if strings.Contains(unitOut, "apache2.service") {
		inst.SystemdUnit = "apache2.service"
		inst.ReloadCommand = "systemctl reload apache2"
	} else if strings.Contains(unitOut, "httpd.service") {
		inst.SystemdUnit = "httpd.service"
		inst.ReloadCommand = "systemctl reload httpd"
	} else {
		inst.ReloadCommand = fmt.Sprintf("%s -k graceful", ctlBin)
	}

	return inst
}

// DiscoverPHPRuntimes gathers all installed PHP versions, active FPM pools/sockets, Apache mod_php, and detects orphaned units.
func DiscoverPHPRuntimes(ctx context.Context, client *ssh.SSHClient, sockets []ListeningSocket) ([]PHPRuntimeInstance, error) {
	execWithTimeout := func(cmd string, timeout time.Duration) (string, int) {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		out, _, code, _ := client.NewCommand(cmd).WithSudo().Exec(cmdCtx)
		return string(out), code
	}

	var runtimes []PHPRuntimeInstance

	// 1. Scan for PHP binaries (CLI and CGI)
	binScript := `
for b in $(which -a php php7.2 php7.3 php7.4 php8.0 php8.1 php8.2 php8.3 php8.4 2>/dev/null) /usr/bin/php /usr/bin/php[0-9]* /usr/bin/php-[0-9]* /usr/local/bin/php /usr/local/bin/php[0-9]* /opt/*/bin/php /opt/*/bin/php[0-9]*; do
    [ -x "$b" ] || continue
    case "$b" in
        *phpize*|*phpdbg*|*php-config*|*phar*|*php-cgi*) continue ;;
    esac
    REAL=$(readlink -f "$b" 2>/dev/null || echo "$b")
    V=$("$b" -n -r 'echo PHP_VERSION;' 2>/dev/null || true)
    if [ -n "$V" ]; then
        echo "PHP_BIN:$b:$REAL:$V"
    fi
done

for b in /usr/bin/php-cgi* /usr/local/bin/php-cgi* /opt/*/bin/php-cgi*; do
    [ -x "$b" ] || continue
    REAL=$(readlink -f "$b" 2>/dev/null || echo "$b")
    V=$("$b" -v 2>/dev/null | head -n1 | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' | head -n1 || true)
    if [ -n "$V" ]; then
        echo "PHP_CGI:$b:$REAL:$V"
    fi
done
`
	binOut, _ := execWithTimeout(binScript, 5*time.Second)
	seenRealPaths := make(map[string]bool)

	for _, line := range strings.Split(binOut, "\n") {
		line = strings.TrimSpace(line)
		isCGI := strings.HasPrefix(line, "PHP_CGI:")
		if !strings.HasPrefix(line, "PHP_BIN:") && !isCGI {
			continue
		}
		prefix := "PHP_BIN:"
		if isCGI {
			prefix = "PHP_CGI:"
		}
		parts := strings.Split(strings.TrimPrefix(line, prefix), ":")
		if len(parts) >= 3 {
			binPath := parts[0]
			realPath := parts[1]
			ver := parts[2]

			if !reStrictPhpVer.MatchString(ver) {
				continue
			}
			m := reStrictPhpVer.FindStringSubmatch(ver)
			cleanVer := m[1]
			majorMinor := extractMajorMinor(cleanVer)

			dedupKey := fmt.Sprintf("%s|%v", realPath, isCGI)
			if seenRealPaths[dedupKey] {
				continue
			}
			seenRealPaths[dedupKey] = true

			kind := "cli"
			if isCGI {
				kind = "cgi"
			}

			canonicalBin := binPath
			if strings.HasPrefix(binPath, "/bin/") && strings.HasPrefix(realPath, "/usr/bin/") {
				canonicalBin = realPath
			}

			rt := PHPRuntimeInstance{
				ID:           fmt.Sprintf("php-%s-%s-%s", kind, majorMinor, sanitizeID(canonicalBin)),
				Version:      cleanVer,
				MajorMinor:   majorMinor,
				Kind:         kind,
				BinaryPath:   canonicalBin,
				Status:       "active",
				IsManageable: true,
			}
			runtimes = append(runtimes, rt)
		}
	}

	// 2. Scan for PHP-FPM services and sockets
	fpmScript := `
for s in $(systemctl list-unit-files 'php*-fpm.service' 'php-fpm.service' --no-legend 2>/dev/null | awk '{print $1}'); do
    LOAD=$(systemctl is-active "$s" 2>/dev/null || echo "inactive")
    EXEC=$(systemctl cat "$s" 2>/dev/null | grep -E '^ExecStart=' | head -n1 | awk '{print $1}' | cut -d= -f2 || true)
    BIN_EXISTS="no"
    if [ -n "$EXEC" ] && [ -x "$EXEC" ]; then
        BIN_EXISTS="yes"
    fi
    echo "FPM_UNIT:$s:$LOAD:$BIN_EXISTS:$EXEC"
done
`
	fpmOut, _ := execWithTimeout(fpmScript, 4*time.Second)
	for _, line := range strings.Split(fpmOut, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "FPM_UNIT:") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(line, "FPM_UNIT:"), ":")
		if len(parts) >= 4 {
			unit := parts[0]
			loadState := parts[1]
			binExists := parts[2] == "yes"
			execPath := parts[3]
			majorMinor := extractMajorMinor(unit)

			if !binExists {
				// ORPHANED UNIT!
				rt := PHPRuntimeInstance{
					ID:           fmt.Sprintf("php-orphan-%s", sanitizeID(unit)),
					Version:      majorMinor,
					MajorMinor:   majorMinor,
					Kind:         "orphaned_unit",
					Status:       "failed",
					IsOrphaned:   true,
					IsManageable: false,
					SystemdUnit:  unit,
					Limitations:  []string{fmt.Sprintf("Leftover orphaned systemd service: executable %s does not exist on disk", execPath)},
				}
				runtimes = append(runtimes, rt)
				continue
			}

			// Valid FPM service
			status := "inactive"
			if loadState == "active" {
				status = "active"
			} else if loadState == "failed" {
				status = "failed"
			}

			rt := PHPRuntimeInstance{
				ID:           fmt.Sprintf("php-fpm-%s", majorMinor),
				Version:      majorMinor,
				MajorMinor:   majorMinor,
				Kind:         "fpm",
				BinaryPath:   execPath,
				Status:       status,
				IsManageable: true,
				SystemdUnit:  unit,
			}

			// Scan sockets matching this version
			sockScript := fmt.Sprintf("ls -1 /run/php/php%s-fpm.sock /var/run/php/php%s-fpm.sock /run/php-fpm/www.sock 2>/dev/null || true", majorMinor, majorMinor)
			sockOut, _ := execWithTimeout(sockScript, 2*time.Second)
			for _, sockLine := range strings.Split(sockOut, "\n") {
				sockLine = strings.TrimSpace(sockLine)
				if sockLine != "" {
					rt.Sockets = append(rt.Sockets, sockLine)
				}
			}

			runtimes = append(runtimes, rt)
		}
	}

	// 3. Scan for Apache mod_php
	modPHPScript := `
ls -1 /etc/apache2/mods-enabled/php*.load /etc/httpd/conf.modules.d/*php*.conf 2>/dev/null || true
`
	modPHPOut, _ := execWithTimeout(modPHPScript, 2*time.Second)
	for _, line := range strings.Split(modPHPOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := rePhpVersionNum.FindStringSubmatch(line); len(m) > 1 {
			ver := m[1]
			rt := PHPRuntimeInstance{
				ID:           fmt.Sprintf("php-mod-apache-%s", ver),
				Version:      ver,
				MajorMinor:   ver,
				Kind:         "mod_php",
				Status:       "active",
				IsManageable: false, // mod_php lifecycle is bound to Apache
				Limitations:  []string{"PHP is embedded in Apache worker process via mod_php; restarts follow Apache"},
			}
			runtimes = append(runtimes, rt)
			break
		}
	}

	return runtimes, nil
}

// DiscoverAPTDiagnostics checks APT repositories for codename mismatches, missing GPG keys, and duplicate sources.
func DiscoverAPTDiagnostics(ctx context.Context, client *ssh.SSHClient, hostCodename string) (APTDiagnostics, error) {
	execWithTimeout := func(cmd string, timeout time.Duration) (string, int) {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		out, _, code, _ := client.NewCommand(cmd).WithSudo().Exec(cmdCtx)
		return string(out), code
	}

	diag := APTDiagnostics{
		Summary: "APT sources analyzed",
	}

	script := `
cat /etc/apt/sources.list /etc/apt/sources.list.d/*.list 2>/dev/null || true
echo "===APT_KEYS==="
apt-key list 2>/dev/null || true
echo "===APT_ERRORS==="
apt-get -s update 2>&1 | grep -E 'NO_PUBKEY|W:|E:' || true
`
	out, _ := execWithTimeout(script, 6*time.Second)

	sections := strings.Split(out, "===")
	sourcesPart := sections[0]
	var errorsPart string
	if len(sections) > 2 {
		errorsPart = sections[2]
	}

	// Parse sources
	seenSources := make(map[string]string)
	for _, line := range strings.Split(sourcesPart, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := reAptDebLine.FindStringSubmatch(line); len(m) > 4 {
			repoURL := m[2]
			repoDist := m[3]
			component := m[4]

			normKey := fmt.Sprintf("%s|%s|%s", repoURL, repoDist, component)
			if _, exists := seenSources[normKey]; exists {
				diag.DuplicateSources = append(diag.DuplicateSources, line)
				continue
			}
			seenSources[normKey] = line

			item := RepoDiagnostic{
				URL:              repoURL,
				Codename:         repoDist,
				Component:        component,
				ExpectedCodename: hostCodename,
				Status:           "ok",
			}

			// Detect codename mismatch
			if hostCodename != "" && !IsCodenameCompatible(repoDist, hostCodename) {
				item.IsMismatch = true
				item.Status = "warning"
				item.Message = fmt.Sprintf("Repository uses codename '%s', but host is '%s'", repoDist, hostCodename)
				diag.CodenameMismatches = append(diag.CodenameMismatches, repoURL)
			}

			diag.Sources = append(diag.Sources, item)
		}
	}

	// Parse GPG errors
	for _, line := range strings.Split(errorsPart, "\n") {
		if m := reNoPubkey.FindStringSubmatch(line); len(m) > 1 {
			key := m[1]
			if !containsString(diag.MissingGPGKeys, key) {
				diag.MissingGPGKeys = append(diag.MissingGPGKeys, key)
			}
		}
	}

	// Associate missing keys with sources
	for i := range diag.Sources {
		for _, key := range diag.MissingGPGKeys {
			diag.Sources[i].MissingKeys = append(diag.Sources[i].MissingKeys, key)
			diag.Sources[i].Status = "error"
			diag.Sources[i].Message = fmt.Sprintf("Missing GPG public key: NO_PUBKEY %s", key)
			diag.HasErrors = true
		}
	}

	if len(diag.MissingGPGKeys) > 0 || len(diag.CodenameMismatches) > 0 {
		diag.HasErrors = true
		diag.Summary = fmt.Sprintf("Found %d GPG errors and %d distribution mismatches", len(diag.MissingGPGKeys), len(diag.CodenameMismatches))
	} else {
		diag.Summary = "All APT sources match the current distribution release"
	}

	return diag, nil
}

// DiscoverTopology builds the verified topology chains connecting Domains to WebServers, Backends, and PHP Runtimes.
func DiscoverTopology(
	ctx context.Context,
	client *ssh.SSHClient,
	sites []*db.SiteRecord,
	webServers []WebServerInstance,
	phpRuntimes []PHPRuntimeInstance,
	certs []*db.SSLCertificateRecord,
) []SiteTopologyChain {
	execWithTimeout := func(cmd string, timeout time.Duration) string {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		out, _, _, _ := client.NewCommand(cmd).WithSudo().Exec(cmdCtx)
		return string(out)
	}

	var chains []SiteTopologyChain

	// Create lookup maps
	var primaryNginx *WebServerInstance
	var primaryApache *WebServerInstance
	for i := range webServers {
		ws := &webServers[i]
		if ws.Engine == "nginx" && primaryNginx == nil {
			primaryNginx = ws
		} else if (ws.Engine == "apache2" || ws.Engine == "httpd") && primaryApache == nil {
			primaryApache = ws
		}
	}

	fpmBySocket := make(map[string]*PHPRuntimeInstance)
	var modPHPRuntime *PHPRuntimeInstance
	for i := range phpRuntimes {
		rt := &phpRuntimes[i]
		if rt.Kind == "mod_php" {
			modPHPRuntime = rt
		}
		for _, s := range rt.Sockets {
			fpmBySocket[s] = rt
		}
	}

	certByDomain := make(map[string]string)
	for _, c := range certs {
		if c.PrimaryDomain != "" {
			certByDomain[strings.ToLower(c.PrimaryDomain)] = c.CertPath
		}
		for _, san := range c.SanDomains {
			certByDomain[strings.ToLower(san)] = c.CertPath
		}
	}

	for _, s := range sites {
		domainLower := strings.ToLower(s.Domain)
		chain := SiteTopologyChain{
			Domain:      s.Domain,
			VhostPath:   s.VhostPath,
			SSLEnabled:  s.SSLEnabled,
			SSLCertPath: certByDomain[domainLower],
			Confidence:  "unproven",
		}

		// Find vhost content if available
		vhostContent := ""
		if s.VhostPath != "" {
			vhostContent = execWithTimeout(fmt.Sprintf("head -n 200 %s 2>/dev/null", ssh.ShellEscape(s.VhostPath)), 2*time.Second)
		}

		// Check if front is Nginx
		if primaryNginx != nil {
			chain.WebServerInstanceID = primaryNginx.ID
			chain.WebServerEngine = primaryNginx.Engine
			chain.ListenPorts = primaryNginx.ListeningPorts

			// Check if Nginx proxies to Apache backend (e.g. proxy_pass http://127.0.0.1:8080)
			if m := reProxyPass.FindStringSubmatch(vhostContent); len(m) > 2 {
				proxyPort, _ := strconv.Atoi(m[2])
				chain.BackendType = "reverse_proxy"
				chain.BackendTarget = fmt.Sprintf("http://%s:%d", m[1], proxyPort)

				if primaryApache != nil && containsInt(primaryApache.ListeningPorts, proxyPort) {
					// Follow through to Apache
					if modPHPRuntime != nil {
						chain.PHPVersion = modPHPRuntime.Version
						chain.PHPRuntimeID = modPHPRuntime.ID
						chain.BackendType = "mod_php"
						chain.Confidence = "proven"
					} else {
						chain.Confidence = "inferred"
					}
				}
			} else if m := reFastcgiPass.FindStringSubmatch(vhostContent); len(m) > 1 {
				sock := m[1]
				chain.BackendType = "fastcgi_fpm"
				chain.BackendTarget = sock
				if rt, ok := fpmBySocket[sock]; ok {
					chain.PHPVersion = rt.Version
					chain.PHPRuntimeID = rt.ID
					chain.Confidence = "proven"
				} else {
					chain.Confidence = "unproven"
					chain.Warnings = append(chain.Warnings, fmt.Sprintf("FastCGI socket %s does not match any running PHP-FPM service", sock))
				}
			} else {
				chain.BackendType = "static"
				chain.Confidence = "proven"
			}
		} else if primaryApache != nil {
			chain.WebServerInstanceID = primaryApache.ID
			chain.WebServerEngine = primaryApache.Engine
			chain.ListenPorts = primaryApache.ListeningPorts
			if modPHPRuntime != nil {
				chain.BackendType = "mod_php"
				chain.PHPVersion = modPHPRuntime.Version
				chain.PHPRuntimeID = modPHPRuntime.ID
				chain.Confidence = "proven"
			} else {
				chain.BackendType = "static"
				chain.Confidence = "inferred"
			}
		}

		// If SSL enabled but no cert found on disk, downgrade confidence
		if chain.SSLEnabled && chain.SSLCertPath == "" {
			chain.Confidence = "unproven"
			chain.Warnings = append(chain.Warnings, "SSL enabled in site settings, but certificate not found on disk")
		}

		chains = append(chains, chain)
	}

	return chains
}

// ParseProcesses parses `ps -eo pid,ppid,user,args` output into []ProcessInfo.
func ParseProcesses(psOut string) []ProcessInfo {
	var procs []ProcessInfo
	for _, line := range strings.Split(psOut, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "PID") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 4 {
			pid, _ := strconv.Atoi(fields[0])
			ppid, _ := strconv.Atoi(fields[1])
			user := fields[2]
			cmdline := strings.Join(fields[3:], " ")
			exe := fields[3]
			name := path.Base(exe)

			procs = append(procs, ProcessInfo{
				PID:     pid,
				PPID:    ppid,
				User:    user,
				Name:    name,
				Exe:     exe,
				Cmdline: cmdline,
			})
		}
	}
	return procs
}

// PopulateDatabases converts dbFacts and listening sockets into DatabaseInstance structs.
func PopulateDatabases(dbFacts []DatabaseFacts, sockets []ListeningSocket) []DatabaseInstance {
	var instances []DatabaseInstance
	for _, f := range dbFacts {
		inst := DatabaseInstance{
			Engine:      f.Engine,
			Version:     f.Version,
			Status:      "active",
			InstallType: "apt",
		}
		// Match ports based on engine
		var targetPort int
		switch strings.ToLower(f.Engine) {
		case "mysql", "mariadb":
			targetPort = 3306
		case "postgresql", "postgres":
			targetPort = 5432
		case "redis":
			targetPort = 6379
		case "mongodb":
			targetPort = 27017
		}
		if targetPort > 0 {
			for _, s := range sockets {
				if s.Port == targetPort {
					inst.Ports = append(inst.Ports, s.Port)
				}
			}
		}
		instances = append(instances, inst)
	}
	return instances
}

// ProcessInfo holds information about a running process from ps/proc.
type ProcessInfo struct {
	PID     int
	PPID    int
	User    string
	Name    string
	Exe     string
	Cmdline string
}

func sanitizeID(path string) string {
	res := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		if r == '/' || r == '.' {
			return '-'
		}
		return -1
	}, strings.ToLower(path))
	return strings.Trim(res, "-")
}

func extractMajorMinor(val string) string {
	if m := rePhpVersionNum.FindStringSubmatch(val); len(m) > 1 {
		return m[1]
	}
	return val
}

func isUniversalRepo(codename string) bool {
	cn := strings.ToLower(codename)
	return cn == "stable" || cn == "testing" || cn == "unstable" || cn == "sid"
}

// IsCodenameCompatible returns true if repoDist matches hostCodename or is a valid pocket/universal repo.
func IsCodenameCompatible(repoDist, hostCodename string) bool {
	if isUniversalRepo(repoDist) {
		return true
	}
	rd := strings.ToLower(repoDist)
	hc := strings.ToLower(hostCodename)
	if rd == hc {
		return true
	}
	if strings.HasPrefix(rd, hc+"-") { // e.g. focal-updates, focal-security, focal-backports
		return true
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func containsString(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
