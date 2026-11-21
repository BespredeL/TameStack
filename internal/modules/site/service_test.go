package site

import (
	"context"
	"encoding/json"
	"path/filepath"
	"tamestack/internal/testutil"
	"strings"
	"testing"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

func TestSiteTemplatesAndPlanning(t *testing.T) {
	// 1. Test Nginx template generation for Laravel.
	inLaravel := SiteInput{
		Domain:       "shop.example.com",
		Aliases:      []string{"www.shop.example.com"},
		DocumentRoot: "/var/www/shop.example.com",
		SiteType:     "laravel",
		PHPVersion:   "8.3",
	}

	cfg := RenderNginxConfig(inLaravel)
	if !strings.Contains(cfg, "root /var/www/shop.example.com/public;") {
		t.Fatalf("expected /public document root in Laravel config: %s", cfg)
	}
	if !strings.Contains(cfg, "php8.3-fpm.sock") {
		t.Fatalf("expected php8.3-fpm.sock in config: %s", cfg)
	}

	// 2. Test Reverse Proxy template.
	inProxy := SiteInput{
		Domain:       "api.example.com",
		SiteType:     "reverse_proxy",
		ProxyPassURL: "http://127.0.0.1:8080",
	}
	cfgProxy := RenderNginxConfig(inProxy)
	if !strings.Contains(cfgProxy, "proxy_pass http://127.0.0.1:8080;") {
		t.Fatalf("expected proxy_pass directive: %s", cfgProxy)
	}

	// 3. Test Site Planning with Mock DB.
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_sites.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	srv := &db.ServerRecord{
		ID:              "srv-site-test",
		Name:            "Web Node 1",
		Host:            "192.168.1.200",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "fake_secret",
	}
	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	saltPath := filepath.Join(tmpDir, "vault.salt")
	appCfg := &config.AppConfig{
		DataDir:        tmpDir,
		DatabasePath:   dbPath,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
		SaltPath:       saltPath,
	}
	vault, _ := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	pool := ssh.NewPool()
	engine := tasks.NewEngine(database, 2)
	service := NewService(appCfg, database, vault, pool, engine)

	plan, err := service.PlanSiteCreation(ctx, SiteInput{
		ServerID:   "srv-site-test",
		Domain:     "mycoolapp.test",
		SiteType:   "php",
		PHPVersion: "8.4",
	})
	if err != nil {
		t.Fatalf("failed to generate site plan: %v", err)
	}

	if plan.ConfigFile != "/etc/nginx/sites-available/mycoolapp.test.conf" {
		t.Fatalf("unexpected config file: %s", plan.ConfigFile)
	}

	// 4. Test Apache template generation and planning.
	inApache := SiteInput{
		ServerID:     "srv-site-test",
		Domain:       "apache.test",
		Aliases:      []string{"www.apache.test"},
		DocumentRoot: "/var/www/apache.test",
		SiteType:     "php",
		PHPVersion:   "8.2",
		WebServer:    "apache",
	}
	cfgApache := RenderApacheConfig(inApache)
	if !strings.Contains(cfgApache, "<VirtualHost *:80>") {
		t.Fatalf("expected VirtualHost in Apache config: %s", cfgApache)
	}
	if !strings.Contains(cfgApache, "ServerName apache.test") {
		t.Fatalf("expected ServerName in Apache config: %s", cfgApache)
	}
	if !strings.Contains(cfgApache, "proxy:unix:/run/php/php8.2-fpm.sock|fcgi://localhost") {
		t.Fatalf("expected php-fpm proxy in Apache config: %s", cfgApache)
	}

	apachePlan, err := service.PlanSiteCreation(ctx, inApache)
	if err != nil {
		t.Fatalf("failed to plan apache site: %v", err)
	}
	if !strings.Contains(apachePlan.ConfigFile, "apache2") {
		t.Fatalf("expected apache2 in config path: %s", apachePlan.ConfigFile)
	}
}

func TestDiscoverSitesJSONExtraction(t *testing.T) {
	// Simulate output with SSH MOTD noise before and after JSON.
	rawOutput := `
Welcome to Ubuntu 20.04.6 LTS (GNU/Linux 5.4.0-189-generic x86_64)
System information as of Sat Nov 14 09:00:00 UTC 2026
[
  {
    "domain": "my-home-site.ru",
    "aliases": ["www.my-home-site.ru"],
    "document_root": "/home/webuser/my-home-site.ru/public_html",
    "site_type": "wordpress",
    "php_version": "8.2",
    "web_server": "nginx+apache",
    "ssl_enabled": true,
    "status": "active"
  }
]
Last login: Sat Nov 14 08:50:00 2026 from 192.168.1.50
`
	outStr := strings.TrimSpace(rawOutput)
	jsonStart := strings.Index(outStr, "[")
	jsonEnd := strings.LastIndex(outStr, "]")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		t.Fatalf("failed to find JSON boundaries in output")
	}
	outStr = outStr[jsonStart : jsonEnd+1]

	var sites []DiscoveredSiteRaw
	if err := json.Unmarshal([]byte(outStr), &sites); err != nil {
		t.Fatalf("failed to parse extracted JSON: %v", err)
	}

	if len(sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(sites))
	}

	site := sites[0]
	if site.Domain != "my-home-site.ru" {
		t.Errorf("expected domain 'my-home-site.ru', got '%s'", site.Domain)
	}
	if site.DocumentRoot != "/home/webuser/my-home-site.ru/public_html" {
		t.Errorf("expected /home/ document root, got '%s'", site.DocumentRoot)
	}
	if site.WebServer != "nginx+apache" {
		t.Errorf("expected web_server 'nginx+apache', got '%s'", site.WebServer)
	}
	if !site.SSLEnabled {
		t.Errorf("expected SSLEnabled true")
	}
}

