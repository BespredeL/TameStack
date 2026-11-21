package osadapter

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/ssh"
)

// DebianAdapter manages Debian and Ubuntu based servers.
type DebianAdapter struct {
	osInfo *OSInfo
}

// NewDebianAdapter constructs a DebianAdapter.
func NewDebianAdapter(info *OSInfo) *DebianAdapter {
	return &DebianAdapter{osInfo: info}
}

func (a *DebianAdapter) GetOSInfo() *OSInfo {
	return a.osInfo
}

func (a *DebianAdapter) CollectHardware(ctx context.Context, client *ssh.SSHClient) (*HardwareInfo, error) {
	script := `
echo "===CPU_CORES==="; nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo
echo "===CPU_MODEL==="; grep -m1 "model name" /proc/cpuinfo | cut -d: -f2 | xargs
echo "===RAM_TOTAL==="; grep MemTotal /proc/meminfo | awk '{print $2 * 1024}'
echo "===DISK_INFO==="; df -B1 / | tail -n 1 | awk '{print $2, $3}'
echo "===UPTIME==="; cat /proc/uptime | awk '{print int($1)}'
echo "===SSH_VER==="; ssh -V 2>&1 | head -n 1
`
	out, _, _, err := client.Exec(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to collect hardware info: %w", err)
	}

	return parseHardwareOutput(string(out)), nil
}

func (a *DebianAdapter) DetectSoftware(ctx context.Context, client *ssh.SSHClient) (*InstalledSoftware, error) {
	script := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

echo "===WEBSERVERS==="
NGINX_FOUND=""
for b in nginx /usr/sbin/nginx /usr/bin/nginx /usr/local/nginx/sbin/nginx /usr/local/sbin/nginx /usr/local/bin/nginx /opt/nginx/sbin/nginx /snap/bin/nginx; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then
        NGINX_FOUND="yes"
        break
    fi
done
if [ "$NGINX_FOUND" = "yes" ]; then
    echo "nginx"
fi
APACHE_FOUND=""
for b in apache2 httpd /usr/sbin/apache2 /usr/sbin/httpd /usr/bin/apache2 /usr/bin/httpd /usr/local/apache2/bin/httpd; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then
        APACHE_FOUND="yes"
        break
    fi
done
if [ "$APACHE_FOUND" = "yes" ]; then
    echo "apache2"
fi

echo "===PHP==="
for d in /etc/php/*; do
    if [ -d "$d" ]; then
        basename "$d"
    fi
done

echo "===DB==="
command -v mysql >/dev/null 2>&1 && echo "mysql"
command -v mariadb >/dev/null 2>&1 && echo "mariadb"
command -v psql >/dev/null 2>&1 && echo "postgresql"
command -v redis-server >/dev/null 2>&1 && echo "redis"

echo "===DOCKER==="
docker --version 2>/dev/null || true
`
	out, _, _, err := client.Exec(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to detect installed software: %w", err)
	}

	return parseSoftwareOutput(string(out), true), nil
}

func (a *DebianAdapter) UpdatePackageIndex(ctx context.Context, client *ssh.SSHClient) (string, error) {
	out, stderr, code, err := client.NewCommand("apt-get update -y").
		WithSudo().
		WithLock("pkg_mgr", 120).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("apt-get update failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *DebianAdapter) InstallPackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}
	pkgList := strings.Join(packages, " ")
	baseCmd := fmt.Sprintf("apt-get install -y %s", pkgList)
	out, stderr, code, err := client.NewCommand(baseCmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("package installation failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *DebianAdapter) SecurityUpgrade(ctx context.Context, client *ssh.SSHClient) (string, error) {
	cmd := `apt-get update && apt-get --only-upgrade install -y $(apt-get -s upgrade | grep '^Inst' | grep -i security | awk '{print $2}') || apt-get upgrade -y`
	out, stderr, code, err := client.NewCommand(cmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("security upgrade failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *DebianAdapter) RemovePackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}
	cmd := fmt.Sprintf("apt-get remove -y %s", strings.Join(packages, " "))
	out, stderr, code, err := client.NewCommand(cmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("package removal failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *DebianAdapter) UpgradePackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error) {
	var cmd string
	if len(packages) > 0 {
		cmd = fmt.Sprintf("apt-get install --only-upgrade -y %s", strings.Join(packages, " "))
	} else {
		cmd = "apt-get dist-upgrade -y"
	}
	out, stderr, code, err := client.NewCommand(cmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("package upgrade failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *DebianAdapter) SimulateInstall(ctx context.Context, client *ssh.SSHClient, packages []string) (string, string, error) {
	cmd := fmt.Sprintf("apt-get -s install -y %s", strings.Join(packages, " "))
	return a.simulate(ctx, client, cmd)
}

func (a *DebianAdapter) SimulateUpgrade(ctx context.Context, client *ssh.SSHClient) (string, string, error) {
	return a.simulate(ctx, client, "apt-get -s dist-upgrade")
}

func (a *DebianAdapter) SimulateRemove(ctx context.Context, client *ssh.SSHClient, packages []string) (string, string, error) {
	cmd := fmt.Sprintf("apt-get -s remove -y %s", strings.Join(packages, " "))
	return a.simulate(ctx, client, cmd)
}

func (a *DebianAdapter) simulate(ctx context.Context, client *ssh.SSHClient, cmd string) (string, string, error) {
	out, stderr, code, err := client.NewCommand(cmd).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out), cmd, fmt.Errorf("apt simulation failed with code %d: %s: %w", code, strings.TrimSpace(string(stderr)), err)
	}
	return string(out), cmd, nil
}

func (a *DebianAdapter) ListInstalledPackages(ctx context.Context, client *ssh.SSHClient, query string) (string, error) {
	cmd := "dpkg-query -W -f='${Package}\\t${Version}\\t${Architecture}\\t${Status}\\t${binary:Summary}\\n'"
	if q := strings.TrimSpace(query); q != "" {
		safeQuery := strings.ReplaceAll(q, "'", "")
		cmd = fmt.Sprintf("%s | grep -i '%s' || true", cmd, safeQuery)
	}
	out, stderr, code, err := client.NewCommand(cmd).Exec(ctx)
	if err != nil || code != 0 {
		return string(out), fmt.Errorf("dpkg-query failed with code %d: %s: %w", code, strings.TrimSpace(string(stderr)), err)
	}
	return string(out), nil
}

func (a *DebianAdapter) ServiceStatus(ctx context.Context, client *ssh.SSHClient, serviceName string) (string, error) {
	out, _, code, _ := client.NewCommand(fmt.Sprintf("systemctl is-active %s", serviceName)).Exec(ctx)
	state := strings.TrimSpace(string(out))
	if code == 0 && state == "active" {
		return "active", nil
	}
	return state, nil
}

func (a *DebianAdapter) ServiceAction(ctx context.Context, client *ssh.SSHClient, serviceName string, action string) error {
	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "reload": true, "enable": true, "disable": true}
	if !validActions[action] {
		return fmt.Errorf("unsupported service action: %s", action)
	}
	cmd := fmt.Sprintf("systemctl %s %s", action, serviceName)
	_, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("systemctl %s %s failed (code %d): %s", action, serviceName, code, string(stderr))
	}
	return nil
}
