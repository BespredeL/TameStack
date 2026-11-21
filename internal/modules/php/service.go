package php

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/platform/capabilities"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// Service coordinates PHP management and execution tasks.
type Service struct {
	module.Base
	engine *tasks.Engine
}

// NewService instantiates PHP service and registers task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}

	engine.RegisterHandler("php_install", s.handlePHPInstallTask)
	engine.RegisterHandler("php_ext_install", s.handlePHPExtInstallTask)
	engine.RegisterHandler("php_ext_toggle", s.handlePHPExtToggleTask)
	engine.RegisterHandler("php_ext_remove", s.handlePHPExtRemoveTask)
	engine.RegisterHandler("php_ini_save", s.handleSaveIniTask)
	engine.RegisterHandler("php_fpm_restart", s.handleRestartFPMTask)
	engine.RegisterHandler("php_fpm_toggle", s.handleToggleFPMTask)
	engine.RegisterHandler("php_update", s.handlePHPUpdateTask)

	return s
}

var versionExtractRegex = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)+)`)

func cleanCandidateVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if m := versionExtractRegex.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	return raw
}

// InspectPHPVersions scans installed PHP binaries, FPM daemons, and active extensions.
func (s *Service) InspectPHPVersions(ctx context.Context, serverID string) ([]PHPVersionInfo, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// Fast remote script to detect versions and modules in single SSH round-trip.
	script := `
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
DEFAULT_CLI=$(php -r 'echo PHP_MAJOR_VERSION.".".PHP_MINOR_VERSION;' 2>/dev/null || echo "")

VERSIONS="7.2 7.3 7.4 8.0 8.1 8.2 8.3 8.4 8.5"
if [ -n "$DEFAULT_CLI" ]; then
    case " $VERSIONS " in
        *" $DEFAULT_CLI "*) ;;
        *) VERSIONS="$DEFAULT_CLI $VERSIONS" ;;
    esac
fi

