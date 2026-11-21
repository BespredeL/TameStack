package apps

import (
	"context"
	"fmt"
	"path"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// Service provides detection, management and maintenance of web application frameworks (Laravel, WordPress, etc.).
type Service struct {
	module.Base
	engine *tasks.Engine
}

// NewService constructs a new Apps management service.
func NewService(cfg *config.AppConfig, database *db.DB, v *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	return &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: v, Pool: pool},
		engine: engine,
	}
}

func (s *Service) getSite(ctx context.Context, siteID string) (*db.SiteRecord, error) {
	site, err := s.Database.GetSite(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("site %s not found: %w", siteID, err)
	}
	return site, nil
}

// InspectSiteApp inspects the site document root and discovers application framework details.
func (s *Service) InspectSiteApp(ctx context.Context, siteID string) (*AppInspectResult, error) {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return nil, err
	}

	return InspectSiteApp(ctx, client, site)
}

// RunArtisan executes an approved artisan command for a Laravel site.
func (s *Service) RunArtisan(ctx context.Context, siteID, command string, confirm bool) (*ArtisanCommandResult, error) {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return nil, err
	}

	return RunArtisanCommand(ctx, client, site, command, confirm)
}

// FixAppPermissions repairs ownership and read/write permissions for framework cache/storage folders.
func (s *Service) FixAppPermissions(ctx context.Context, siteID string) error {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	projectRoot := ResolveProjectRoot(site.DocumentRoot)
	safeRoot := ssh.ShellEscape(projectRoot)

	// Command fixes storage, bootstrap/cache for Laravel, and wp-content for WordPress.
	fixCmd := fmt.Sprintf(
		"cd %s && ("+
			"if [ -d storage ]; then chown -R www-data:www-data storage && chmod -R 775 storage; fi; "+
			"if [ -d bootstrap/cache ]; then chown -R www-data:www-data bootstrap/cache && chmod -R 775 bootstrap/cache; fi; "+
			"if [ -d wp-content ]; then chown -R www-data:www-data wp-content && chmod -R 775 wp-content; fi"+
			")",
		safeRoot,
	)

	_, stderr, code, err := client.NewCommand(fixCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to fix permissions: %s (%v)", string(stderr), err)
	}

	return nil
}

// GetSiteEnv reads the site .env file (if present) and returns its content.
func (s *Service) GetSiteEnv(ctx context.Context, siteID string) (*EnvFileResult, error) {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return nil, err
	}

	return GetEnvFile(ctx, client, site)
}

// SaveSiteEnv updates the site .env file with automatic backup and 0600 permissions.
func (s *Service) SaveSiteEnv(ctx context.Context, siteID, content string) error {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	return SaveEnvFile(ctx, client, site, content)
}

// ToggleLaravelScheduler enables or disables the artisan schedule:run cron job.
func (s *Service) ToggleLaravelScheduler(ctx context.Context, siteID string, enable bool) error {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	projectRoot := ResolveProjectRoot(site.DocumentRoot)
	return ToggleLaravelScheduler(ctx, client, projectRoot, "www-data", enable)
}

// ConfigureQueueWorker creates and starts a systemd unit for queue worker.
func (s *Service) ConfigureQueueWorker(ctx context.Context, siteID string, cfg QueueWorkerConfig) error {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	return InstallQueueWorker(ctx, client, site, cfg)
}

// ControlQueueWorker manages the systemd queue worker unit (start/stop/restart).
func (s *Service) ControlQueueWorker(ctx context.Context, siteID, action string) error {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return err
	}

	return ControlQueueWorker(ctx, client, site, action)
}

// ReadLaravelLog tails recent log lines from storage/logs/laravel.log.
func (s *Service) ReadLaravelLog(ctx context.Context, siteID string, maxLines int) (string, error) {
	site, err := s.getSite(ctx, siteID)
	if err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return "", err
	}

	if maxLines <= 0 || maxLines > 1000 {
		maxLines = 200
	}

	projectRoot := ResolveProjectRoot(site.DocumentRoot)
	logFile := path.Join(projectRoot, "storage", "logs", "laravel.log")
	safeFile := ssh.ShellEscape(logFile)

	tailCmd := fmt.Sprintf("if [ -f %s ]; then tail -n %d %s; else echo 'Файл лога еще не создан.'; fi", safeFile, maxLines, safeFile)
	stdout, _, _, err := client.NewCommand(tailCmd).WithSudo().Exec(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read log: %w", err)
	}

	return string(stdout), nil
}