func TestSiteCreationPlanAndConflictDetection(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_conflict.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	_ = database.SaveServer(ctx, &db.ServerRecord{
		ID:       "srv-conflict",
		Name:     "Test Node",
		Host:     "10.0.0.1",
		Port:     22,
		Username: "root",
		AuthType: "password",
	})

	saltPath := filepath.Join(tmpDir, "vault.salt")
	appCfg := &config.AppConfig{
		DataDir:        tmpDir,
		DatabasePath:   dbPath,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
		SaltPath:       saltPath,
	}
	vault, _ := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	pool := ssh.NewPool()
	engine := tasks.NewEngine(database, 2)
	service := NewService(appCfg, database, vault, pool, engine)

	// 1. Plan for new domain.
	plan1, err := service.PlanSiteCreation(ctx, SiteInput{
		ServerID:         "srv-conflict",
		Domain:           "alpha.example.com",
		SiteType:         "php",
		PHPVersion:       "8.3",
		CreateDatabase:   true,
		DatabaseName:     "db_alpha",
		DatabaseUser:     "user_alpha",
		DatabasePassword: "StrongPassword123!",
	})
	if err != nil {
		t.Fatalf("failed to plan site: %v", err)
	}
	if plan1.ExistingSiteConflict {
		t.Errorf("expected no conflict for new site")
	}
	if plan1.OwnerUser != "www-data:www-data" {
		t.Errorf("expected OwnerUser to be www-data:www-data, got %s", plan1.OwnerUser)
	}
	if !plan1.DatabasePlanned || plan1.DatabaseName != "db_alpha" {
		t.Errorf("expected database plan to include db_alpha")
	}

	// 2. Save site in database.
	_ = database.SaveSite(ctx, &db.SiteRecord{
		ID:           "site-alpha",
		ServerID:     "srv-conflict",
		Domain:       "alpha.example.com",
		Aliases:      []string{"www.alpha.example.com"},
		DocumentRoot: "/var/www/alpha.example.com",
		SiteType:     "php",
		Status:       "active",
		Origin:       "managed",
	})

	// 3. Test Conflict Detection.
	planConflict, err := service.PlanSiteCreation(ctx, SiteInput{
		ServerID: "srv-conflict",
		Domain:   "alpha.example.com",
		SiteType: "php",
	})
	if err != nil {
		t.Fatalf("unexpected error on plan with conflict: %v", err)
	}
	if !planConflict.ExistingSiteConflict {
		t.Errorf("expected ExistingSiteConflict to be true for existing domain")
	}
	if !strings.Contains(planConflict.ConflictMessage, "alpha.example.com") {
		t.Errorf("expected ConflictMessage to mention domain: %s", planConflict.ConflictMessage)
	}

	// 4. Test Alias conflict.
	planAliasConflict, err := service.PlanSiteCreation(ctx, SiteInput{
		ServerID: "srv-conflict",
		Domain:   "www.alpha.example.com",
		SiteType: "php",
	})
	if err != nil {
		t.Fatalf("unexpected error on plan with alias conflict: %v", err)
	}
	if !planAliasConflict.ExistingSiteConflict {
		t.Errorf("expected ExistingSiteConflict to be true for existing alias")
	}

	// 5. Test CreateSite reject on conflict.
	_, errCreate := service.CreateSite(ctx, SiteInput{
		ServerID: "srv-conflict",
		Domain:   "alpha.example.com",
		SiteType: "php",
	})
	if errCreate == nil || !strings.Contains(errCreate.Error(), "conflict") {
		t.Errorf("expected CreateSite to reject duplicate domain with conflict error, got: %v", errCreate)
	}
}

