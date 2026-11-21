package database

import (
	"testing"

	"tamestack/internal/osadapter"
)

func TestNormalizeInstallVariant(t *testing.T) {
	cases := []struct {
		engine, variant, wantEngine, wantVariant string
	}{
		{"mysql", "", "mariadb", "mariadb"},
		{"mysql", "mariadb", "mariadb", "mariadb"},
		{"mysql", "mysql", "mysql", "mysql"},
		{"mariadb", "mysql", "mysql", "mysql"},
		{"postgres", "", "postgres", "postgres"},
		{"redis", "", "redis", "redis"},
	}
	for _, c := range cases {
		eng, v, err := normalizeInstallVariant(c.engine, c.variant)
		if err != nil || string(eng) != c.wantEngine || v != c.wantVariant {
			t.Errorf("normalizeInstallVariant(%q, %q) = %q, %q, %v; want %q, %q", c.engine, c.variant, eng, v, err, c.wantEngine, c.wantVariant)
		}
	}
	for _, c := range [][2]string{{"mysql", "percona"}, {"mongodb", ""}, {"", ""}} {
		if _, _, err := normalizeInstallVariant(c[0], c[1]); err == nil {
			t.Errorf("normalizeInstallVariant(%q, %q) expected error", c[0], c[1])
		}
	}
}

func TestDBInstallSpec(t *testing.T) {
	cases := []struct {
		variant, family, wantPkg, wantSvc string
	}{
		{"mariadb", osadapter.FamilyDebian, "mariadb-server", "mariadb"},
		{"mariadb", osadapter.FamilyRhel, "mariadb-server", "mariadb"},
		{"mysql", osadapter.FamilyDebian, "mysql-server", "mysql"},
		{"postgres", osadapter.FamilyDebian, "postgresql", "postgresql"},
		{"postgres", osadapter.FamilyRhel, "postgresql-server", "postgresql"},
		{"redis", osadapter.FamilyDebian, "redis-server", "redis-server"},
		{"redis", osadapter.FamilyRhel, "redis", "redis"},
	}
	for _, c := range cases {
		pkgs, svc := dbInstallSpec(c.variant, c.family)
		if len(pkgs) == 0 || pkgs[0] != c.wantPkg || svc != c.wantSvc {
			t.Errorf("dbInstallSpec(%q, %q) = %v, %s; want %s, %s", c.variant, c.family, pkgs, svc, c.wantPkg, c.wantSvc)
		}
	}
}

func TestServiceControlScriptRejectsBadInput(t *testing.T) {
	if _, err := serviceControlScript(EngineMySQL, "restart; rm -rf /"); err == nil {
		t.Error("expected error for invalid action")
	}
	if _, err := serviceControlScript(EngineType("mongodb"), "start"); err == nil {
		t.Error("expected error for unsupported engine")
	}
	if _, err := serviceControlScript(EngineRedis, "enable"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}