for VER in $VERSIONS; do
    echo "===VER:${VER}==="
    BIN_PATH=$(which php${VER} 2>/dev/null || true)
    if [ -z "$BIN_PATH" ] && [ "$DEFAULT_CLI" = "$VER" ]; then
        BIN_PATH=$(which php 2>/dev/null || true)
    fi

    if [ -n "$BIN_PATH" ] || [ -f "/usr/bin/php${VER}" ] || [ -f "/etc/php/${VER}/fpm/php.ini" ] || [ -f "/etc/php/${VER}/apache2/php.ini" ] || [ -f "/etc/opt/remi/php${VER//./}/php.ini" ]; then
        echo "INSTALLED:yes"
        if [ "$DEFAULT_CLI" = "$VER" ]; then
            echo "DEFAULT_CLI:yes"
        else
            echo "DEFAULT_CLI:no"
        fi

        # Patch version
        PATCH_VER=""
        if [ -n "$BIN_PATH" ]; then
            PATCH_VER=$($BIN_PATH -r 'echo PHP_VERSION;' 2>/dev/null || true)
        elif [ "$DEFAULT_CLI" = "$VER" ]; then
            PATCH_VER=$(php -r 'echo PHP_VERSION;' 2>/dev/null || true)
        fi
        echo "PATCH_VERSION:${PATCH_VER}"

        # Candidate / upgrade check
        CANDIDATE_VER=""
        UPDATE_AVAIL="no"
        if command -v apt-cache >/dev/null 2>&1; then
            PKG_POLICY=$(apt-cache policy "php${VER}-fpm" 2>/dev/null || apt-cache policy "php${VER}-common" 2>/dev/null || true)
            INST_LINE=$(echo "$PKG_POLICY" | grep -m1 "Installed:" | awk '{print $2}' || true)
            CAND_LINE=$(echo "$PKG_POLICY" | grep -m1 "Candidate:" | awk '{print $2}' || true)
            if [ -n "$CAND_LINE" ] && [ "$CAND_LINE" != "(none)" ]; then
                CANDIDATE_VER="$CAND_LINE"
                if [ -n "$INST_LINE" ] && [ "$INST_LINE" != "(none)" ] && [ "$INST_LINE" != "$CAND_LINE" ]; then
                    UPDATE_AVAIL="yes"
                fi
            fi
        elif command -v rpm >/dev/null 2>&1; then
            CLEAN_VER="${VER//./}"
            INST_RPM=$(rpm -q "php${CLEAN_VER}-php-fpm" --qf '%{VERSION}-%{RELEASE}' 2>/dev/null || rpm -q "php-fpm" --qf '%{VERSION}-%{RELEASE}' 2>/dev/null || true)
            if [ -n "$INST_RPM" ]; then
                CANDIDATE_VER="$INST_RPM"
            fi
        fi
        echo "CANDIDATE_VERSION:${CANDIDATE_VER}"
        echo "UPDATE_AVAILABLE:${UPDATE_AVAIL}"

        # Check FPM service status (Debian phpX.Y-fpm, or RHEL php-fpm / phpXY-php-fpm)
        FPM_SRV="php${VER}-fpm"
        STATUS="inactive"
        for s in "php${VER}-fpm" "php-fpm" "rh-php${VER//./}-php-fpm" "php${VER//./}-php-fpm"; do
            if systemctl is-active --quiet "$s" 2>/dev/null; then
                STATUS="active"
                FPM_SRV="$s"
                break
            elif systemctl is-failed --quiet "$s" 2>/dev/null; then
                STATUS="failed"
                FPM_SRV="$s"
            elif systemctl cat "$s" >/dev/null 2>&1 || [ -f "/lib/systemd/system/${s}.service" ] || [ -f "/etc/systemd/system/${s}.service" ]; then
                if [ "$FPM_SRV" = "php${VER}-fpm" ]; then
                    FPM_SRV="$s"
                fi
            fi
        done
        if [ "$STATUS" = "inactive" ] && (pgrep -f "php-fpm.*${VER}" >/dev/null 2>&1 || pgrep -f "php-fpm: master process" >/dev/null 2>&1); then
            STATUS="active"
        fi

        echo "FPM_STATUS:${STATUS}"
        echo "FPM_SERVICE:${FPM_SRV}"

        # Socket path
        if [ -S "/run/php/php${VER}-fpm.sock" ]; then
            echo "SOCKET:/run/php/php${VER}-fpm.sock"
        elif [ -S "/var/run/php/php${VER}-fpm.sock" ]; then
            echo "SOCKET:/var/run/php/php${VER}-fpm.sock"
        elif [ -S "/run/php-fpm/www.sock" ]; then
            echo "SOCKET:/run/php-fpm/www.sock"
        elif [ -S "/var/run/php-fpm/www.sock" ]; then
            echo "SOCKET:/var/run/php-fpm/www.sock"
        else
            echo "SOCKET:/run/php/php${VER}-fpm.sock"
        fi

        # INI path
        if [ -f "/etc/php/${VER}/fpm/php.ini" ]; then
            echo "INI_PATH:/etc/php/${VER}/fpm/php.ini"
        elif [ -f "/etc/php/${VER}/apache2/php.ini" ]; then
            echo "INI_PATH:/etc/php/${VER}/apache2/php.ini"
        elif [ -f "/etc/opt/remi/php${VER//./}/php.ini" ]; then
            echo "INI_PATH:/etc/opt/remi/php${VER//./}/php.ini"
        elif [ -f "/etc/php.ini" ]; then
            echo "INI_PATH:/etc/php.ini"
        else
            echo "INI_PATH:/etc/php/${VER}/fpm/php.ini"
        fi

        # Modules
        echo "---MODULES---"
        if [ -n "$BIN_PATH" ]; then
            $BIN_PATH -m 2>/dev/null || true
        else
            php -m 2>/dev/null || true
        fi
    else
        echo "INSTALLED:no"
        echo "DEFAULT_CLI:no"
        echo "FPM_STATUS:not_installed"
        echo "FPM_SERVICE:php${VER}-fpm"
        echo "SOCKET:/run/php/php${VER}-fpm.sock"
        echo "INI_PATH:/etc/php/${VER}/fpm/php.ini"
    fi
