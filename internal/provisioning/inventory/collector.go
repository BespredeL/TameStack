package inventory

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"tamestack/internal/ssh"
)

// ServerInventory represents the live state of a server discovered before provisioning.
type ServerInventory struct {
	Distro         string   `json:"distro"`
	Family         string   `json:"family"` // "debian", "rhel"
	Version        string   `json:"version"`
	Codename       string   `json:"codename"`
	Arch           string   `json:"arch"`
	Kernel         string   `json:"kernel"`
	CPUCores       int      `json:"cpu_cores"`
	RAMTotalMB     int64    `json:"ram_total_mb"`
	RAMFreeMB      int64    `json:"ram_free_mb"`
	DiskTotalGB    int64    `json:"disk_total_gb"`
	DiskFreeGB     int64    `json:"disk_free_gb"`
	PackageManager string   `json:"package_manager"`
	SSHPort        int      `json:"ssh_port"`
	ListeningPorts []int    `json:"listening_ports"`
	InstalledWeb   []string `json:"installed_web"` // "nginx", "apache2"
	InstalledPHP   []string `json:"installed_php"` // "8.1", "8.2", ...
	InstalledDB    []string `json:"installed_db"`  // "mysql", "mariadb", "postgres", "redis"
	HasDocker      bool     `json:"has_docker"`
	HasUFW         bool     `json:"has_ufw"`
	HasFirewalld   bool     `json:"has_firewalld"`
	HasFail2ban    bool     `json:"has_fail2ban"`
	HasComposer         bool     `json:"has_composer"`
	HasNode             bool     `json:"has_node"`
	HasGit              bool     `json:"has_git"`
	ExistingSitesCount  int      `json:"existing_sites_count"`
	IsAlreadyConfigured bool     `json:"is_already_configured"`
}

