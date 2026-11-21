package site

import (
	"strings"
	"testing"
)

func TestReplaceFastCGISocketInConfig(t *testing.T) {
	nginxVhost := `server {
    listen 80;
    server_name myapp.example.com;
    root /var/www/myapp/public;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php7.4-fpm.sock;
    }
}`

	updated, err := ReplaceFastCGISocketInConfig(nginxVhost, "8.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(updated, "fastcgi_pass unix:/run/php/php8.2-fpm.sock;") {
		t.Errorf("expected php8.2 socket, got:\n%s", updated)
	}
	if !strings.Contains(updated, "server_name myapp.example.com;") {
		t.Errorf("expected server_name to be preserved")
	}
}

func TestReplaceFastCGISocketInDefaultNginxConfig(t *testing.T) {
	defaultVhost := `server {
    listen 80 default_server;
    listen [::]:80 default_server;
    root /var/www/html;
    index index.html index.htm index.nginx-debian.html;
    server_name _;
    location / {
        try_files $uri $uri/ =404;
    }
}`

	updated, err := ReplaceFastCGISocketInConfig(defaultVhost, "7.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"location ~ \\.php$ {",
		"include fastcgi_params;",
		"fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;",
		"fastcgi_pass unix:/run/php/php7.4-fpm.sock;",
		"index index.php index.html index.htm index.nginx-debian.html;",
	} {
		if !strings.Contains(updated, want) {
			t.Errorf("expected %q in updated config, got:\n%s", want, updated)
		}
	}
	if strings.Count(updated, "location ~ \\.php$") != 1 {
		t.Errorf("expected exactly one php location, got:\n%s", updated)
	}
}

func TestReplaceFastCGISocketWithoutIndexDirective(t *testing.T) {
	vhost := `server {
    listen 80;
    root /var/www/html;
}`
	updated, err := ReplaceFastCGISocketInConfig(vhost, "8.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(updated, "index index.php index.html;") {
		t.Errorf("expected fallback index directive, got:\n%s", updated)
	}
	if !strings.Contains(updated, "fastcgi_pass unix:/run/php/php8.1-fpm.sock;") {
		t.Errorf("expected php8.1 socket, got:\n%s", updated)
	}
}

func TestInjectApacheFCGIHandlerIntoDefaultVhost(t *testing.T) {
	apacheDefault := `<VirtualHost *:80>
	ServerAdmin webmaster@localhost
	DocumentRoot /var/www/html
	ErrorLog ${APACHE_LOG_DIR}/error.log
	CustomLog ${APACHE_LOG_DIR}/access.log combined
</VirtualHost>`

	updated, err := InjectApacheFCGIHandler(apacheDefault, "7.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(updated, `SetHandler "proxy:unix:/run/php/php7.4-fpm.sock|fcgi://localhost"`) {
		t.Errorf("expected php7.4 handler injected, got:\n%s", updated)
	}
	if !strings.Contains(updated, "DocumentRoot /var/www/html") {
		t.Errorf("expected DocumentRoot preserved, got:\n%s", updated)
	}
}

func TestInjectApacheFCGIHandler(t *testing.T) {
	apacheVhost := `<VirtualHost *:80>
    ServerName legacy.example.com
    DocumentRoot /var/www/legacy
    <Directory /var/www/legacy>
        AllowOverride All
    </Directory>
</VirtualHost>`

	updated, err := InjectApacheFCGIHandler(apacheVhost, "8.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(updated, `SetHandler "proxy:unix:/run/php/php8.3-fpm.sock|fcgi://localhost"`) {
		t.Errorf("expected php8.3 handler injected, got:\n%s", updated)
	}
	if !strings.Contains(updated, "ServerName legacy.example.com") {
		t.Errorf("expected ServerName to be preserved")
	}
}
