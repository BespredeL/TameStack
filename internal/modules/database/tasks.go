package database

import (
	"context"
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
)

type DeleteUserPayload struct {
	ServerID string `json:"server_id"`
	Engine   string `json:"engine"`
	Username string `json:"username"`
	Host     string `json:"host"`
}

func (s *Service) handleDeleteUserTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[DeleteUserPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Удаление пользователя БД %s (%s)...", p.Username, p.Engine))
	if err := s.DeleteUser(ctx, p.ServerID, EngineType(p.Engine), p.Username, p.Host); err != nil {
		return "", err
	}
	logger.Info("Пользователь БД успешно удален.")
	return "Пользователь БД удален", nil
}
