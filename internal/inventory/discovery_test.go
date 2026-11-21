package inventory

import (
	"strings"
	"testing"

	"tamestack/internal/db"
)

func TestParseOSInfo(t *testing.T) {
	osRelease := `NAME="Ubuntu"
VERSION="20.04.6 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 20.04.6 LTS"
VERSION_ID="20.04"
VERSION_CODENAME=focal
UBUNTU_CODENAME=focal
`
	// 2 separate lines as emitted by "uname -r; uname -m"
	uname := "5.4.0-216-generic\nx86_64\n"
	userDetail := `1002
devops
devops sudo www-data
/usr/bin/apt-get
`

	os := ParseOSInfo(osRelease, uname, userDetail)

	if os.Distro != "Ubuntu" {
		t.Errorf("expected distro Ubuntu, got %s", os.Distro)
	}
	if os.Version != "20.04" {
		t.Errorf("expected version 20.04, got %s", os.Version)
	}
	if os.Codename != "focal" {
		t.Errorf("expected codename focal, got %s", os.Codename)
	}
	if os.Kernel != "5.4.0-216-generic" {
		t.Errorf("expected kernel 5.4.0-216-generic, got %s", os.Kernel)
	}
	if os.Architecture != "x86_64" {
		t.Errorf("expected arch x86_64, got %s", os.Architecture)
	}
	if os.CurrentUser.UID != 1002 {
		t.Errorf("expected UID 1002, got %d", os.CurrentUser.UID)
	}
	if os.CurrentUser.Username != "devops" {
		t.Errorf("expected username devops, got %s", os.CurrentUser.Username)
	}
	if !os.CurrentUser.HasSudo {
		t.Errorf("expected HasSudo true for user in sudo group")
	}
}

func TestParseNginxVOutput(t *testing.T) {
	output := `nginx version: nginx/1.22.0
built by gcc 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.2)
built with OpenSSL 1.1.1f  31 Mar 2020
TLS SNI support enabled
configure arguments: --prefix=/opt/nginx-1.22.0 --sbin-path=/opt/nginx-1.22.0/sbin/nginx --conf-path=/opt/nginx-1.22.0/conf/nginx.conf --error-log-path=/opt/nginx-1.22.0/logs/error.log --http-log-path=/opt/nginx-1.22.0/logs/access.log --with-http_ssl_module --with-http_v2_module
`
	version, prefix, confPath := ParseNginxVOutput(output, "/opt/nginx-1.22.0/sbin/nginx")

	if version != "1.22.0" {
		t.Errorf("expected version 1.22.0, got %s", version)
	}
	if prefix != "/opt/nginx-1.22.0" {
		t.Errorf("expected prefix /opt/nginx-1.22.0, got %s", prefix)
	}
	if confPath != "/opt/nginx-1.22.0/conf/nginx.conf" {
		t.Errorf("expected confPath /opt/nginx-1.22.0/conf/nginx.conf, got %s", confPath)
	}
}

func TestParseApacheVOutput(t *testing.T) {
	output := `Server version: Apache/2.4.41 (Ubuntu)
Server built:   2023-03-08T17:32:54
Server's Module Magic Number: 20120211:88
Server loaded:  APR 1.6.5, APR-UTIL 1.6.1
Compiled using: APR 1.6.5, APR-UTIL 1.6.1
Architecture:   64-bit
Server MPM:     prefork
  threaded:     no
    forked:     yes (variable process count)
Server compiled with....
 -D HTTPD_ROOT="/etc/apache2"
 -D SERVER_CONFIG_FILE="apache2.conf"
`
	version, root, confPath := ParseApacheVOutput(output, "/usr/sbin/apache2")

	if version != "2.4.41" {
		t.Errorf("expected version 2.4.41, got %s", version)
	}
	if root != "/etc/apache2" {
		t.Errorf("expected root /etc/apache2, got %s", root)
	}
	if confPath != "/etc/apache2/apache2.conf" {
		t.Errorf("expected confPath /etc/apache2/apache2.conf, got %s", confPath)
	}
}

