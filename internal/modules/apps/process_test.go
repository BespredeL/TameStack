package apps

import (
	"strings"
	"testing"

	"tamestack/internal/db"
)

func TestValidateAppProcessConfig(t *testing.T) {
	valid := AppProcessConfig{
		Runtime:    "nodejs",
		WorkingDir: "/var/www/myapp",
		ExecStart:  "/usr/bin/node server.js",
		User:       "www-data",
		Port:       3000,
	}
	if err := ValidateAppProcessConfig(valid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	badRuntime := valid
	badRuntime.Runtime = "ruby"
	if err := ValidateAppProcessConfig(badRuntime); err == nil {
		t.Errorf("expected error for unsupported runtime")
	}

	badExec := valid
	badExec.ExecStart = "/usr/bin/node server.js; id"
	if err := ValidateAppProcessConfig(badExec); err == nil {
		t.Errorf("expected error for shell metacharacters in ExecStart")
	}

	site := &db.SiteRecord{ID: "12345678-abcd", Domain: "app.example.com"}
	unit := GenerateAppProcessUnit(site, valid, "/etc/tamestack/apps/tamestack-app-12345678.env")
	if !strings.Contains(unit, "ExecStart=/usr/bin/node server.js") || !strings.Contains(unit, "WorkingDirectory=/var/www/myapp") {
		t.Errorf("unexpected unit output:\n%s", unit)
	}
}
