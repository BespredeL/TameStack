package main

import (
	"path/filepath"

	"tamestack/internal/db"
	"tamestack/internal/modules/backup"
	"tamestack/internal/tasks"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// --- Backup Manager RPC ---

func (a *App) ListBackups(serverID string) ([]*db.BackupRecord, error) {
	return withLock(a, func() ([]*db.BackupRecord, error) {
		return a.backupService.ListBackups(a.ctx, serverID)
	})
}

func (a *App) CreateBackup(req backup.CreateBackupRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.backupService.CreateBackup(a.ctx, req)
	})
}

// DiscoverBackups scans the server's backup directories for archives created
// by any TameStack version and registers the ones missing from the database.
func (a *App) DiscoverBackups(serverID string) (*backup.DiscoverBackupsResult, error) {
	return withLock(a, func() (*backup.DiscoverBackupsResult, error) {
		return a.backupService.DiscoverBackups(a.ctx, serverID, "")
	})
}

func (a *App) RestoreBackup(req backup.RestoreBackupRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.backupService.RestoreBackup(a.ctx, req)
	})
}

func (a *App) DeleteBackup(serverID, backupID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	payload, err := tasks.MarshalPayload(backup.DeleteBackupPayload{ServerID: serverID, BackupID: backupID})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, serverID, "backup_delete", "Удаление бэкапа", payload)
}

func (a *App) InspectBackupArchive(serverID, filePath string) (*backup.ArchiveInspectionResult, error) {
	return withLock(a, func() (*backup.ArchiveInspectionResult, error) {
		return a.backupService.InspectArchive(a.ctx, serverID, filePath)
	})
}

func (a *App) DownloadBackup(serverID, backupID string) (string, error) {
	return withLock(a, func() (string, error) {
		suggestedName := "backup.tar.gz"
		if b, err := a.database.GetBackup(a.ctx, backupID); err == nil && b.FilePath != "" {
			suggestedName = filepath.Base(b.FilePath)
		}

		localPath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
			DefaultFilename: suggestedName,
			Title:           "Сохранить резервную копию на компьютер",
			Filters: []wailsRuntime.FileFilter{
				{
					DisplayName: "Архивы бэкапов (*.tar.gz, *.sql.gz)",
					Pattern:     "*.tar.gz;*.sql.gz;*.gz;*.tar",
				},
				{
					DisplayName: "Все файлы (*.*)",
					Pattern:     "*.*",
				},
			},
		})
		if err != nil {
			return "", err
		}
		if localPath == "" {
			return "", nil
		}

		checksum, err := a.backupService.DownloadBackup(a.ctx, serverID, backupID, localPath, func(transferred, total int64) {
			pct := 0
			if total > 0 {
				pct = int((transferred * 100) / total)
			}
			wailsRuntime.EventsEmit(a.ctx, "backup:download:progress", map[string]any{
				"backup_id":   backupID,
				"transferred": transferred,
				"total":       total,
				"percent":     pct,
			})
		})
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "backup:download:error", map[string]any{
				"backup_id": backupID,
				"error":     err.Error(),
			})
			return "", err
		}

		wailsRuntime.EventsEmit(a.ctx, "backup:download:complete", map[string]any{
			"backup_id":  backupID,
			"local_path": localPath,
			"checksum":   checksum,
		})

		return localPath, nil
	})
}

func (a *App) VerifyBackupArchive(serverID, backupID string) (bool, error) {
	return withLock(a, func() (bool, error) {
		return a.backupService.VerifyBackup(a.ctx, serverID, backupID)
	})
}
