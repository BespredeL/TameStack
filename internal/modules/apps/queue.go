package apps

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

// GenerateQueueWorkerUnit builds a minimal systemd service unit without systemd 255-specific parameters.
func GenerateQueueWorkerUnit(siteID, domain, projectRoot, user string, cfg QueueWorkerConfig) string {
	if user == "" {
		user = "www-data"
	}
	sleep := cfg.Sleep
	if sleep <= 0 {
		sleep = 3
	}
	tries := cfg.Tries
	if tries <= 0 {
		tries = 3
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 3600
	}

	queueArgs := ""
	if cfg.Connection != "" {
		queueArgs = cfg.Connection
	}
	if cfg.Queue != "" {
		queueArgs = fmt.Sprintf("%s --queue=%s", queueArgs, cfg.Queue)
	}

	shortID := siteID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	serviceIdentifier := fmt.Sprintf("sh-queue-%s", shortID)

	return fmt.Sprintf(`[Unit]
Description=TameStack Queue Worker (%s)
After=network.target

[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=/usr/bin/php %s/artisan queue:work %s --sleep=%d --tries=%d --max-time=%d
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal
SyslogIdentifier=%s

[Install]
WantedBy=multi-user.target
`, domain, user, projectRoot, projectRoot, strings.TrimSpace(queueArgs), sleep, tries, timeout, serviceIdentifier)
}

// QueueUnitName returns the systemd unit name for a site queue worker.
func QueueUnitName(siteID string) string {
	shortID := siteID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("sh-queue-%s.service", shortID)
}

// InstallQueueWorker deploys and activates the systemd unit file.
func InstallQueueWorker(ctx context.Context, client *ssh.SSHClient, site *db.SiteRecord, cfg QueueWorkerConfig) error {
	if cfg.Connection != "" {
		if err := ssh.ValidateToken(cfg.Connection, "queue connection"); err != nil {
			return err
		}
	}
	if cfg.Queue != "" {
		// Queue names can be comma-separated (e.g. "high,default").
		for _, q := range strings.Split(cfg.Queue, ",") {
			if err := ssh.ValidateToken(strings.TrimSpace(q), "queue name"); err != nil {
				return err
			}
		}
	}
	projectRoot := ResolveProjectRoot(site.DocumentRoot)
	unitName := QueueUnitName(site.ID)
	unitPath := fmt.Sprintf("/etc/systemd/system/%s", unitName)

	unitContent := GenerateQueueWorkerUnit(site.ID, site.Domain, projectRoot, "www-data", cfg)
	if err := client.WriteFile(ctx, unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("failed to write systemd unit %s: %w", unitPath, err)
	}

	cmd := fmt.Sprintf("systemctl daemon-reload && systemctl enable %s && systemctl restart %s", unitName, unitName)
	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to start queue worker: %s %s (%w)", string(out), string(stderr), err)
	}

	return nil
}

// ControlQueueWorker manages the lifecycle of the queue worker service.
func ControlQueueWorker(ctx context.Context, client *ssh.SSHClient, site *db.SiteRecord, action string) error {
	unitName := QueueUnitName(site.ID)
	switch action {
	case "start", "stop", "restart", "enable", "disable":
		cmd := fmt.Sprintf("systemctl %s %s", action, unitName)
		out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
		if err != nil || code != 0 {
			return fmt.Errorf("systemctl %s %s failed: %s %s", action, unitName, string(out), string(stderr))
		}
		return nil
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}
