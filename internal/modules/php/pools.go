package php

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"tamestack/internal/ssh"
)

var poolNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)

func sanitizeVersion(version string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(version) {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidatePoolName verifies pool identifier constraints.
func ValidatePoolName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("имя пула не может быть пустым")
	}
	if name == "www" {
		return fmt.Errorf("пул 'www' является системным по умолчанию и не может быть перезаписан")
	}
	if !poolNameRegex.MatchString(name) {
		return fmt.Errorf("недопустимое имя пула %q: разрешены только строчные буквы a-z, цифры 0-9, дефис и подчеркивание", name)
	}
	return nil
}

const poolConfigTemplate = `; Pool configuration for {{ .PoolName }} created by TameStack
[{{ .PoolName }}]
user = {{ .User }}
group = {{ .Group }}

listen = {{ .ListenSocket }}
listen.owner = {{ if .ListenOwner }}{{ .ListenOwner }}{{ else }}{{ .User }}{{ end }}
listen.group = {{ if .ListenGroup }}{{ .ListenGroup }}{{ else }}{{ .Group }}{{ end }}
listen.mode = {{ if .ListenMode }}{{ .ListenMode }}{{ else }}0660{{ end }}

pm = {{ .PmMode }}
pm.max_children = {{ .MaxChildren }}
{{- if eq .PmMode "dynamic" }}
pm.start_servers = {{ if .StartServers }}{{ .StartServers }}{{ else }}2{{ end }}
pm.min_spare_servers = {{ if .MinSpareServers }}{{ .MinSpareServers }}{{ else }}1{{ end }}
pm.max_spare_servers = {{ if .MaxSpareServers }}{{ .MaxSpareServers }}{{ else }}3{{ end }}
{{- else if eq .PmMode "ondemand" }}
pm.process_idle_timeout = {{ if .ProcessIdleTimeout }}{{ .ProcessIdleTimeout }}{{ else }}10s{{ end }}
{{- end }}
{{- if .PmMaxRequests }}
pm.max_requests = {{ .PmMaxRequests }}
{{- end }}

{{- if .Slowlog }}
slowlog = /var/log/php{{ .Version }}-fpm-{{ .PoolName }}.slow.log
request_slowlog_timeout = {{ if .SlowlogTimeout }}{{ .SlowlogTimeout }}{{ else }}5s{{ end }}
{{- end }}

{{- if .AccessLog }}
access.log = {{ .AccessLog }}
{{- end }}

; Per-pool PHP limits
{{- if .MemoryLimit }}
php_admin_value[memory_limit] = {{ .MemoryLimit }}
{{- end }}
{{- if .MaxExecutionTime }}
php_admin_value[max_execution_time] = {{ .MaxExecutionTime }}
{{- end }}
{{- if .UploadMaxFilesize }}
php_admin_value[upload_max_filesize] = {{ .UploadMaxFilesize }}
{{- end }}
{{- if .PostMaxSize }}
php_admin_value[post_max_size] = {{ .PostMaxSize }}
{{- end }}

{{- range $k, $v := .PhpValues }}
php_value[{{ $k }}] = {{ $v }}
{{- end }}

{{- range $k, $v := .PhpAdminValues }}
php_admin_value[{{ $k }}] = {{ $v }}
{{- end }}
`

// RenderPoolConfig renders the INI configuration for an isolated FPM worker pool.
func RenderPoolConfig(in CreatePoolInput) (string, error) {
	tmpl, err := template.New("pool").Parse(poolConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse pool template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, in); err != nil {
		return "", fmt.Errorf("failed to render pool template: %w", err)
	}
	return buf.String(), nil
}

func getPoolDir(version string) string {
	cleanVer := sanitizeVersion(version)
	return fmt.Sprintf("/etc/php/%s/fpm/pool.d", cleanVer)
}

