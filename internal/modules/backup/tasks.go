package backup

import (
	"context"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
)

type DeleteBackupPayload struct {
	ServerID string `json:"server_id"`
	BackupID string `json:"backup_id"`
}

func (s *Service) handleDeleteBackupTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[DeleteBackupPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info("Удаление резервной копии...")
	if err := s.DeleteBackup(ctx, p.ServerID, p.BackupID); err != nil {
		return "", err
	}
	logger.Info("Резервная копия удалена.")
	return "Бэкап удален", nil
}