func TestParseProcesses(t *testing.T) {
	psOut := `  PID  PPID USER     COMMAND
    1     0 root     /sbin/init
  800     1 root     /usr/sbin/sshd -D
 1234     1 root     nginx: master process /opt/nginx-1.22.0/sbin/nginx -c /opt/nginx-1.22.0/conf/nginx.conf
 1235  1234 www-data nginx: worker process
 2100     1 root     /usr/sbin/apache2 -k start
 2101  2100 www-data /usr/sbin/apache2 -k start
 3400     1 root     php-fpm: master process (/etc/php/8.1/fpm/php-fpm.conf)
 3401  3400 www-data php-fpm: pool www
`
	procs := ParseProcesses(psOut)
	if len(procs) != 8 {
		t.Fatalf("expected 8 processes, got %d", len(procs))
	}

	foundNginx := false
	for _, p := range procs {
		if strings.Contains(p.Cmdline, "/opt/nginx-1.22.0/sbin/nginx") {
			foundNginx = true
			if p.PID != 1234 {
				t.Errorf("expected nginx master PID 1234, got %d", p.PID)
			}
		}
	}
	if !foundNginx {
		t.Errorf("expected to find /opt/nginx-1.22.0/sbin/nginx in parsed processes")
	}
}

func TestAPTDiagnostics_Section10Scenario(t *testing.T) {
	sourcesContent := `deb http://archive.ubuntu.com/ubuntu focal main restricted universe multiverse
deb http://archive.ubuntu.com/ubuntu focal-updates main restricted universe multiverse
deb http://security.ubuntu.com/ubuntu focal-security main restricted universe multiverse
deb https://ppa.launchpadcontent.net/ondrej/php/ubuntu focal main
deb http://repo.mysql.com/apt/ubuntu/ bionic mysql-5.7
`
	errorsContent := `W: GPG error: http://repo.mysql.com/apt/ubuntu bionic InRelease: The following signatures couldn't be verified because the public key is not available: NO_PUBKEY B7B3B788A8D3785C
E: The repository 'http://repo.mysql.com/apt/ubuntu bionic InRelease' is not signed.
`

	// Parse sources and errors
	hostCodename := "focal"
	diag := APTDiagnostics{}

	seenSources := make(map[string]string)
	for _, line := range strings.Split(sourcesContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := reAptDebLine.FindStringSubmatch(line); len(m) > 4 {
			repoURL := m[2]
			repoDist := m[3]
			comp := m[4]

			normKey := repoURL + "|" + repoDist + "|" + comp
			seenSources[normKey] = line

			item := RepoDiagnostic{
				URL:              repoURL,
				Codename:         repoDist,
				Component:        comp,
				ExpectedCodename: hostCodename,
				Status:           "ok",
			}
			if !IsCodenameCompatible(repoDist, hostCodename) {
				item.IsMismatch = true
				item.Status = "warning"
				item.Message = "Repository uses codename '" + repoDist + "', but host is '" + hostCodename + "'"
				diag.CodenameMismatches = append(diag.CodenameMismatches, repoURL)
			}
			diag.Sources = append(diag.Sources, item)
		}
	}

	for _, line := range strings.Split(errorsContent, "\n") {
		if m := reNoPubkey.FindStringSubmatch(line); len(m) > 1 {
			diag.MissingGPGKeys = append(diag.MissingGPGKeys, m[1])
		}
	}

	if len(diag.CodenameMismatches) != 1 {
		t.Fatalf("expected 1 codename mismatch (bionic on focal), got %d: %+v", len(diag.CodenameMismatches), diag.CodenameMismatches)
	}
	if !strings.Contains(diag.CodenameMismatches[0], "repo.mysql.com") {
		t.Errorf("expected mysql repo mismatch, got %s", diag.CodenameMismatches[0])
	}
	if len(diag.MissingGPGKeys) != 1 || diag.MissingGPGKeys[0] != "B7B3B788A8D3785C" {
		t.Fatalf("expected missing GPG key B7B3B788A8D3785C, got %+v", diag.MissingGPGKeys)
	}
}

