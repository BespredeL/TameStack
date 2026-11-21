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

// InstallPHPExtensionsPlan details the extension packages to install for an
// already-installed PHP version.
type InstallPHPExtensionsPlan struct {
	TargetVersion      string   `json:"target_version"`
	OSFamily           string   `json:"os_family"`
	OSDistro           string   `json:"os_distro"`
	OSCompatible       bool     `json:"os_compatible"`
	Extensions         []string `json:"extensions"`
	PackagesToInstall  []string `json:"packages_to_install"`
	FPMServiceToReload string   `json:"fpm_service_to_reload"`
	Message            string   `json:"message"`
}

// normalizeExtensions trims, lowercases, drops empties and opcache (bundled with
// the PHP core packages), validates shell tokens and de-duplicates preserving order.
func normalizeExtensions(extensions []string) ([]string, error) {
	seen := make(map[string]bool)
	out := make([]string, 0, len(extensions))
	for _, ext := range extensions {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" || ext == "opcache" {
			continue
		}
		if err := ssh.ValidateToken(ext, "PHP extension"); err != nil {
			return nil, err
		}
		if seen[ext] {
			continue
		}
		seen[ext] = true
		out = append(out, ext)
	}
	return out, nil
}

// extensionPackages maps extension names to distribution package names.
func extensionPackages(osFamily, version string, extensions []string) []string {
	pkgs := make([]string, 0, len(extensions))
	if osFamily == "rhel" {
		verCompact := strings.ReplaceAll(version, ".", "")
		for _, ext := range extensions {
			pkgs = append(pkgs, fmt.Sprintf("php%s-php-%s", verCompact, ext))
		}
	} else {
		for _, ext := range extensions {
			pkgs = append(pkgs, fmt.Sprintf("php%s-%s", version, ext))
		}
	}
	return pkgs
}

// fpmServiceName returns the OS-specific PHP-FPM systemd unit for a version.
func fpmServiceName(osFamily, version string) string {
	if osFamily == "rhel" {
		return fmt.Sprintf("php%s-php-fpm", strings.ReplaceAll(version, ".", ""))
	}
	return fmt.Sprintf("php%s-fpm", version)
}

// PlanPHPExtensionsInstall builds the package blueprint for adding extensions to
// an existing PHP installation. Repositories are assumed already configured.
func (s *Service) PlanPHPExtensionsInstall(ctx context.Context, serverID, version string, extensions []string) (*InstallPHPExtensionsPlan, error) {
	if err := ssh.ValidatePHPVersion(version); err != nil {
		return nil, err
	}
	exts, err := normalizeExtensions(extensions)
	if err != nil {
		return nil, err
	}

	diag, err := s.Database.GetDiagnostics(ctx, serverID)
	if err != nil || diag == nil {
		return nil, fmt.Errorf("diagnostics not found for server %s (run diagnostics first)", serverID)
	}

	plan := &InstallPHPExtensionsPlan{
		TargetVersion:      version,
		OSFamily:           diag.OSFamily,
		OSDistro:           diag.OSName,
		OSCompatible:       true,
		Extensions:         exts,
		PackagesToInstall:  []string{},
		FPMServiceToReload: fpmServiceName(diag.OSFamily, version),
	}

	if diag.OSFamily != "debian" && diag.OSFamily != "rhel" {
		plan.OSCompatible = false
		plan.Message = fmt.Sprintf("Unsupported OS family: %s", diag.OSFamily)
		return plan, nil
	}

	plan.PackagesToInstall = extensionPackages(diag.OSFamily, version, exts)
	if len(exts) == 0 {
		plan.Message = "No extensions selected"
	} else {
		plan.Message = fmt.Sprintf("Ready to install %d extension(s) for PHP %s on %s", len(exts), version, diag.OSName)
	}
	return plan, nil
}