func TestSiteDeletionPlanDiscoveredVsManaged(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_del.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	_ = database.SaveServer(ctx, &db.ServerRecord{
		ID:       "srv-del",
		Name:     "Test Node",
		Host:     "10.0.0.1",
		Port:     22,
		Username: "root",
		AuthType: "password",
	})

	_ = database.SaveSite(ctx, &db.SiteRecord{
		ID:           "site-managed",
		ServerID:     "srv-del",
		Domain:       "managed.example.com",
		DocumentRoot: "/var/www/managed.example.com",
		WebServer:    "nginx",
		Origin:       "managed",
	})

	_ = database.SaveSite(ctx, &db.SiteRecord{
		ID:           "site-discovered",
		ServerID:     "srv-del",
		Domain:       "foreign.example.com",
		DocumentRoot: "/var/www/foreign.example.com",
		WebServer:    "nginx",
		Origin:       "discovered",
	})

	saltPath := filepath.Join(tmpDir, "vault.salt")
	appCfg := &config.AppConfig{
		DataDir:        tmpDir,
		DatabasePath:   dbPath,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
		SaltPath:       saltPath,
	}
	vault, _ := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	pool := ssh.NewPool()
	engine := tasks.NewEngine(database, 2)
	service := NewService(appCfg, database, vault, pool, engine)

	// Test managed deletion plan.
	planManaged, err := service.PlanSiteDeletion(ctx, "site-managed", true)
	if err != nil {
		t.Fatalf("unexpected error for managed plan: %v", err)
	}
	if planManaged.IsDiscovered {
		t.Errorf("expected IsDiscovered to be false")
	}
	if planManaged.WarningMessage != "" {
		t.Errorf("expected no warning message for managed site, got: %s", planManaged.WarningMessage)
	}
	if !strings.Contains(planManaged.FilesWarning, "каталог сайта") {
		t.Errorf("expected regular files warning, got: %s", planManaged.FilesWarning)
	}

	// Test discovered deletion plan.
	planDiscovered, err := service.PlanSiteDeletion(ctx, "site-discovered", true)
	if err != nil {
		t.Fatalf("unexpected error for discovered plan: %v", err)
	}
	if !planDiscovered.IsDiscovered {
		t.Errorf("expected IsDiscovered to be true")
	}
	if !strings.Contains(planDiscovered.WarningMessage, "сторонней панелью") {
		t.Errorf("expected warning message to mention third-party origin: %s", planDiscovered.WarningMessage)
	}
	if !strings.Contains(planDiscovered.FilesWarning, "чужого сайта") {
		t.Errorf("expected files warning to emphasize foreign site directory: %s", planDiscovered.FilesWarning)
	}
}

func TestSitePlanCapabilityRestrictions(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_caps.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	_ = database.SaveServer(ctx, &db.ServerRecord{
		ID:       "srv-bionic",
		Name:     "Ubuntu 18.04 Node",
		Host:     "10.0.0.18",
		Port:     22,
		Username: "root",
		AuthType: "password",
	})

	// Save diagnostics with Ubuntu 18.04.
	_ = database.SaveDiagnostics(ctx, &db.DiagnosticsRecord{
		ServerID:             "srv-bionic",
		OSFamily:             "ubuntu",
		OSVersion:            "18.04",
		OSName:               "Ubuntu 18.04.6 LTS",
		InstalledWebServers:  []string{"nginx"},
		InstalledPHPVersions: []string{"7.2"},
	})

	saltPath := filepath.Join(tmpDir, "vault.salt")
	appCfg := &config.AppConfig{
		DataDir:        tmpDir,
		DatabasePath:   dbPath,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
		SaltPath:       saltPath,
	}
	vault, _ := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	pool := ssh.NewPool()
	engine := tasks.NewEngine(database, 2)
	service := NewService(appCfg, database, vault, pool, engine)

	// Attempt to plan PHP 8.3 on Ubuntu 18.04.
	planForbidden, err := service.PlanSiteCreation(ctx, SiteInput{
		ServerID:   "srv-bionic",
		Domain:     "forbidden.example.com",
		SiteType:   "php",
		PHPVersion: "8.3",
	})
	if err != nil {
		t.Fatalf("unexpected error during planning: %v", err)
	}

	// Must NOT include php8.3-fpm in packages to install.
	for _, pkg := range planForbidden.PackagesToInstall {
		if strings.Contains(pkg, "php8.3") {
			t.Errorf("expected php8.3 to NOT be in packages to install on Ubuntu 18.04, got: %v", planForbidden.PackagesToInstall)
		}
	}
	if !planForbidden.ExistingSiteConflict {
		t.Errorf("expected plan to be flagged with conflict/refusal for forbidden PHP version")
	}
	if !strings.Contains(planForbidden.ConflictMessage, "требует более нового релиза") {
		t.Errorf("expected ConflictMessage to mention refusal, got: %s", planForbidden.ConflictMessage)
	}
}
