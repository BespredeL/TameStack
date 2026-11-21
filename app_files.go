package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/files"
)

// --- File Manager RPC ---

func (a *App) ListDirectory(serverID, remotePath string) (*files.DirectoryListing, error) {
	return withLock(a, func() (*files.DirectoryListing, error) {
		return a.filesService.ListDirectory(a.ctx, serverID, remotePath)
	})
}

func (a *App) ReadRemoteFile(serverID, remotePath string) (*files.FileContentResult, error) {
	return withLock(a, func() (*files.FileContentResult, error) {
		return a.filesService.ReadFile(a.ctx, serverID, remotePath)
	})
}

func (a *App) SaveRemoteFile(req files.SaveFileRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.filesService.SaveFile(a.ctx, req)
}

func (a *App) CreateRemoteEntry(req files.CreateEntryRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.filesService.CreateEntry(a.ctx, req)
}

func (a *App) DeleteRemoteEntry(req files.DeleteEntryRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.filesService.DeleteEntry(a.ctx, req)
}

func (a *App) RenameRemoteEntry(req files.RenameEntryRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.filesService.RenameEntry(a.ctx, req)
}

func (a *App) ChmodRemoteEntry(req files.ChmodRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.filesService.Chmod(a.ctx, req)
}

func (a *App) AnalyzeDiskUsage(serverID, targetPath string) (*files.DiskAnalysisReport, error) {
	return withLock(a, func() (*files.DiskAnalysisReport, error) {
		return a.filesService.AnalyzeDiskUsage(a.ctx, serverID, targetPath)
	})
}

func (a *App) CleanDiskCategory(serverID, categoryID string) (*db.TaskRecord, error) {
	if err := a.requireUnlocked(); err != nil {
		return nil, err
	}
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.filesService.CleanDiskCategory(a.ctx, serverID, categoryID)
	})
}
