package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/ssl"
)

// --- SSL Manager RPC ---

func (a *App) ListSSLCertificates(serverID string) ([]*db.SSLCertificateRecord, error) {
	return withLock(a, func() ([]*db.SSLCertificateRecord, error) {
		return a.sslService.ListCertificates(a.ctx, serverID)
	})
}

func (a *App) GetSSLMetrics(serverID string) (*ssl.SSLMetrics, error) {
	return withLock(a, func() (*ssl.SSLMetrics, error) {
		return a.sslService.GetMetrics(a.ctx, serverID)
	})
}

func (a *App) IssueSSLCertificate(in ssl.IssueSSLInput) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.sslService.IssueCertificate(a.ctx, in)
	})
}

func (a *App) RenewSSLCertificate(certID string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.sslService.RenewCertificate(a.ctx, certID)
	})
}

func (a *App) DeleteSSLCertificate(certID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.sslService.DeleteCertificate(a.ctx, certID)
}

func (a *App) DiscoverSSLCertificates(serverID string) ([]*db.SSLCertificateRecord, error) {
	return withLock(a, func() ([]*db.SSLCertificateRecord, error) {
		return a.sslService.DiscoverCertificates(a.ctx, serverID)
	})
}

// GetCertbotStatus reports whether certbot is installed, its version and auto-renew state.
func (a *App) GetCertbotStatus(serverID string) (*ssl.CertbotStatus, error) {
	return withLock(a, func() (*ssl.CertbotStatus, error) {
		return a.sslService.GetCertbotStatus(a.ctx, serverID)
	})
}

// PlanCertbotInstall previews a certbot installation (packages, plugin, OS compatibility).
func (a *App) PlanCertbotInstall(serverID string) (*ssl.CertbotInstallPlan, error) {
	return withLock(a, func() (*ssl.CertbotInstallPlan, error) {
		return a.sslService.PlanCertbotInstall(a.ctx, serverID)
	})
}

// InstallCertbot queues an asynchronous certbot installation task.
func (a *App) InstallCertbot(serverID string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.sslService.InstallCertbot(serverID)
	})
}

// GetCertbotConfig reads /etc/letsencrypt/cli.ini and checks the auto-renewal timer.
func (a *App) GetCertbotConfig(serverID string) (*ssl.CertbotConfig, error) {
	return withLock(a, func() (*ssl.CertbotConfig, error) {
		return a.sslService.GetCertbotConfig(a.ctx, serverID)
	})
}

// SaveCertbotConfig updates /etc/letsencrypt/cli.ini and enables/disables the auto-renewal timer.
func (a *App) SaveCertbotConfig(serverID string, cfg ssl.CertbotConfig) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.sslService.SaveCertbotConfig(a.ctx, serverID, cfg)
}

// RunCertbotRenew executes certbot renew (dryRun=true for testing without renewing).
func (a *App) RunCertbotRenew(serverID string, dryRun bool) (*ssl.CertbotRenewResult, error) {
	if err := a.requireUnlocked(); err != nil {
		return nil, err
	}
	return a.sslService.RunCertbotRenew(a.ctx, serverID, dryRun)
}

// ListCertbotCertificates returns all certificates registered in certbot.
func (a *App) ListCertbotCertificates(serverID string) ([]ssl.CertbotCertificateItem, error) {
	return withLock(a, func() ([]ssl.CertbotCertificateItem, error) {
		return a.sslService.ListCertbotCertificates(a.ctx, serverID)
	})
}

// DeleteCertbotCertificate deletes a certbot-managed certificate by name.
func (a *App) DeleteCertbotCertificate(serverID, certName string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.sslService.DeleteCertbotCertificate(a.ctx, serverID, certName)
}