// ListFPMPools inspects active and disabled pool configuration files.
func (s *Service) ListFPMPools(ctx context.Context, serverID, version string) ([]FpmPoolConfig, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cleanVer := sanitizeVersion(version)
	script := fmt.Sprintf(`
POOL_DIR="/etc/php/%s/fpm/pool.d"
if [ ! -d "$POOL_DIR" ]; then
    POOL_DIR="/etc/php-fpm.d"
fi

for f in "$POOL_DIR"/*.conf "$POOL_DIR"/*.conf.disabled; do
    if [ -f "$f" ]; then
        echo "===POOL==="
        FNAME=$(basename "$f")
        echo "FILE:$FNAME"
        if [[ "$FNAME" == *.disabled ]]; then
            echo "ENABLED:no"
            echo "NAME:$(echo "$FNAME" | sed 's/\.conf\.disabled$//')"
        else
            echo "ENABLED:yes"
            echo "NAME:$(basename "$f" .conf)"
        fi
        echo "USER:$(grep -E '^\s*user\s*=' "$f" | awk '{print $3}' || echo 'www-data')"
        echo "GROUP:$(grep -E '^\s*group\s*=' "$f" | awk '{print $3}' || echo 'www-data')"
        echo "LISTEN:$(grep -E '^\s*listen\s*=' "$f" | awk '{print $3}' || echo '')"
        echo "LISTEN_OWNER:$(grep -E '^\s*listen\.owner\s*=' "$f" | awk '{print $3}' || echo '')"
        echo "LISTEN_GROUP:$(grep -E '^\s*listen\.group\s*=' "$f" | awk '{print $3}' || echo '')"
        echo "LISTEN_MODE:$(grep -E '^\s*listen\.mode\s*=' "$f" | awk '{print $3}' || echo '')"
        echo "PM:$(grep -E '^\s*pm\s*=' "$f" | awk '{print $3}' || echo 'dynamic')"
        echo "MAX_CHILDREN:$(grep -E '^\s*pm\.max_children\s*=' "$f" | awk '{print $3}' || echo '5')"
        echo "START_SERVERS:$(grep -E '^\s*pm\.start_servers\s*=' "$f" | awk '{print $3}' || echo '2')"
        echo "MIN_SPARE:$(grep -E '^\s*pm\.min_spare_servers\s*=' "$f" | awk '{print $3}' || echo '1')"
        echo "MAX_SPARE:$(grep -E '^\s*pm\.max_spare_servers\s*=' "$f" | awk '{print $3}' || echo '3')"
        echo "MAX_REQUESTS:$(grep -E '^\s*pm\.max_requests\s*=' "$f" | awk '{print $3}' || echo '0')"
        echo "SLOWLOG:$(grep -E '^\s*slowlog\s*=' "$f" | head -n1 || echo '')"
        echo "SLOWLOG_TIMEOUT:$(grep -E '^\s*request_slowlog_timeout\s*=' "$f" | awk '{print $3}' || echo '')"
        echo "ACCESS_LOG:$(grep -E '^\s*access\.log\s*=' "$f" | awk '{print $3}' || echo '')"
        echo "MEM_LIMIT:$(grep -E 'php_admin_value\[memory_limit\]' "$f" | awk '{print $3}' || echo '')"
        echo "MAX_EXEC:$(grep -E 'php_admin_value\[max_execution_time\]' "$f" | awk '{print $3}' || echo '')"
        echo "UPLOAD_MAX:$(grep -E 'php_admin_value\[upload_max_filesize\]' "$f" | awk '{print $3}' || echo '')"
        echo "POST_MAX:$(grep -E 'php_admin_value\[post_max_size\]' "$f" | awk '{print $3}' || echo '')"
    fi
done
`, cleanVer)

	out, _, _, err := client.Exec(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect FPM pools: %w", err)
	}

	var pools []FpmPoolConfig
	for _, sec := range strings.Split(string(out), "===POOL===") {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}

		p := FpmPoolConfig{
			IsEnabled:   true,
			MaxChildren: 5,
			PmMode:      "dynamic",
			User:        "www-data",
			Group:       "www-data",
		}

		for _, line := range strings.Split(sec, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "NAME:") {
				p.PoolName = strings.TrimPrefix(line, "NAME:")
			} else if strings.HasPrefix(line, "ENABLED:") {
				p.IsEnabled = strings.TrimPrefix(line, "ENABLED:") == "yes"
			} else if strings.HasPrefix(line, "USER:") {
				p.User = strings.TrimPrefix(line, "USER:")
			} else if strings.HasPrefix(line, "GROUP:") {
				p.Group = strings.TrimPrefix(line, "GROUP:")
			} else if strings.HasPrefix(line, "LISTEN:") {
				p.Listen = strings.TrimPrefix(line, "LISTEN:")
			} else if strings.HasPrefix(line, "LISTEN_OWNER:") {
				p.ListenOwner = strings.TrimPrefix(line, "LISTEN_OWNER:")
			} else if strings.HasPrefix(line, "LISTEN_GROUP:") {
				p.ListenGroup = strings.TrimPrefix(line, "LISTEN_GROUP:")
			} else if strings.HasPrefix(line, "LISTEN_MODE:") {
				p.ListenMode = strings.TrimPrefix(line, "LISTEN_MODE:")
			} else if strings.HasPrefix(line, "PM:") {
				p.PmMode = strings.TrimPrefix(line, "PM:")
			} else if strings.HasPrefix(line, "MAX_CHILDREN:") {
				p.MaxChildren, _ = strconv.Atoi(strings.TrimPrefix(line, "MAX_CHILDREN:"))
			} else if strings.HasPrefix(line, "START_SERVERS:") {
				p.StartServers, _ = strconv.Atoi(strings.TrimPrefix(line, "START_SERVERS:"))
			} else if strings.HasPrefix(line, "MIN_SPARE:") {
				p.MinSpareServers, _ = strconv.Atoi(strings.TrimPrefix(line, "MIN_SPARE:"))
			} else if strings.HasPrefix(line, "MAX_SPARE:") {
				p.MaxSpareServers, _ = strconv.Atoi(strings.TrimPrefix(line, "MAX_SPARE:"))
			} else if strings.HasPrefix(line, "MAX_REQUESTS:") {
				p.PmMaxRequests, _ = strconv.Atoi(strings.TrimPrefix(line, "MAX_REQUESTS:"))
			} else if strings.HasPrefix(line, "SLOWLOG:") {
				p.Slowlog = strings.TrimPrefix(line, "SLOWLOG:") != ""
			} else if strings.HasPrefix(line, "SLOWLOG_TIMEOUT:") {
				p.SlowlogTimeout = strings.TrimPrefix(line, "SLOWLOG_TIMEOUT:")
			} else if strings.HasPrefix(line, "ACCESS_LOG:") {
				p.AccessLog = strings.TrimPrefix(line, "ACCESS_LOG:")
			} else if strings.HasPrefix(line, "MEM_LIMIT:") {
				p.MemoryLimit = strings.TrimPrefix(line, "MEM_LIMIT:")
			} else if strings.HasPrefix(line, "MAX_EXEC:") {
				p.MaxExecutionTime, _ = strconv.Atoi(strings.TrimPrefix(line, "MAX_EXEC:"))
			} else if strings.HasPrefix(line, "UPLOAD_MAX:") {
				p.UploadMaxFilesize = strings.TrimPrefix(line, "UPLOAD_MAX:")
			} else if strings.HasPrefix(line, "POST_MAX:") {
				p.PostMaxSize = strings.TrimPrefix(line, "POST_MAX:")
			}
		}

		if p.PoolName != "" {
			pools = append(pools, p)
		}
	}

	return pools, nil
}

