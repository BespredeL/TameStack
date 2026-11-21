package site

import (
	"context"
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
)

// Typed payloads shared between the RPC submit sites and the task handlers.

type DeleteSitePayload struct {
	SiteID      string `json:"site_id"`
	Domain      string `json:"domain"`
	DeleteFiles bool   `json:"delete_files"`
}

type SaveConfigPayload struct {
	SiteID  string `json:"site_id"`
	Domain  string `json:"domain"`
	Content string `json:"content"`
}

type SwitchPHPPayload struct {
	SiteID           string `json:"site_id"`
	Domain           string `json:"domain"`
	TargetPHPVersion string `json:"target_php_version"`
}

type UpdateSettingsPayload struct {
	SiteID       string   `json:"site_id"`
	Domain       string   `json:"domain"`
	DocumentRoot string   `json:"document_root"`
	Aliases      []string `json:"aliases"`
}

type ToggleStatusPayload struct {
	SiteID string `json:"site_id"`
	Domain string `json:"domain"`
	Enable bool   `json:"enable"`
}

func (s *Service) handleDeleteSiteTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[DeleteSitePayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info("Удаление сайта и виртуального хоста...")
	if err := s.DeleteSite(ctx, p.SiteID, p.DeleteFiles); err != nil {
		return "", err
	}
	logger.Info("Сайт успешно удален.")
	return "Сайт удален", nil
}

func (s *Service) handleSaveConfigTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[SaveConfigPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info("Сохранение и атомарная проверка конфигурации виртуального хоста...")
	if err := s.SaveSiteConfigFile(ctx, p.SiteID, p.Content); err != nil {
		return "", err
	}
	logger.Info("Конфигурация успешно проверена и сохранена.")
	return "Конфигурация сохранена", nil
}

func (s *Service) handlePHPSwitchTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[SwitchPHPPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Переключение версии PHP сайта на %s...", p.TargetPHPVersion))
	if err := s.SwitchSitePHPVersion(ctx, p.SiteID, p.TargetPHPVersion); err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("Версия PHP успешно переключена на %s.", p.TargetPHPVersion))
	return "PHP переключен", nil
}

func (s *Service) handleSettingsUpdateTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[UpdateSettingsPayload](task.Payload)
	if err != nil {
		return "", err
	}
	logger.Info("Обновление настроек сайта...")
	if err := s.UpdateSiteSettings(ctx, p.SiteID, p.DocumentRoot, p.Aliases); err != nil {
		return "", err
	}
	logger.Info("Настройки сайта успешно обновлены.")
	return "Настройки обновлены", nil
}

func (s *Service) handleStatusToggleTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	p, err := tasks.DecodePayload[ToggleStatusPayload](task.Payload)
	if err != nil {
		return "", err
	}
	action := "Включение"
	if !p.Enable {
		action = "Выключение"
	}
	logger.Info(fmt.Sprintf("%s сайта...", action))
	if err := s.ToggleSiteStatus(ctx, p.SiteID, p.Enable); err != nil {
		return "", err
	}
	logger.Info("Статус сайта успешно изменен.")
	return "Статус изменен", nil
}
