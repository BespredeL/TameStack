package php

import (
	"context"
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
)

type SaveIniPayload struct {
	ServerID string `json:"server_id"`
	Version  string `json:"version"`
	Content  string `json:"content"`
}

type RestartFPMPayload struct {
	ServerID string `json:"server_id"`
	Version  string `json:"version"`
}

type ToggleFPMPayload struct {
	ServerID string `json:"server_id"`
	Version  string `json:"version"`
	Enable   bool   `json:"enable"`
}

func (s *Service) handleSaveIniTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[SaveIniPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Сохранение php.ini для PHP %s...", p.Version))
	if err := s.SavePHPIni(ctx, p.ServerID, p.Version, p.Content); err != nil {
		return "", err
	}
	logger.Info("php.ini успешно сохранен.")
	return "php.ini сохранен", nil
}

func (s *Service) handleRestartFPMTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[RestartFPMPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Перезапуск службы PHP-FPM %s...", p.Version))
	if err := s.RestartFPM(ctx, p.ServerID, p.Version); err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Служба PHP-FPM %s перезапущена.", p.Version))
	return "PHP-FPM перезапущен", nil
}

func (s *Service) handleToggleFPMTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[ToggleFPMPayload](task.Payload)
	if err != nil {
		return "", err
	}
	action := "Остановка"
	if p.Enable {
		action = "Включение"
	}
	logger.Info(fmt.Sprintf("%s службы PHP-FPM %s...", action, p.Version))
	if err := s.ToggleFPM(ctx, p.ServerID, p.Version, p.Enable); err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Служба PHP-FPM %s: %s выполнено.", p.Version, action))
	return fmt.Sprintf("PHP-FPM %s: %s", p.Version, action), nil
}