done
`
	out, _, _, err := client.Exec(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to probe PHP versions: %w", err)
	}

	diag, _ := s.Database.GetDiagnostics(ctx, serverID)
	osID := ""
	osVersion := ""
	if diag != nil {
		osID = diag.OSFamily
		if strings.Contains(strings.ToLower(diag.OSName), "ubuntu") {
			osID = "ubuntu"
		}
		osVersion = diag.OSVersion
	}

	var results []PHPVersionInfo
	sections := strings.Split(string(out), "===VER:")

	for _, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}

		parts := strings.SplitN(sec, "===", 2)
		if len(parts) < 2 {
			continue
		}

		version := strings.TrimSpace(parts[0])
		content := parts[1]

		info := PHPVersionInfo{
			Version:    version,
			Extensions: make([]PHPExtensionInfo, 0, len(StandardExtensions)),
		}

		mainContent := content
		var modulesContent string
		if strings.Contains(content, "---MODULES---") {
			mParts := strings.SplitN(content, "---MODULES---", 2)
			mainContent = mParts[0]
			modulesContent = mParts[1]
		}

		lines := strings.Split(mainContent, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "INSTALLED:") {
				info.IsInstalled = strings.TrimPrefix(line, "INSTALLED:") == "yes"
			} else if strings.HasPrefix(line, "DEFAULT_CLI:") {
				info.IsDefaultCLI = strings.TrimPrefix(line, "DEFAULT_CLI:") == "yes"
			} else if strings.HasPrefix(line, "PATCH_VERSION:") {
				info.PatchVersion = strings.TrimSpace(strings.TrimPrefix(line, "PATCH_VERSION:"))
			} else if strings.HasPrefix(line, "CANDIDATE_VERSION:") {
				info.CandidateVersion = cleanCandidateVersion(strings.TrimPrefix(line, "CANDIDATE_VERSION:"))
			} else if strings.HasPrefix(line, "UPDATE_AVAILABLE:") {
				info.UpdateAvailable = strings.TrimPrefix(line, "UPDATE_AVAILABLE:") == "yes"
			} else if strings.HasPrefix(line, "FPM_STATUS:") {
				info.FPMStatus = strings.TrimPrefix(line, "FPM_STATUS:")
			} else if strings.HasPrefix(line, "FPM_SERVICE:") {
				info.FPMService = strings.TrimPrefix(line, "FPM_SERVICE:")
			} else if strings.HasPrefix(line, "SOCKET:") {
				info.SocketPath = strings.TrimPrefix(line, "SOCKET:")
			} else if strings.HasPrefix(line, "INI_PATH:") {
				info.IniPath = strings.TrimPrefix(line, "INI_PATH:")
			}
		}

		loadedModules := make(map[string]bool)
		for _, modLine := range strings.Split(modulesContent, "\n") {
			mod := strings.ToLower(strings.TrimSpace(modLine))
			if mod != "" && !strings.HasPrefix(mod, "[") {
				loadedModules[mod] = true
			}
		}

		info.AvailableHandlers = []string{}
		if info.IsInstalled {
			info.AvailableHandlers = append(info.AvailableHandlers, "cli")
			if info.FPMStatus == "active" || info.FPMStatus == "inactive" {
				info.AvailableHandlers = append(info.AvailableHandlers, "fpm")
			}
		}

		for _, catExt := range ExtensionCatalog {
			extNameLower := strings.ToLower(catExt.Name)
			isLoaded := loadedModules[extNameLower]
			if !isLoaded {
				if extNameLower == "mysql" && (loadedModules["pdo_mysql"] || loadedModules["mysqli"]) {
					isLoaded = true
				} else if extNameLower == "xml" && (loadedModules["libxml"] || loadedModules["dom"]) {
					isLoaded = true
				} else if extNameLower == "pgsql" && loadedModules["pdo_pgsql"] {
					isLoaded = true
				} else if extNameLower == "sqlite3" && (loadedModules["pdo_sqlite"] || loadedModules["sqlite3"]) {
					isLoaded = true
				}
			}

			info.Extensions = append(info.Extensions, PHPExtensionInfo{
				Name:        catExt.Name,
				Category:    catExt.Category,
				Description: catExt.Description,
				IsInstalled: info.IsInstalled && isLoaded,
				IsEnabled:   info.IsInstalled && isLoaded,
			})
		}

		if info.IsInstalled {
			info.IsAvailable = true
		} else {
			if osID != "" {
				allowed, reason := capabilities.AllowsPHPInstallByVersion(osID, osVersion, info.Version)
				if !allowed {
					info.IsAvailable = false
					info.UnavailableReason = reason
				} else {
					info.IsAvailable = true
				}
			} else {
				if info.Version == "8.5" {
					info.IsAvailable = false
					info.UnavailableReason = "Версия 8.5 пока не выпущена в стабильных репозиториях"
				} else {
					info.IsAvailable = true
				}
			}
		}

		results = append(results, info)
	}

	return results, nil
}

// PlanPHPInstall inspects distribution repositories and calculates the safe installation blueprint.
func (s *Service) PlanPHPInstall(ctx context.Context, serverID, version string, extensions []string) (*InstallPHPPlan, error) {
	diag, err := s.Database.GetDiagnostics(ctx, serverID)
	if err != nil || diag == nil {
		return nil, fmt.Errorf("diagnostics not found for server %s (run diagnostics first)", serverID)
	}

	plan := &InstallPHPPlan{
		TargetVersion:       version,
		OSFamily:            diag.OSFamily,
		OSDistro:            diag.OSName,
		OSCompatible:        true,
		RepositoriesNeeded:  []string{},
		PackagesToInstall:   []string{},
		FPMServiceToStart:   fmt.Sprintf("php%s-fpm", version),
		EstimatedDownloadMB: 45,
	}

	// Capability check using OS release profile (ADR-S1, ADR-S2).
	osReleaseMock := fmt.Sprintf("ID=%s\nVERSION_ID=\"%s\"\nNAME=\"%s\"\n", diag.OSFamily, diag.OSVersion, diag.OSName)
	if strings.Contains(strings.ToLower(diag.OSName), "ubuntu") {
		osReleaseMock = fmt.Sprintf("ID=ubuntu\nVERSION_ID=\"%s\"\nNAME=\"%s\"\n", diag.OSVersion, diag.OSName)
	}
	if allowed, reason := capabilities.AllowsPHPInstall(osReleaseMock, version); !allowed {
		plan.OSCompatible = false
		plan.Message = reason
		plan.PackagesToInstall = []string{}
		plan.RepositoriesNeeded = []string{}
		return plan, nil
	}

	if diag.OSFamily == "debian" {
		plan.PackagesToInstall = []string{
			fmt.Sprintf("php%s-fpm", version),
			fmt.Sprintf("php%s-cli", version),
			fmt.Sprintf("php%s-common", version),
			fmt.Sprintf("php%s-opcache", version),
		}

		// Repository check: ondrej/php for Ubuntu or packages.sury.org for Debian.
		// EOL Ubuntu releases: 18.04 installs from the distro archive only.
		if strings.Contains(strings.ToLower(diag.OSName), "ubuntu") {
			if strings.HasPrefix(diag.OSVersion, "18.04") || (strings.HasPrefix(diag.OSVersion, "20.04") && version == "7.4") {
				plan.RepositoriesNeeded = append(plan.RepositoriesNeeded, "архив дистрибутива (archive.ubuntu.com)")
			} else {
				plan.RepositoriesNeeded = append(plan.RepositoriesNeeded, "ppa:ondrej/php (Automated APT repository)")
			}
		} else {
			plan.RepositoriesNeeded = append(plan.RepositoriesNeeded, "packages.sury.org/php (Debian multi-version repository)")
		}

		for _, ext := range extensions {
			ext = strings.TrimSpace(ext)
			if ext == "" || ext == "opcache" {
				continue
			}
			plan.PackagesToInstall = append(plan.PackagesToInstall, fmt.Sprintf("php%s-%s", version, ext))
		}
		plan.Message = fmt.Sprintf("Ready to install PHP %s with %d packages on %s", version, len(plan.PackagesToInstall), diag.OSName)
	} else if diag.OSFamily == "rhel" {
		plan.RepositoriesNeeded = append(plan.RepositoriesNeeded, "epel-release", "remi-release (Remi RPM repository)")
		plan.PackagesToInstall = []string{
			fmt.Sprintf("php%s-php-fpm", strings.ReplaceAll(version, ".", "")),
			fmt.Sprintf("php%s-php-cli", strings.ReplaceAll(version, ".", "")),
		}
		plan.Message = fmt.Sprintf("Ready to configure Remi RPM repository and install PHP %s on %s", version, diag.OSName)
	} else {
		plan.OSCompatible = false
		plan.Message = fmt.Sprintf("Unsupported OS family: %s", diag.OSFamily)
	}

	return plan, nil
}

// InstallPHPVersion triggers asynchronous installation task in TaskEngine.
func (s *Service) InstallPHPVersion(serverID, version string, extensions []string) (*db.TaskRecord, error) {
	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	payloadMap := map[string]interface{}{
		"server_id":  record.ID,
		"version":    version,
		"extensions": extensions,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	title := fmt.Sprintf("Install PHP %s on %s (%s)", version, record.Name, record.Host)
	return s.engine.Submit(record.ID, "php_install", title, string(payloadBytes))
}

// UpdatePHPVersion queues an asynchronous task to upgrade packages for an installed PHP version.
func (s *Service) UpdatePHPVersion(serverID, version string) (*db.TaskRecord, error) {
	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	payloadMap := map[string]interface{}{
		"server_id": record.ID,
		"version":   version,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	title := fmt.Sprintf("Update PHP %s on %s (%s)", version, record.Name, record.Host)
	return s.engine.Submit(record.ID, "php_update", title, string(payloadBytes))
}

// ToggleExtension enables or disables an extension for a specific PHP version.
func (s *Service) ToggleExtension(serverID, version, extension string, enable bool) (*db.TaskRecord, error) {
	record, err := s.Database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	actionStr := "Enable"
	if !enable {
		actionStr = "Disable"
	}

	payloadMap := map[string]interface{}{
		"server_id": record.ID,
		"version":   version,
		"extension": extension,
		"enable":    enable,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	title := fmt.Sprintf("%s PHP %s extension '%s' on %s", actionStr, version, extension, record.Name)
	return s.engine.Submit(record.ID, "php_ext_toggle", title, string(payloadBytes))
}

// GetPHPIni reads php.ini for the specified PHP version.
func (s *Service) GetPHPIni(ctx context.Context, serverID, version string) (string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	iniPath := fmt.Sprintf("/etc/php/%s/fpm/php.ini", version)
	data, err := client.ReadFile(ctx, iniPath)
	if err != nil {
		// Fallback to cli php.ini.
		iniPath = fmt.Sprintf("/etc/php/%s/cli/php.ini", version)
		data, err = client.ReadFile(ctx, iniPath)
		if err != nil {
			return "", fmt.Errorf("failed to read php.ini for version %s: %w", version, err)
		}
	}

	return string(data), nil
}

// SavePHPIni saves php.ini with backup and syntax validation before restarting FPM.
func (s *Service) SavePHPIni(ctx context.Context, serverID, version string, content string) error {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	iniPath := fmt.Sprintf("/etc/php/%s/fpm/php.ini", version)
	backupPath := fmt.Sprintf("%s.bak.%d", iniPath, time.Now().Unix())

	_, _, _, _ = client.NewCommand(fmt.Sprintf("cp %s %s", ssh.ShellEscape(iniPath), ssh.ShellEscape(backupPath))).WithSudo().Exec(ctx)

	tmpPath := fmt.Sprintf("/tmp/php.ini.test.%d", time.Now().Unix())
	if err := client.WriteFile(ctx, tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temporary php.ini: %w", err)
	}

	testCmd := fmt.Sprintf("php%s -c %s -v", version, ssh.ShellEscape(tmpPath))
	out, stderr, code, err := client.NewCommand(testCmd).Exec(ctx)
	if err != nil || code != 0 {
		_ = client.RemovePath(ctx, tmpPath, false)
		return fmt.Errorf("php.ini syntax validation failed: %s %s", string(out), string(stderr))
	}

	mvCmd := fmt.Sprintf("mv %s %s && systemctl reload php%s-fpm", ssh.ShellEscape(tmpPath), ssh.ShellEscape(iniPath), version)
	out, stderr, code, err = client.NewCommand(mvCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		// Rollback from backup.
		rbCmd := fmt.Sprintf("cp %s %s && systemctl reload php%s-fpm", ssh.ShellEscape(backupPath), ssh.ShellEscape(iniPath), version)
		_, _, _, _ = client.NewCommand(rbCmd).WithSudo().Exec(ctx)
		return fmt.Errorf("failed to apply php.ini: %s %s", string(out), string(stderr))
	}

	return nil
}

// RestartFPM safely reloads or restarts PHP-FPM for the given version.
func (s *Service) RestartFPM(ctx context.Context, serverID, version string) error {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	cleanVer := strings.ReplaceAll(version, ".", "")
	cmd := fmt.Sprintf("systemctl restart php%s-fpm 2>/dev/null || systemctl restart php-fpm 2>/dev/null || systemctl restart rh-php%s-php-fpm 2>/dev/null || systemctl restart php%s-php-fpm 2>/dev/null", version, cleanVer, cleanVer)
	_, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to restart PHP-FPM for version %s: %s", version, string(stderr))
	}
	return nil
}

// ToggleFPM safely starts/enables or stops/disables PHP-FPM for the given version.
func (s *Service) ToggleFPM(ctx context.Context, serverID, version string, enable bool) error {
	if err := ssh.ValidatePHPVersion(version); err != nil {
		return err
	}
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	cleanVer := strings.ReplaceAll(version, ".", "")
	var cmd string
	if enable {
		// Unmask, enable on boot, and start the service
		cmd = fmt.Sprintf(
			"systemctl unmask php%s-fpm 2>/dev/null; systemctl enable php%s-fpm 2>/dev/null; systemctl start php%s-fpm 2>/dev/null || "+
				"systemctl start php-fpm 2>/dev/null || systemctl start rh-php%s-php-fpm 2>/dev/null || systemctl start php%s-php-fpm 2>/dev/null",
			version, version, version, cleanVer, cleanVer,
		)
	} else {
		// Stop the service and disable from auto-starting
		cmd = fmt.Sprintf(
			"systemctl stop php%s-fpm 2>/dev/null || systemctl stop php-fpm 2>/dev/null || systemctl stop rh-php%s-php-fpm 2>/dev/null || systemctl stop php%s-php-fpm 2>/dev/null; "+
				"systemctl disable php%s-fpm 2>/dev/null || systemctl disable php-fpm 2>/dev/null || systemctl disable rh-php%s-php-fpm 2>/dev/null || systemctl disable php%s-php-fpm 2>/dev/null || true",
			version, cleanVer, cleanVer, version, cleanVer, cleanVer,
		)
	}

	stdout, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || (enable && code != 0) {
		action := "stop"
		if enable {
			action = "start"
		}
		return fmt.Errorf("failed to %s PHP-FPM %s: %s %s", action, version, string(stdout), string(stderr))
	}
	return nil
}

// handlePHPInstallTask executes the full PHP provisioning pipeline in background.
func (s *Service) handlePHPInstallTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
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
	for _, ext := range payload.Extensions {
		ext = strings.TrimSpace(ext)
		if ext == "" || ext == "opcache" {
			continue
		}
		if err := ssh.ValidateToken(ext, "PHP extension"); err != nil {
			return "", err
		}
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

	// Capture current default /usr/bin/php before installation to preserve CLI compatibility.
	outPrevCLI, _, _, _ := client.Exec(ctx, "readlink -f /usr/bin/php 2>/dev/null || true")
	prevCLI := strings.TrimSpace(string(outPrevCLI))

	// Check free disk space before installing packages.
	logger.SetStep("Checking free disk space", 15)
	outDF, _, _, _ := client.Exec(ctx, "df -k / 2>/dev/null | tail -n 1 | awk '{print $4}'")
	freeKB, _ := strconv.ParseInt(strings.TrimSpace(string(outDF)), 10, 64)
	if freeKB > 0 && freeKB < 150*1024 { // Less than 150MB
		return "", fmt.Errorf("недостаточно свободного места на диске для установки PHP: свободно %d МБ, требуется минимум 150 МБ", freeKB/1024)
	}

	logger.SetStep("Checking and configuring repositories", 25)
	if err := InstallVersionPackages(ctx, client, osFamily, payload.Version, payload.Extensions, logger); err != nil {
		return "", err
	}

	// Preserve existing default CLI alternative if one was configured.
	if prevCLI != "" && strings.Contains(prevCLI, "/php") {
		_, _, _, _ = client.NewCommand(fmt.Sprintf("update-alternatives --set php %s 2>/dev/null || true", ssh.ShellEscape(prevCLI))).WithSudo().Exec(ctx)
	}

	// Update cached diagnostics.
	if diag != nil {
		hasVer := false
		for _, v := range diag.InstalledPHPVersions {
			if v == payload.Version {
				hasVer = true
				break
			}
		}
		if !hasVer {
			diag.InstalledPHPVersions = append(diag.InstalledPHPVersions, payload.Version)
			_ = s.Database.SaveDiagnostics(ctx, diag)
		}
	}

	logger.SetStep("PHP installation successfully completed", 100)
	return fmt.Sprintf("PHP %s and FPM service installed successfully on %s", payload.Version, record.Host), nil
}

// handlePHPExtToggleTask enables or disables a single extension.
func (s *Service) handlePHPExtToggleTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID  string `json:"server_id"`
		Version   string `json:"version"`
		Extension string `json:"extension"`
		Enable    bool   `json:"enable"`
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

	actionCmd := "phpenmod"
	if !payload.Enable {
		actionCmd = "phpdismod"
	}

	logger.SetStep(fmt.Sprintf("Executing %s for %s", actionCmd, payload.Extension), 50)
	ver := ssh.ShellEscape(payload.Version)
	ext := ssh.ShellEscape(payload.Extension)
	cmd := fmt.Sprintf("%s -v %s %s || (export DEBIAN_FRONTEND=noninteractive && apt-get install -y php%s-%s)",
		actionCmd, ver, ext, payload.Version, payload.Extension)

	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	logger.Info(string(out))
	if err != nil || code != 0 {
		return "", fmt.Errorf("failed to toggle extension: %s", string(stderr))
	}

	logger.SetStep(fmt.Sprintf("Reloading php%s-fpm service", payload.Version), 80)
	serviceName := "php" + payload.Version + "-fpm"
	relOut, relErr, relCode, reloadErr := client.NewCommand("systemctl reload " + ssh.ShellEscape(serviceName)).WithSudo().Exec(ctx)
	if reloadErr != nil || relCode != 0 {
		return "", fmt.Errorf("failed to reload %s: %s %v", serviceName, string(relErr), relOut)
	}

	logger.SetStep("Completed", 100)
	return fmt.Sprintf("Extension %s successfully updated for PHP %s", payload.Extension, payload.Version), nil
}

// handlePHPUpdateTask upgrades installed packages for a specific PHP version and restarts FPM.
func (s *Service) handlePHPUpdateTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID string `json:"server_id"`
		Version  string `json:"version"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidatePHPVersion(payload.Version); err != nil {
		return "", err
	}

	logger.SetStep("Подключение к серверу", 10)
	client, record, err := s.GetSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	diag, _ := s.Database.GetDiagnostics(ctx, payload.ServerID)
	osFamily := "debian"
	if diag != nil && diag.OSFamily != "" {
		osFamily = diag.OSFamily
	}

	// Check free disk space before installing updates.
	logger.SetStep("Проверка свободного места на диске", 15)
	outDF, _, _, _ := client.Exec(ctx, "df -k / 2>/dev/null | tail -n 1 | awk '{print $4}'")
	freeKB, _ := strconv.ParseInt(strings.TrimSpace(string(outDF)), 10, 64)
	if freeKB > 0 && freeKB < 150*1024 { // Less than 150MB
		return "", fmt.Errorf("недостаточно свободного места на диске: свободно %d МБ, требуется минимум 150 МБ", freeKB/1024)
	}

	logger.SetStep("Обновление списков пакетов репозиториев", 25)
	if osFamily == "debian" {
		if err := module.RunPrivileged(ctx, client, "apt-get update -y"); err != nil {
			logger.Info(fmt.Sprintf("Предупреждение при apt-get update: %v", err))
		}
	} else if osFamily == "rhel" {
		if err := module.RunPrivileged(ctx, client, "dnf makecache -y || yum makecache -y"); err != nil {
			logger.Info(fmt.Sprintf("Предупреждение при dnf makecache: %v", err))
		}
	}

	logger.SetStep(fmt.Sprintf("Обновление установленных пакетов PHP %s", payload.Version), 50)
	var updateCmd string
	if osFamily == "debian" {
		findCmd := fmt.Sprintf(`dpkg-query -W -f='${Package} ${Status}\n' 'php%s*' 2>/dev/null | grep 'install ok installed' | awk '{print $1}' | tr '\n' ' '`, payload.Version)
		outPkgs, _, _, _ := client.Exec(ctx, findCmd)
		pkgs := strings.TrimSpace(string(outPkgs))
		if pkgs == "" {
			pkgs = fmt.Sprintf("php%s-fpm php%s-cli php%s-common", payload.Version, payload.Version, payload.Version)
		}
		logger.Info(fmt.Sprintf("Обновляемые пакеты: %s", pkgs))
		updateCmd = fmt.Sprintf("apt-get install --only-upgrade -y --no-install-recommends %s", pkgs)
	} else {
		cleanVer := strings.ReplaceAll(payload.Version, ".", "")
		updateCmd = fmt.Sprintf("dnf upgrade -y php%s* php-* || yum update -y php%s* php-*", cleanVer, cleanVer)
	}

	outUp, stderrUp, codeUp, errUp := client.NewCommand(updateCmd).
		WithSudo().
		WithLock("pkg_mgr", 180).
		WithEnv("DEBIAN_FRONTEND", "noninteractive").
		Exec(ctx)
	logger.Info(string(outUp))
	if errUp != nil || codeUp != 0 {
		return "", fmt.Errorf("ошибка обновления пакетов PHP %s (код %d): %s", payload.Version, codeUp, strings.TrimSpace(string(stderrUp)))
	}

	logger.SetStep(fmt.Sprintf("Перезапуск службы PHP-FPM %s", payload.Version), 80)
	if err := s.RestartFPM(ctx, payload.ServerID, payload.Version); err != nil {
		logger.Info(fmt.Sprintf("Предупреждение при перезапуске FPM: %v", err))
	}

	logger.SetStep("Проверка установленной версии PHP", 90)
	verCmd := fmt.Sprintf("php%s -v 2>/dev/null | head -n 1 || php -v 2>/dev/null | head -n 1 || true", payload.Version)
	outVer, _, _, _ := client.Exec(ctx, verCmd)
	phpVerLine := strings.TrimSpace(string(outVer))
	if phpVerLine != "" {
		logger.Info(fmt.Sprintf("Актуальная версия: %s", phpVerLine))
	}

	logger.SetStep("Обновление PHP успешно завершено", 100)
	return fmt.Sprintf("PHP %s успешно обновлен на сервере %s (%s)", payload.Version, record.Name, phpVerLine), nil
}
