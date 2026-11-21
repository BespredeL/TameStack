package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
)

// --- Security & Monitoring RPC ---

func (a *App) GetFirewallStatus(serverID string) (*security.FirewallStatus, error) {
	return withLock(a, func() (*security.FirewallStatus, error) {
		return a.securityService.GetFirewallStatus(a.ctx, serverID)
	})
}

func (a *App) ToggleFirewall(serverID string, enable bool) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.ToggleFirewall(a.ctx, serverID, enable)
	})
}

// PlanFirewallInstall previews a firewall installation (packages, service, OS compatibility).
func (a *App) PlanFirewallInstall(serverID, firewallType string) (*security.FirewallInstallPlan, error) {
	return withLock(a, func() (*security.FirewallInstallPlan, error) {
		return a.securityService.PlanFirewallInstall(a.ctx, serverID, firewallType)
	})
}

// InstallFirewall queues an asynchronous firewall installation task.
func (a *App) InstallFirewall(serverID, firewallType string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.InstallFirewall(serverID, firewallType)
	})
}

func (a *App) AddFirewallRule(req security.AddFirewallRuleRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.AddFirewallRule(a.ctx, req)
	})
}

func (a *App) UpdateFirewallRule(req security.UpdateFirewallRuleRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.UpdateFirewallRule(a.ctx, req)
	})
}

func (a *App) DeleteFirewallRule(serverID string, ruleNum int, port string, proto string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.DeleteFirewallRule(a.ctx, serverID, ruleNum, port, proto)
	})
}

func (a *App) GetFail2banStatus(serverID string) (*security.Fail2banStatus, error) {
	return withLock(a, func() (*security.Fail2banStatus, error) {
		return a.securityService.GetFail2banStatus(a.ctx, serverID)
	})
}

func (a *App) UnbanIP(req security.UnbanIPRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.UnbanIP(a.ctx, req)
	})
}

func (a *App) BanIP(req security.BanIPRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.BanIP(a.ctx, req)
	})
}

func (a *App) ControlFail2ban(serverID string, action string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.ControlFail2ban(a.ctx, serverID, action)
	})
}

func (a *App) SaveFail2banJail(req security.SaveFail2banJailRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.SaveFail2banJail(a.ctx, req)
	})
}

func (a *App) DeleteFail2banJail(req security.DeleteFail2banJailRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.DeleteFail2banJail(a.ctx, req)
	})
}

func (a *App) ToggleFail2banJail(req security.ToggleFail2banJailRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.ToggleFail2banJail(a.ctx, req)
	})
}

func (a *App) RepairWebAccess(serverID string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.RepairWebAccess(a.ctx, serverID)
	})
}

func (a *App) InstallFail2ban(serverID string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.InstallFail2ban(a.ctx, serverID)
	})
}

func (a *App) RunSecurityAudit(serverID string) (*security.SecurityAuditReport, error) {
	return withLock(a, func() (*security.SecurityAuditReport, error) {
		return a.securityService.RunSecurityAudit(a.ctx, serverID)
	})
}

func (a *App) ApplyHardeningFix(serverID string, fixType string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.securityService.ApplyHardeningFix(a.ctx, serverID, fixType)
	})
}

func (a *App) GetRealtimeSystemMetrics(serverID string) (*security.RealtimeSystemMetrics, error) {
	return withLock(a, func() (*security.RealtimeSystemMetrics, error) {
		return a.securityService.GetRealtimeSystemMetrics(a.ctx, serverID)
	})
}

func (a *App) CheckEndpoint(targetURL string) (*security.SiteEndpointCheck, error) {
	return withLock(a, func() (*security.SiteEndpointCheck, error) {
		return a.securityService.CheckEndpoint(targetURL)
	})
}

// --- File Security, Malware Scan & Integrity Baselines ---

func (a *App) ScanSecurityFiles(req security.ScanFilesRequest) (*security.ScanFilesResult, error) {
	return withLock(a, func() (*security.ScanFilesResult, error) {
		return a.securityService.ScanFiles(a.ctx, req)
	})
}

func (a *App) DeleteSecurityFile(serverID, filePath string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.securityService.DeleteScannedFile(a.ctx, serverID, filePath)
}

func (a *App) ReadSecurityFile(serverID, filePath string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.securityService.ReadScannedFile(a.ctx, serverID, filePath)
	})
}

func (a *App) SaveSecurityFile(serverID, filePath, content string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.securityService.SaveScannedFile(a.ctx, serverID, filePath, content)
}

func (a *App) ListFileExclusions(serverID string) ([]security.FileSecurityExclusion, error) {
	return withLock(a, func() ([]security.FileSecurityExclusion, error) {
		return a.securityService.ListFileExclusions(a.ctx, serverID)
	})
}

func (a *App) AddFileExclusion(serverID, pattern, reason string) (*security.FileSecurityExclusion, error) {
	return withLock(a, func() (*security.FileSecurityExclusion, error) {
		return a.securityService.AddFileExclusion(a.ctx, serverID, pattern, reason)
	})
}

func (a *App) DeleteFileExclusion(exclusionID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.securityService.DeleteFileExclusion(a.ctx, exclusionID)
}

func (a *App) CreateFileSnapshot(serverID, siteID, name, basePath string) (*security.FileSecuritySnapshot, error) {
	return withLock(a, func() (*security.FileSecuritySnapshot, error) {
		return a.securityService.CreateFileSnapshot(a.ctx, serverID, siteID, name, basePath)
	})
}

func (a *App) ListFileSnapshots(serverID string) ([]security.FileSecuritySnapshot, error) {
	return withLock(a, func() ([]security.FileSecuritySnapshot, error) {
		return a.securityService.ListFileSnapshots(a.ctx, serverID)
	})
}

func (a *App) DeleteFileSnapshot(snapshotID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.securityService.DeleteFileSnapshot(a.ctx, snapshotID)
}

func (a *App) CompareFileSnapshot(snapshotID string) (*security.SnapshotDiffResult, error) {
	return withLock(a, func() (*security.SnapshotDiffResult, error) {
		return a.securityService.CompareWithSnapshot(a.ctx, snapshotID)
	})
}

func (a *App) ListFileSecuritySignatures(serverID string) ([]security.FileSecuritySignature, error) {
	return withLock(a, func() ([]security.FileSecuritySignature, error) {
		return a.securityService.ListFileSignatures(a.ctx, serverID)
	})
}

func (a *App) SaveFileSecuritySignature(sig security.FileSecuritySignature) (*security.FileSecuritySignature, error) {
	if err := a.requireUnlocked(); err != nil {
		return nil, err
	}
	return a.securityService.SaveFileSignature(a.ctx, sig)
}

func (a *App) DeleteFileSecuritySignature(id string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.securityService.DeleteFileSignature(a.ctx, id)
}

func (a *App) ToggleFileSecuritySignature(id string, enabled bool) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.securityService.ToggleFileSignature(a.ctx, id, enabled)
}