// CreatePool provisions an isolated PHP-FPM worker pool with syntax checking and rollback.
func (s *Service) CreatePool(ctx context.Context, in CreatePoolInput) (*FpmPoolConfig, error) {
	if err := ValidatePoolName(in.PoolName); err != nil {
		return nil, err
	}
	cleanVer := sanitizeVersion(in.Version)
	if cleanVer == "" {
		return nil, fmt.Errorf("не указана версия PHP")
	}

	if in.User == "" {
		in.User = "www-data"
	}
	if in.Group == "" {
		in.Group = "www-data"
	}
	if in.PmMode == "" {
		in.PmMode = "ondemand"
	}
	if in.MaxChildren <= 0 {
		in.MaxChildren = 10
	}
	if in.ListenSocket == "" {
		in.ListenSocket = fmt.Sprintf("/run/php/php%s-fpm-%s.sock", cleanVer, in.PoolName)
	}

	// Warn or reject binding to public 0.0.0.0 without explicit confirmation.
	if strings.HasPrefix(in.ListenSocket, "0.0.0.0:") {
		return nil, fmt.Errorf("открытие сокета FPM на 0.0.0.0 небезопасно: используйте Unix socket (по умолчанию) или локальный адрес 127.0.0.1")
	}

	content, err := RenderPoolConfig(in)
	if err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, in.ServerID)
	if err != nil {
		return nil, err
	}

	poolDir := getPoolDir(cleanVer)
	poolFilePath := filepath.ToSlash(filepath.Join(poolDir, fmt.Sprintf("%s.conf", in.PoolName)))
	testCmd := fmt.Sprintf("php-fpm%s -t 2>&1 || php-fpm -t 2>&1", cleanVer)
	reloadCmd := fmt.Sprintf("systemctl reload php%s-fpm 2>/dev/null || systemctl restart php%s-fpm 2>/dev/null", cleanVer, cleanVer)

	guardRes, err := client.SafeWriteWithTest(ctx, poolFilePath, []byte(content), testCmd, reloadCmd)
	if err != nil {
		errMsg := err.Error()
		if guardRes != nil && guardRes.TestOutput != "" {
			errMsg = fmt.Sprintf("%s: %s", errMsg, guardRes.TestOutput)
		}
		return nil, fmt.Errorf("ошибка создания пула PHP-FPM (откачено): %s", errMsg)
	}

	return &FpmPoolConfig{
		PoolName:    in.PoolName,
		IsEnabled:   true,
		User:        in.User,
		Group:       in.Group,
		Listen:      in.ListenSocket,
		PmMode:      in.PmMode,
		MaxChildren: in.MaxChildren,
	}, nil
}

// UpdatePool modifies an existing PHP-FPM pool configuration.
func (s *Service) UpdatePool(ctx context.Context, in CreatePoolInput) (*FpmPoolConfig, error) {
	return s.CreatePool(ctx, in)
}

