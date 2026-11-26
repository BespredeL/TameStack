package main

import (
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/modules/apps"
	"tamestack/internal/modules/site"
	"tamestack/internal/modules/siteaudit"
	"tamestack/internal/tasks"
)

// --- Sites Manager RPC ---

func (a *App) ListSites(serverID string) ([]*db.SiteRecord, error) {
	return withLock(a, func() ([]*db.SiteRecord, error) {
		return a.siteService.ListSites(a.ctx, serverID)
	})
}

func (a *App) GetSite(id string) (*db.SiteRecord, error) {
	return withLock(a, func() (*db.SiteRecord, error) {
		return a.siteService.GetSite(a.ctx, id)
	})
}

func (a *App) CheckSiteDNS(domain, serverID string) (*site.SiteDNSCheckResult, error) {
	return withLock(a, func() (*site.SiteDNSCheckResult, error) {
		return a.siteService.CheckDNS(a.ctx, domain, serverID)
	})
}

func (a *App) PlanSiteCreation(in site.SiteInput) (*site.CreateSitePlan, error) {
	return withLock(a, func() (*site.CreateSitePlan, error) {
		return a.siteService.PlanSiteCreation(a.ctx, in)
	})
}

func (a *App) CreateSite(in site.SiteInput) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.siteService.CreateSite(a.ctx, in)
	})
}

func (a *App) PlanSiteDeletion(id string, deleteFiles bool) (*site.SiteDeletePlan, error) {
	return withLock(a, func() (*site.SiteDeletePlan, error) {
		return a.siteService.PlanSiteDeletion(a.ctx, id, deleteFiles)
	})
}

func (a *App) DeleteSite(id string, deleteFiles bool) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	rec, err := a.database.GetSite(a.ctx, id)
	if err != nil || rec == nil {
		return fmt.Errorf("site not found: %w", err)
	}
	title := "Удаление сайта " + rec.Domain
	if rec.Origin == "discovered" {
		title = "Удаление стороннего сайта " + rec.Domain
	}
	payload, err := tasks.MarshalPayload(site.DeleteSitePayload{SiteID: id, Domain: rec.Domain, DeleteFiles: deleteFiles})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, rec.ServerID, "site_delete", title, payload)
}

func (a *App) AcceptDiscoveredSite(siteID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.siteService.AcceptDiscoveredSite(a.ctx, siteID)
}

func (a *App) CaptureSiteFileSnapshot(siteID string) (*siteaudit.Snapshot, error) {
	return withLock(a, func() (*siteaudit.Snapshot, error) {
		return a.siteAuditService.Capture(a.ctx, siteID, false)
	})
}

func (a *App) ListSiteAuditFindings(siteID string) ([]siteaudit.Finding, error) {
	return withLock(a, func() ([]siteaudit.Finding, error) {
		return a.siteAuditService.ListFindings(a.ctx, siteID)
	})
}

func (a *App) QuarantineSiteFile(siteID, relPath string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.siteAuditService.Quarantine(a.ctx, siteID, relPath)
	})
}

func (a *App) GetSiteLogs(id, logType string, lines int, preferredServer string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.siteService.GetSiteLogs(a.ctx, id, logType, lines, preferredServer)
	})
}

func (a *App) DiscoverSites(serverID string) ([]*db.SiteRecord, error) {
	return withLock(a, func() ([]*db.SiteRecord, error) {
		return a.siteService.DiscoverSites(a.ctx, serverID)
	})
}

func (a *App) SwitchSitePHPVersion(siteID string, targetPHPVersion string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	rec, err := a.database.GetSite(a.ctx, siteID)
	if err != nil || rec == nil {
		return fmt.Errorf("site not found: %w", err)
	}
	payload, err := tasks.MarshalPayload(site.SwitchPHPPayload{SiteID: siteID, Domain: rec.Domain, TargetPHPVersion: targetPHPVersion})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, rec.ServerID, "site_php_switch", fmt.Sprintf("Смена PHP на %s для %s", targetPHPVersion, rec.Domain), payload)
}

func (a *App) PlanSiteRuntimeAssignment(in site.SiteRuntimeInput) (*site.SiteRuntimePlan, error) {
	return withLock(a, func() (*site.SiteRuntimePlan, error) {
		return a.siteService.PlanSiteRuntimeAssignment(a.ctx, in)
	})
}

func (a *App) AssignSiteRuntime(in site.SiteRuntimeInput) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.siteService.AssignSiteRuntime(a.ctx, in)
}

