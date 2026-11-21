package siteaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

type quarantinePayload struct {
	SiteID string `json:"site_id"`
	Path   string `json:"path"`
}

// Quarantine queues a move of one file out of the document root.
func (s *Service) Quarantine(ctx context.Context, siteID, relPath string) (*db.TaskRecord, error) {
	if err := validateRelative(relPath); err != nil {
		return nil, err
	}
	site, err := s.database.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(quarantinePayload{SiteID: siteID, Path: relPath})
	if err != nil {
		return nil, err
	}
	return s.engine.Submit(site.ServerID, "site_file_quarantine", "Карантин "+relPath, string(raw))
}

func (s *Service) handleQuarantine(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload quarantinePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", err
	}
	if err := validateRelative(payload.Path); err != nil {
		return "", err
	}
	site, err := s.database.GetSite(ctx, payload.SiteID)
	if err != nil {
		return "", err
	}
	client, err := s.getSSH(ctx, site.ServerID)
	if err != nil {
		return "", err
	}
	destDir := "/var/lib/TameStack/quarantine/" + site.ID
	dest := destDir + "/" + strings.ReplaceAll(payload.Path, "/", "_")
	src := strings.TrimRight(site.DocumentRoot, "/") + "/" + payload.Path
	cmd := fmt.Sprintf("mkdir -p %s && mv %s %s", ssh.ShellEscape(destDir), ssh.ShellEscape(src), ssh.ShellEscape(dest))
	logger.Info("Перенос файла в карантин, без запуска содержимого.")
	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return "", fmt.Errorf("карантин не выполнен: %s %s %v", out, stderr, err)
	}
	return "Файл перенесён в " + dest, nil
}

func validateRelative(rel string) error {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
		return fmt.Errorf("путь файла вне каталога сайта")
	}
	return nil
}
