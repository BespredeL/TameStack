package webserver

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"tamestack/internal/ssh"
)

type ServerTopology string

const (
	TopologyNginxOnly  ServerTopology = "nginx_only"
	TopologyApacheOnly ServerTopology = "apache_only"
	TopologyHybrid     ServerTopology = "hybrid_nginx_apache"
	TopologyUnknown    ServerTopology = "unknown"
)

type PHPRuntimesInfo struct {
	ActiveFpmSockets []string `json:"active_fpm_sockets"`
	HasApacheModPHP  bool     `json:"has_apache_mod_php"`
	ModPHPVersion    string   `json:"mod_php_version,omitempty"`
}

type WebStackTopology struct {
	Topology     ServerTopology  `json:"topology"`
	FrontendPort int             `json:"frontend_port"`
	BackendPort  int             `json:"backend_port"`
	NginxActive  bool            `json:"nginx_active"`
	ApacheActive bool            `json:"apache_active"`
	PHPRuntimes  PHPRuntimesInfo `json:"php_runtimes"`
}

var (
	portRegex   = regexp.MustCompile(`[:](\d+)`)
	modPHPRegex = regexp.MustCompile(`php(\d+\.\d+)\.load`)
)

// ParseListeningSockets extracts listening ports per process name from ss or netstat output.
func ParseListeningSockets(output string) map[string][]int {
	result := make(map[string][]int)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "LISTEN") && !strings.Contains(line, "tcp") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Look for port in local address.
		var port int
		for _, f := range fields {
			if strings.Contains(f, ":") && !strings.HasPrefix(f, "users:") {
				m := portRegex.FindStringSubmatch(f)
				if len(m) > 1 {
					if p, err := strconv.Atoi(m[1]); err == nil && p > 0 {
						port = p
						break
					}
				}
			}
		}

		if port == 0 {
			continue
		}

		lineLower := strings.ToLower(line)
		procName := ""
		if strings.Contains(lineLower, "nginx") {
			procName = "nginx"
		} else if strings.Contains(lineLower, "apache2") || strings.Contains(lineLower, "httpd") {
			procName = "apache2"
		} else if strings.Contains(lineLower, "caddy") {
			procName = "caddy"
		}

		if procName != "" {
			already := false
			for _, existing := range result[procName] {
				if existing == port {
					already = true
					break
				}
			}
			if !already {
				result[procName] = append(result[procName], port)
			}
		}
	}
	return result
}

// EvaluateTopology determines whether the stack is Nginx, Apache, or Hybrid.
func EvaluateTopology(ports map[string][]int, nginxActive, apacheActive bool) *WebStackTopology {
	topo := &WebStackTopology{
		Topology:     TopologyUnknown,
		NginxActive:  nginxActive,
		ApacheActive: apacheActive,
	}

	nginxPorts := ports["nginx"]
	apachePorts := ports["apache2"]

	hasNginx80 := containsPort(nginxPorts, 80) || containsPort(nginxPorts, 443)
	hasApache80 := containsPort(apachePorts, 80) || containsPort(apachePorts, 443)

	if nginxActive && apacheActive {
		if hasNginx80 && !hasApache80 {
			topo.Topology = TopologyHybrid
			topo.FrontendPort = 80
			if len(apachePorts) > 0 {
				topo.BackendPort = apachePorts[0]
			} else {
				topo.BackendPort = 8080
			}
			return topo
		}
		topo.Topology = TopologyHybrid
		if hasNginx80 {
			topo.FrontendPort = 80
		} else if hasApache80 {
			topo.FrontendPort = 80
		} else {
			topo.FrontendPort = 80
		}
		if len(apachePorts) > 0 && apachePorts[0] != topo.FrontendPort {
			topo.BackendPort = apachePorts[0]
		} else {
			topo.BackendPort = 8080
		}
		return topo
	}

	if nginxActive && (hasNginx80 || len(nginxPorts) > 0) && !apacheActive {
		topo.Topology = TopologyNginxOnly
		topo.FrontendPort = 80
		return topo
	}

	if apacheActive && (hasApache80 || len(apachePorts) > 0) && !nginxActive {
		topo.Topology = TopologyApacheOnly
		topo.FrontendPort = 80
		return topo
	}

	if nginxActive {
		topo.Topology = TopologyNginxOnly
		topo.FrontendPort = 80
	} else if apacheActive {
		topo.Topology = TopologyApacheOnly
		topo.FrontendPort = 80
	}

	return topo
}

