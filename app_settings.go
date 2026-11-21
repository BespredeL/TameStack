package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdRuntime "runtime"
	"strings"
	"time"

	"tamestack/internal/logger"
)

type AppSettingsInfo struct {
	DataDir           string `json:"data_dir"`
	DatabasePath      string `json:"database_path"`
	DatabaseSizeBytes int64  `json:"database_size_bytes"`
	DatabaseSizeHuman string `json:"database_size_human"`
	KnownHostsPath    string `json:"known_hosts_path"`
	KnownHostsCount   int    `json:"known_hosts_count"`
	SaltPath          string `json:"salt_path"`
	VaultKeyPath      string `json:"vault_key_path"`
	LogPath           string `json:"log_path"`
	LogSizeBytes      int64  `json:"log_size_bytes"`
	LogSizeHuman      string `json:"log_size_human"`
	OperatingSystem   string `json:"operating_system"`
	Architecture      string `json:"architecture"`
	GoVersion         string `json:"go_version"`
	Version           string `json:"version"`
	AIMemoryFactCount int    `json:"ai_memory_fact_count"`
	ActiveWorkers     int    `json:"active_workers"`
	DBEncrypted       bool   `json:"db_encrypted"`
}

func getFileDiagnostic(path string) (int64, string) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, "0 B"
	}
	bytes := fi.Size()
	return bytes, formatByteUnits(bytes)
}

func formatByteUnits(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func countKnownHostsEntries(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}

func (a *App) GetAppSettings() (*AppSettingsInfo, error) {
	return withLock(a, func() (*AppSettingsInfo, error) {
		if a.database == nil {
			return nil, fmt.Errorf("database is locked")
		}
		activeDBPath := a.database.Path()
		dbBytes, dbHuman := getFileDiagnostic(activeDBPath)
		logBytes, logHuman := getFileDiagnostic(a.cfg.LogPath)
		hostsCount := countKnownHostsEntries(a.cfg.KnownHostsPath)

		factCount, _ := a.database.CountAIMemoryFacts(a.ctx)

		info := &AppSettingsInfo{
			DataDir:           a.cfg.DataDir,
			DatabasePath:      activeDBPath,
			DatabaseSizeBytes: dbBytes,
			DatabaseSizeHuman: dbHuman,
			DBEncrypted:       a.dbSession != nil,
			KnownHostsPath:    a.cfg.KnownHostsPath,
			KnownHostsCount:   hostsCount,
			SaltPath:          a.cfg.SaltPath,
			VaultKeyPath:      a.cfg.VaultKeyPath,
			LogPath:           a.cfg.LogPath,
			LogSizeBytes:      logBytes,
			LogSizeHuman:      logHuman,
			OperatingSystem:   stdRuntime.GOOS,
			Architecture:      stdRuntime.GOARCH,
			GoVersion:         stdRuntime.Version(),
			Version:           AppVersion,
			AIMemoryFactCount: factCount,
			ActiveWorkers:     4,
		}

		return info, nil
	})
}

func (a *App) OpenDataDirectory() error {
	dir := a.cfg.DataDir
	var cmd *exec.Cmd
	switch stdRuntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}

func (a *App) VacuumDatabase() error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	_, err := a.database.ExecContext(a.ctx, "VACUUM;")
	return err
}

func (a *App) SetAppLogLevel(level string) error {
	logger.SetLogLevel(level)
	return nil
}

func (a *App) ExportSupportBundle() (string, error) {
	diagInfo, err := a.GetAppSettings()
	if err != nil {
		return "", fmt.Errorf("failed to get app diagnostics: %w", err)
	}

	diagMap := map[string]interface{}{
		"os":             diagInfo.OperatingSystem,
		"arch":           diagInfo.Architecture,
		"go_version":     diagInfo.GoVersion,
		"app_version":    diagInfo.Version,
		"known_hosts":    diagInfo.KnownHostsCount,
		"db_size_human":  diagInfo.DatabaseSizeHuman,
		"log_size_human": diagInfo.LogSizeHuman,
		"active_workers": diagInfo.ActiveWorkers,
		"ai_facts_count": diagInfo.AIMemoryFactCount,
		"exported_at":    time.Now().Format(time.RFC3339),
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		userHome = "."
	}
	targetDir := filepath.Join(userHome, "Downloads")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		targetDir = userHome
	}

	fileName := fmt.Sprintf("tamestack_support_bundle_%s.zip", time.Now().Format("20060102_150405"))
	bundlePath := filepath.Join(targetDir, fileName)

	if err := logger.GenerateSupportBundle(bundlePath, diagMap); err != nil {
		return "", fmt.Errorf("failed to create support bundle: %w", err)
	}

	return bundlePath, nil
}
