package site

import (
	"context"
	"fmt"
	"path"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

func nginxVhostCandidates(domain string) []string {
	return []string{
		fmt.Sprintf("/etc/nginx/sites-available/%s.conf", domain),
		fmt.Sprintf("/etc/nginx/sites-available/%s", domain),
		fmt.Sprintf("/etc/nginx/conf.d/%s.conf", domain),
		fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", domain),
		fmt.Sprintf("/etc/nginx/sites-enabled/%s", domain),
	}
}

func apacheVhostCandidates(domain string) []string {
	return []string{
		fmt.Sprintf("/etc/apache2/sites-available/%s.conf", domain),
		fmt.Sprintf("/etc/apache2/sites-available/%s", domain),
		fmt.Sprintf("/etc/httpd/conf.d/%s.conf", domain),
		fmt.Sprintf("/etc/apache2/sites-enabled/%s.conf", domain),
		fmt.Sprintf("/etc/apache2/sites-enabled/%s", domain),
	}
}

func isHybridWebServer(webServer string) bool {
	ws := strings.ToLower(webServer)
	return strings.Contains(ws, "nginx") && strings.Contains(ws, "apache")
}

func prefersApache(webServer string) bool {
	ws := strings.ToLower(strings.TrimSpace(webServer))
	if ws == "" || ws == "unknown" {
		return false
	}
	if isHybridWebServer(ws) {
		return false // hybrid: public front is nginx
	}
	return strings.Contains(ws, "apache")
}

func firstExisting(ctx context.Context, client *ssh.SSHClient, candidates []string) (string, bool) {
	for _, p := range candidates {
		if client.FileExists(ctx, p) {
			return p, true
		}
	}
	return "", false
}

// resolveVhost finds the virtual host file using SiteRecord.web_server as the
// primary hint, then falls back to the other stack if the preferred path is missing.
func (s *Service) resolveVhost(ctx context.Context, client *ssh.SSHClient, site *db.SiteRecord) (vhostPath string, isApache bool, err error) {
	domain := site.Domain
	preferApache := prefersApache(site.WebServer)

	// If a specific discovered or managed vhost path was preserved, use it directly.
	if site.VhostPath != "" && client.FileExists(ctx, site.VhostPath) {
		isAp := prefersApache(site.WebServer) || strings.Contains(site.VhostPath, "apache") || strings.Contains(site.VhostPath, "httpd")
		return site.VhostPath, isAp, nil
	}

	tryApache := func() (string, bool) {
		return firstExisting(ctx, client, apacheVhostCandidates(domain))
	}
	tryNginx := func() (string, bool) {
		return firstExisting(ctx, client, nginxVhostCandidates(domain))
	}

	if preferApache {
		if p, ok := tryApache(); ok {
			return p, true, nil
		}
		if p, ok := tryNginx(); ok {
			return p, false, nil
		}
	} else {
		if p, ok := tryNginx(); ok {
			return p, false, nil
		}
		if p, ok := tryApache(); ok {
			return p, true, nil
		}
	}

	return "", false, fmt.Errorf("configuration file for domain %s not found on server (web_server=%s)", domain, site.WebServer)
}

func nginxEnabledPath(vhostPath, domain string) string {
	base := path.Base(vhostPath)
	if base == "." || base == "/" || base == "" {
		base = domain + ".conf"
	}
	return "/etc/nginx/sites-enabled/" + base
}

func apacheSiteName(vhostPath, domain string) string {
	base := path.Base(vhostPath)
	if base == "" || base == "." || base == "/" {
		return domain + ".conf"
	}
	return base
}

func testAndReloadCmds(isApache bool) (testCmd, reloadCmd string) {
	if isApache {
		return "apache2ctl configtest 2>/dev/null || apachectl configtest 2>/dev/null || httpd -t 2>/dev/null",
			"systemctl reload apache2 2>/dev/null || systemctl reload httpd 2>/dev/null"
	}
	return `export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
BIN=""
for b in $(ps -eo args 2>/dev/null | grep -E '[n]ginx: master process' | awk '{print $3}') /usr/sbin/nginx /opt/*/sbin/nginx /usr/local/nginx/sbin/nginx /usr/local/sbin/nginx /snap/bin/nginx; do
    if [ -x "$b" ]; then BIN="$b"; break; fi
done
[ -z "$BIN" ] && BIN="nginx"
$BIN -t`, "systemctl reload nginx 2>/dev/null || service nginx reload 2>/dev/null || kill -HUP $(pgrep -f 'nginx: master process' | head -n1 2>/dev/null) 2>/dev/null"
}
