package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ForcePortable can be set at compile time or in tests to force portable mode.
var ForcePortable = false

// AppConfig represents configuration and standard paths for TameStack.
type AppConfig struct {
	IsPortable            bool
	DataDir               string
	DatabasePath          string
	DatabaseContainerPath string
	RunDir                string
	WorkingDatabasePath   string
	KnownHostsPath        string
	SaltPath              string
	VaultKeyPath          string
	LogPath               string
}

// isPortableMode detects whether TameStack should run in portable mode.
func isPortableMode() bool {
	if ForcePortable {
		return true
	}

	exePath, err := os.Executable()
	if err == nil {
		exeName := strings.ToLower(filepath.Base(exePath))
		// If binary name contains "portable" (e.g. TameStack-portable.exe, TameStack-windows-amd64-portable.exe)
		if strings.Contains(exeName, "portable") {
			return true
		}

		exeDir := filepath.Dir(exePath)
		// Or if a "data" directory exists alongside the executable
		if fi, err := os.Stat(filepath.Join(exeDir, "data")); err == nil && fi.IsDir() {
			return true
		}

		// Or if a ".portable" marker file exists alongside the executable
		if _, err := os.Stat(filepath.Join(exeDir, ".portable")); err == nil {
			return true
		}
	}

	return false
}

// LoadConfig resolves or initializes user directories for TameStack.
func LoadConfig() (*AppConfig, error) {
	var baseDir string
	portable := isPortableMode()

	if portable {
		exePath, err := os.Executable()
		if err == nil {
			baseDir = filepath.Join(filepath.Dir(exePath), "data")
		} else {
			baseDir = filepath.Join(".", "data")
		}
	} else {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			userHome, errHome := os.UserHomeDir()
			if errHome != nil {
				baseDir = filepath.Join(".", ".tamestack")
			} else {
				baseDir = filepath.Join(userHome, ".tamestack")
			}
		} else {
			baseDir = filepath.Join(userConfigDir, "TameStack")
		}
	}

	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory %s: %w", baseDir, err)
	}

	runDir := filepath.Join(baseDir, "run")
	cfg := &AppConfig{
		IsPortable:            portable,
		DataDir:               baseDir,
		DatabasePath:          filepath.Join(baseDir, "tamestack.db"),
		DatabaseContainerPath: filepath.Join(baseDir, "tamestack.db.enc"),
		RunDir:                runDir,
		WorkingDatabasePath:   filepath.Join(runDir, "tamestack.db"),
		KnownHostsPath:        filepath.Join(baseDir, "known_hosts"),
		SaltPath:              filepath.Join(baseDir, "vault.salt"),
		VaultKeyPath:          filepath.Join(baseDir, "vault.key"),
		LogPath:               filepath.Join(baseDir, "tamestack.log"),
	}

	// Ensure salt exists for key derivation.
	if _, err := os.Stat(cfg.SaltPath); os.IsNotExist(err) {
		salt := make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("failed to generate crypto salt: %w", err)
		}
		if err := os.WriteFile(cfg.SaltPath, salt, 0600); err != nil {
			return nil, fmt.Errorf("failed to write salt file: %w", err)
		}
	}

	// Ensure known_hosts file exists with safe permissions.
	if _, err := os.Stat(cfg.KnownHostsPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfg.KnownHostsPath, []byte(""), 0600); err != nil {
			return nil, fmt.Errorf("failed to initialize known_hosts: %w", err)
		}
	}

	return cfg, nil
}
