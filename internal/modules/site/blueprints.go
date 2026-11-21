package site

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/tasks"

	"github.com/google/uuid"
)

type BlueprintType string

const (
	BlueprintNodeJS   BlueprintType = "nodejs"
	BlueprintPython   BlueprintType = "python"
	BlueprintLaravel  BlueprintType = "laravel"
	BlueprintGoBinary BlueprintType = "go_binary"
	BlueprintStatic   BlueprintType = "static"
)

type BlueprintSpec struct {
	Type           BlueprintType     `json:"type"`
	Domain         string            `json:"domain"`
	AppPort        int               `json:"app_port"`
	NodeVersion    string            `json:"node_version,omitempty"`
	PHPVersion     string            `json:"php_version,omitempty"`
	PythonVersion  string            `json:"python_version,omitempty"`
	EntryFile      string            `json:"entry_file,omitempty"`
	WorkDir        string            `json:"work_dir,omitempty"`
	ExecCommand    string            `json:"exec_command,omitempty"`
	User           string            `json:"user,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	InstallDeps    bool              `json:"install_deps"`
	UseSystemdUnit bool              `json:"use_systemd_unit"`
}

// GenerateSystemdService creates a production-grade systemd service unit file with auto-restart.
func GenerateSystemdService(serviceName, execPath, workDir, user string, envVars map[string]string) string {
	if user == "" {
		user = "www-data"
	}

	var envLines []string
	var keys []string
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		envLines = append(envLines, fmt.Sprintf("Environment=\"%s=%s\"", k, envVars[k]))
	}

	envBlock := ""
	if len(envLines) > 0 {
		envBlock = strings.Join(envLines, "\n") + "\n"
	}

	return fmt.Sprintf(`[Unit]
Description=TameStack Managed App (%s)
After=network.target

[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=3
%sStandardOutput=journal
StandardError=journal
SyslogIdentifier=%s

[Install]
WantedBy=multi-user.target
`, serviceName, user, workDir, execPath, envBlock, serviceName)
}

// GenerateReverseProxyVhost generates a modern Nginx reverse proxy configuration with WebSocket support.
func GenerateReverseProxyVhost(domain string, appPort int, enableWebSocket bool) string {
	wsBlock := ""
	if enableWebSocket {
		wsBlock = `        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";`
	}

	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
%s
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        proxy_read_timeout 300s;
        proxy_connect_timeout 60s;
    }
}
`, domain, domain, domain, appPort, wsBlock)
}

// ApplyBlueprint schedules a background task to configure and deploy a blueprint stack.
func (s *Service) ApplyBlueprint(ctx context.Context, serverID string, spec BlueprintSpec) (*db.TaskRecord, error) {
	payloadBytes, err := json.Marshal(struct {
		ServerID string        `json:"server_id"`
		Spec     BlueprintSpec `json:"spec"`
	}{
		ServerID: serverID,
		Spec:     spec,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blueprint payload: %w", err)
	}

	title := fmt.Sprintf("Apply blueprint %s for %s", spec.Type, spec.Domain)
	return s.engine.Submit(serverID, "site_blueprint", title, string(payloadBytes))
}

// handleBlueprintTask provisions the application blueprint on the remote server.
func (s *Service) handleBlueprintTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var in struct {
		ServerID string        `json:"server_id"`
		Spec     BlueprintSpec `json:"spec"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &in); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	logger.SetStep("Connecting to server", 15)
	client, _, err := s.GetSSHClient(ctx, in.ServerID)
	if err != nil {
		return "", err
	}

	spec := in.Spec
	workDir := spec.WorkDir
	if workDir == "" {
		workDir = fmt.Sprintf("/var/www/%s", spec.Domain)
	}

	logger.SetStep("Preparing work directory and permissions", 30)
	_ = client.EnsureDirectory(ctx, workDir, 0755, "www-data:www-data")

	if spec.UseSystemdUnit && spec.ExecCommand != "" {
		logger.SetStep("Configuring systemd service unit", 50)
		serviceName := fmt.Sprintf("sh-%s", strings.ReplaceAll(spec.Domain, ".", "-"))
		unitContent := GenerateSystemdService(serviceName, spec.ExecCommand, workDir, spec.User, spec.EnvVars)
		unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

		if err := client.WriteFile(ctx, unitPath, []byte(unitContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write systemd unit: %w", err)
		}

		_, _, _, _ = client.NewCommand("systemctl daemon-reload").WithSudo().Exec(ctx)
		_, _, _, _ = client.NewCommand(fmt.Sprintf("systemctl enable --now %s", serviceName)).WithSudo().Exec(ctx)
		logger.Info(fmt.Sprintf("Systemd service %s enabled and started.", serviceName))
	}

	if spec.AppPort > 0 {
		logger.SetStep("Configuring Nginx Reverse Proxy", 75)
		vhostContent := GenerateReverseProxyVhost(spec.Domain, spec.AppPort, true)
		vhostPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", spec.Domain)
		symlinkPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", spec.Domain)

		testCmd := "nginx -t"
		reloadCmd := "systemctl reload nginx"

		res, err := client.SafeWriteWithTest(ctx, vhostPath, []byte(vhostContent), testCmd, reloadCmd)
		if err != nil {
			return "", fmt.Errorf("failed to configure reverse proxy: %s", res.ErrorMessage)
		}

		_, _, _, _ = client.NewCommand(fmt.Sprintf("ln -sf %s %s", vhostPath, symlinkPath)).WithSudo().Exec(ctx)
		_, _, _, _ = client.NewCommand(reloadCmd).WithSudo().Exec(ctx)
		logger.Info(fmt.Sprintf("Nginx reverse proxy configured for %s -> 127.0.0.1:%d", spec.Domain, spec.AppPort))
	}

	now := time.Now()
	siteRec := &db.SiteRecord{
		ID:           uuid.New().String(),
		ServerID:     in.ServerID,
		Domain:       spec.Domain,
		SiteType:     string(spec.Type),
		DocumentRoot: workDir,
		WebServer:    "nginx",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = s.Database.SaveSite(ctx, siteRec)

	logger.SetStep("Blueprint applied successfully", 100)
	return fmt.Sprintf("Blueprint %s successfully applied for %s", spec.Type, spec.Domain), nil
}
