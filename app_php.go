package main

import (
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/modules/php"
	"tamestack/internal/tasks"
)

// --- PHP Manager RPC ---

func (a *App) InspectPHPVersions(serverID string) ([]php.PHPVersionInfo, error) {
	return withLock(a, func() ([]php.PHPVersionInfo, error) {
		return a.phpService.InspectPHPVersions(a.ctx, serverID)
	})
}

func (a *App) PlanPHPInstall(serverID, version string, extensions []string) (*php.InstallPHPPlan, error) {
	return withLock(a, func() (*php.InstallPHPPlan, error) {
		return a.phpService.PlanPHPInstall(a.ctx, serverID, version, extensions)
	})
}

func (a *App) InstallPHPVersion(serverID, version string, extensions []string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.phpService.InstallPHPVersion(serverID, version, extensions)
	})
}

func (a *App) UpdatePHPVersion(serverID, version string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.phpService.UpdatePHPVersion(serverID, version)
	})
}

func (a *App) TogglePHPExtension(serverID, version, extension string, enable bool) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.phpService.ToggleExtension(serverID, version, extension, enable)
	})
}

// PlanPHPExtensionsInstall previews the packages needed to add extensions to an
// already-installed PHP version.
func (a *App) PlanPHPExtensionsInstall(serverID, version string, extensions []string) (*php.InstallPHPExtensionsPlan, error) {
	return withLock(a, func() (*php.InstallPHPExtensionsPlan, error) {
		return a.phpService.PlanPHPExtensionsInstall(a.ctx, serverID, version, extensions)
	})
}

// InstallPHPExtensions queues an asynchronous task installing extensions for an
// already-installed PHP version.
func (a *App) InstallPHPExtensions(serverID, version string, extensions []string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.phpService.InstallPHPExtensions(serverID, version, extensions)
	})
}

func (a *App) GetPHPIni(serverID, version string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.phpService.GetPHPIni(a.ctx, serverID, version)
	})
}

func (a *App) SavePHPIni(serverID, version string, content string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	payload, err := tasks.MarshalPayload(php.SaveIniPayload{ServerID: serverID, Version: version, Content: content})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, serverID, "php_ini_save", fmt.Sprintf("Сохранение php.ini (%s)", version), payload)
}

func (a *App) RestartPHPFPM(serverID, version string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	payload, err := tasks.MarshalPayload(php.RestartFPMPayload{ServerID: serverID, Version: version})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, serverID, "php_fpm_restart", fmt.Sprintf("Перезапуск PHP-FPM %s", version), payload)
}

func (a *App) TogglePHPFPM(serverID, version string, enable bool) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	actionName := "Остановка"
	if enable {
		actionName = "Запуск"
	}
	payload, err := tasks.MarshalPayload(php.ToggleFPMPayload{ServerID: serverID, Version: version, Enable: enable})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, serverID, "php_fpm_toggle", fmt.Sprintf("%s PHP-FPM %s", actionName, version), payload)
}

func (a *App) ListPHPPools(serverID, version string) ([]php.FpmPoolConfig, error) {
	return withLock(a, func() ([]php.FpmPoolConfig, error) {
		return a.phpService.ListFPMPools(a.ctx, serverID, version)
	})
}

func (a *App) CreatePHPPool(in php.CreatePoolInput) (*php.FpmPoolConfig, error) {
	return withLock(a, func() (*php.FpmPoolConfig, error) {
		return a.phpService.CreatePool(a.ctx, in)
	})
}

func (a *App) DeletePHPPool(serverID, version, poolName string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.phpService.DeletePool(a.ctx, serverID, version, poolName)
}

func (a *App) UpdatePHPPool(in php.CreatePoolInput) (*php.FpmPoolConfig, error) {
	return withLock(a, func() (*php.FpmPoolConfig, error) {
		return a.phpService.UpdatePool(a.ctx, in)
	})
}

func (a *App) TogglePHPPool(serverID, version, poolName string, enable bool) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.phpService.TogglePool(a.ctx, serverID, version, poolName, enable)
}

func (a *App) ClonePHPPool(serverID, version, sourcePool, targetPool string) (*php.FpmPoolConfig, error) {
	return withLock(a, func() (*php.FpmPoolConfig, error) {
		return a.phpService.ClonePool(a.ctx, serverID, version, sourcePool, targetPool)
	})
}

func (a *App) SetDefaultCLIPHP(serverID, version string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.phpService.SetDefaultCLI(a.ctx, serverID, version)
}

func (a *App) GetFPMServiceOverview(serverID, version string) (*php.FPMServiceOverview, error) {
	return withLock(a, func() (*php.FPMServiceOverview, error) {
		return a.phpService.GetFPMServiceOverview(a.ctx, serverID, version)
	})
}

func (a *App) GetPHPFPMServiceLogs(serverID, version string, lines int) (string, error) {
	return withLock(a, func() (string, error) {
		return a.phpService.GetFPMServiceLogs(a.ctx, serverID, version, lines)
	})
}

func (a *App) CheckPHPFPMConfig(serverID, version string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.phpService.CheckFPMConfig(a.ctx, serverID, version)
	})
}

func (a *App) GetPHPSettings(serverID, version, target string) (*php.PHPSettings, error) {
	return withLock(a, func() (*php.PHPSettings, error) {
		return a.phpService.GetParsedPHPSettings(a.ctx, serverID, version, target)
	})
}

func (a *App) SavePHPSettings(serverID, version, target string, settings php.PHPSettings) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.phpService.SaveParsedPHPSettings(a.ctx, serverID, version, target, settings)
}

func (a *App) ListPHPIniBackups(serverID, version, target string) ([]php.PHPIniBackupItem, error) {
	return withLock(a, func() ([]php.PHPIniBackupItem, error) {
		return a.phpService.ListPHPIniBackups(a.ctx, serverID, version, target)
	})
}

func (a *App) RestorePHPIniBackup(serverID, version, target, backupFile string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.phpService.RestorePHPIniBackup(a.ctx, serverID, version, target, backupFile)
}

func (a *App) RemovePHPExtension(serverID, version, extension string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.phpService.RemovePHPExtension(serverID, version, extension)
	})
}

func (a *App) CheckPHPAppRequirements(serverID, version, preset string) (*php.PHPAppRequirement, error) {
	return withLock(a, func() (*php.PHPAppRequirement, error) {
		return a.phpService.CheckAppDependencies(a.ctx, serverID, version, preset)
	})
}

func (a *App) DetectPHPHandlers(serverID string) ([]string, error) {
	return withLock(a, func() ([]string, error) {
		return a.phpService.DetectPHPHandlers(a.ctx, serverID)
	})
}
