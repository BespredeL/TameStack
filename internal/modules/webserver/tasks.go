package webserver

import (
	"context"
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
)

type ControlPayload struct {
	ServerID      string `json:"server_id"`
	WebServerType string `json:"webserver_type"`
	Action        string `json:"action"`
}

func (s *Service) handleControlTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[ControlPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Выполнение %s для %s...", p.Action, p.WebServerType))
	if err := s.ControlService(ctx, p.ServerID, p.WebServerType, p.Action); err != nil {
		return "", err
	}
	logger.Info("Команда веб-сервера выполнена.")
	return "Служба обновлена", nil
}
