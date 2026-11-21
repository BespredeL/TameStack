package security

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"tamestack/internal/ssh"
)

// LogTarget selects which set of log files a read-only tail inspects. It is an opaque
// code-owned value so that no caller can push a file list or a shell fragment through it.
type LogTarget string

const (
	LogTargetApache LogTarget = "apache"
	LogTargetNginx  LogTarget = "nginx"
	LogTargetPHPFPM LogTarget = "php-fpm"
	LogTargetAuto   LogTarget = "auto"
)

// configReadPrefixes limits where the AI may read configuration from.
var configReadPrefixes = []string{"/etc/", "/var/log/", "/run/php/"}

// secretPathMarkers blocks files that hold credentials or private keys.
var secretPathMarkers = []string{
	"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa",
	".key", "privkey", "shadow", "gshadow",
	"master.key", ".env", "passwd-",
}

var logFilesByTarget = map[LogTarget]map[string][]string{
	LogTargetApache: {
		"error":  {"/var/log/apache2/error.log", "/var/log/httpd/error_log"},
		"access": {"/var/log/apache2/access.log", "/var/log/apache2/other_vhosts_access.log", "/var/log/httpd/access_log"},
	},
	LogTargetNginx: {
		"error":  {"/var/log/nginx/error.log"},
		"access": {"/var/log/nginx/access.log"},
	},
	LogTargetPHPFPM: {
		"error":  {"/var/log/php*-fpm.log", "/var/log/php-fpm/*.log", "/var/log/php/error.log"},
		"access": {"/var/log/php*-fpm.log", "/var/log/php-fpm/*.log"},
	},
}

var phpSocketGlob = "/run/php/*.sock"

// maxInspectionLines bounds how much a single read-only call can pull back.
const maxInspectionLines = 200

// ReadLogTail tails the log files of one web-server family on the remote host.
func (s *Service) ReadLogTail(ctx context.Context, serverID string, target LogTarget, logType string, lines int) (string, error) {
	if err := ssh.ValidateLogType(logType); err != nil {
		return "", err
	}
	lines = clampLines(lines, maxInspectionLines)

	if target == LogTargetAuto {
		return s.execReadOnly(ctx, serverID, autoLogTailScript(logType, lines))
	}

	files, ok := logFilesByTarget[target][logType]
	if !ok {
		return "", fmt.Errorf("unsupported log target %q", target)
	}
	return s.execReadOnly(ctx, serverID, firstExistingTailScript(files, lines, fmt.Sprintf("Логи %s не обнаружены.", target)))
}

// ReadServiceStatus returns systemctl status output for one validated unit name.
func (s *Service) ReadServiceStatus(ctx context.Context, serverID, serviceName string) (string, error) {
	if err := ssh.ValidateServiceName(serviceName); err != nil {
		return "", err
	}
	quoted := ssh.ShellEscape(serviceName)
	cmd := fmt.Sprintf("systemctl status %s --no-pager -l 2>&1 || service %s status 2>&1", quoted, quoted)
	return s.execReadOnly(ctx, serverID, cmd)
}

// ReadServiceJournal returns journalctl output for one validated unit name.
func (s *Service) ReadServiceJournal(ctx context.Context, serverID, serviceName string, lines int) (string, error) {
	if err := ssh.ValidateServiceName(serviceName); err != nil {
		return "", err
	}
	lines = clampLines(lines, maxInspectionLines)
	cmd := fmt.Sprintf("journalctl -u %s -n %d --no-pager 2>&1", ssh.ShellEscape(serviceName), lines)
	return s.execReadOnly(ctx, serverID, cmd)
}

// ReadListeningSockets lists sockets and the processes behind them.
func (s *Service) ReadListeningSockets(ctx context.Context, serverID string) (string, error) {
	return s.execReadOnly(ctx, serverID, "ss -tulpn 2>/dev/null || netstat -tulpn 2>/dev/null")
}

// ReadPHPEnvironment reports installed PHP CLI versions, FPM sockets and, when a version is
// given, the modules loaded by that interpreter.
func (s *Service) ReadPHPEnvironment(ctx context.Context, serverID, version string) (string, error) {
	cmd := `
echo "=== CLI ==="
php -v 2>&1 | head -n 1 || true

echo "=== BINARIES ==="
ls -1 /usr/bin/php[0-9].* 2>/dev/null || true

echo "=== SOCKETS ==="
ls -1 ` + phpSocketGlob + ` 2>/dev/null || true

echo "=== FPM_SERVICES ==="
systemctl list-units --type=service --state=running 2>/dev/null | grep -E 'php[0-9].*-fpm' || true
`
	version = strings.TrimSpace(version)
	if version != "" {
		if err := ssh.ValidatePHPVersion(version); err != nil {
			return "", err
		}
		cmd += fmt.Sprintf(`
echo "=== MODULES_%s ==="
php%s -m 2>/dev/null || php -m 2>/dev/null || true
`, version, version)
	}
	return s.execReadOnly(ctx, serverID, cmd)
}

