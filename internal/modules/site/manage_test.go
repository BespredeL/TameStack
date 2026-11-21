package site

import (
	"strings"
	"testing"
)

func TestUpdateDocumentRootInConfig(t *testing.T) {
	nginxVhost := `server {
    listen 80;
    server_name myapp.example.com;
    root /var/www/myapp/old_root;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
}`

	updated, err := UpdateDocumentRootInConfig(nginxVhost, "/var/www/myapp/public", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(updated, "root /var/www/myapp/public;") {
		t.Errorf("expected new root, got:\n%s", updated)
	}

	apacheVhost := `<VirtualHost *:80>
    ServerName myapp.example.com
    DocumentRoot /var/www/myapp/old_root
    <Directory /var/www/myapp/old_root>
        AllowOverride All
    </Directory>
</VirtualHost>`

	updatedApache, err := UpdateDocumentRootInConfig(apacheVhost, "/var/www/myapp/public", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(updatedApache, "DocumentRoot /var/www/myapp/public") {
		t.Errorf("expected new DocumentRoot in Apache vhost, got:\n%s", updatedApache)
	}
}

func TestUpdateDocumentRootPrefersApacheWhenFlagged(t *testing.T) {
	// Apache configs should not be rewritten as nginx root just because a "root" word appears elsewhere.
	mixed := `<VirtualHost *:80>
    # root of the project lives below
    ServerName example.com
    DocumentRoot /var/www/old
    <Directory /var/www/old>
        AllowOverride All
    </Directory>
</VirtualHost>`
	updated, err := UpdateDocumentRootInConfig(mixed, "/var/www/new", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated, "DocumentRoot /var/www/new") {
		t.Fatalf("expected Apache DocumentRoot update, got:\n%s", updated)
	}
	if strings.Contains(updated, "root /var/www/new;") {
		t.Fatalf("should not inject nginx root into apache vhost:\n%s", updated)
	}
}
