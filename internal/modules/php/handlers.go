package php

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"tamestack/internal/ssh"
)

// DetectPHPHandlers detects the active execution handlers on the server.
func (s *Service) DetectPHPHandlers(ctx context.Context, serverID string) ([]string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	script := `
echo "===HANDLERS==="
if command -v php >/dev/null 2>&1; then echo "cli"; fi
if pgrep -f "php-fpm" >/dev/null 2>&1 || systemctl list-units --type=service "php*-fpm" 2>/dev/null | grep -q "fpm"; then echo "fpm"; fi
if command -v apache2ctl >/dev/null 2>&1 || command -v httpd >/dev/null 2>&1; then
    AP_MODS=$(apache2ctl -M 2>/dev/null || httpd -M 2>/dev/null || true)
    if echo "$AP_MODS" | grep -q "php"; then echo "mod_php"; fi
    if echo "$AP_MODS" | grep -q "proxy_fcgi"; then echo "proxy_fcgi"; fi
fi
if command -v nginx >/dev/null 2>&1; then echo "nginx_fpm"; fi
if command -v docker >/dev/null 2>&1; then
    if docker ps 2>/dev/null | grep -iq "php"; then echo "docker"; fi
fi
`
	out, _, _, err := client.Exec(ctx, script)
	if err != nil {
		return nil, err
	}

	var handlers []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "===HANDLERS===" {
			continue
		}
		if !seen[line] {
			seen[line] = true
			handlers = append(handlers, line)
		}
	}

	return handlers, nil
}

// SetDefaultCLI sets the system default PHP CLI binary via alternatives.
func (s *Service) SetDefaultCLI(ctx context.Context, serverID, version string) error {
	if err := ssh.ValidatePHPVersion(version); err != nil {
		return err
	}
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	cleanVer := sanitizeVersion(version)
	script := fmt.Sprintf(`
TARGET_BIN="/usr/bin/php%s"
if [ ! -x "$TARGET_BIN" ]; then
    TARGET_BIN=$(which php%s 2>/dev/null || true)
fi

if [ -z "$TARGET_BIN" ] || [ ! -x "$TARGET_BIN" ]; then
    echo "ERROR: PHP %s binary not found on system"
    exit 1
fi

if command -v update-alternatives >/dev/null 2>&1; then
    update-alternatives --set php "$TARGET_BIN" 2>&1 || update-alternatives --install /usr/bin/php php "$TARGET_BIN" 50
    if [ -x "/usr/bin/phar%s" ]; then
        update-alternatives --set phar "/usr/bin/phar%s" 2>/dev/null || true
    fi
    if [ -x "/usr/bin/phpize%s" ]; then
        update-alternatives --set phpize "/usr/bin/phpize%s" 2>/dev/null || true
    fi
    if [ -x "/usr/bin/php-config%s" ]; then
        update-alternatives --set php-config "/usr/bin/php-config%s" 2>/dev/null || true
    fi
elif command -v alternatives >/dev/null 2>&1; then
    alternatives --set php "$TARGET_BIN" 2>&1 || true
else
    ln -sf "$TARGET_BIN" /usr/bin/php
fi

echo "SUCCESS"
php -v | head -n 1
`, cleanVer, cleanVer, cleanVer, cleanVer, cleanVer, cleanVer, cleanVer, cleanVer, cleanVer)

	out, stderr, code, err := client.NewCommand(script).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to set default CLI PHP %s: %s %s", version, string(out), string(stderr))
	}

	return nil
}