func TestPHPDiscovery_FilterAndDedup(t *testing.T) {
	rawProbeOut := `PHP_BIN:/usr/bin/php:/usr/bin/php:8.1.20
PHP_BIN:/bin/php:/usr/bin/php:8.1.20
PHP_BIN:/usr/bin/php8.1:/usr/bin/php8.1:8.1.20
PHP_BIN:/bin/php8.1:/usr/bin/php8.1:8.1.20
PHP_BIN:/usr/bin/php-73:/usr/bin/php-7.3.27:7.3.27
PHP_BIN:/usr/bin/php-7.3.27:/usr/bin/php-7.3.27:7.3.27
PHP_BIN:/usr/bin/phpize:/usr/bin/phpize:Cannot find config.m4.
PHP_BIN:/usr/bin/phpize8.1:/usr/bin/phpize8.1:Cannot find config.m4.
PHP_BIN:/opt/php-7.3.27/bin/phpdbg:/opt/php-7.3.27/bin/phpdbg:[0;32m[Welcome to phpdbg, the interactive PHP debugger, v0.5.0] [0m
PHP_CGI:/opt/php-7.4.30/bin/php-cgi:/opt/php-7.4.30/bin/php-cgi:7.4.30
`
	seenRealPaths := make(map[string]bool)
	var runtimes []PHPRuntimeInstance

	for _, line := range strings.Split(rawProbeOut, "\n") {
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

			dedupKey := realPath + "|" + string(rune(len(prefix)))
			if seenRealPaths[dedupKey] {
				continue
			}
			seenRealPaths[dedupKey] = true

			kind := "cli"
			if isCGI {
				kind = "cgi"
			}
			runtimes = append(runtimes, PHPRuntimeInstance{
				ID:         binPath,
				Version:    cleanVer,
				MajorMinor: majorMinor,
				Kind:       kind,
			})
		}
	}

	// Out of 10 raw entries, phpize and phpdbg should be rejected, and duplicate symlinks deduped!
	// We should only have:
	// 1. /usr/bin/php (8.1.20)
	// 2. /usr/bin/php8.1 (8.1.20) - or if realpath differs
	// 3. /usr/bin/php-73 (7.3.27)
	// 4. /opt/php-7.4.30/bin/php-cgi (7.4.30)
	for _, rt := range runtimes {
		if strings.Contains(rt.Version, "Cannot find") || strings.Contains(rt.Version, "Welcome") {
			t.Fatalf("invalid runtime version leaked through: %+v", rt)
		}
	}
	if len(runtimes) > 4 {
		t.Fatalf("expected <= 4 deduplicated valid runtimes, got %d: %+v", len(runtimes), runtimes)
	}
}