// CheckWebserverSyntax validates web-server configuration without applying any change.
func (s *Service) CheckWebserverSyntax(ctx context.Context, serverID, target string) (string, error) {
	switch target {
	case "nginx":
		return s.execReadOnly(ctx, serverID, "nginx -t 2>&1 || echo 'Nginx не установлен.'")
	case "apache":
		return s.execReadOnly(ctx, serverID, "apache2ctl configtest 2>&1 || apachectl configtest 2>&1 || httpd -t 2>&1 || echo 'Apache не установлен.'")
	case "", "all":
		return s.execReadOnly(ctx, serverID, syntaxCheckAllScript)
	default:
		return "", fmt.Errorf("unsupported webserver type %q", target)
	}
}

// ReadConfigHead reads the first lines of a system configuration file. The path is checked
// before and after symlink resolution so that a link out of the allowed area is rejected.
func (s *Service) ReadConfigHead(ctx context.Context, serverID, filePath string, maxLines int) (string, error) {
	clean, err := validateConfigReadPath(filePath)
	if err != nil {
		return "", err
	}

	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}
	resolved, err := client.ResolveRemotePath(ctx, clean)
	if err != nil {
		return "", err
	}
	if _, err := validateConfigReadPath(resolved); err != nil {
		return "", err
	}

	maxLines = clampLines(maxLines, 300)
	cmd := fmt.Sprintf("head -n %d %s 2>&1", maxLines, ssh.ShellEscape(resolved))
	return s.execReadOnly(ctx, serverID, cmd)
}

func validateConfigReadPath(filePath string) (string, error) {
	if err := ssh.ValidatePolicyPath(filePath); err != nil {
		return "", err
	}
	clean := path.Clean(filePath)
	if !strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
		return "", fmt.Errorf("только абсолютные пути без .. доступны для чтения конфигураций")
	}
	allowed := false
	for _, prefix := range configReadPrefixes {
		if strings.HasPrefix(clean, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("доступ разрешен только к системным конфигурациям в %s", strings.Join(configReadPrefixes, ", "))
	}
	lower := strings.ToLower(clean)
	for _, marker := range secretPathMarkers {
		if strings.Contains(lower, marker) {
			return "", fmt.Errorf("доступ к файлу %s запрещен правилами безопасности (содержит закрытые ключи или учетные данные)", clean)
		}
	}
	return clean, nil
}

func clampLines(lines, max int) int {
	if lines <= 0 {
		return 50
	}
	if lines > max {
		return max
	}
	return lines
}

// firstExistingTailScript prints the tail of the first candidate that exists.
func firstExistingTailScript(files []string, lines int, missingMessage string) string {
	return fmt.Sprintf(`LINES=%s
for f in %s; do
    if [ -f "$f" ]; then
        echo "=== FILE: $f ==="
        tail -n "$LINES" "$f" 2>&1
        exit 0
    fi
done
echo %s`, strconv.Itoa(lines), strings.Join(files, " "), ssh.ShellEscape(missingMessage))
}

// autoLogTailScript prints the tail of every standard log that exists.
func autoLogTailScript(logType string, lines int) string {
	var files []string
	for _, target := range []LogTarget{LogTargetNginx, LogTargetApache, LogTargetPHPFPM} {
		files = append(files, logFilesByTarget[target][logType]...)
	}
	return fmt.Sprintf(`LINES=%s
FOUND=0
for f in %s; do
    if [ -f "$f" ]; then
        echo "=== FILE: $f ==="
        tail -n "$LINES" "$f" 2>&1
        FOUND=1
        echo ""
    fi
done
if [ "$FOUND" -eq 0 ]; then
    echo "Стандартные логи веб-серверов не найдены."
fi`, strconv.Itoa(lines), strings.Join(files, " "))
}

const syntaxCheckAllScript = `
echo "=== NGINX TEST ==="
if command -v nginx >/dev/null 2>&1; then
    nginx -t 2>&1 || true
else
    echo "Nginx не установлен."
fi

echo "=== APACHE TEST ==="
if command -v apache2ctl >/dev/null 2>&1; then
    apache2ctl configtest 2>&1 || true
elif command -v apachectl >/dev/null 2>&1; then
    apachectl configtest 2>&1 || true
elif command -v httpd >/dev/null 2>&1; then
    httpd -t 2>&1 || true
else
    echo "Apache/httpd не установлен."
fi
`