// GetFPMServiceOverview retrieves service health, worker count, and socket paths.
func (s *Service) GetFPMServiceOverview(ctx context.Context, serverID, version string) (*FPMServiceOverview, error) {
	if err := ssh.ValidatePHPVersion(version); err != nil {
		return nil, err
	}
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cleanVer := sanitizeVersion(version)
	cleanCompact := strings.ReplaceAll(version, ".", "")
	script := fmt.Sprintf(`
SRV="php%s-fpm"
for s in "php%s-fpm" "php-fpm" "rh-php%s-php-fpm" "php%s-php-fpm"; do
    if systemctl is-active --quiet "$s" 2>/dev/null || systemctl cat "$s" >/dev/null 2>&1; then
        SRV="$s"
        break
    fi
done

echo "SRV:$SRV"
STATUS="inactive"
if systemctl is-active --quiet "$SRV" 2>/dev/null; then
    STATUS="active"
elif systemctl is-failed --quiet "$SRV" 2>/dev/null; then
    STATUS="failed"
fi
echo "STATUS:$STATUS"

PID=$(pgrep -f "php-fpm: master process" | head -n 1 || echo 0)
echo "PID:$PID"

WORKERS=$(pgrep -f "php-fpm: pool" | wc -l || echo 0)
echo "WORKERS:$WORKERS"

echo "---SOCKETS---"
ls -1 /run/php/php%s-fpm*.sock /var/run/php/php%s-fpm*.sock /run/php-fpm/*.sock 2>/dev/null || true

echo "---TEST---"
php-fpm%s -t 2>&1 || php-fpm -t 2>&1 || true
`, cleanVer, cleanVer, cleanCompact, cleanCompact, cleanVer, cleanVer, cleanVer)

	out, _, _, err := client.Exec(ctx, script)
	if err != nil {
		return nil, err
	}

	overview := &FPMServiceOverview{
		Version:     version,
		ConfigFile:  fmt.Sprintf("/etc/php/%s/fpm/php-fpm.conf", cleanVer),
		ConfigValid: true,
	}

	parts := strings.Split(string(out), "---SOCKETS---")
	header := parts[0]
	var tail string
	if len(parts) > 1 {
		subParts := strings.Split(parts[1], "---TEST---")
		for _, sockLine := range strings.Split(subParts[0], "\n") {
			s := strings.TrimSpace(sockLine)
			if s != "" {
				overview.Sockets = append(overview.Sockets, s)
			}
		}
		if len(subParts) > 1 {
			tail = strings.TrimSpace(subParts[1])
			overview.ConfigOutput = tail
			if strings.Contains(strings.ToLower(tail), "failed") || strings.Contains(strings.ToLower(tail), "error") {
				overview.ConfigValid = false
			}
		}
	}

	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SRV:") {
			overview.ServiceName = strings.TrimPrefix(line, "SRV:")
		} else if strings.HasPrefix(line, "STATUS:") {
			overview.Status = strings.TrimPrefix(line, "STATUS:")
		} else if strings.HasPrefix(line, "PID:") {
			overview.MainPID, _ = strconv.Atoi(strings.TrimPrefix(line, "PID:"))
		} else if strings.HasPrefix(line, "WORKERS:") {
			overview.TotalWorkers, _ = strconv.Atoi(strings.TrimPrefix(line, "WORKERS:"))
		}
	}

	return overview, nil
}

// GetFPMServiceLogs reads recent journalctl entries for the service.
func (s *Service) GetFPMServiceLogs(ctx context.Context, serverID, version string, lines int) (string, error) {
	if lines <= 0 || lines > 500 {
		lines = 100
	}
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	cleanVer := sanitizeVersion(version)
	cmd := fmt.Sprintf(`journalctl -u php%s-fpm -u php-fpm -n %d --no-pager 2>/dev/null || cat /var/log/php%s-fpm.log 2>/dev/null | tail -n %d`,
		cleanVer, lines, cleanVer, lines)

	out, stderr, _, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get FPM logs: %s %w", string(stderr), err)
	}

	return string(out), nil
}

// CheckFPMConfig runs php-fpm syntax check.
func (s *Service) CheckFPMConfig(ctx context.Context, serverID, version string) (string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	cleanVer := sanitizeVersion(version)
	cmd := fmt.Sprintf(`php-fpm%s -t 2>&1 || php-fpm -t 2>&1`, cleanVer)
	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	res := strings.TrimSpace(string(out))
	if len(stderr) > 0 {
		res += "\n" + strings.TrimSpace(string(stderr))
	}
	if err != nil || code != 0 {
		return res, fmt.Errorf("FPM configuration test failed (code %d): %s", code, res)
	}
	return res, nil
}

// CheckAppDependencies checks required vs missing extensions for common PHP applications.
func (s *Service) CheckAppDependencies(ctx context.Context, serverID, version, appPreset string) (*PHPAppRequirement, error) {
	versions, err := s.InspectPHPVersions(ctx, serverID)
	if err != nil {
		return nil, err
	}

	var targetVer *PHPVersionInfo
	for i := range versions {
		if versions[i].Version == version {
			targetVer = &versions[i]
			break
		}
	}
	if targetVer == nil {
		return nil, fmt.Errorf("PHP version %s not found on server", version)
	}

	loaded := make(map[string]bool)
	for _, ext := range targetVer.Extensions {
		if ext.IsInstalled {
			loaded[strings.ToLower(ext.Name)] = true
		}
	}

	appPreset = strings.ToLower(strings.TrimSpace(appPreset))
	var required []string
	switch appPreset {
	case "laravel":
		required = []string{"curl", "mbstring", "mysql", "xml", "zip", "bcmath", "fileinfo", "redis"}
	case "wordpress":
		required = []string{"curl", "mysql", "xml", "zip", "gd", "imagick", "intl", "mbstring", "fileinfo"}
	case "drupal":
		required = []string{"curl", "mysql", "xml", "zip", "gd", "mbstring", "opcache"}
	default:
		required = []string{"curl", "mbstring", "mysql", "xml", "zip"}
	}

	result := &PHPAppRequirement{
		AppType:            appPreset,
		RequiredExtensions: required,
		IsCompatible:       true,
	}

	for _, req := range required {
		if loaded[req] {
			result.InstalledExtensions = append(result.InstalledExtensions, req)
		} else {
			result.MissingExtensions = append(result.MissingExtensions, req)
			result.IsCompatible = false
			result.Recommendations = append(result.Recommendations, fmt.Sprintf("Install missing extension 'php%s-%s'", version, req))
		}
	}

	return result, nil
}
