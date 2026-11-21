package ssh

import (
	"strings"
	"testing"
	"time"
)

func TestBuildBackupPath(t *testing.T) {
	fixedTime := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	path := "/etc/nginx/sites-available/example.com.conf"
	backup := BuildBackupPath(path, fixedTime)

	expected := "/etc/nginx/sites-available/example.com.conf.tamestack.bak_20261115_093000"
	if backup != expected {
		t.Fatalf("expected '%s', got '%s'", expected, backup)
	}
}

func TestGuardianResultFormatting(t *testing.T) {
	res := &GuardianResult{
		Success:      false,
		BackupPath:   "/tmp/test.bak",
		TestOutput:   "nginx: [emerg] unknown directive",
		ErrorMessage: "syntax test failed",
		RolledBack:   true,
	}

	if !res.RolledBack {
		t.Errorf("expected RolledBack to be true")
	}
	if !strings.Contains(res.TestOutput, "unknown directive") {
		t.Errorf("expected TestOutput to contain error details")
	}
}
