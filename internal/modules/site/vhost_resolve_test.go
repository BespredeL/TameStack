package site

import (
	"testing"

	"tamestack/internal/db"
)

func TestPrefersApacheAndHybrid(t *testing.T) {
	if !prefersApache("apache") {
		t.Fatal("apache should prefer apache")
	}
	if !prefersApache("Apache2") {
		t.Fatal("Apache2 should prefer apache")
	}
	if prefersApache("nginx") {
		t.Fatal("nginx should not prefer apache")
	}
	if prefersApache("nginx+apache") {
		t.Fatal("hybrid should prefer nginx front")
	}
	if prefersApache("") {
		t.Fatal("empty should default to nginx")
	}
	if !isHybridWebServer("nginx+apache") {
		t.Fatal("expected hybrid")
	}
}

func TestNginxEnabledAndApacheSiteName(t *testing.T) {
	if got := nginxEnabledPath("/etc/nginx/sites-available/app.example.com.conf", "app.example.com"); got != "/etc/nginx/sites-enabled/app.example.com.conf" {
		t.Fatalf("enabled path: %s", got)
	}
	if got := nginxEnabledPath("/etc/nginx/sites-available/app.example.com", "app.example.com"); got != "/etc/nginx/sites-enabled/app.example.com" {
		t.Fatalf("enabled path without conf: %s", got)
	}
	if got := apacheSiteName("/etc/apache2/sites-available/app.example.com.conf", "app.example.com"); got != "app.example.com.conf" {
		t.Fatalf("apache site name: %s", got)
	}
}

func TestEnginesForSite(t *testing.T) {
	reg := NewEngineRegistry()
	hybrid := reg.EnginesForSite("nginx+apache")
	if len(hybrid) != 2 {
		t.Fatalf("hybrid should yield nginx and apache, got %d", len(hybrid))
	}
	eng, err := reg.GetEngine("nginx+apache")
	if err != nil {
		t.Fatal(err)
	}
	if eng.Name() != "nginx" {
		t.Fatalf("hybrid create engine should be nginx, got %s", eng.Name())
	}
	eng, err = reg.GetEngine("apache")
	if err != nil || eng.Name() != "apache" {
		t.Fatalf("apache engine: %v %v", eng, err)
	}
}

func TestResolveHintsUseSiteRecord(t *testing.T) {
	site := &db.SiteRecord{Domain: "a.example.com", WebServer: "apache"}
	if !prefersApache(site.WebServer) {
		t.Fatal("site record apache must drive resolver preference")
	}
	site.WebServer = "nginx+apache"
	if prefersApache(site.WebServer) {
		t.Fatal("hybrid site must drive nginx preference")
	}
}
