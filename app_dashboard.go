package main

import (
	"tamestack/internal/modules/server"
)

// --- Dashboard Statistics RPC ---

func (a *App) GetDashboardStats() (*server.DashboardStats, error) {
	return withLock(a, func() (*server.DashboardStats, error) {
		return a.serverService.GetDashboardStats(a.ctx)
	})
}
