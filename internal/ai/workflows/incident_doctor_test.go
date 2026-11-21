package workflows

import (
	"strings"
	"testing"
)

func TestAnalyzeIncidentContext_502(t *testing.T) {
	incidentType := "502_bad_gateway"
	details := "upstream prematurely closed connection"
	logs := "2026/11/15 10:00:00 [error] 1234#1234: *1 connect() to unix:/run/php/php8.2-fpm.sock failed (2: No such file or directory) while connecting to upstream"

	diag := AnalyzeIncidentContext(incidentType, details, logs)
	if diag == nil {
		t.Fatalf("expected non-nil diagnosis")
	}

	if !strings.Contains(diag.RootCause, "php8.2-fpm") && !strings.Contains(diag.RootCause, "сокет") && !strings.Contains(diag.RootCause, "socket") {
		t.Errorf("expected root cause to mention php8.2-fpm socket, got: %s", diag.RootCause)
	}

	if !diag.CanAutoFix {
		t.Errorf("expected CanAutoFix to be true for php-fpm socket failure")
	}

	if !strings.Contains(diag.RemediationCmd, "php8.2-fpm") {
		t.Errorf("expected remediation cmd to restart php8.2-fpm, got: %s", diag.RemediationCmd)
	}
}

func TestAnalyzeIncidentContext_DiskFull(t *testing.T) {
	incidentType := "disk_full"
	details := "no space left on device"
	logs := "write error: No space left on device on /var/log/journal"

	diag := AnalyzeIncidentContext(incidentType, details, logs)
	if diag == nil {
		t.Fatalf("expected non-nil diagnosis")
	}

	if !strings.Contains(diag.RemediationCmd, "journalctl") && !strings.Contains(diag.RemediationCmd, "clean") {
		t.Errorf("expected clean up command for disk full, got: %s", diag.RemediationCmd)
	}
}
