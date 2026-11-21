package main

import (
	"tamestack/internal/modules/logs"
)

// --- Logs ---

func (a *App) GetTailLog(serverID string, filePath string, lines int) (*logs.LogTailResult, error) {
	return withLock(a, func() (*logs.LogTailResult, error) {
		return a.logsService.GetTailLog(a.ctx, serverID, filePath, lines)
	})
}
