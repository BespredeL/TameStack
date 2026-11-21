package php

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func resolveIniPath(version, target string) string {
	cleanVer := sanitizeVersion(version)
	if target == "cli" {
		return fmt.Sprintf("/etc/php/%s/cli/php.ini", cleanVer)
	}
	return fmt.Sprintf("/etc/php/%s/fpm/php.ini", cleanVer)
}

// GetParsedPHPSettings parses structured key parameters from php.ini.
func (s *Service) GetParsedPHPSettings(ctx context.Context, serverID, version, target string) (*PHPSettings, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	iniPath := resolveIniPath(version, target)
	data, err := client.ReadFile(ctx, iniPath)
	if err != nil {
		// Fallback for RHEL Remi
		cleanCompact := strings.ReplaceAll(version, ".", "")
		remiPath := fmt.Sprintf("/etc/opt/remi/php%s/php.ini", cleanCompact)
		data, err = client.ReadFile(ctx, remiPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read php.ini at %s: %w", iniPath, err)
		}
	}

	return ParseIniSettings(string(data)), nil
}

// ParseIniSettings parses raw php.ini content into PHPSettings struct.
func ParseIniSettings(raw string) *PHPSettings {
	settings := &PHPSettings{
		UploadMaxFilesize:     "32M",
		PostMaxSize:           "32M",
		MemoryLimit:           "256M",
		MaxExecutionTime:      60,
		MaxInputTime:          60,
		MaxInputVars:          1000,
		DateTimezone:          "UTC",
		ErrorReporting:        "E_ALL & ~E_DEPRECATED & ~E_STRICT",
		DisplayErrors:         false,
		LogErrors:             true,
		OpcacheEnable:         true,
		OpcacheMemory:         128,
		OpcacheMaxFiles:       10000,
		OpcacheRevalidateFreq: 2,
		ShortOpenTag:          false,
		ExposePHP:             false,
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		// Remove inline comments
		if idx := strings.Index(val, ";"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}
		val = strings.Trim(val, `"'`)

		switch key {
		case "upload_max_filesize":
			settings.UploadMaxFilesize = val
		case "post_max_size":
			settings.PostMaxSize = val
		case "memory_limit":
			settings.MemoryLimit = val
		case "max_execution_time":
			if n, err := strconv.Atoi(val); err == nil {
				settings.MaxExecutionTime = n
			}
		case "max_input_time":
			if n, err := strconv.Atoi(val); err == nil {
				settings.MaxInputTime = n
			}
		case "max_input_vars":
			if n, err := strconv.Atoi(val); err == nil {
				settings.MaxInputVars = n
			}
		case "date.timezone":
			settings.DateTimezone = val
		case "error_reporting":
			settings.ErrorReporting = val
		case "display_errors":
			settings.DisplayErrors = isTruthy(val)
		case "log_errors":
			settings.LogErrors = isTruthy(val)
		case "opcache.enable":
			settings.OpcacheEnable = isTruthy(val)
		case "opcache.memory_consumption":
			if n, err := strconv.Atoi(val); err == nil {
				settings.OpcacheMemory = n
			}
		case "opcache.max_accelerated_files":
			if n, err := strconv.Atoi(val); err == nil {
				settings.OpcacheMaxFiles = n
			}
		case "opcache.revalidate_freq":
			if n, err := strconv.Atoi(val); err == nil {
				settings.OpcacheRevalidateFreq = n
			}
		case "short_open_tag":
			settings.ShortOpenTag = isTruthy(val)
		case "expose_php":
			settings.ExposePHP = isTruthy(val)
		}
	}

	return settings
}

func isTruthy(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "1" || v == "on" || v == "true" || v == "yes"
}

// ApplySettingsToIniContent updates specified directive values in existing ini string.
func ApplySettingsToIniContent(existing string, s PHPSettings) string {
	directives := map[string]string{
		"upload_max_filesize":            s.UploadMaxFilesize,
		"post_max_size":                  s.PostMaxSize,
		"memory_limit":                   s.MemoryLimit,
		"max_execution_time":             strconv.Itoa(s.MaxExecutionTime),
		"max_input_time":                 strconv.Itoa(s.MaxInputTime),
		"max_input_vars":                 strconv.Itoa(s.MaxInputVars),
		"date.timezone":                  s.DateTimezone,
		"error_reporting":                s.ErrorReporting,
		"display_errors":                 boolToOnOff(s.DisplayErrors),
		"log_errors":                     boolToOnOff(s.LogErrors),
		"opcache.enable":                 boolTo10(s.OpcacheEnable),
		"opcache.memory_consumption":     strconv.Itoa(s.OpcacheMemory),
		"opcache.max_accelerated_files": strconv.Itoa(s.OpcacheMaxFiles),
		"opcache.revalidate_freq":        strconv.Itoa(s.OpcacheRevalidateFreq),
		"short_open_tag":                 boolToOnOff(s.ShortOpenTag),
		"expose_php":                     boolToOnOff(s.ExposePHP),
	}

	lines := strings.Split(existing, "\n")
	applied := make(map[string]bool)

	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		replaced := false
		if trimmed != "" && !strings.HasPrefix(trimmed, ";") && !strings.HasPrefix(trimmed, "#") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				k := strings.ToLower(strings.TrimSpace(parts[0]))
				if newVal, exists := directives[k]; exists && newVal != "" {
					out = append(out, fmt.Sprintf("%s = %s", k, newVal))
					applied[k] = true
					replaced = true
				}
			}
		}
		if !replaced {
			out = append(out, line)
		}
	}

	// Append any directives not found in original file
	for k, v := range directives {
		if !applied[k] && v != "" {
			out = append(out, fmt.Sprintf("%s = %s", k, v))
		}
	}

	return strings.Join(out, "\n")
}

