package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigInitializesPathsAndSalt(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AppData", base)
	t.Setenv("XDG_CONFIG_HOME", base)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != filepath.Join(base, "TameStack") {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, filepath.Join(base, "TameStack"))
	}
	for _, p := range []string{
		cfg.DatabasePath, cfg.DatabaseContainerPath, cfg.WorkingDatabasePath,
		cfg.KnownHostsPath, cfg.SaltPath, cfg.VaultKeyPath, cfg.LogPath,
	} {
		if !strings.HasPrefix(p, cfg.DataDir+string(filepath.Separator)) {
			t.Errorf("path %q escapes DataDir", p)
		}
	}
	if cfg.RunDir != filepath.Join(cfg.DataDir, "run") {
		t.Errorf("RunDir = %q", cfg.RunDir)
	}

	salt, err := os.ReadFile(cfg.SaltPath)
	if err != nil {
		t.Fatalf("salt not created: %v", err)
	}
	if len(salt) != 32 {
		t.Errorf("salt length = %d, want 32", len(salt))
	}
	if _, err := os.Stat(cfg.KnownHostsPath); err != nil {
		t.Errorf("known_hosts not initialized: %v", err)
	}

	cfg2, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	salt2, err := os.ReadFile(cfg2.SaltPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(salt, salt2) {
		t.Error("salt must be reused across loads, not regenerated")
	}
}

func TestLoadConfigPortableMode(t *testing.T) {
	ForcePortable = true
	defer func() { ForcePortable = false }()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed in portable mode: %v", err)
	}

	if !cfg.IsPortable {
		t.Fatal("expected IsPortable = true")
	}

	exePath, err := os.Executable()
	var wantDir string
	if err == nil {
		wantDir = filepath.Join(filepath.Dir(exePath), "data")
	} else {
		wantDir = filepath.Join(".", "data")
	}

	if cfg.DataDir != wantDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, wantDir)
	}

	// Clean up created test data directory
	defer os.RemoveAll(wantDir)
}