func containsPort(ports []int, target int) bool {
	for _, p := range ports {
		if p == target {
			return true
		}
	}
	return false
}

// DetectPort80Conflict checks if port 80 is occupied by a different web server than the one chosen.
func DetectPort80Conflict(ports map[string][]int, chosenServer string) (hasConflict bool, warning string) {
	chosen := strings.ToLower(strings.TrimSpace(chosenServer))
	nginxPorts := ports["nginx"]
	apachePorts := append(ports["apache2"], ports["apache"]...)
	apachePorts = append(apachePorts, ports["httpd"]...)

	hasNginx80 := containsPort(nginxPorts, 80)
	hasApache80 := containsPort(apachePorts, 80)

	if chosen == "nginx" && hasApache80 && !hasNginx80 {
		return true, "Порт 80 занят веб-сервером Apache. Виртуальный хост Nginx не сможет принимать входящий HTTP-трафик напрямую. Выберите Apache или настройте гибридный стек."
	}
	if chosen == "apache" && hasNginx80 && !hasApache80 {
		return true, "Порт 80 занят веб-сервером Nginx. Виртуальный хост Apache не сможет принимать входящий HTTP-трафик напрямую."
	}
	return false, ""
}

// ParsePHPRuntimesInfo extracts active PHP sockets and Apache mod_php modules.
func ParsePHPRuntimesInfo(socketsOutput, apacheModsOutput string) PHPRuntimesInfo {
	var info PHPRuntimesInfo
	for _, line := range strings.Split(socketsOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/run/php/") || strings.HasPrefix(line, "/var/run/php/") {
			info.ActiveFpmSockets = append(info.ActiveFpmSockets, line)
		}
	}

	for _, line := range strings.Split(apacheModsOutput, "\n") {
		line = strings.TrimSpace(line)
		if m := modPHPRegex.FindStringSubmatch(line); len(m) > 1 {
			info.HasApacheModPHP = true
			info.ModPHPVersion = m[1]
			break
		}
	}
	return info
}

// DetectTopology gathers runtime socket and service information from remote host via SSH.
func DetectTopology(ctx context.Context, client *ssh.SSHClient) (*WebStackTopology, error) {
	outSS, _, _, _ := client.NewCommand("export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin; ss -tulpn 2>/dev/null || netstat -tlpn 2>/dev/null").WithSudo().Exec(ctx)
	ports := ParseListeningSockets(string(outSS))

	outNginx, _, _, _ := client.Exec(ctx, "export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin; systemctl is-active nginx 2>/dev/null || systemctl is-active openresty 2>/dev/null || systemctl is-active ea-nginx 2>/dev/null || (pgrep -x nginx >/dev/null 2>&1 && echo active) || true")
	nginxActive := strings.TrimSpace(string(outNginx)) == "active"

	outApache, _, _, _ := client.Exec(ctx, "export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin; systemctl is-active apache2 2>/dev/null || systemctl is-active httpd 2>/dev/null || systemctl is-active apache 2>/dev/null || (pgrep -x apache2 >/dev/null 2>&1 && echo active) || (pgrep -x httpd >/dev/null 2>&1 && echo active) || true")
	apacheActive := strings.TrimSpace(string(outApache)) == "active"

	topo := EvaluateTopology(ports, nginxActive, apacheActive)

	outSockets, _, _, _ := client.NewCommand("ls -1 /run/php/*.sock /var/run/php/*.sock 2>/dev/null || true").WithSudo().Exec(ctx)
	outMods, _, _, _ := client.NewCommand("ls -1 /etc/apache2/mods-enabled/*.load /etc/httpd/conf.modules.d/*.conf 2>/dev/null || true").WithSudo().Exec(ctx)

	topo.PHPRuntimes = ParsePHPRuntimesInfo(string(outSockets), string(outMods))
	return topo, nil
}
