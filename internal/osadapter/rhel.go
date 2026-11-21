package osadapter

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/ssh"
)

// RhelAdapter manages RHEL, AlmaLinux, Rocky Linux and CentOS servers.
type RhelAdapter struct {
	osInfo *OSInfo
}

// NewRhelAdapter constructs a RhelAdapter.
func NewRhelAdapter(info *OSInfo) *RhelAdapter {
	return &RhelAdapter{osInfo: info}
}

func (a *RhelAdapter) GetOSInfo() *OSInfo {
	return a.osInfo
}

func (a *RhelAdapter) CollectHardware(ctx context.Context, client *ssh.SSHClient) (*HardwareInfo, error) {
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

func (a *RhelAdapter) DetectSoftware(ctx context.Context, client *ssh.SSHClient) (*InstalledSoftware, error) {
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
for b in httpd apache2 /usr/sbin/httpd /usr/sbin/apache2 /usr/bin/httpd /usr/bin/apache2 /usr/local/apache2/bin/httpd; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then
        APACHE_FOUND="yes"
        break
    fi
done
if [ "$APACHE_FOUND" = "yes" ]; then
    echo "apache2"
fi

echo "===PHP==="
rpm -qa | grep -E '^php[0-9]*' | cut -d- -f1 | sort -u || true

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

	return parseSoftwareOutput(string(out), false), nil
}

func (a *RhelAdapter) UpdatePackageIndex(ctx context.Context, client *ssh.SSHClient) (string, error) {
	out, stderr, code, err := client.NewCommand("dnf check-update -y || [ $? -eq 100 ]").
		WithSudo().
		WithLock("pkg_mgr", 120).
		Exec(ctx)
	if err != nil && code != 0 && code != 100 {
		return string(out) + string(stderr), fmt.Errorf("dnf check-update failed: %w", err)
	}
	return string(out), nil
}

func (a *RhelAdapter) InstallPackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}
	pkgList := strings.Join(packages, " ")
	baseCmd := fmt.Sprintf("dnf install -y %s || yum install -y %s", pkgList, pkgList)
	out, stderr, code, err := client.NewCommand(baseCmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("package installation failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *RhelAdapter) SecurityUpgrade(ctx context.Context, client *ssh.SSHClient) (string, error) {
	cmd := "dnf upgrade -y --security || yum update -y --security"
	out, stderr, code, err := client.NewCommand(cmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("security upgrade failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *RhelAdapter) RemovePackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}
	pkgList := strings.Join(packages, " ")
	cmd := fmt.Sprintf("dnf remove -y %s || yum remove -y %s", pkgList, pkgList)
	out, stderr, code, err := client.NewCommand(cmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("package removal failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *RhelAdapter) UpgradePackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error) {
	var cmd string
	if len(packages) > 0 {
		pkgList := strings.Join(packages, " ")
		cmd = fmt.Sprintf("dnf upgrade -y %s || yum update -y %s", pkgList, pkgList)
	} else {
		cmd = "dnf upgrade -y || yum update -y"
	}
	out, stderr, code, err := client.NewCommand(cmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		Exec(ctx)
	if err != nil || code != 0 {
		return string(out) + string(stderr), fmt.Errorf("package upgrade failed with code %d: %w", code, err)
	}
	return string(out), nil
}

func (a *RhelAdapter) SimulateInstall(ctx context.Context, client *ssh.SSHClient, packages []string) (string, string, error) {
	cmd := fmt.Sprintf("dnf install --assumeno %s", strings.Join(packages, " "))
	return a.simulate(ctx, client, cmd)
}

func (a *RhelAdapter) SimulateUpgrade(ctx context.Context, client *ssh.SSHClient) (string, string, error) {
	return a.simulate(ctx, client, "dnf upgrade --assumeno")
}

func (a *RhelAdapter) SimulateRemove(ctx context.Context, client *ssh.SSHClient, packages []string) (string, string, error) {
	cmd := fmt.Sprintf("dnf remove --assumeno %s", strings.Join(packages, " "))
	return a.simulate(ctx, client, cmd)
}

// simulate runs a dnf --assumeno preview. dnf aborts the transaction with a
// non-zero exit after printing it, so callers tolerate the error when output exists.
func (a *RhelAdapter) simulate(ctx context.Context, client *ssh.SSHClient, cmd string) (string, string, error) {
	out, stderr, code, err := client.NewCommand(cmd).Exec(ctx)
	if err != nil || code != 0 {
		return string(out), cmd, fmt.Errorf("dnf simulation exited with code %d: %s: %w", code, strings.TrimSpace(string(stderr)), err)
	}
	return string(out), cmd, nil
}

func (a *RhelAdapter) ListInstalledPackages(ctx context.Context, client *ssh.SSHClient, query string) (string, error) {
	cmd := "rpm -qa --queryformat '%{NAME}\\t%{VERSION}-%{RELEASE}\\t%{ARCH}\\t%{SUMMARY}\\n'"
	if q := strings.TrimSpace(query); q != "" {
		safeQuery := strings.ReplaceAll(q, "'", "")
		cmd = fmt.Sprintf("%s | grep -i '%s' || true", cmd, safeQuery)
	}
	out, stderr, code, err := client.NewCommand(cmd).Exec(ctx)
	if err != nil || code != 0 {
		return string(out), fmt.Errorf("rpm query failed with code %d: %s: %w", code, strings.TrimSpace(string(stderr)), err)
	}
	return string(out), nil
}

func (a *RhelAdapter) ServiceStatus(ctx context.Context, client *ssh.SSHClient, serviceName string) (string, error) {
	out, _, code, _ := client.NewCommand(fmt.Sprintf("systemctl is-active %s", serviceName)).Exec(ctx)
	state := strings.TrimSpace(string(out))
	if code == 0 && state == "active" {
		return "active", nil
	}
	return state, nil
}

func (a *RhelAdapter) ServiceAction(ctx context.Context, client *ssh.SSHClient, serviceName string, action string) error {
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