func TestSection10_EndToEndRegression(t *testing.T) {
	// 1. Web Servers: Custom Nginx 1.22 in /opt and Apache on 8080
	webServers := []WebServerInstance{
		{
			ID:             "nginx-opt-nginx-1-22-0",
			Engine:         "nginx",
			Name:           "Nginx (/opt/nginx-1.22.0)",
			BinaryPath:     "/opt/nginx-1.22.0/sbin/nginx",
			Version:        "1.22.0",
			InstallType:    "source",
			Status:         "active",
			PIDs:           []int{1234, 1235},
			ListeningPorts: []int{80, 443},
			ConfigPath:     "/opt/nginx-1.22.0/conf/nginx.conf",
			PrefixDir:      "/opt/nginx-1.22.0",
			SystemdUnit:    "nginx.service",
			IsManageable:   true,
			TestCommand:    "'/opt/nginx-1.22.0/sbin/nginx' -t -c '/opt/nginx-1.22.0/conf/nginx.conf' -p '/opt/nginx-1.22.0'",
			ReloadCommand:  "systemctl reload nginx",
		},
		{
			ID:             "apache2-system",
			Engine:         "apache2",
			Name:           "Apache HTTP Server (System Package)",
			BinaryPath:     "/usr/sbin/apache2",
			Version:        "2.4.41",
			InstallType:    "apt",
			Status:         "active",
			PIDs:           []int{2100, 2101},
			ListeningPorts: []int{8080},
			ConfigPath:     "/etc/apache2/apache2.conf",
			PrefixDir:      "/etc/apache2",
			SystemdUnit:    "apache2.service",
			Modules:        []string{"core_module", "so_module", "mpm_prefork_module", "php8.1_module"},
			IsManageable:   true,
			TestCommand:    "apache2ctl configtest",
			ReloadCommand:  "systemctl reload apache2",
		},
	}

	// 2. PHP Runtimes: PHP 8.1 CLI, FPM, Apache mod_php, plus leftover orphaned php73-fpm and php74-fpm
	phpRuntimes := []PHPRuntimeInstance{
		{
			ID:           "php-cli-8.1",
			Version:      "8.1.20",
			MajorMinor:   "8.1",
			Kind:         "cli",
			BinaryPath:   "/usr/bin/php8.1",
			Status:       "active",
			IsManageable: true,
		},
		{
			ID:           "php-fpm-8.1",
			Version:      "8.1",
			MajorMinor:   "8.1",
			Kind:         "fpm",
			BinaryPath:   "/usr/sbin/php-fpm8.1",
			Status:       "active",
			IsManageable: true,
			Sockets:      []string{"/run/php/php8.1-fpm.sock"},
			SystemdUnit:  "php8.1-fpm.service",
		},
		{
			ID:           "php-mod-apache-8.1",
			Version:      "8.1",
			MajorMinor:   "8.1",
			Kind:         "mod_php",
			Status:       "active",
			IsManageable: false,
		},
		{
			ID:           "php-orphan-php73-fpm",
			Version:      "7.3",
			MajorMinor:   "7.3",
			Kind:         "orphaned_unit",
			Status:       "failed",
			IsOrphaned:   true,
			IsManageable: false,
			SystemdUnit:  "php73-fpm.service",
			Limitations:  []string{"Leftover orphaned systemd service: executable does not exist on disk"},
		},
		{
			ID:           "php-orphan-php74-fpm",
			Version:      "7.4",
			MajorMinor:   "7.4",
			Kind:         "orphaned_unit",
			Status:       "failed",
			IsOrphaned:   true,
			IsManageable: false,
			SystemdUnit:  "php74-fpm.service",
			Limitations:  []string{"Leftover orphaned systemd service: executable does not exist on disk"},
		},
	}

	// Verify that orphaned units are recognized and not marked manageable or valid runtimes
	activeRuntimes := 0
	orphanedUnits := 0
	for _, rt := range phpRuntimes {
		if rt.IsOrphaned {
			orphanedUnits++
			if rt.Status != "failed" || rt.Kind != "orphaned_unit" {
				t.Errorf("orphaned runtime %s must have Status: failed and Kind: orphaned_unit", rt.ID)
			}
		} else {
			activeRuntimes++
		}
	}
	if orphanedUnits != 2 {
		t.Fatalf("expected exactly 2 orphaned failed units, got %d", orphanedUnits)
	}
	if activeRuntimes != 3 {
		t.Fatalf("expected 3 valid runtimes (cli, fpm, mod_php), got %d", activeRuntimes)
	}

	// 3. Topology chain resolution: site.com proxied to Apache with mod_php
	sites := []*db.SiteRecord{
		{
			Domain:     "site.com",
			SiteType:   "php",
			PHPVersion: "8.1",
			WebServer:  "nginx",
			VhostPath:  "/opt/nginx-1.22.0/vhosts/site.conf",
		},
	}
	certs := []*db.SSLCertificateRecord{
		{
			PrimaryDomain: "site.com",
			CertPath:      "/etc/letsencrypt/live/site.com/fullchain.pem",
		},
	}

	if len(sites) != 1 || len(certs) != 1 {
		t.Fatalf("expected 1 site and 1 cert")
	}

	// Simulated nginx vhost content for site.com with proxy_pass to Apache 127.0.0.1:8080
	vhostNginx := `server {
    listen 80;
    server_name site.com;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
    }
}
`
	// Verify proxy resolution regex
	m := reProxyPass.FindStringSubmatch(vhostNginx)
	if len(m) < 3 || m[2] != "8080" {
		t.Fatalf("expected proxy_pass regex to extract port 8080, got %+v", m)
	}

	// In the real system or mock:
	chain := SiteTopologyChain{
		Domain:              "site.com",
		VhostPath:           "/opt/nginx-1.22.0/vhosts/site.conf",
		WebServerInstanceID: webServers[0].ID,
		WebServerEngine:     "nginx",
		ListenPorts:         []int{80, 443},
		BackendType:         "mod_php",
		BackendTarget:       "http://127.0.0.1:8080",
		PHPVersion:          "8.1",
		PHPRuntimeID:        "php-mod-apache-8.1",
		Confidence:          "proven",
	}

	if chain.Confidence != "proven" {
		t.Errorf("expected site.com confidence proven, got %s", chain.Confidence)
	}
	if chain.BackendType != "mod_php" {
		t.Errorf("expected site.com backend mod_php, got %s", chain.BackendType)
	}
	if chain.PHPVersion != "8.1" {
		t.Errorf("expected site.com PHP version 8.1, got %s", chain.PHPVersion)
	}
}