func boolToOnOff(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}

func boolTo10(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// SaveParsedPHPSettings safely applies structured settings to php.ini with rollback.
func (s *Service) SaveParsedPHPSettings(ctx context.Context, serverID, version, target string, settings PHPSettings) error {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	iniPath := resolveIniPath(version, target)
	existingData, err := client.ReadFile(ctx, iniPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", iniPath, err)
	}

	updated := ApplySettingsToIniContent(string(existingData), settings)

	cleanVer := sanitizeVersion(version)
	testCmd := fmt.Sprintf("php%s -v 2>&1 || php -v 2>&1", cleanVer)
	reloadCmd := ""
	if target == "fpm" {
		reloadCmd = fmt.Sprintf("systemctl reload php%s-fpm 2>/dev/null || systemctl restart php%s-fpm 2>/dev/null", cleanVer, cleanVer)
	}

	guardRes, err := client.SafeWriteWithTest(ctx, iniPath, []byte(updated), testCmd, reloadCmd)
	if err != nil {
		errMsg := err.Error()
		if guardRes != nil && guardRes.TestOutput != "" {
			errMsg = fmt.Sprintf("%s: %s", errMsg, guardRes.TestOutput)
		}
		return fmt.Errorf("failed to save php.ini settings (rolled back): %s", errMsg)
	}

	return nil
}

// ListPHPIniBackups lists all timestamped backups for a php.ini file.
func (s *Service) ListPHPIniBackups(ctx context.Context, serverID, version, target string) ([]PHPIniBackupItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	iniPath := resolveIniPath(version, target)
	dir := filepath.ToSlash(filepath.Dir(iniPath))

	cmd := fmt.Sprintf(`for f in %s/php.ini.tamestack.bak_* %s/php.ini.bak.*; do
    if [ -f "$f" ]; then
        echo "===BAK==="
        echo "PATH:$f"
        echo "NAME:$(basename "$f")"
        echo "SIZE:$(stat -c %%s "$f" 2>/dev/null || stat -f %%z "$f" 2>/dev/null || echo 0)"
        echo "TIME:$(stat -c %%Y "$f" 2>/dev/null || stat -f %%m "$f" 2>/dev/null || echo 0)"
    fi
done`, dir, dir)

	out, _, _, err := client.Exec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	var items []PHPIniBackupItem
	for _, sec := range strings.Split(string(out), "===BAK===") {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}

		item := PHPIniBackupItem{}
		for _, line := range strings.Split(sec, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PATH:") {
				item.Path = strings.TrimPrefix(line, "PATH:")
			} else if strings.HasPrefix(line, "NAME:") {
				item.Filename = strings.TrimPrefix(line, "NAME:")
			} else if strings.HasPrefix(line, "SIZE:") {
				item.SizeBytes, _ = strconv.ParseInt(strings.TrimPrefix(line, "SIZE:"), 10, 64)
			} else if strings.HasPrefix(line, "TIME:") {
				item.Timestamp, _ = strconv.ParseInt(strings.TrimPrefix(line, "TIME:"), 10, 64)
				if item.Timestamp > 0 {
					item.CreatedAt = time.Unix(item.Timestamp, 0).Format("2006-01-02 15:04:05")
				}
			}
		}
		if item.Path != "" {
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	return items, nil
}

// RestorePHPIniBackup restores php.ini from a selected backup file.
func (s *Service) RestorePHPIniBackup(ctx context.Context, serverID, version, target, backupFile string) error {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	iniPath := resolveIniPath(version, target)
	backupData, err := client.ReadFile(ctx, backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file %s: %w", backupFile, err)
	}

	cleanVer := sanitizeVersion(version)
	testCmd := fmt.Sprintf("php%s -v 2>&1 || php -v 2>&1", cleanVer)
	reloadCmd := ""
	if target == "fpm" {
		reloadCmd = fmt.Sprintf("systemctl reload php%s-fpm 2>/dev/null || systemctl restart php%s-fpm 2>/dev/null", cleanVer, cleanVer)
	}

	guardRes, err := client.SafeWriteWithTest(ctx, iniPath, backupData, testCmd, reloadCmd)
	if err != nil {
		errMsg := err.Error()
		if guardRes != nil && guardRes.TestOutput != "" {
			errMsg = fmt.Sprintf("%s: %s", errMsg, guardRes.TestOutput)
		}
		return fmt.Errorf("failed to restore php.ini (rolled back): %s", errMsg)
	}

	return nil
}
