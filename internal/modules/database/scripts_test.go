package database

import (
	"strings"
	"testing"
)

func TestRenderDetectMySQLClientScript(t *testing.T) {
	out, err := renderScript("detect_mysql_client.sh", struct{ Defaults string }{"'/root/.sh_defaults'"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "DEFAULTS='/root/.sh_defaults'") {
		t.Error("defaults placeholder not rendered")
	}
	if !strings.Contains(out, `printf '[client]\nuser=%s\npassword=%s\n' "$U" "$P" > "$f"`) {
		t.Error("printf verbs must stay literal shell format strings")
	}
	if strings.Contains(out, "{{") {
		t.Error("unrendered template action left in script")
	}
}

func TestRenderDetectMySQLServiceStatusScript(t *testing.T) {
	out, err := renderScript("detect_mysql_service_status.sh", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "for svc in mariadb mysql mysqld; do") || !strings.Contains(out, `echo "inactive"`) {
		t.Error("script body missing")
	}
}
