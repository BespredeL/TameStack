package webserver

import (
	"testing"

	"tamestack/internal/osadapter"
)

func TestNormalizeWebServerType(t *testing.T) {
	cases := map[string]string{
		"nginx":   "nginx",
		"NGINX":   "nginx",
		"apache":  "apache2",
		"apache2": "apache2",
		"httpd":   "apache2",
	}
	for in, want := range cases {
		got, err := normalizeWebServerType(in)
		if err != nil || got != want {
			t.Errorf("normalizeWebServerType(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "caddy", "nginx; rm -rf /"} {
		if _, err := normalizeWebServerType(in); err == nil {
			t.Errorf("normalizeWebServerType(%q) expected error", in)
		}
	}
}

func TestInstallSpec(t *testing.T) {
	pkgs, svc := installSpec("nginx", osadapter.FamilyRhel)
	if len(pkgs) != 1 || pkgs[0] != "nginx" || svc != "nginx" {
		t.Errorf("nginx rhel spec = %v, %s", pkgs, svc)
	}
	pkgs, svc = installSpec("apache2", osadapter.FamilyDebian)
	if len(pkgs) != 1 || pkgs[0] != "apache2" || svc != "apache2" {
		t.Errorf("apache debian spec = %v, %s", pkgs, svc)
	}
	pkgs, svc = installSpec("apache2", osadapter.FamilyRhel)
	if len(pkgs) != 1 || pkgs[0] != "httpd" || svc != "httpd" {
		t.Errorf("apache rhel spec = %v, %s", pkgs, svc)
	}
}