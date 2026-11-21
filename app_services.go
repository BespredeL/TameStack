package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/services"
)

// --- Services, Processes & Network ---

func (a *App) ListSystemUnits(serverID string, unitType string) ([]services.SystemUnit, error) {
	return withLock(a, func() ([]services.SystemUnit, error) {
		return a.servicesService.ListUnits(a.ctx, serverID, unitType)
	})
}

func (a *App) ControlSystemUnit(serverID string, unitName string, action string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.servicesService.ControlUnit(a.ctx, serverID, unitName, action)
	})
}

func (a *App) GetSystemUnitLogs(serverID string, unitName string, lines int) (string, error) {
	return withLock(a, func() (string, error) {
		return a.servicesService.GetUnitLogs(a.ctx, serverID, unitName, lines)
	})
}

func (a *App) ListSystemProcesses(serverID string) ([]services.ProcessItem, error) {
	return withLock(a, func() ([]services.ProcessItem, error) {
		return a.servicesService.ListProcesses(a.ctx, serverID)
	})
}

func (a *App) KillSystemProcess(serverID string, pid int, signal string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.servicesService.KillProcess(a.ctx, serverID, pid, signal)
	})
}

func (a *App) ListSystemListeningPorts(serverID string) ([]services.ListeningPortItem, error) {
	return withLock(a, func() ([]services.ListeningPortItem, error) {
		return a.servicesService.ListListeningPorts(a.ctx, serverID)
	})
}

func (a *App) CheckDNS(serverID string, domain string, recordType string) (*services.DNSCheckResult, error) {
	return withLock(a, func() (*services.DNSCheckResult, error) {
		return a.servicesService.CheckDNS(a.ctx, serverID, domain, recordType)
	})
}

func (a *App) CheckPortConnection(serverID string, host string, port int) (*services.PortCheckResult, error) {
	return withLock(a, func() (*services.PortCheckResult, error) {
		return a.servicesService.CheckPort(a.ctx, serverID, host, port)
	})
}

func (a *App) GetNetworkOverview(serverID string) (*services.NetworkOverview, error) {
	return withLock(a, func() (*services.NetworkOverview, error) {
		return a.servicesService.GetNetworkOverview(a.ctx, serverID)
	})
}