// CollectServerInventory runs non-intrusive discovery on the target server.
func CollectServerInventory(ctx context.Context, client *ssh.SSHClient, knownSSHPort int) (*ServerInventory, error) {
	script := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
echo "===OS==="
cat /etc/os-release 2>/dev/null || true
echo "===UNAME==="
uname -m
uname -r
echo "===HW==="
nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo
free -m | grep -i Mem | awk '{print $2, $7}'
df -BG / 2>/dev/null | tail -n 1 | awk '{print $2, $4}' | tr -d 'G'
echo "===PORTS==="
ss -tulpn 2>/dev/null | grep LISTEN | awk '{print $5}' || netstat -tulpn 2>/dev/null | grep LISTEN | awk '{print $4}'
echo "===SOFTWARE==="
command -v nginx >/dev/null 2>&1 && echo "SW:nginx"
(command -v apache2 >/dev/null 2>&1 || command -v httpd >/dev/null 2>&1) && echo "SW:apache"
command -v mariadb >/dev/null 2>&1 && echo "SW:mariadb"
command -v mysql >/dev/null 2>&1 && echo "SW:mysql"
command -v psql >/dev/null 2>&1 && echo "SW:postgres"
command -v redis-server >/dev/null 2>&1 && echo "SW:redis"
command -v docker >/dev/null 2>&1 && echo "SW:docker"
command -v ufw >/dev/null 2>&1 && echo "SW:ufw"
command -v firewall-cmd >/dev/null 2>&1 && echo "SW:firewalld"
command -v fail2ban-client >/dev/null 2>&1 && echo "SW:fail2ban"
command -v composer >/dev/null 2>&1 && echo "SW:composer"
command -v node >/dev/null 2>&1 && echo "SW:node"
command -v git >/dev/null 2>&1 && echo "SW:git"
echo "===PHP==="
for d in /etc/php/* /etc/opt/remi/php*; do
    if [ -d "$d" ]; then basename "$d"; fi
done
echo "===SITES==="
ls -1 /var/www 2>/dev/null | grep -v 'html' | wc -l || echo "0"
`

	out, _, _, err := client.Exec(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to collect server inventory: %w", err)
	}

	inv := &ServerInventory{
		SSHPort: knownSSHPort,
	}
	if inv.SSHPort <= 0 {
		inv.SSHPort = 22
	}

	sections := strings.Split(string(out), "===")
	for i := 1; i < len(sections); i += 2 {
		title := strings.TrimSpace(sections[i])
		body := ""
		if i+1 < len(sections) {
			body = strings.TrimSpace(sections[i+1])
		}

		switch title {
		case "OS":
			parseOSRelease(body, inv)
		case "UNAME":
			lines := strings.Split(body, "\n")
			if len(lines) > 0 {
				inv.Arch = strings.TrimSpace(lines[0])
			}
			if len(lines) > 1 {
				inv.Kernel = strings.TrimSpace(lines[1])
			}
		case "HW":
			lines := strings.Split(body, "\n")
			if len(lines) > 0 {
				inv.CPUCores, _ = strconv.Atoi(strings.TrimSpace(lines[0]))
			}
			if len(lines) > 1 {
				memParts := strings.Fields(lines[1])
				if len(memParts) >= 2 {
					inv.RAMTotalMB, _ = strconv.ParseInt(memParts[0], 10, 64)
					inv.RAMFreeMB, _ = strconv.ParseInt(memParts[1], 10, 64)
				}
			}
			if len(lines) > 2 {
				diskParts := strings.Fields(lines[2])
				if len(diskParts) >= 2 {
					inv.DiskTotalGB, _ = strconv.ParseInt(diskParts[0], 10, 64)
					inv.DiskFreeGB, _ = strconv.ParseInt(diskParts[1], 10, 64)
				}
			}
		case "PORTS":
			portMap := make(map[int]bool)
			for _, line := range strings.Split(body, "\n") {
				line = strings.TrimSpace(line)
				if idx := strings.LastIndex(line, ":"); idx != -1 {
					pStr := line[idx+1:]
					if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
						portMap[p] = true
					}
				}
			}
			for p := range portMap {
				inv.ListeningPorts = append(inv.ListeningPorts, p)
			}
		case "SOFTWARE":
			for _, line := range strings.Split(body, "\n") {
				line = strings.TrimSpace(line)
				switch line {
				case "SW:nginx":
					inv.InstalledWeb = append(inv.InstalledWeb, "nginx")
				case "SW:apache":
					inv.InstalledWeb = append(inv.InstalledWeb, "apache")
				case "SW:mariadb":
					inv.InstalledDB = append(inv.InstalledDB, "mariadb")
				case "SW:mysql":
					inv.InstalledDB = append(inv.InstalledDB, "mysql")
				case "SW:postgres":
					inv.InstalledDB = append(inv.InstalledDB, "postgres")
				case "SW:redis":
					inv.InstalledDB = append(inv.InstalledDB, "redis")
				case "SW:docker":
					inv.HasDocker = true
				case "SW:ufw":
					inv.HasUFW = true
				case "SW:firewalld":
					inv.HasFirewalld = true
				case "SW:fail2ban":
					inv.HasFail2ban = true
				case "SW:composer":
					inv.HasComposer = true
				case "SW:node":
					inv.HasNode = true
				case "SW:git":
					inv.HasGit = true
				}
			}
		case "PHP":
			for _, line := range strings.Split(body, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					ver := strings.TrimPrefix(line, "php")
					if len(ver) == 2 && !strings.Contains(ver, ".") { // "82" -> "8.2"
						ver = string(ver[0]) + "." + string(ver[1])
					}
					inv.InstalledPHP = append(inv.InstalledPHP, ver)
				}
			}
		case "SITES":
			cnt, _ := strconv.Atoi(strings.TrimSpace(body))
			inv.ExistingSitesCount = cnt
		}
	}

	inv.IsAlreadyConfigured = len(inv.InstalledWeb) > 0 || len(inv.InstalledPHP) > 0 || len(inv.InstalledDB) > 0 || inv.ExistingSitesCount > 0

	return inv, nil
}

func parseOSRelease(body string, inv *ServerInventory) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], `"'`)
		switch key {
		case "ID":
			inv.Distro = strings.ToLower(val)
			if inv.Distro == "ubuntu" || inv.Distro == "debian" {
				inv.Family = "debian"
				inv.PackageManager = "apt"
			} else {
				inv.Family = "rhel"
				inv.PackageManager = "dnf"
			}
		case "VERSION_ID":
			inv.Version = val
		case "VERSION_CODENAME":
			inv.Codename = val
		}
	}
}
