package webserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// Service manages Nginx and Apache web servers.
type Service struct {
	module.Base
	engine *tasks.Engine
}

// NewService instantiates WebServer Service and registers task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}

	engine.RegisterHandler("webserver_control", s.handleControlTask)
	engine.RegisterHandler("webserver_install", s.handleInstallTask)

	return s
}

// InspectWebServers checks installation and service status of Nginx and Apache.
func (s *Service) InspectWebServers(ctx context.Context, serverID string) ([]WebServerStatus, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	script := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

echo "===NGINX==="
NGINX_BIN=""
MASTER_CMD=$(ps -eo args 2>/dev/null | grep -E '[n]ginx: master process' | head -n1 || true)
PROC_BIN=""
PROC_CONF=""
if [ -n "$MASTER_CMD" ]; then
    PROC_BIN=$(echo "$MASTER_CMD" | sed -n 's/.*nginx: master process \([^ ]*\).*/\1/p' || true)
    PROC_CONF=$(echo "$MASTER_CMD" | sed -n 's/.*-c \([^ ]*\).*/\1/p' || true)
fi
if [ -n "$PROC_BIN" ] && [ -x "$PROC_BIN" ]; then
    NGINX_BIN="$PROC_BIN"
fi

if [ -z "$NGINX_BIN" ]; then
    UNIT_EXEC=$(systemctl cat nginx.service 2>/dev/null | grep -E '^ExecStart=' | awk '{print $1}' | cut -d= -f2 || true)
    if [ -n "$UNIT_EXEC" ] && [ -x "$UNIT_EXEC" ]; then
        NGINX_BIN="$UNIT_EXEC"
    fi
fi

if [ -z "$NGINX_BIN" ]; then
    for b in $(ls -1d /opt/*/sbin/nginx /opt/*/bin/nginx 2>/dev/null | sort -V -r || true) nginx /usr/sbin/nginx /usr/bin/nginx /usr/local/nginx/sbin/nginx /usr/local/sbin/nginx /usr/local/bin/nginx /opt/nginx/sbin/nginx /opt/cpanel/ea-nginx/sbin/nginx /snap/bin/nginx; do
        if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then
            NGINX_BIN="$b"
            break
        fi
    done
fi

NGINX_INSTALLED="no"
NGINX_STATUS="inactive"
if [ -n "$NGINX_BIN" ]; then
    NGINX_INSTALLED="yes"
    for s in nginx openresty ea-nginx; do
        if systemctl is-active --quiet "$s" 2>/dev/null; then
            NGINX_STATUS="active"
            break
        fi
    done
    if [ "$NGINX_STATUS" = "inactive" ]; then
        if pgrep -x nginx >/dev/null 2>&1 || pidof nginx >/dev/null 2>&1 || [ -n "$PROC_BIN" ]; then
            NGINX_STATUS="active"
        fi
    fi
fi

if [ "$NGINX_INSTALLED" = "yes" ]; then
    echo "INSTALLED:yes"
    echo "STATUS:$NGINX_STATUS"

    # Version
    VER=""
    if [ -n "$NGINX_BIN" ]; then
        VER=$($NGINX_BIN -v 2>&1 | cut -d/ -f2 || true)
    elif command -v nginx >/dev/null 2>&1; then
        VER=$(nginx -v 2>&1 | cut -d/ -f2 || true)
    fi
    echo "VERSION:$VER"

    # Prefix
    PREFIX=""
    if [ -n "$NGINX_BIN" ]; then
        PREFIX=$($NGINX_BIN -V 2>&1 | grep -oE -- '--prefix=[^ ]+' | cut -d= -f2 || true)
        if [ -z "$PREFIX" ]; then
            case "$NGINX_BIN" in
                /opt/*|/usr/local/*)
                    BIN_D=$(dirname "$NGINX_BIN")
                    PREFIX=$(dirname "$BIN_D")
                    ;;
            esac
        fi
    fi

    # Config Path
    CONFIG_PATH=""
    if [ -n "$PROC_CONF" ] && [ -f "$PROC_CONF" ]; then
        CONFIG_PATH="$PROC_CONF"
    fi
    if [ -z "$CONFIG_PATH" ] && [ -n "$NGINX_BIN" ]; then
        CP=$($NGINX_BIN -V 2>&1 | grep -oE -- '--conf-path=[^ ]+' | cut -d= -f2 || true)
        if [ -n "$CP" ] && [ -f "$CP" ]; then
            CONFIG_PATH="$CP"
        fi
    fi
    if [ -z "$CONFIG_PATH" ] && [ -n "$PREFIX" ] && [ -f "$PREFIX/conf/nginx.conf" ]; then
        CONFIG_PATH="$PREFIX/conf/nginx.conf"
    fi
    if [ -z "$CONFIG_PATH" ] && [ -n "$NGINX_BIN" ]; then
        NT_CONF=$($NGINX_BIN -t 2>&1 | grep -oE 'configuration file [^ ]+' | awk '{print $3}' || true)
        if [ -n "$NT_CONF" ] && [ -f "$NT_CONF" ]; then
            CONFIG_PATH="$NT_CONF"
        fi
    fi
    if [ -z "$CONFIG_PATH" ] || [ ! -f "$CONFIG_PATH" ]; then
        for cf in /etc/nginx/nginx.conf /usr/local/nginx/conf/nginx.conf /etc/openresty/nginx.conf; do
            if [ -f "$cf" ]; then
                CONFIG_PATH="$cf"
                break
            fi
        done
    fi
    if [ -z "$CONFIG_PATH" ]; then
        CONFIG_PATH="/etc/nginx/nginx.conf"
    fi
    echo "CONFIG:$CONFIG_PATH"

    # Test syntax
    TEST_OUT=""
    TEST_CONF="$CONFIG_PATH"
    if [ -n "$PROC_CONF" ] && [ -f "$PROC_CONF" ]; then
        TEST_CONF="$PROC_CONF"
    fi
    if [ -n "$NGINX_BIN" ] && [ -x "$NGINX_BIN" ]; then
        if [ -n "$TEST_CONF" ] && [ -f "$TEST_CONF" ]; then
            if [ -n "$PREFIX" ]; then
                TEST_OUT=$($NGINX_BIN -t -c "$TEST_CONF" -p "$PREFIX" 2>&1 || true)
            else
                TEST_OUT=$($NGINX_BIN -t -c "$TEST_CONF" 2>&1 || true)
            fi
        else
            TEST_OUT=$($NGINX_BIN -t 2>&1 || true)
        fi
        if ! echo "$TEST_OUT" | grep -qi "syntax.*ok"; then
            TEST_OUT=$($NGINX_BIN -t 2>&1 || true)
        fi
    else
        TEST_OUT=$(nginx -t 2>&1 || true)
    fi

    if (echo "$TEST_OUT" | grep -qi "syntax.*ok" || echo "$TEST_OUT" | grep -qi "test is successful") && ! echo "$TEST_OUT" | grep -qi "test failed"; then
        echo "TEST_PASS:yes"
    else
        echo "TEST_PASS:no"
    fi
    echo "TEST_OUT:${TEST_OUT}"
else
    echo "INSTALLED:no"
    echo "STATUS:not_installed"
fi

echo "===APACHE==="
APACHE_BIN=""
for b in apache2 httpd /usr/sbin/apache2 /usr/sbin/httpd /usr/local/apache2/bin/httpd /opt/*/bin/httpd /usr/bin/apache2 /usr/bin/httpd; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then
        APACHE_BIN="$b"
        break
    fi
done

APACHE_INSTALLED="no"
APACHE_STATUS="inactive"
if [ -n "$APACHE_BIN" ]; then
    APACHE_INSTALLED="yes"
    for s in apache2 httpd apache; do
        if systemctl is-active --quiet "$s" 2>/dev/null; then
            APACHE_STATUS="active"
            break
        fi
    done
    if [ "$APACHE_STATUS" = "inactive" ]; then
        if pgrep -x apache2 >/dev/null 2>&1 || pgrep -x httpd >/dev/null 2>&1 || pidof apache2 >/dev/null 2>&1 || pidof httpd >/dev/null 2>&1; then
            APACHE_STATUS="active"
        fi
    fi
fi

if [ "$APACHE_INSTALLED" = "yes" ]; then
    echo "INSTALLED:yes"
    echo "STATUS:$APACHE_STATUS"
    
    # Version
    VER=""
    if [ -n "$APACHE_BIN" ]; then
        VER=$($APACHE_BIN -v 2>&1 | head -n 1 | cut -d/ -f2 | awk '{print $1}' || true)
    elif which apache2 >/dev/null 2>&1; then
        VER=$(apache2 -v 2>&1 | head -n 1 | cut -d/ -f2 | awk '{print $1}' || true)
    elif which httpd >/dev/null 2>&1; then
        VER=$(httpd -v 2>&1 | head -n 1 | cut -d/ -f2 | awk '{print $1}' || true)
    fi
    echo "VERSION:$VER"

    # Config Path
    CONFIG_PATH=""
    if [ -n "$APACHE_BIN" ]; then
        ROOT=$($APACHE_BIN -V 2>&1 | grep -oE -- '-D HTTPD_ROOT="[^"]+"' | cut -d'"' -f2 || true)
        CONF=$($APACHE_BIN -V 2>&1 | grep -oE -- '-D SERVER_CONFIG_FILE="[^"]+"' | cut -d'"' -f2 || true)
        if [ -n "$ROOT" ] && [ -n "$CONF" ]; then
            case "$CONF" in
                /*) CONFIG_PATH="$CONF" ;;
                *) CONFIG_PATH="$ROOT/$CONF" ;;
            esac
        fi
    fi
    if [ -z "$CONFIG_PATH" ] || [ ! -f "$CONFIG_PATH" ]; then
        for cf in /etc/apache2/apache2.conf /etc/httpd/conf/httpd.conf /etc/apache2/conf/httpd.conf /etc/httpd/httpd.conf; do
            if [ -f "$cf" ]; then
                CONFIG_PATH="$cf"
                break
            fi
        done
    fi
    if [ -z "$CONFIG_PATH" ]; then
        CONFIG_PATH="/etc/apache2/apache2.conf"
    fi
    echo "CONFIG:$CONFIG_PATH"

    # Test syntax
    TEST_CMD=""
    for c in apache2ctl apachectl /usr/sbin/apache2ctl /usr/sbin/apachectl "$APACHE_BIN"; do
        if [ -n "$c" ] && (command -v "$c" >/dev/null 2>&1 || [ -x "$c" ]); then
            TEST_CMD="$c"
            break
        fi
    done

    TEST_OUT=""
    if [ -n "$TEST_CMD" ]; then
        TEST_OUT=$($TEST_CMD configtest 2>&1 || $TEST_CMD -t 2>&1 || true)
    fi

    if echo "$TEST_OUT" | grep -qi "syntax ok"; then
        echo "TEST_PASS:yes"
    else
        echo "TEST_PASS:no"
    fi
    echo "TEST_OUT:${TEST_OUT}"
else
    echo "INSTALLED:no"
    echo "STATUS:not_installed"
fi
`
	out, _, _, err := client.NewCommand(script).WithSudo().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect web servers: %w", err)
	}

	var results []WebServerStatus
	sections := strings.Split(string(out), "===")

	for i := 1; i < len(sections); i += 2 {
		tag := strings.TrimSpace(sections[i])
		val := strings.TrimSpace(sections[i+1])

		var name, displayName string
		if tag == "NGINX" {
			name = "nginx"
			displayName = "Nginx Web Server"
		} else if tag == "APACHE" {
			name = "apache2"
			displayName = "Apache HTTP Server"
		} else {
			continue
		}

		ws := WebServerStatus{
			Name:        name,
			DisplayName: displayName,
		}

		lines := strings.Split(val, "\n")
		var testOutLines []string
		isTestOut := false

		for _, l := range lines {
			lTrim := strings.TrimSpace(l)
			if strings.HasPrefix(lTrim, "INSTALLED:") {
				ws.IsInstalled = strings.TrimPrefix(lTrim, "INSTALLED:") == "yes"
			} else if strings.HasPrefix(lTrim, "STATUS:") {
				ws.Status = strings.TrimPrefix(lTrim, "STATUS:")
			} else if strings.HasPrefix(lTrim, "VERSION:") {
				ws.Version = strings.TrimPrefix(lTrim, "VERSION:")
			} else if strings.HasPrefix(lTrim, "CONFIG:") {
				ws.ConfigPath = strings.TrimPrefix(lTrim, "CONFIG:")
			} else if strings.HasPrefix(lTrim, "TEST_PASS:") {
				ws.TestPassed = strings.TrimPrefix(lTrim, "TEST_PASS:") == "yes"
			} else if strings.HasPrefix(lTrim, "TEST_OUT:") {
				isTestOut = true
				testOutLines = append(testOutLines, strings.TrimPrefix(lTrim, "TEST_OUT:"))
			} else if isTestOut {
				testOutLines = append(testOutLines, l)
			}
		}

		ws.TestOutput = strings.Join(testOutLines, "\n")
		lowerTest := strings.ToLower(ws.TestOutput)
		if (strings.Contains(lowerTest, "syntax is ok") || strings.Contains(lowerTest, "syntax ok") || strings.Contains(lowerTest, "test is successful")) && !strings.Contains(lowerTest, "test failed") && !strings.Contains(lowerTest, "[emerg]") && !strings.Contains(lowerTest, "[error]") {
			ws.TestPassed = true
		}
		results = append(results, ws)
	}

	return results, nil
}

// TestConfig runs syntax check for Nginx or Apache.
func (s *Service) TestConfig(ctx context.Context, serverID, webserverType string) (bool, string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return false, "", err
	}

	var cmd string
	if webserverType == "nginx" {
		cmd = `export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
BIN=""
CONF=""
MASTER_CMD=$(ps -eo args 2>/dev/null | grep -E '[n]ginx: master process' | head -n1 || true)
if [ -n "$MASTER_CMD" ]; then
    BIN=$(echo "$MASTER_CMD" | sed -n 's/.*nginx: master process \([^ ]*\).*/\1/p' || true)
    CONF=$(echo "$MASTER_CMD" | sed -n 's/.*-c \([^ ]*\).*/\1/p' || true)
fi
if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
    UNIT_EXEC=$(systemctl cat nginx.service 2>/dev/null | grep -E '^ExecStart=' | awk '{print $1}' | cut -d= -f2 || true)
    if [ -n "$UNIT_EXEC" ] && [ -x "$UNIT_EXEC" ]; then
        BIN="$UNIT_EXEC"
    fi
fi
if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
    for b in $(ls -1d /opt/*/sbin/nginx /opt/*/bin/nginx 2>/dev/null | sort -V -r || true) nginx /usr/sbin/nginx /opt/*/sbin/nginx /usr/local/nginx/sbin/nginx /snap/bin/nginx; do
        if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then BIN="$b"; break; fi
    done
fi
if [ -z "$BIN" ]; then echo "nginx: binary not found"; exit 1; fi

PREFIX=$($BIN -V 2>&1 | grep -oE -- '--prefix=[^ ]+' | cut -d= -f2 || true)
if [ -z "$PREFIX" ]; then
    case "$BIN" in
        /opt/*|/usr/local/*)
            BIN_D=$(dirname "$BIN")
            PREFIX=$(dirname "$BIN_D")
            ;;
    esac
fi

if [ -z "$CONF" ] || [ ! -f "$CONF" ]; then
    CP=$($BIN -V 2>&1 | grep -oE -- '--conf-path=[^ ]+' | cut -d= -f2 || true)
    if [ -n "$CP" ] && [ -f "$CP" ]; then
        CONF="$CP"
    elif [ -n "$PREFIX" ] && [ -f "$PREFIX/conf/nginx.conf" ]; then
        CONF="$PREFIX/conf/nginx.conf"
    else
        for cf in /etc/nginx/nginx.conf /usr/local/nginx/conf/nginx.conf /etc/openresty/nginx.conf; do
            if [ -f "$cf" ]; then CONF="$cf"; break; fi
        done
    fi
fi

if [ -n "$CONF" ] && [ -f "$CONF" ]; then
    if [ -n "$PREFIX" ]; then
        $BIN -t -c "$CONF" -p "$PREFIX" 2>&1
    else
        $BIN -t -c "$CONF" 2>&1
    fi
else
    $BIN -t 2>&1
fi`
	} else {
		cmd = `export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
BIN=""
for b in apache2ctl apachectl /usr/sbin/apache2ctl /usr/sbin/apachectl httpd /usr/sbin/httpd /opt/*/bin/httpd; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then BIN="$b"; break; fi
done
if [ -z "$BIN" ]; then echo "apache: binary not found"; exit 1; fi
case "$BIN" in *ctl) $BIN configtest 2>&1;; *) $BIN -t 2>&1;; esac`
	}

	out, stderr, code, _ := client.NewCommand(cmd).WithSudo().Exec(ctx)
	outStr := string(out)
	if outStr == "" && len(stderr) > 0 {
		outStr = string(stderr)
	} else if len(stderr) > 0 {
		outStr = outStr + "\n" + string(stderr)
	}
	lowerOut := strings.ToLower(outStr)
	passed := (code == 0 || strings.Contains(lowerOut, "syntax is ok") || strings.Contains(lowerOut, "syntax ok") || strings.Contains(lowerOut, "test is successful")) && !strings.Contains(lowerOut, "test failed") && !strings.Contains(lowerOut, "[emerg]") && !strings.Contains(lowerOut, "[error]")
	return passed, outStr, nil
}

// ControlService starts, stops, reloads, or restarts the web server.
func (s *Service) ControlService(ctx context.Context, serverID, webserverType, action string) error {
	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}
	if !validActions[action] {
		return fmt.Errorf("invalid action: %s", action)
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	var cmd string
	if webserverType == "nginx" {
		cmd = fmt.Sprintf(`ACTION=%q
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
for svc in nginx openresty ea-nginx; do
    if systemctl is-active --quiet "$svc" 2>/dev/null || systemctl list-unit-files "$svc.service" 2>/dev/null | grep -q "$svc"; then
        systemctl $ACTION "$svc"
        exit 0
    fi
