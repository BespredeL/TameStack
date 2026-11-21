package site

import (
	"strings"
	"testing"
)

func TestEngineRegistry(t *testing.T) {
	registry := NewEngineRegistry()

	nginx, err := registry.GetEngine("nginx")
	if err != nil || nginx.Name() != "nginx" {
		t.Fatalf("expected nginx engine, got: %v (err: %v)", nginx, err)
	}

	apache, err := registry.GetEngine("apache")
	if err != nil || apache.Name() != "apache" {
		t.Fatalf("expected apache engine, got: %v (err: %v)", apache, err)
	}

	apache2, err := registry.GetEngine("apache2")
	if err != nil || apache2.Name() != "apache" {
		t.Fatalf("expected apache engine for apache2, got: %v (err: %v)", apache2, err)
	}

	hybrid, err := registry.GetEngine("nginx+apache")
	if err != nil || hybrid.Name() != "nginx" {
		t.Fatalf("expected nginx front for hybrid, got: %v (err: %v)", hybrid, err)
	}
}

func TestNginxEngine_BuildVirtualHost(t *testing.T) {
	engine := &NginxEngine{}

	// 1. Static
	staticInput := SiteInput{
		Domain:       "test-static.com",
		DocumentRoot: "/var/www/test-static",
		SiteType:     "static",
	}
	cfgStatic, err := engine.BuildVirtualHost(staticInput)
	if err != nil {
		t.Fatalf("failed to build static vhost: %v", err)
	}
	if !strings.Contains(cfgStatic, "server_name test-static.com;") ||
		!strings.Contains(cfgStatic, "root /var/www/test-static;") ||
		!strings.Contains(cfgStatic, "try_files $uri $uri/ =404;") {
		t.Fatalf("unexpected static vhost:\n%s", cfgStatic)
	}

	// 2. Laravel
	laravelInput := SiteInput{
		Domain:       "test-laravel.com",
		DocumentRoot: "/var/www/test-laravel",
		SiteType:     "laravel",
		PHPVersion:   "8.2",
	}
	cfgLaravel, err := engine.BuildVirtualHost(laravelInput)
	if err != nil {
		t.Fatalf("failed to build laravel vhost: %v", err)
	}
	if !strings.Contains(cfgLaravel, "root /var/www/test-laravel/public;") ||
		!strings.Contains(cfgLaravel, "fastcgi_pass unix:/run/php/php8.2-fpm.sock;") ||
		!strings.Contains(cfgLaravel, "try_files $uri $uri/ /index.php?$query_string;") {
		t.Fatalf("unexpected laravel vhost:\n%s", cfgLaravel)
	}

	// 3. Reverse Proxy.
	proxyInput := SiteInput{
		Domain:       "test-proxy.com",
		SiteType:     "reverse_proxy",
		ProxyPassURL: "http://127.0.0.1:8080",
	}
	cfgProxy, err := engine.BuildVirtualHost(proxyInput)
	if err != nil {
		t.Fatalf("failed to build reverse proxy vhost: %v", err)
	}
	if !strings.Contains(cfgProxy, "proxy_pass http://127.0.0.1:8080;") ||
		!strings.Contains(cfgProxy, "proxy_set_header Upgrade $http_upgrade;") {
		t.Fatalf("unexpected reverse proxy vhost:\n%s", cfgProxy)
	}
}

func TestApacheEngine_BuildVirtualHost(t *testing.T) {
	engine := &ApacheEngine{}

	// 1. Static
	staticInput := SiteInput{
		Domain:       "test-apache.com",
		Aliases:      []string{"www.test-apache.com", "m.test-apache.com"},
		DocumentRoot: "/var/www/test-apache",
		SiteType:     "static",
	}
	cfgStatic, err := engine.BuildVirtualHost(staticInput)
	if err != nil {
		t.Fatalf("failed to build apache static vhost: %v", err)
	}
	if !strings.Contains(cfgStatic, "<VirtualHost *:80>") ||
		!strings.Contains(cfgStatic, "ServerName test-apache.com") ||
		!strings.Contains(cfgStatic, "ServerAlias www.test-apache.com m.test-apache.com") ||
		!strings.Contains(cfgStatic, "DocumentRoot /var/www/test-apache") {
		t.Fatalf("unexpected apache static vhost:\n%s", cfgStatic)
	}

	// 2. PHP Application.
	phpInput := SiteInput{
		Domain:       "test-php.com",
		DocumentRoot: "/var/www/test-php",
		SiteType:     "php",
		PHPVersion:   "8.3",
	}
	cfgPHP, err := engine.BuildVirtualHost(phpInput)
	if err != nil {
		t.Fatalf("failed to build apache php vhost: %v", err)
	}
	if !strings.Contains(cfgPHP, `SetHandler "proxy:unix:/run/php/php8.3-fpm.sock|fcgi://localhost"`) {
		t.Fatalf("unexpected apache php vhost:\n%s", cfgPHP)
	}

	// 3. Reverse Proxy.
	proxyInput := SiteInput{
		Domain:       "test-proxy.com",
		SiteType:     "reverse_proxy",
		ProxyPassURL: "http://127.0.0.1:5000",
	}
	cfgProxy, err := engine.BuildVirtualHost(proxyInput)
	if err != nil {
		t.Fatalf("failed to build apache proxy vhost: %v", err)
	}
	if !strings.Contains(cfgProxy, "ProxyPass / http://127.0.0.1:5000/") ||
		!strings.Contains(cfgProxy, "ProxyPassReverse / http://127.0.0.1:5000/") {
		t.Fatalf("unexpected apache proxy vhost:\n%s", cfgProxy)
	}
}

