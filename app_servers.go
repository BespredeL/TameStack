package main

import (
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/inventory"
	"tamestack/internal/modules/server"
	"tamestack/internal/ssh"
)

// --- Server Management RPC ---

func (a *App) ListServers() ([]*db.ServerRecord, error) {
	return withLock(a, func() ([]*db.ServerRecord, error) {
		return a.serverService.ListServers(a.ctx)
	})
}

func (a *App) GetServer(id string) (*db.ServerRecord, error) {
	return withLock(a, func() (*db.ServerRecord, error) {
		return a.serverService.GetServer(a.ctx, id)
	})
}

func (a *App) AddServer(in server.ServerInput) (*db.ServerRecord, error) {
	return withLock(a, func() (*db.ServerRecord, error) {
		return a.serverService.AddServer(a.ctx, in)
	})
}

func (a *App) UpdateServer(in server.ServerInput) (*db.ServerRecord, error) {
	return withLock(a, func() (*db.ServerRecord, error) {
		return a.serverService.UpdateServer(a.ctx, in)
	})
}

func (a *App) SetServerSudoPassword(serverID string, sudoPassword string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.serverService.SetSudoPassword(a.ctx, serverID, sudoPassword)
}

func (a *App) DeleteServer(id string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.serverService.DeleteServer(a.ctx, id)
}

func (a *App) InspectHostKey(host string, port int) (*ssh.HostKeyInspection, error) {
	return withLock(a, func() (*ssh.HostKeyInspection, error) {
		return a.serverService.InspectHostKey(host, port)
	})
}

func (a *App) AcceptHostKey(rawEntry string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.serverService.AcceptHostKey(rawEntry)
}

func (a *App) TestConnection(serverID string) (map[string]interface{}, error) {
	return withLock(a, func() (map[string]interface{}, error) {
		ok, msg, err := a.serverService.TestConnection(a.ctx, serverID)
		return map[string]interface{}{
			"success": ok,
			"message": msg,
			"error":   fmt.Sprintf("%v", err),
		}, err
	})
}

func (a *App) TriggerDiagnostics(serverID string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.serverService.TriggerDiagnostics(serverID)
	})
}

func (a *App) GetDiagnostics(serverID string) (*db.DiagnosticsRecord, error) {
	return withLock(a, func() (*db.DiagnosticsRecord, error) {
		return a.serverService.GetDiagnostics(a.ctx, serverID)
	})
}

// SetServerMode updates the operating mode of a server ("audit" or "management").
func (a *App) SetServerMode(serverID string, mode string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.serverService.SetServerMode(a.ctx, serverID, mode)
}

// GetServerMode returns the operating mode of a server ("audit" or "management").
func (a *App) GetServerMode(serverID string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.serverService.GetServerMode(a.ctx, serverID)
	})
}

// CollectServerInventory triggers a read-only inventory collection and returns the snapshot.
func (a *App) CollectServerInventory(serverID string) (*inventory.ServerInventorySnapshot, error) {
	return withLock(a, func() (*inventory.ServerInventorySnapshot, error) {
		err := a.engine.SubmitAndWait(a.ctx, serverID, "inventory_snapshot", "Сбор инвентаря сервера", "")
		if err != nil {
			return nil, err
		}
		return a.inventoryService.GetLatest(a.ctx, serverID)
	})
}

// GetLatestServerInventory returns the latest collected inventory snapshot.
func (a *App) GetLatestServerInventory(serverID string) (*inventory.ServerInventorySnapshot, error) {
	return withLock(a, func() (*inventory.ServerInventorySnapshot, error) {
		return a.inventoryService.GetLatest(a.ctx, serverID)
	})
}

// GetServerTopology returns the correlated architecture and topology view.
func (a *App) GetServerTopology(serverID string) (*inventory.ServerTopology, error) {
	return withLock(a, func() (*inventory.ServerTopology, error) {
		snap, err := a.inventoryService.GetLatest(a.ctx, serverID)
		if err != nil {
			return nil, err
		}
		if snap == nil {
			return nil, nil
		}
		return &inventory.ServerTopology{
			OS:          snap.OS,
			WebServers:  snap.WebServers,
			PHPRuntimes: snap.PHPRuntimes,
			Databases:   snap.DBInstances,
			APT:         snap.APT,
			SiteChains:  snap.SiteChains,
		}, nil
	})
}