func (a *App) GetSiteConfigFile(siteID string) (*site.SiteConfigFileResult, error) {
	return withLock(a, func() (*site.SiteConfigFileResult, error) {
		return a.siteService.GetSiteConfigFile(a.ctx, siteID)
	})
}

func (a *App) SaveSiteConfigFile(siteID string, content string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	rec, err := a.database.GetSite(a.ctx, siteID)
	if err != nil || rec == nil {
		return fmt.Errorf("site not found: %w", err)
	}
	payload, err := tasks.MarshalPayload(site.SaveConfigPayload{SiteID: siteID, Domain: rec.Domain, Content: content})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, rec.ServerID, "site_config_save", "Сохранение конфига "+rec.Domain, payload)
}

func (a *App) ListSiteConfigBackups(siteID string) ([]db.SiteConfigBackupRecord, error) {
	return withLock(a, func() ([]db.SiteConfigBackupRecord, error) {
		return a.siteService.ListSiteConfigBackups(a.ctx, siteID)
	})
}

func (a *App) RollbackSiteConfigFile(backupID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.siteService.RollbackSiteConfigFile(a.ctx, backupID)
}

func (a *App) UpdateSiteSettings(siteID string, documentRoot string, aliases []string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	rec, err := a.database.GetSite(a.ctx, siteID)
	if err != nil || rec == nil {
		return fmt.Errorf("site not found: %w", err)
	}
	payload, err := tasks.MarshalPayload(site.UpdateSettingsPayload{SiteID: siteID, Domain: rec.Domain, DocumentRoot: documentRoot, Aliases: aliases})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, rec.ServerID, "site_settings_update", "Обновление настроек "+rec.Domain, payload)
}

// --- Apps & Framework Management Methods ---

func (a *App) InspectSiteApp(siteID string) (*apps.AppInspectResult, error) {
	return withLock(a, func() (*apps.AppInspectResult, error) {
		return a.appsService.InspectSiteApp(a.ctx, siteID)
	})
}

func (a *App) RunArtisan(siteID, command string, confirm bool) (*apps.ArtisanCommandResult, error) {
	return withLock(a, func() (*apps.ArtisanCommandResult, error) {
		return a.appsService.RunArtisan(a.ctx, siteID, command, confirm)
	})
}

func (a *App) FixAppPermissions(siteID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.FixAppPermissions(a.ctx, siteID)
}

func (a *App) GetSiteEnv(siteID string) (*apps.EnvFileResult, error) {
	return withLock(a, func() (*apps.EnvFileResult, error) {
		return a.appsService.GetSiteEnv(a.ctx, siteID)
	})
}

func (a *App) SaveSiteEnv(siteID, content string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.SaveSiteEnv(a.ctx, siteID, content)
}

func (a *App) ToggleLaravelScheduler(siteID string, enable bool) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.ToggleLaravelScheduler(a.ctx, siteID, enable)
}

func (a *App) ConfigureQueueWorker(siteID string, cfg apps.QueueWorkerConfig) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.ConfigureQueueWorker(a.ctx, siteID, cfg)
}

func (a *App) ControlQueueWorker(siteID, action string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.ControlQueueWorker(a.ctx, siteID, action)
}

func (a *App) ReadLaravelLog(siteID string, maxLines int) (string, error) {
	return withLock(a, func() (string, error) {
		return a.appsService.ReadLaravelLog(a.ctx, siteID, maxLines)
	})
}

func (a *App) GetAppProcessStatus(siteID string) (*apps.AppProcessStatus, error) {
	return withLock(a, func() (*apps.AppProcessStatus, error) {
		return a.appsService.GetAppProcessStatus(a.ctx, siteID)
	})
}

func (a *App) ConfigureAppProcess(siteID string, cfg apps.AppProcessConfig) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.ConfigureAppProcess(a.ctx, siteID, cfg)
}

func (a *App) ControlAppProcess(siteID, action string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.ControlAppProcess(a.ctx, siteID, action)
}

func (a *App) DeleteAppProcess(siteID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.appsService.DeleteAppProcess(a.ctx, siteID)
}

func (a *App) ToggleSiteStatus(siteID string, enable bool) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	rec, err := a.database.GetSite(a.ctx, siteID)
	if err != nil || rec == nil {
		return fmt.Errorf("site not found: %w", err)
	}
	action := "Включение"
	if !enable {
		action = "Выключение"
	}
	payload, err := tasks.MarshalPayload(site.ToggleStatusPayload{SiteID: siteID, Domain: rec.Domain, Enable: enable})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, rec.ServerID, "site_status_toggle", action+" сайта "+rec.Domain, payload)
}
