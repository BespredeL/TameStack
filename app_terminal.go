package main

import (
	"tamestack/internal/modules/terminal"
)

// --- Terminal (SSH PTY) RPC ---

func (a *App) StartTerminalSession(req terminal.StartSessionRequest) (*terminal.TerminalSessionInfo, error) {
	return withLock(a, func() (*terminal.TerminalSessionInfo, error) {
		return a.terminalService.StartSession(a.ctx, req)
	})
}

func (a *App) WriteTerminalInput(sessionID string, data string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.terminalService.WriteInput(sessionID, data)
}

func (a *App) ResizeTerminal(req terminal.ResizeTerminalRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.terminalService.Resize(req.SessionID, req.Cols, req.Rows)
}

func (a *App) CloseTerminalSession(sessionID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.terminalService.CloseSession(sessionID)
}