// TogglePool enables or disables a pool by renaming its configuration file.
func (s *Service) TogglePool(ctx context.Context, serverID, version, poolName string, enable bool) error {
	if err := ValidatePoolName(poolName); err != nil {
		return err
	}
	cleanVer := sanitizeVersion(version)
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	poolDir := getPoolDir(cleanVer)
	activePath := filepath.ToSlash(filepath.Join(poolDir, fmt.Sprintf("%s.conf", poolName)))
	disabledPath := filepath.ToSlash(filepath.Join(poolDir, fmt.Sprintf("%s.conf.disabled", poolName)))

	var cmd string
	if enable {
		cmd = fmt.Sprintf("if [ -f %s ]; then mv %s %s && (php-fpm%s -t && (systemctl reload php%s-fpm 2>/dev/null || systemctl restart php%s-fpm 2>/dev/null)); fi",
			ssh.ShellEscape(disabledPath), ssh.ShellEscape(disabledPath), ssh.ShellEscape(activePath), cleanVer, cleanVer, cleanVer)
	} else {
		cmd = fmt.Sprintf("if [ -f %s ]; then mv %s %s && (systemctl reload php%s-fpm 2>/dev/null || systemctl restart php%s-fpm 2>/dev/null); fi",
			ssh.ShellEscape(activePath), ssh.ShellEscape(activePath), ssh.ShellEscape(disabledPath), cleanVer, cleanVer)
	}

	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to toggle pool %s: %s %s", poolName, string(out), string(stderr))
	}
	return nil
}

// ClonePool creates a copy of an existing pool with a new name and socket.
func (s *Service) ClonePool(ctx context.Context, serverID, version, sourcePool, targetPool string) (*FpmPoolConfig, error) {
	if err := ValidatePoolName(sourcePool); err != nil {
		return nil, fmt.Errorf("исходный пул недопустим: %w", err)
	}
	if err := ValidatePoolName(targetPool); err != nil {
		return nil, fmt.Errorf("новое имя пула недопустимо: %w", err)
	}
	cleanVer := sanitizeVersion(version)
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	poolDir := getPoolDir(cleanVer)
	srcPath := filepath.ToSlash(filepath.Join(poolDir, fmt.Sprintf("%s.conf", sourcePool)))
	srcData, err := client.ReadFile(ctx, srcPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать исходный пул %s: %w", sourcePool, err)
	}

	content := string(srcData)
	// Replace pool header
	content = strings.Replace(content, fmt.Sprintf("[%s]", sourcePool), fmt.Sprintf("[%s]", targetPool), 1)
	// Replace default socket path if present
	content = strings.Replace(content, fmt.Sprintf("-%s.sock", sourcePool), fmt.Sprintf("-%s.sock", targetPool), 1)

	tgtPath := filepath.ToSlash(filepath.Join(poolDir, fmt.Sprintf("%s.conf", targetPool)))
	testCmd := fmt.Sprintf("php-fpm%s -t 2>&1 || php-fpm -t 2>&1", cleanVer)
	reloadCmd := fmt.Sprintf("systemctl reload php%s-fpm 2>/dev/null || systemctl restart php%s-fpm 2>/dev/null", cleanVer, cleanVer)

	guardRes, err := client.SafeWriteWithTest(ctx, tgtPath, []byte(content), testCmd, reloadCmd)
	if err != nil {
		errMsg := err.Error()
		if guardRes != nil && guardRes.TestOutput != "" {
			errMsg = fmt.Sprintf("%s: %s", errMsg, guardRes.TestOutput)
		}
		return nil, fmt.Errorf("ошибка клонирования пула (откачено): %s", errMsg)
	}

	return &FpmPoolConfig{
		PoolName:  targetPool,
		IsEnabled: true,
	}, nil
}

// DeletePool removes a custom PHP-FPM worker pool and reloads the service.
func (s *Service) DeletePool(ctx context.Context, serverID, version, poolName string) error {
	if err := ValidatePoolName(poolName); err != nil {
		return err
	}
	cleanVer := sanitizeVersion(version)
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	poolDir := getPoolDir(cleanVer)
	poolFilePath := filepath.ToSlash(filepath.Join(poolDir, fmt.Sprintf("%s.conf", poolName)))
	disabledFilePath := filepath.ToSlash(filepath.Join(poolDir, fmt.Sprintf("%s.conf.disabled", poolName)))
	delCmd := fmt.Sprintf("rm -f %s %s && (systemctl reload php%s-fpm 2>/dev/null || systemctl restart php%s-fpm 2>/dev/null)",
		ssh.ShellEscape(poolFilePath), ssh.ShellEscape(disabledFilePath), cleanVer, cleanVer)
	_, stderr, code, err := client.NewCommand(delCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("ошибка удаления пула: %s", string(stderr))
	}
	return nil
}
