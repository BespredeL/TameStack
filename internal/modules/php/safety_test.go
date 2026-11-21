package php

import (
	"strings"
	"testing"
)

func TestDetectDistroCompatibility(t *testing.T) {
	// Ubuntu 18.04 Bionic (EOL).
	osRelease18 := `
NAME="Ubuntu"
VERSION="18.04.6 LTS (Bionic Beaver)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 18.04.6 LTS"
VERSION_ID="18.04"
`
	compat18 := DetectDistroCompatibility(osRelease18)
	if compat18.DistroID != "ubuntu" || compat18.VersionID != "18.04" {
		t.Fatalf("expected ubuntu 18.04, got %s %s", compat18.DistroID, compat18.VersionID)
	}
	if !compat18.IsEOL {
		t.Errorf("expected Ubuntu 18.04 to be marked as IsEOL: true")
	}
	if compat18.ArchiveRepo != "" || compat18.PPAKeyringCmd != "" {
		t.Errorf("Ubuntu 18.04 must not get an untrusted PHP repository line")
	}

	// Ubuntu 20.04 Focal.
	osRelease20 := `
NAME="Ubuntu"
VERSION="20.04.6 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian
VERSION_ID="20.04"
`
	compat20 := DetectDistroCompatibility(osRelease20)
	if compat20.DistroID != "ubuntu" || compat20.VersionID != "20.04" {
		t.Fatalf("expected ubuntu 20.04, got %s %s", compat20.DistroID, compat20.VersionID)
	}
	if !compat20.IsEOL {
		t.Errorf("expected Ubuntu 20.04 to be marked as IsEOL (PPA dropped focal after May 2025)")
	}

	// Debian 11 Bullseye.
	osReleaseDebian := `
NAME="Debian GNU/Linux"
VERSION_ID="11"
VERSION="11 (bullseye)"
ID=debian
`
	compatDeb := DetectDistroCompatibility(osReleaseDebian)
	if compatDeb.DistroID != "debian" || compatDeb.VersionID != "11" {
		t.Fatalf("expected debian 11, got %s %s", compatDeb.DistroID, compatDeb.VersionID)
	}
}

func TestGenerateApacheProxyFCGIBlock(t *testing.T) {
	block := GenerateApacheProxyFCGIBlock("test.local", "8.2")
	if !strings.Contains(block, "proxy:unix:/run/php/php8.2-fpm.sock|fcgi://localhost") {
		t.Errorf("expected socket path for php8.2 in apache block, got:\n%s", block)
	}
	if !strings.Contains(block, "SetHandler") {
		t.Errorf("expected SetHandler directive in apache block, got:\n%s", block)
	}
}

func TestValidateAndRenderPoolConfig(t *testing.T) {
	// 1. Validation tests.
	if err := ValidatePoolName(""); err == nil {
		t.Errorf("expected error for empty pool name")
	}
	if err := ValidatePoolName("www"); err == nil {
		t.Errorf("expected error for default www pool name")
	}
	if err := ValidatePoolName("invalid.dot"); err == nil {
		t.Errorf("expected error for pool name with dot")
	}
	if err := ValidatePoolName("pool_valid-1"); err != nil {
		t.Errorf("expected pool_valid-1 to be valid: %v", err)
	}

	// 2. Rendering tests.
	in := CreatePoolInput{
		PoolName:           "site_alpha",
		Version:            "8.3",
		User:               "user_alpha",
		Group:              "user_alpha",
		ListenSocket:       "/run/php/php8.3-fpm-site_alpha.sock",
		PmMode:             "ondemand",
		MaxChildren:        15,
		ProcessIdleTimeout: "20s",
		MemoryLimit:        "512M",
		MaxExecutionTime:   120,
		UploadMaxFilesize:  "128M",
		PostMaxSize:        "128M",
		Slowlog:            true,
		SlowlogTimeout:     "3s",
	}

	rendered, err := RenderPoolConfig(in)
	if err != nil {
		t.Fatalf("failed to render pool config: %v", err)
	}

	expectedSnippets := []string{
		"[site_alpha]",
		"user = user_alpha",
		"group = user_alpha",
		"listen = /run/php/php8.3-fpm-site_alpha.sock",
		"pm = ondemand",
		"pm.max_children = 15",
		"pm.process_idle_timeout = 20s",
		"slowlog = /var/log/php8.3-fpm-site_alpha.slow.log",
		"request_slowlog_timeout = 3s",
		"php_admin_value[memory_limit] = 512M",
		"php_admin_value[max_execution_time] = 120",
		"php_admin_value[upload_max_filesize] = 128M",
		"php_admin_value[post_max_size] = 128M",
	}

	for _, s := range expectedSnippets {
		if !strings.Contains(rendered, s) {
			t.Errorf("expected rendered pool to contain %q, but got:\n%s", s, rendered)
		}
	}
}
