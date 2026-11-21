package apps

import (
	"strings"
	"testing"
)

func TestResolveProjectRoot(t *testing.T) {
	tests := []struct {
		docRoot  string
		expected string
	}{
		{"/var/www/example.com/public", "/var/www/example.com"},
		{"/var/www/example.com/public_html", "/var/www/example.com"},
		{"/var/www/example.com/web", "/var/www/example.com"},
		{"/var/www/example.com", "/var/www/example.com"},
		{"/var/www/example.com/public/", "/var/www/example.com"},
	}

	for _, tt := range tests {
		got := ResolveProjectRoot(tt.docRoot)
		if got != tt.expected {
			t.Errorf("ResolveProjectRoot(%q) = %q, want %q", tt.docRoot, got, tt.expected)
		}
	}
}

func TestParseComposerJSON(t *testing.T) {
	laravelComposer := `{
		"name": "laravel/laravel",
		"require": {
			"php": "^8.2",
			"laravel/framework": "^11.0"
		}
	}`

	version, reqPHP, isLaravel, isSymfony := ParseComposerJSON([]byte(laravelComposer))
	if !isLaravel {
		t.Errorf("expected isLaravel to be true")
	}
	if isSymfony {
		t.Errorf("expected isSymfony to be false")
	}
	if version != "^11.0" {
		t.Errorf("expected ^11.0, got %s", version)
	}
	if reqPHP != "^8.2" {
		t.Errorf("expected ^8.2, got %s", reqPHP)
	}

	symfonyComposer := `{
		"name": "symfony/skeleton",
		"require": {
			"php": ">=8.1",
			"symfony/framework-bundle": "6.4.*"
		}
	}`

	sVersion, sReqPHP, sIsLaravel, sIsSymfony := ParseComposerJSON([]byte(symfonyComposer))
	if !sIsSymfony {
		t.Errorf("expected isSymfony to be true")
	}
	if sIsLaravel {
		t.Errorf("expected isLaravel to be false")
	}
	if sVersion != "6.4.*" {
		t.Errorf("expected 6.4.*, got %s", sVersion)
	}
	if sReqPHP != ">=8.1" {
		t.Errorf("expected >=8.1, got %s", sReqPHP)
	}
}

func TestAllowedArtisanCommands(t *testing.T) {
	allowed, dangerous := IsAllowedArtisanCommand("cache:clear")
	if !allowed || dangerous {
		t.Errorf("expected cache:clear to be allowed and non-dangerous")
	}

	allowed, dangerous = IsAllowedArtisanCommand("migrate")
	if !allowed || !dangerous {
		t.Errorf("expected migrate to be allowed and dangerous")
	}

	allowed, dangerous = IsAllowedArtisanCommand("migrate:fresh")
	if !allowed || !dangerous {
		t.Errorf("expected migrate:fresh to be allowed and dangerous")
	}

	allowed, _ = IsAllowedArtisanCommand("tinker")
	if allowed {
		t.Errorf("tinker must NOT be allowed (interactive shell)")
	}

	allowed, _ = IsAllowedArtisanCommand("custom:malicious")
	if allowed {
		t.Errorf("arbitrary command must NOT be allowed")
	}
}

func TestGenerateQueueWorkerUnit(t *testing.T) {
	cfg := QueueWorkerConfig{
		Queue:   "default",
		Tries:   3,
		Timeout: 90,
	}

	unit := GenerateQueueWorkerUnit("site-12345678", "example.com", "/var/www/example.com", "www-data", cfg)
	unitName := QueueUnitName("site-12345678")
	if unitName != "sh-queue-site-123.service" {
		t.Errorf("expected sh-queue-site-123.service, got %s", unitName)
	}

	if !strings.Contains(unit, "WorkingDirectory=/var/www/example.com") {
		t.Errorf("missing WorkingDirectory")
	}
	if !strings.Contains(unit, "--queue=default") {
		t.Errorf("missing --queue=default")
	}

	// Verify absence of systemd 255-only features (for Ubuntu 18.04 LTS compatibility).
	disallowedDirectives := []string{
		"ProtectSystem=strict",
		"MemoryDenyWriteExecute",
		"RestrictAddressFamilies",
		"SystemCallFilter",
	}
	for _, dir := range disallowedDirectives {
		if strings.Contains(unit, dir) {
			t.Errorf("unit contains systemd 255-only directive: %s", dir)
		}
	}
}

func TestScrubEnvForLogging(t *testing.T) {
	rawEnv := `APP_NAME=Laravel
APP_ENV=local
APP_KEY=base64:AbCdEf1234567890
DB_PASSWORD=secretPassword123!
REDIS_PASSWORD=anotherSecret
MAIL_USERNAME=user@example.com
`
	scrubbed := ScrubEnvForLogging(rawEnv)

	if strings.Contains(scrubbed, "secretPassword123!") {
		t.Errorf("DB_PASSWORD was not scrubbed")
	}
	if strings.Contains(scrubbed, "anotherSecret") {
		t.Errorf("REDIS_PASSWORD was not scrubbed")
	}
	if strings.Contains(scrubbed, "base64:AbCdEf1234567890") {
		t.Errorf("APP_KEY was not scrubbed")
	}
	if !strings.Contains(scrubbed, "APP_NAME=Laravel") {
		t.Errorf("APP_NAME should remain visible")
	}
}