// InstallPHPExtensions queues an asynchronous task installing the given extensions.
func (s *Service) InstallPHPExtensions(serverID, version string, extensions []string) (*db.TaskRecord, error) {
	if err := ssh.ValidatePHPVersion(version); err != nil {
		return nil, err
	}
	exts, err := normalizeExtensions(extensions)
	if err != nil {
		return nil, err
	}
	if len(exts) == 0 {
		return nil, fmt.Errorf("no extensions selected")
	}

	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	payloadMap := map[string]interface{}{
		"server_id":  record.ID,
		"version":    version,
		"extensions": exts,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	title := fmt.Sprintf("Install PHP %s extensions (%s) on %s", version, strings.Join(exts, ", "), record.Name)
	return s.engine.Submit(record.ID, "php_ext_install", title, string(payloadBytes))
}

// handlePHPExtInstallTask installs extension packages, enables the modules and
// reloads PHP-FPM for an already-installed PHP version.
func (s *Service) handlePHPExtInstallTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID   string   `json:"server_id"`
		Version    string   `json:"version"`
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidatePHPVersion(payload.Version); err != nil {
		return "", err
	}
	exts, err := normalizeExtensions(payload.Extensions)
	if err != nil {
		return "", err
	}
	if len(exts) == 0 {
		return "", fmt.Errorf("no extensions selected")
	}

	logger.SetStep("Connecting to server", 10)
	client, record, err := s.GetSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	diag, _ := s.Database.GetDiagnostics(ctx, payload.ServerID)
	osFamily := "debian"
	if diag != nil && diag.OSFamily != "" {
		osFamily = diag.OSFamily
	}
	if osFamily != "debian" && osFamily != "rhel" {
		return "", fmt.Errorf("unsupported OS family: %s", osFamily)
	}

	packages := extensionPackages(osFamily, payload.Version, exts)
	logger.Info(fmt.Sprintf("Packages to install: %s", strings.Join(packages, " ")))

	if osFamily == "debian" {
		logger.SetStep("Updating package index", 30)
		if err := module.RunPrivileged(ctx, client, "apt-get update -y"); err != nil {
			return "", err
		}
	}

	logger.SetStep(fmt.Sprintf("Installing extensions for PHP %s", payload.Version), 55)
	var installCmd string
	if osFamily == "debian" {
		installCmd = fmt.Sprintf("apt-get install -y --no-install-recommends %s", strings.Join(packages, " "))
	} else {
		installCmd = fmt.Sprintf("dnf install -y %s", strings.Join(packages, " "))
	}
	outInstall, errInstall, code, err := client.NewCommand(installCmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	logger.Info(string(outInstall))
	if err != nil || code != 0 {
		return "", fmt.Errorf("extension installation failed (exit %d): %s", code, strings.TrimSpace(string(errInstall)))
	}

	// Debian uses phpenmod; RHEL remi packages auto-enable via php.d ini files.
	if osFamily == "debian" {
		logger.SetStep("Enabling extensions", 75)
		enableCmd := fmt.Sprintf("phpenmod -v %s %s", ssh.ShellEscape(payload.Version), strings.Join(exts, " "))
		outEn, errEn, codeEn, enErr := client.NewCommand(enableCmd).WithSudo().Exec(ctx)
		logger.Info(string(outEn))
		if enErr != nil || codeEn != 0 {
			logger.Warn(fmt.Sprintf("phpenmod returned non-zero: %s", strings.TrimSpace(string(errEn))))
		}
	}

	logger.SetStep("Reloading PHP-FPM", 90)
	serviceName := fpmServiceName(osFamily, payload.Version)
	reloadCmd := fmt.Sprintf("systemctl reload %s 2>/dev/null || systemctl restart %s", ssh.ShellEscape(serviceName), ssh.ShellEscape(serviceName))
	relOut, relErr, relCode, reloadErr := client.NewCommand(reloadCmd).WithSudo().Exec(ctx)
	if reloadErr != nil || relCode != 0 {
		return "", fmt.Errorf("failed to reload %s: %s %v", serviceName, strings.TrimSpace(string(relErr)), string(relOut))
	}

	logger.SetStep("Completed", 100)
	return fmt.Sprintf("Extensions %s installed for PHP %s on %s", strings.Join(exts, ", "), payload.Version, record.Host), nil
}