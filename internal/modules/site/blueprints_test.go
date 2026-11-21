package site

import (
	"strings"
	"testing"
)

func TestGenerateSystemdService(t *testing.T) {
	env := map[string]string{
		"NODE_ENV": "production",
		"PORT":     "3000",
	}
	content := GenerateSystemdService("my-node-app", "/usr/bin/node server.js", "/var/www/my-node-app", "www-data", env)

	if !strings.Contains(content, "[Unit]") || !strings.Contains(content, "[Service]") {
		t.Fatalf("missing standard systemd sections")
	}
	if !strings.Contains(content, "ExecStart=/usr/bin/node server.js") {
		t.Errorf("expected ExecStart, got:\n%s", content)
	}
	if !strings.Contains(content, "WorkingDirectory=/var/www/my-node-app") {
		t.Errorf("expected WorkingDirectory, got:\n%s", content)
	}
	if !strings.Contains(content, "Environment=\"PORT=3000\"") {
		t.Errorf("expected Environment PORT, got:\n%s", content)
	}
	if !strings.Contains(content, "Restart=always") {
		t.Errorf("expected Restart=always")
	}
}

func TestGenerateReverseProxyVhost(t *testing.T) {
	vhost := GenerateReverseProxyVhost("api.myproject.com", 8080, true)

	if !strings.Contains(vhost, "server_name api.myproject.com;") {
		t.Errorf("expected server_name, got:\n%s", vhost)
	}
	if !strings.Contains(vhost, "proxy_pass http://127.0.0.1:8080;") {
		t.Errorf("expected proxy_pass to port 8080")
	}
	if !strings.Contains(vhost, "proxy_set_header Upgrade $http_upgrade;") {
		t.Errorf("expected websocket upgrade header")
	}
}
