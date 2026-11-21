package inventory

import (
	"testing"

	"tamestack/internal/db"
	"tamestack/internal/modules/php"
)

func TestParseUnits(t *testing.T) {
	sample := `nginx.service   loaded active running A high performance web server
php8.2-fpm.service loaded active running The PHP 8.2 FastCGI Process Manager
sshd.service    loaded active running OpenSSH Daemon
`
	units := ParseUnits(sample)
	if len(units) != 3 {
		t.Fatalf("expected 3 units, got %d", len(units))
	}
	if units[0].Unit != "nginx.service" || units[0].Active != "active" {
		t.Fatalf("unexpected unit: %+v", units[0])
	}
}

func TestParseSockets(t *testing.T) {
	sample := `Netid State  Recv-Q Send-Q Local Address:Port  Peer Address:PortProcess
tcp   LISTEN 0      511          0.0.0.0:80         0.0.0.0:*    users:(("nginx",pid=1234,fd=6))
tcp   LISTEN 0      128          0.0.0.0:22         0.0.0.0:*    users:(("sshd",pid=800,fd=3))
`
	sockets := ParseSockets(sample)
	if len(sockets) != 2 {
		t.Fatalf("expected 2 sockets, got %d", len(sockets))
	}
	if sockets[0].Port != 80 || sockets[0].Process != "nginx" || sockets[0].Pid != 1234 {
		t.Fatalf("unexpected socket 0: %+v", sockets[0])
	}
	if sockets[1].Port != 22 || sockets[1].Process != "sshd" {
		t.Fatalf("unexpected socket 1: %+v", sockets[1])
	}
}

func TestParsePasswd(t *testing.T) {
	sample := `root:x:0:0:root:/root:/bin/bash
www-data:x:33:33:www-data:/var/www:/usr/sbin/nologin
deploy:x:1001:1001::/home/deploy:/bin/bash
`
	users := ParsePasswd(sample)
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
	if !users[0].IsSystem || users[0].Username != "root" {
		t.Fatalf("expected root system user, got %+v", users[0])
	}
	if !users[1].IsSystem || users[1].Username != "www-data" {
		t.Fatalf("expected www-data system user, got %+v", users[1])
	}
	if users[2].IsSystem || users[2].Username != "deploy" {
		t.Fatalf("expected non-system user deploy, got %+v", users[2])
	}
}

func TestCorrelateLinks(t *testing.T) {
	sites := []*db.SiteRecord{
		{
			Domain:     "app.example.com",
			SiteType:   "php",
			PHPVersion: "8.2",
			SSLEnabled: true,
			VhostPath:  "/etc/nginx/sites-available/app.example.com.conf",
		},
		{
			Domain:     "orphan.example.com",
			SiteType:   "php",
			PHPVersion: "7.4",
			SSLEnabled: false,
		},
	}

	pools := []php.FpmPoolConfig{
		{
			PoolName: "www",
			Listen:   "unix:/run/php/php8.2-fpm.sock",
		},
	}

	certs := []*db.SSLCertificateRecord{
		{
			PrimaryDomain: "app.example.com",
			CertPath:      "/etc/letsencrypt/live/app.example.com/fullchain.pem",
		},
	}

	links := CorrelateLinks(sites, pools, certs)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}

	// First site: proven link to php8.2-fpm and cert.
	if links[0].Status != "proven" || links[0].PoolName != "www" || links[0].CertPath == "" {
		t.Fatalf("expected site 0 proven link, got %+v", links[0])
	}

	// Second site: missing php7.4 pool.
	if links[1].Status != "unproven" {
		t.Fatalf("expected site 1 unproven link, got %+v", links[1])
	}
}

func TestParseCronSystem(t *testing.T) {
	out := `
SHELL=/bin/sh
PATH=/usr/sbin:/usr/bin:/sbin:/bin
17 *	* * *	root	cd / && run-parts --report /etc/cron.hourly
*/5 * * * * www-data /usr/local/bin/sync.sh --fast
@daily deploy php /app/artisan schedule:run >> /dev/null 2>&1
broken line
`
	entries := ParseCronSystem(out, "/etc/cron.d")
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].User != "root" || entries[0].Schedule != "17 * * * *" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].User != "www-data" || entries[1].Command != "/usr/local/bin/sync.sh --fast" {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}
	if entries[2].Schedule != "@daily" || entries[2].User != "deploy" {
		t.Fatalf("unexpected shortcut entry: %+v", entries[2])
	}
}

func TestShortVersion(t *testing.T) {
	cases := map[string]string{
		"mysql  Ver 15.1 Distrib 10.6.22-MariaDB, for debian-linux-gnu": "v10.6.22",
		"mysql  Ver 8.0.43-0ubuntu0.22.04.1 for Linux":                  "v8.0.43",
		"psql (PostgreSQL) 14.10 (Ubuntu 14.10-0ubuntu0.22.04.1)":       "v14.10",
		"Redis server v=7.0.15 sha=00000000:0 malloc=jemalloc-5.2.1":    "v7.0.15",
		"":                 "",
		"weird-thing only": "weird-thing only",
	}
	for raw, want := range cases {
		if got := ShortVersion(raw); got != want {
			t.Errorf("ShortVersion(%q) = %q, want %q", raw, got, want)
		}
	}
}
