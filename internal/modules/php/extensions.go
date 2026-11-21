package php

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// ExtensionCatalog contains metadata and categories for standard extensions.
var ExtensionCatalog = []struct {
	Name        string
	Category    string
	Description string
}{
	{"curl", "Web", "cURL HTTP client library"},
	{"mbstring", "String", "Multibyte string support for UTF-8"},
	{"mysql", "Database", "MySQL & MariaDB PDO driver"},
	{"pgsql", "Database", "PostgreSQL PDO driver"},
	{"sqlite3", "Database", "SQLite 3 database driver"},
	{"xml", "XML", "XML parsing & DOM tools"},
	{"zip", "Archive", "ZIP archive handling"},
	{"gd", "Image", "GD image generation and resizing"},
	{"intl", "String", "Internationalization functions (ICU)"},
	{"imagick", "Image", "ImageMagick advanced image processing"},
	{"redis", "Cache", "High-performance Redis client driver"},
	{"igbinary", "Serialization", "Compact binary serializer"},
	{"soap", "Web", "SOAP protocol client and server"},
	{"imap", "Mail", "IMAP, POP3, and NNTP email library"},
	{"opcache", "Performance", "Zend bytecode opcode cache for speed"},
	{"bcmath", "Math", "Arbitrary precision mathematics"},
	{"sockets", "Network", "Low-level network sockets API"},
	{"fileinfo", "System", "MIME type and file signature detection"},
}

// RemovePHPExtension queues an asynchronous task to uninstall an extension package.
func (s *Service) RemovePHPExtension(serverID, version, extension string) (*db.TaskRecord, error) {
	if err := ssh.ValidatePHPVersion(version); err != nil {
		return nil, err
	}
	if err := ssh.ValidateToken(extension, "PHP extension"); err != nil {
		return nil, err
	}

	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	payloadMap := map[string]interface{}{
		"server_id": record.ID,
		"version":   version,
		"extension": extension,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	title := fmt.Sprintf("Remove PHP %s extension '%s' on %s", version, extension, record.Name)
	return s.engine.Submit(record.ID, "php_ext_remove", title, string(payloadBytes))
}

// handlePHPExtRemoveTask uninstalls extension package and reloads FPM.
func (s *Service) handlePHPExtRemoveTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID  string `json:"server_id"`
		Version   string `json:"version"`
		Extension string `json:"extension"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidatePHPVersion(payload.Version); err != nil {
		return "", err
	}
	if err := ssh.ValidateToken(payload.Extension, "PHP extension"); err != nil {
		return "", err
	}

	logger.SetStep("Connecting to server", 20)
	client, _, err := s.GetSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	diag, _ := s.Database.GetDiagnostics(ctx, payload.ServerID)
	osFamily := "debian"
	if diag != nil && diag.OSFamily != "" {
		osFamily = diag.OSFamily
	}

	logger.SetStep(fmt.Sprintf("Disabling extension %s", payload.Extension), 40)
	if osFamily == "debian" {
		_ = module.RunPrivileged(ctx, client, fmt.Sprintf("phpdismod -v %s %s 2>/dev/null || true", payload.Version, payload.Extension))
	}

	logger.SetStep(fmt.Sprintf("Removing package for extension %s", payload.Extension), 60)
	var removeCmd string
	if osFamily == "debian" {
		removeCmd = fmt.Sprintf("apt-get remove -y --purge php%s-%s", payload.Version, payload.Extension)
	} else {
		cleanCompact := strings.ReplaceAll(payload.Version, ".", "")
		removeCmd = fmt.Sprintf("dnf remove -y php%s-php-%s || yum remove -y php%s-php-%s", cleanCompact, payload.Extension, cleanCompact, payload.Extension)
	}

	out, stderr, code, err := client.NewCommand(removeCmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	logger.Info(string(out))
	if err != nil || code != 0 {
		return "", fmt.Errorf("failed to remove extension package (exit %d): %s", code, string(stderr))
	}

	logger.SetStep("Reloading PHP-FPM service", 85)
	serviceName := fmt.Sprintf("php%s-fpm", payload.Version)
	if osFamily == "rhel" {
		serviceName = fmt.Sprintf("php%s-php-fpm", strings.ReplaceAll(payload.Version, ".", ""))
	}
	_, _, _, _ = client.NewCommand("systemctl reload " + ssh.ShellEscape(serviceName) + " 2>/dev/null || systemctl restart " + ssh.ShellEscape(serviceName)).WithSudo().Exec(ctx)

	logger.SetStep("Completed", 100)
	return fmt.Sprintf("Extension %s for PHP %s removed successfully", payload.Extension, payload.Version), nil
}
