package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/packages"
)

// --- Packages ---

func (a *App) SimulateInstallPackages(serverID string, pkgs []string) (*packages.AptSimulationResult, error) {
	return withLock(a, func() (*packages.AptSimulationResult, error) {
		return a.packagesService.SimulateInstall(a.ctx, serverID, pkgs)
	})
}

func (a *App) SimulateUpgradePackages(serverID string) (*packages.AptSimulationResult, error) {
	return withLock(a, func() (*packages.AptSimulationResult, error) {
		return a.packagesService.SimulateUpgrade(a.ctx, serverID)
	})
}

func (a *App) SimulateRemovePackages(serverID string, pkgs []string) (*packages.AptSimulationResult, error) {
	return withLock(a, func() (*packages.AptSimulationResult, error) {
		return a.packagesService.SimulateRemove(a.ctx, serverID, pkgs)
	})
}

func (a *App) InstallPackages(serverID string, pkgs []string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.packagesService.InstallPackages(a.ctx, serverID, pkgs)
	})
}

func (a *App) RemovePackages(serverID string, pkgs []string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.packagesService.RemovePackages(a.ctx, serverID, pkgs)
	})
}

func (a *App) UpgradePackages(serverID string, pkgs []string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.packagesService.UpgradePackages(a.ctx, serverID, pkgs)
	})
}

func (a *App) ListInstalledPackages(serverID string, query string) ([]packages.PackageItem, error) {
	return withLock(a, func() ([]packages.PackageItem, error) {
		return a.packagesService.ListInstalledPackages(a.ctx, serverID, query)
	})
}

func (a *App) ListPackageHistory(serverID string) ([]packages.PackageTaskHistoryItem, error) {
	return withLock(a, func() ([]packages.PackageTaskHistoryItem, error) {
		return a.packagesService.ListPackageHistory(a.ctx, serverID)
	})
}
