package apps

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

// AppProcessConfig defines a systemd-managed backend process (Node.js, Python, Go, or custom).
type AppProcessConfig struct {
	SiteID     string `json:"site_id"`
	Runtime    string `json:"runtime"` // "nodejs", "python", "go", "custom"
	WorkingDir string `json:"working_dir"`
	ExecStart  string `json:"exec_start"`
	User       string `json:"user"`
	Port       int    `json:"port"`
	EnvVars    string `json:"env_vars"`
}

// AppProcessStatus describes the current runtime state and logs of a managed app service.
type AppProcessStatus struct {
	Configured bool   `json:"configured"`
	UnitName   string `json:"unit_name"`
	Status     string `json:"status"`
	PID        string `json:"pid"`
	Memory     string `json:"memory"`
	Runtime    string `json:"runtime"`
	WorkingDir string `json:"working_dir"`
	ExecStart  string `json:"exec_start"`
	User       string `json:"user"`
	Port       int    `json:"port"`
	EnvVars    string `json:"env_vars"`
	RecentLogs string `json:"recent_logs"`
}

// AppUnitName returns the systemd unit name for a site's backend process.
func AppUnitName(siteID string) string {
	shortID := siteID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("tamestack-app-%s.service", shortID)
}

func appEnvFilePath(siteID string) string {
	shortID := siteID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("/etc/tamestack/apps/tamestack-app-%s.env", shortID)
}

// ValidateAppProcessConfig validates systemd app process parameters against injection.
func ValidateAppProcessConfig(cfg AppProcessConfig) error {
	switch strings.ToLower(strings.TrimSpace(cfg.Runtime)) {
	case "nodejs", "python", "go", "custom":
	default:
		return fmt.Errorf("недопустимый runtime: %s (допустимы: nodejs, python, go, custom)", cfg.Runtime)
	}
	if err := ssh.ValidateRemotePath(strings.TrimSpace(cfg.WorkingDir), false); err != nil {
		return fmt.Errorf("недопустимый рабочий каталог: %w", err)
	}
	execCmd := strings.TrimSpace(cfg.ExecStart)
	if execCmd == "" {
		return fmt.Errorf("команда запуска (ExecStart) не может быть пустой")
	}
	if strings.ContainsAny(execCmd, "\r\n;|&$`><") {
		return fmt.Errorf("команда запуска содержит недопустимые символы")
	}
	if cfg.User != "" {
		if err := ssh.ValidateIdent(cfg.User, "process user"); err != nil {
			return err
		}
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("недопустимый порт: %d", cfg.Port)
	}
	if strings.ContainsRune(cfg.EnvVars, '\x00') {
		return fmt.Errorf("переменные окружения содержат недопустимый нулевой байт")
	}
	return nil
}

// GenerateAppProcessUnit builds a systemd unit file for Node.js, Python, Go, or custom apps.
func GenerateAppProcessUnit(site *db.SiteRecord, cfg AppProcessConfig, envPath string) string {
	user := strings.TrimSpace(cfg.User)
	if user == "" {
		user = "www-data"
	}
	shortID := site.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf(`[Unit]
Description=TameStack App (%s - %s)
After=network.target

[Service]
Type=simple
User=%s
WorkingDirectory=%s
EnvironmentFile=-%s
ExecStart=%s
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal
SyslogIdentifier=tamestack-app-%s

[Install]
WantedBy=multi-user.target
`, site.Domain, strings.ToLower(cfg.Runtime), user, path.Clean(cfg.WorkingDir), envPath, strings.TrimSpace(cfg.ExecStart), shortID)
}