func TestVirtualHost_RedirectAndLogsFlags(t *testing.T) {
	nginxEngine := &NginxEngine{}
	apacheEngine := &ApacheEngine{}

	// Case A: Site without SSL -> MUST NOT contain redirect to https.
	noSSLInput := SiteInput{
		Domain:        "nossl.example.com",
		DocumentRoot:  "/var/www/nossl",
		SiteType:      "php",
		PHPVersion:    "8.2",
		SSLEnabled:    false,
		RedirectHTTPS: true, // Should be ignored because SSLEnabled is false
	}
	cfgNginx, err := nginxEngine.BuildVirtualHost(noSSLInput)
	if err != nil {
		t.Fatalf("failed building nginx vhost: %v", err)
	}
	if strings.Contains(cfgNginx, "return 301 https://") {
		t.Errorf("site without SSL received HTTPS redirect in Nginx config:\n%s", cfgNginx)
	}
	if !strings.Contains(cfgNginx, "access_log /var/log/nginx/nossl.example.com.access.log;") {
		t.Errorf("expected default access_log in config:\n%s", cfgNginx)
	}

	cfgApache, err := apacheEngine.BuildVirtualHost(noSSLInput)
	if err != nil {
		t.Fatalf("failed building apache vhost: %v", err)
	}
	if strings.Contains(cfgApache, "Redirect permanent / https://") {
		t.Errorf("site without SSL received HTTPS redirect in Apache config:\n%s", cfgApache)
	}

	// Case B: Site with SSL and RedirectHTTPS -> MUST contain 301 redirect.
	withSSLInput := SiteInput{
		Domain:        "ssl.example.com",
		DocumentRoot:  "/var/www/ssl",
		SiteType:      "php",
		PHPVersion:    "8.2",
		SSLEnabled:    true,
		RedirectHTTPS: true,
	}
	cfgNginxSSL, _ := nginxEngine.BuildVirtualHost(withSSLInput)
	if !strings.Contains(cfgNginxSSL, "return 301 https://$host$request_uri;") {
		t.Errorf("expected HTTPS redirect for SSL-enabled site in Nginx:\n%s", cfgNginxSSL)
	}
	if !strings.Contains(cfgNginxSSL, "listen 443 ssl;") {
		t.Errorf("expected listen 443 ssl without http2 by default in Nginx vhost:\n%s", cfgNginxSSL)
	}

	// Test HTTP2 flag.
	withHTTP2Input := withSSLInput
	withHTTP2Input.HTTP2Supported = true
	cfgNginxHTTP2, _ := nginxEngine.BuildVirtualHost(withHTTP2Input)
	if !strings.Contains(cfgNginxHTTP2, "listen 443 ssl http2;") {
		t.Errorf("expected listen 443 ssl http2 when HTTP2Supported is true:\n%s", cfgNginxHTTP2)
	}

	// Test custom pool socket in vhost.
	customPoolInput := SiteInput{
		Domain:       "poolsite.example.com",
		DocumentRoot: "/var/www/poolsite",
		SiteType:     "php",
		PHPVersion:   "8.3",
		FPMPool:      "site_alpha",
	}
	cfgNginxPool, _ := nginxEngine.BuildVirtualHost(customPoolInput)
	if !strings.Contains(cfgNginxPool, "fastcgi_pass unix:/run/php/php8.3-fpm-site_alpha.sock;") {
		t.Errorf("expected custom pool socket in Nginx vhost:\n%s", cfgNginxPool)
	}

	cfgApachePool, _ := apacheEngine.BuildVirtualHost(customPoolInput)
	if !strings.Contains(cfgApachePool, `SetHandler "proxy:unix:/run/php/php8.3-fpm-site_alpha.sock|fcgi://localhost"`) {
		t.Errorf("expected custom pool socket in Apache vhost:\n%s", cfgApachePool)
	}

	cfgApacheSSL, _ := apacheEngine.BuildVirtualHost(withSSLInput)
	if !strings.Contains(cfgApacheSSL, "Redirect permanent / https://ssl.example.com/") {
		t.Errorf("expected HTTPS redirect for SSL-enabled site in Apache:\n%s", cfgApacheSSL)
	}

	// Case C: DisableLogs -> MUST NOT contain log directives.
	noLogsInput := SiteInput{
		Domain:       "nologs.example.com",
		DocumentRoot: "/var/www/nologs",
		SiteType:     "static",
		DisableLogs:  true,
	}
	cfgNoLogs, _ := nginxEngine.BuildVirtualHost(noLogsInput)
	if strings.Contains(cfgNoLogs, "access_log") || strings.Contains(cfgNoLogs, "error_log") {
		t.Errorf("expected no logs when DisableLogs is true:\n%s", cfgNoLogs)
	}
}