done
systemctl $ACTION nginx 2>/dev/null || service nginx $ACTION 2>/dev/null
`, action)
	} else {
		cmd = fmt.Sprintf(`ACTION=%q
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
for svc in apache2 httpd apache; do
    if systemctl is-active --quiet "$svc" 2>/dev/null || systemctl list-unit-files "$svc.service" 2>/dev/null | grep -q "$svc"; then
        systemctl $ACTION "$svc"
        exit 0
    fi
done
systemctl $ACTION apache2 2>/dev/null || systemctl $ACTION httpd 2>/dev/null || service apache2 $ACTION 2>/dev/null || service httpd $ACTION 2>/dev/null
`, action)
	}

	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("control %s %s failed (code %d): %s %s", webserverType, action, code, string(out), string(stderr))
	}

	return nil
}

// GetConfig reads the web server configuration file via SFTP.
func (s *Service) GetConfig(ctx context.Context, serverID, remotePath string) (string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	data, err := client.ReadFile(ctx, remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", remotePath, err)
	}
	return string(data), nil
}

// SaveConfig backs up, saves, tests syntax, and applies changes.
func (s *Service) SaveConfig(ctx context.Context, serverID, remotePath, content, webserverType string) error {
	webserverType, err := normalizeWebServerType(webserverType)
	if err != nil {
		return err
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	backupPath := fmt.Sprintf("%s.bak.%d", remotePath, time.Now().Unix())
	tmpPath := fmt.Sprintf("/tmp/vhost.test.%d.conf", time.Now().Unix())

	_, _, _, _ = client.NewCommand(fmt.Sprintf("cp %s %s 2>/dev/null", ssh.ShellEscape(remotePath), ssh.ShellEscape(backupPath))).WithSudo().Exec(ctx)

	if err := client.WriteFile(ctx, tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	_, _, _, _ = client.NewCommand(fmt.Sprintf("cp %s %s", ssh.ShellEscape(tmpPath), ssh.ShellEscape(remotePath))).WithSudo().Exec(ctx)

	passed, testOut, _ := s.TestConfig(ctx, serverID, webserverType)
	if !passed {
		// Rollback
		_, _, _, _ = client.NewCommand(fmt.Sprintf("cp %s %s 2>/dev/null || rm -f %s", ssh.ShellEscape(backupPath), ssh.ShellEscape(remotePath), ssh.ShellEscape(remotePath))).WithSudo().Exec(ctx)
		_ = client.RemovePath(ctx, tmpPath, false)
		return fmt.Errorf("configuration syntax test failed:\n%s", testOut)
	}

	_ = client.RemovePath(ctx, tmpPath, false)
	return s.ControlService(ctx, serverID, webserverType, "reload")
}

// GetTopology scans and returns the web server architecture and PHP runtime topology.
func (s *Service) GetTopology(ctx context.Context, serverID string) (*WebStackTopology, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return DetectTopology(ctx, client)
}
