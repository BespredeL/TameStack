package main

import (
	"tamestack/internal/db"
	"tamestack/internal/provisioning/inventory"
	"tamestack/internal/provisioning/models"
	"tamestack/internal/provisioning/profiles"
)

// --- Server Provisioning & Setup Wizard RPC ---

func (a *App) ProvisioningDiscover(serverID string) (*inventory.ServerInventory, error) {
	return withLock(a, func() (*inventory.ServerInventory, error) {
		return a.provisioningService.DiscoverServer(a.ctx, serverID)
	})
}

func (a *App) ProvisioningListProfiles() ([]profiles.ProfileInfo, error) {
	return withLock(a, func() ([]profiles.ProfileInfo, error) {
		return a.provisioningService.ListProfiles(), nil
	})
}

func (a *App) ProvisioningGetProfileBlueprint(serverID, profileID string) (*models.Blueprint, error) {
	return withLock(a, func() (*models.Blueprint, error) {
		return a.provisioningService.GetProfileBlueprint(a.ctx, serverID, profileID)
	})
}

func (a *App) ProvisioningPlan(serverID string, bp models.Blueprint) (*models.ProvisioningPlan, error) {
	return withLock(a, func() (*models.ProvisioningPlan, error) {
		return a.provisioningService.GeneratePlan(a.ctx, serverID, bp)
	})
}

func (a *App) ProvisioningExecute(serverID string, bp models.Blueprint) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.provisioningService.ExecutePlan(a.ctx, serverID, bp)
	})
}

func (a *App) ProvisioningListBlueprints() ([]*db.ServerBlueprintRecord, error) {
	return withLock(a, func() ([]*db.ServerBlueprintRecord, error) {
		return a.provisioningService.ListBlueprints(a.ctx)
	})
}

func (a *App) ProvisioningSaveBlueprint(name, description string, bp models.Blueprint) (*db.ServerBlueprintRecord, error) {
	return withLock(a, func() (*db.ServerBlueprintRecord, error) {
		return a.provisioningService.SaveBlueprint(a.ctx, name, description, bp)
	})
}

func (a *App) ProvisioningDeleteBlueprint(id string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.provisioningService.DeleteBlueprint(a.ctx, id)
}

func (a *App) ProvisioningListRuns(serverID string, limit int) ([]*db.ProvisioningRunRecord, error) {
	return withLock(a, func() ([]*db.ProvisioningRunRecord, error) {
		return a.provisioningService.ListProvisioningRuns(a.ctx, serverID, limit)
	})
}