// GetAppProcessStatus inspects the systemd unit, env file, and journalctl logs for a site's app process.
func (s *Service) GetAppProcessStatus(ctx context.Context, siteID string) (*AppProcessStatus, error) {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return nil, err
	}

	unitName := AppUnitName(site.ID)
	unitPath := fmt.Sprintf("/etc/systemd/system/%s", unitName)
	envPath := appEnvFilePath(site.ID)

	res := &AppProcessStatus{
		UnitName:   unitName,
		Status:     "not_found",
		WorkingDir: ResolveProjectRoot(site.DocumentRoot),
		User:       "www-data",
		Runtime:    "nodejs",
	}

	if !client.FileExists(ctx, unitPath) {
		return res, nil
	}
	res.Configured = true

	if unitBytes, err := client.ReadFile(ctx, unitPath); err == nil {
		for _, line := range strings.Split(string(unitBytes), "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Description=TameStack App ("):
				if idx := strings.LastIndex(line, " - "); idx != -1 {
					res.Runtime = strings.TrimSuffix(line[idx+3:], ")")
				}
			case strings.HasPrefix(line, "User="):
				res.User = strings.TrimPrefix(line, "User=")
			case strings.HasPrefix(line, "WorkingDirectory="):
				res.WorkingDir = strings.TrimPrefix(line, "WorkingDirectory=")
			case strings.HasPrefix(line, "ExecStart="):
				res.ExecStart = strings.TrimPrefix(line, "ExecStart=")
			}
		}
	}

	if envBytes, err := client.ReadFile(ctx, envPath); err == nil {
		res.EnvVars = string(envBytes)
		for _, line := range strings.Split(res.EnvVars, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PORT=") {
				if p, pErr := strconv.Atoi(strings.TrimPrefix(line, "PORT=")); pErr == nil {
					res.Port = p
				}
			}
		}
	}

	showCmd := fmt.Sprintf("systemctl show %s --property=ActiveState,MainPID,MemoryCurrent --no-pager 2>/dev/null || true", ssh.ShellEscape(unitName))
	out, _, _, _ := client.NewCommand(showCmd).WithSudo().Exec(ctx)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ActiveState=") {
			res.Status = strings.TrimPrefix(line, "ActiveState=")
		} else if strings.HasPrefix(line, "MainPID=") {
			pid := strings.TrimPrefix(line, "MainPID=")
			if pid != "" && pid != "0" {
				res.PID = pid
			}
		} else if strings.HasPrefix(line, "MemoryCurrent=") {
			memStr := strings.TrimPrefix(line, "MemoryCurrent=")
			if memBytes, err := strconv.ParseInt(memStr, 10, 64); err == nil && memBytes > 0 && memBytes < 1<<50 {
				res.Memory = fmt.Sprintf("%.1f MB", float64(memBytes)/(1024*1024))
			}
		}
	}

	logCmd := fmt.Sprintf("journalctl -u %s -n 40 --no-pager 2>/dev/null || true", ssh.ShellEscape(unitName))
	logOut, _, _, _ := client.NewCommand(logCmd).WithSudo().Exec(ctx)
	res.RecentLogs = strings.TrimSpace(string(logOut))

	return res, nil
}

// ConfigureAppProcess writes the systemd unit and env file, then enables and starts/restarts the service.
func (s *Service) ConfigureAppProcess(ctx context.Context, siteID string, cfg AppProcessConfig) error {
	if err := ValidateAppProcessConfig(cfg); err != nil {
		return err
	}
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}
	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	unitName := AppUnitName(site.ID)
	unitPath := fmt.Sprintf("/etc/systemd/system/%s", unitName)
	envPath := appEnvFilePath(site.ID)

	if err := client.EnsureDirectory(ctx, "/etc/tamestack/apps", 0700, "root:root"); err != nil {
		return fmt.Errorf("ошибка создания /etc/tamestack/apps: %w", err)
	}

	envBody := strings.TrimSpace(cfg.EnvVars)
	if cfg.Port > 0 && !strings.Contains(envBody, "PORT=") {
		if envBody != "" {
			envBody += "\n"
		}
		envBody += fmt.Sprintf("PORT=%d", cfg.Port)
	}
	if envBody != "" {
		envBody += "\n"
	}
	if err := client.WriteFile(ctx, envPath, []byte(envBody), 0600); err != nil {
		return fmt.Errorf("ошибка записи env-файла %s: %w", envPath, err)
	}

	unitBody := GenerateAppProcessUnit(site, cfg, envPath)
	if err := client.WriteFile(ctx, unitPath, []byte(unitBody), 0644); err != nil {
		return fmt.Errorf("ошибка записи systemd-юнита %s: %w", unitPath, err)
	}

	cmd := fmt.Sprintf("systemctl daemon-reload && systemctl enable %s && systemctl restart %s", ssh.ShellEscape(unitName), ssh.ShellEscape(unitName))
	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("ошибка запуска сервиса %s: %s %s (%w)", unitName, string(out), string(stderr), err)
	}
	return nil
}

// ControlAppProcess runs start/stop/restart/enable/disable on the site's app systemd unit.
func (s *Service) ControlAppProcess(ctx context.Context, siteID, action string) error {
	if err := ssh.ValidateSystemctlAction(action); err != nil {
		return err
	}
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}
	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	unitName := AppUnitName(site.ID)
	cmd := fmt.Sprintf("systemctl %s %s", action, ssh.ShellEscape(unitName))
	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("systemctl %s %s failed: %s %s", action, unitName, string(out), string(stderr))
	}
	return nil
}

// DeleteAppProcess stops, disables, and removes the systemd unit and env file for the site's app process.
func (s *Service) DeleteAppProcess(ctx context.Context, siteID string) error {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}
	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	unitName := AppUnitName(site.ID)
	unitPath := fmt.Sprintf("/etc/systemd/system/%s", unitName)
	envPath := appEnvFilePath(site.ID)

	cmd := fmt.Sprintf("systemctl stop %s 2>/dev/null || true; systemctl disable %s 2>/dev/null || true; rm -f %s %s; systemctl daemon-reload",
		ssh.ShellEscape(unitName), ssh.ShellEscape(unitName), ssh.ShellEscape(unitPath), ssh.ShellEscape(envPath))
	_, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("ошибка удаления сервиса %s: %s", unitName, string(stderr))
	}
	return nil
}
