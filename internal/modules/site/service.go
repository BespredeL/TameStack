package site

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/modules/php"
	"tamestack/internal/modules/webserver"
	"tamestack/internal/osadapter"
	"tamestack/internal/platform/capabilities"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"

	"github.com/google/uuid"
)

// RFC 1123 domain name validation pattern (also prevents any shell injection characters).
var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// Service manages web sites and virtual host configurations.
type Service struct {
	module.Base
	engine  *tasks.Engine
	engines *EngineRegistry
}

// NewService instantiates Site Service and registers task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:    module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine:  engine,
		engines: NewEngineRegistry(),
	}

	engine.RegisterHandler("site_create", s.handleCreateSiteTask)
	engine.RegisterHandler("site_blueprint", s.handleBlueprintTask)
	engine.RegisterHandler("site_delete", s.handleDeleteSiteTask)
	engine.RegisterHandler("site_config_save", s.handleSaveConfigTask)
	engine.RegisterHandler("site_php_switch", s.handlePHPSwitchTask)
	engine.RegisterHandler("site_settings_update", s.handleSettingsUpdateTask)
	engine.RegisterHandler("site_status_toggle", s.handleStatusToggleTask)

	return s
}

// ListSites returns all registered websites, optionally filtered by server ID.
func (s *Service) ListSites(ctx context.Context, serverID string) ([]*db.SiteRecord, error) {
	return s.Database.ListSites(ctx, serverID)
}

// GetSite returns a specific website by ID.
func (s *Service) GetSite(ctx context.Context, id string) (*db.SiteRecord, error) {
	return s.Database.GetSite(ctx, id)
}

// CheckDNS resolves the domain A records and compares with target server IP.
func (s *Service) CheckDNS(ctx context.Context, domain, serverID string) (*SiteDNSCheckResult, error) {
	server, err := s.Database.GetServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	domain = strings.TrimSpace(strings.ToLower(domain))
	res := &SiteDNSCheckResult{
		Domain:      domain,
		ServerIP:    server.Host,
		ResolvedIPs: []string{},
	}

	ips, err := net.LookupIP(domain)
	if err != nil {
		res.Match = false
		res.Message = fmt.Sprintf("DNS lookup failed: %v. Please ensure DNS A record is pointed to %s.", err, server.Host)
		return res, nil
	}

	matched := false
	for _, ip := range ips {
		ipStr := ip.String()
		res.ResolvedIPs = append(res.ResolvedIPs, ipStr)
		if ipStr == server.Host {
			matched = true
		}
	}

	res.Match = matched
	if matched {
		res.Message = fmt.Sprintf("DNS A record correctly points to server IP (%s).", server.Host)
	} else {
		res.Message = fmt.Sprintf("DNS points to %s, but server IP is %s. DNS changes may take time to propagate.", strings.Join(res.ResolvedIPs, ", "), server.Host)
	}

	return res, nil
}

// PlanSiteCreation checks inputs, DNS, directories, and returns the complete blueprint.
func (s *Service) PlanSiteCreation(ctx context.Context, in SiteInput) (*CreateSitePlan, error) {
	in.Domain = strings.TrimSpace(strings.ToLower(in.Domain))
	if !domainRegex.MatchString(in.Domain) {
		return nil, fmt.Errorf("invalid domain name '%s'. Must be a valid FQDN (e.g. example.com)", in.Domain)
	}

	server, err := s.Database.GetServer(ctx, in.ServerID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// Check server_name conflict in local database.
	var existingConflict bool
	var conflictMsg string
	existingSites, errSites := s.Database.ListSites(ctx, in.ServerID)
	if errSites == nil {
		for _, ex := range existingSites {
			if strings.EqualFold(ex.Domain, in.Domain) {
				existingConflict = true
				conflictMsg = fmt.Sprintf("Домен «%s» уже настроен на этом сервере (ID: %s, origin: %s). Откройте существующий сайт или укажите другой домен.", in.Domain, ex.ID, ex.Origin)
				break
			}
			for _, alias := range ex.Aliases {
				if strings.EqualFold(alias, in.Domain) {
					existingConflict = true
					conflictMsg = fmt.Sprintf("Домен «%s» уже используется как алиас для сайта «%s» (ID: %s).", in.Domain, ex.Domain, ex.ID)
					break
				}
			}
			if existingConflict {
				break
			}
		}
	}

	docRoot := in.DocumentRoot
	if docRoot == "" {
		docRoot = fmt.Sprintf("/var/www/%s", in.Domain)
	}

	isApache := prefersApache(in.WebServer)
	family := s.serverOSFamily(ctx, in.ServerID)

	configFile := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", in.Domain)
	accessLogFile := fmt.Sprintf("/var/log/nginx/%s.access.log", in.Domain)
	errorLogFile := fmt.Sprintf("/var/log/nginx/%s.error.log", in.Domain)
	reloadService := "nginx"

	if isApache {
		if family == "rhel" {
			configFile = fmt.Sprintf("/etc/httpd/conf.d/%s.conf", in.Domain)
			accessLogFile = fmt.Sprintf("/var/log/httpd/%s.access.log", in.Domain)
			errorLogFile = fmt.Sprintf("/var/log/httpd/%s.error.log", in.Domain)
			reloadService = "httpd"
		} else {
			configFile = fmt.Sprintf("/etc/apache2/sites-available/%s.conf", in.Domain)
			accessLogFile = fmt.Sprintf("/var/log/apache2/%s.access.log", in.Domain)
			errorLogFile = fmt.Sprintf("/var/log/apache2/%s.error.log", in.Domain)
			reloadService = "apache2"
		}
	}

	// Inspect required packages from diagnostics.
	var packagesToInstall []string
	diag, _ := s.Database.GetDiagnostics(ctx, in.ServerID)
	if diag != nil {
		if isApache {
			hasApache := false
			for _, ws := range diag.InstalledWebServers {
				if strings.Contains(strings.ToLower(ws), "apache") || strings.Contains(strings.ToLower(ws), "httpd") {
					hasApache = true
					break
				}
			}
			if !hasApache {
				pkg := "apache2"
				if family == "rhel" {
					pkg = "httpd"
				}
				packagesToInstall = append(packagesToInstall, pkg)
			}
		} else {
			hasNginx := false
			for _, ws := range diag.InstalledWebServers {
				if strings.Contains(strings.ToLower(ws), "nginx") {
					hasNginx = true
					break
				}
			}
			if !hasNginx {
				packagesToInstall = append(packagesToInstall, "nginx")
			}
		}

		if in.PHPVersion != "" && in.SiteType != "static" && in.SiteType != "reverse_proxy" {
			hasPHP := false
			for _, v := range diag.InstalledPHPVersions {
				if v == in.PHPVersion {
					hasPHP = true
					break
				}
			}
			if !hasPHP {
				// Check capability profile before planning package installation.
				if allowed, reason := capabilities.AllowsPHPInstallByVersion(diagDistroID(diag), diag.OSVersion, in.PHPVersion); !allowed {
					// Package not in capability does NOT go into packagesToInstall without separate refusal line.
					if conflictMsg == "" {
						conflictMsg = reason
					}
					existingConflict = true
				} else {
					packagesToInstall = append(packagesToInstall, fmt.Sprintf("php%s-fpm", in.PHPVersion))
				}
			}
		}

		// Non-LTS releases block package installation.
		if allowed, reason := capabilities.AllowsPackageInstallByVersion(diagDistroID(diag), diag.OSVersion); !allowed && len(packagesToInstall) > 0 {
			conflictMsg = reason
			existingConflict = true
			packagesToInstall = nil
		}
	}

	fpmSocket := fmt.Sprintf("/run/php/php%s-fpm.sock", in.PHPVersion)
	if in.FPMPool != "" {
		if strings.HasPrefix(in.FPMPool, "/") {
			fpmSocket = in.FPMPool
		} else if strings.HasPrefix(in.FPMPool, "unix:") {
			fpmSocket = strings.TrimPrefix(in.FPMPool, "unix:")
		} else {
			fpmSocket = fmt.Sprintf("/run/php/php%s-fpm-%s.sock", in.PHPVersion, in.FPMPool)
		}
	}

	plan := &CreateSitePlan{
		Domain:               in.Domain,
		ServerHost:           server.Host,
		DocumentRoot:         docRoot,
		ConfigFile:           configFile,
		AccessLogFile:        accessLogFile,
		ErrorLogFile:         errorLogFile,
		PHPVersion:           in.PHPVersion,
		FPMSocket:            fpmSocket,
		WebserverToReload:    reloadService,
		OwnerUser:            "www-data:www-data",
		PackagesToInstall:    packagesToInstall,
		ExistingSiteConflict: existingConflict,
		ConflictMessage:      conflictMsg,
		DatabasePlanned:      in.CreateDatabase,
		DatabaseName:         in.DatabaseName,
		DatabaseUser:         in.DatabaseUser,
		PreflightWarnings:    []string{},
	}
	if existingConflict {
		plan.PreflightWarnings = append(plan.PreflightWarnings, conflictMsg)
	}

	// DNS check
	dnsRes, _ := s.CheckDNS(ctx, in.Domain, in.ServerID)
	if dnsRes != nil {
		plan.DNSMatches = dnsRes.Match
		if !dnsRes.Match {
			plan.PreflightWarnings = append(plan.PreflightWarnings, dnsRes.Message)
		}
	}

	// Web server port 80 conflict check.
	if client, _, err := s.GetSSHClient(ctx, in.ServerID); err == nil && client != nil {
		outSS, _, _, _ := client.NewCommand("ss -tulpn 2>/dev/null || netstat -tulpn 2>/dev/null").Exec(ctx)
		if len(outSS) > 0 {
			portsMap := webserver.ParseListeningSockets(string(outSS))
			if hasConf, warn := webserver.DetectPort80Conflict(portsMap, in.WebServer); hasConf {
				plan.PreflightWarnings = append(plan.PreflightWarnings, warn)
			}
		}
	}

	return plan, nil
}

// CreateSite queues a background site creation job in TaskEngine.
func (s *Service) CreateSite(ctx context.Context, in SiteInput) (*db.TaskRecord, error) {
	in.Domain = strings.TrimSpace(strings.ToLower(in.Domain))
	if !domainRegex.MatchString(in.Domain) {
		return nil, fmt.Errorf("invalid domain name: %s", in.Domain)
	}

	// Reject creation if domain already exists on this server.
	existingSites, errSites := s.Database.ListSites(ctx, in.ServerID)
	if errSites == nil {
		for _, ex := range existingSites {
			if strings.EqualFold(ex.Domain, in.Domain) {
				return nil, fmt.Errorf("conflict: site with domain '%s' already exists on this server (ID: %s)", in.Domain, ex.ID)
			}
		}
	}

	if in.DocumentRoot == "" {
		in.DocumentRoot = fmt.Sprintf("/var/www/%s", in.Domain)
	}
	if in.WebServer == "" {
		in.WebServer = "nginx"
	}
	if in.SiteType == "" {
		in.SiteType = "php"
	}

	payloadBytes, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("Provision website %s (%s, PHP %s)", in.Domain, in.SiteType, in.PHPVersion)
	return s.engine.Submit(in.ServerID, "site_create", title, string(payloadBytes))
}

// PlanSiteDeletion generates a deletion plan with explicit warnings for discovered sites.
func (s *Service) PlanSiteDeletion(ctx context.Context, siteID string, deleteFiles bool) (*SiteDeletePlan, error) {
	site, err := s.Database.GetSite(ctx, siteID)
	if err != nil || site == nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	plan := &SiteDeletePlan{
		SiteID:       site.ID,
		Domain:       site.Domain,
		Origin:       site.Origin,
		WebServer:    site.WebServer,
		DocumentRoot: site.DocumentRoot,
		DeleteFiles:  deleteFiles,
		IsDiscovered: site.Origin == "discovered",
	}

	if plan.IsDiscovered {
		plan.WarningMessage = "Внимание: этот сайт был обнаружен на сервере вне TameStack (создан сторонней панелью или вручную). Удаление конфигурации или файлов может нарушить работу стороннего приложения."
		plan.Description = fmt.Sprintf("Будет удалён виртуальный хост стороннего сайта %s (%s) и перезагружен веб-сервер.", site.Domain, site.WebServer)
		if deleteFiles {
			plan.FilesWarning = fmt.Sprintf("⚠️ Внимание: безвозвратно стереть все файлы каталога чужого сайта (%s).", site.DocumentRoot)
		}
	} else {
		plan.Description = fmt.Sprintf("Будет удалён виртуальный хост %s (%s) и перезагружен веб-сервер.", site.Domain, site.WebServer)
		if deleteFiles {
			plan.FilesWarning = fmt.Sprintf("Также безвозвратно удалить каталог сайта (%s).", site.DocumentRoot)
		}
	}

	return plan, nil
}

// DeleteSite unlinks virtual host, reloads webserver, and optionally purges files.
func (s *Service) DeleteSite(ctx context.Context, siteID string, deleteFiles bool) error {
	site, err := s.Database.GetSite(ctx, siteID)
	if err != nil {
		return fmt.Errorf("site not found: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil || client == nil {
		return fmt.Errorf("SSH connection failed, site record kept: %w", err)
	}
	for _, engine := range s.engines.EnginesForSite(site.WebServer) {
		if rmErr := engine.RemoveVirtualHost(ctx, client, site.Domain); rmErr != nil {
			return fmt.Errorf("virtual host was not removed, site record kept: %w", rmErr)
		}
	}
	if deleteFiles && site.DocumentRoot != "" && site.DocumentRoot != "/" && site.DocumentRoot != "/var/www" && site.DocumentRoot != "/home" {
		if rmErr := client.RemovePath(ctx, site.DocumentRoot, true); rmErr != nil {
			return fmt.Errorf("site files were not removed, site record kept: %w", rmErr)
		}
	}

	return s.Database.DeleteSite(ctx, siteID)
}

// AcceptDiscoveredSite transitions a site from "discovered" to "managed" origin.
func (s *Service) AcceptDiscoveredSite(ctx context.Context, siteID string) error {
	site, err := s.Database.GetSite(ctx, siteID)
	if err != nil || site == nil {
		return fmt.Errorf("site not found: %w", err)
	}
	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	vhostPath, _, err := s.resolveVhost(ctx, client, site)
	if err != nil {
		return fmt.Errorf("у сайта '%s' нет vhost-конфигурации на сервере (находка только по файловой системе). Создайте полноценный сайт через «Создать сайт»", site.Domain)
	}
	if site.VhostPath == "" {
		site.VhostPath = vhostPath
	}
	site.Origin = "managed"
	site.UpdatedAt = time.Now()
	return s.Database.SaveSite(ctx, site)
}

// GetSiteLogs streams recent access or error logs for Nginx, Apache, or hybrid sites.
func (s *Service) GetSiteLogs(ctx context.Context, siteID, logType string, lines int, preferredServer string) (string, error) {
	site, err := s.Database.GetSite(ctx, siteID)
	if err != nil {
		return "", fmt.Errorf("site not found: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, site.ServerID)
	if err != nil {
		return "", err
	}

	if lines <= 0 || lines > 500 {
		lines = 100
	}

	ws := strings.ToLower(site.WebServer)
	isApache := false
	isNginx := false

	switch strings.ToLower(preferredServer) {
	case "apache":
		isApache = true
	case "nginx":
		isNginx = true
	default:
		if strings.Contains(ws, "apache") && strings.Contains(ws, "nginx") {
			isApache = true
			isNginx = true
		} else if strings.Contains(ws, "apache") {
			isApache = true
		} else {
			isNginx = true
		}
	}

	scriptBytes, err := siteFS.ReadFile("scripts/discover_logs.sh")
	if err != nil {
		return "", fmt.Errorf("failed to read log discovery script: %w", err)
	}

	cmd := client.NewCommand(string(scriptBytes)).
		WithSudo().
		WithEnv("DOMAIN", site.Domain).
		WithEnv("LOGTYPE", logType).
		WithEnv("LINES", fmt.Sprintf("%d", lines)).
		WithEnv("DOCROOT", site.DocumentRoot).
		WithEnv("IS_APACHE", fmt.Sprintf("%t", isApache)).
		WithEnv("IS_NGINX", fmt.Sprintf("%t", isNginx))

	out, _, _, _ := cmd.Exec(ctx)
	return strings.TrimSpace(string(out)), nil
}

func (s *Service) serverOSFamily(ctx context.Context, serverID string) string {
	diag, err := s.Database.GetDiagnostics(ctx, serverID)
	if err == nil && diag != nil && diag.OSFamily != "" {
		return diag.OSFamily
	}
	return "debian"
}

// diagDistroID derives the os-release ID from diagnostics. OSFamily only holds
// the package family ("debian"/"rhel"), which would mask Ubuntu-specific rules.
func diagDistroID(diag *db.DiagnosticsRecord) string {
	if strings.Contains(strings.ToLower(diag.OSName), "ubuntu") {
		return "ubuntu"
	}
	return diag.OSFamily
}

func installPackage(ctx context.Context, client *ssh.SSHClient, family, pkg, unit string) error {
	adapter := osadapter.NewAdapterForFamily(family)
	if family != osadapter.FamilyRhel {
		if _, err := adapter.UpdatePackageIndex(ctx, client); err != nil {
			return err
		}
	}
	if _, err := adapter.InstallPackages(ctx, client, []string{pkg}); err != nil {
		return err
	}
	return module.RunPrivileged(ctx, client, "systemctl enable --now "+unit)
}

// handleCreateSiteTask executes the multi-stage provisioning pipeline for Nginx or Apache.
func (s *Service) handleCreateSiteTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var in SiteInput
	if err := json.Unmarshal([]byte(task.Payload), &in); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	engine, err := s.engines.GetEngine(in.WebServer)
	if err != nil {
		return "", err
	}
	isApache := engine.Name() == "apache"

	logger.SetStep("Connecting to server and checking web server availability", 10)
	client, record, err := s.GetSSHClient(ctx, in.ServerID)
	if err != nil {
		return "", err
	}

	// Enforce capability restrictions prior to provisioning.
	if in.PHPVersion != "" {
		if err := ssh.ValidatePHPVersion(in.PHPVersion); err != nil {
			return "", err
		}
	}
	if diag, _ := s.Database.GetDiagnostics(ctx, in.ServerID); diag != nil {
		if in.PHPVersion != "" && in.SiteType != "static" && in.SiteType != "reverse_proxy" {
			if allowed, reason := capabilities.AllowsPHPInstallByVersion(diagDistroID(diag), diag.OSVersion, in.PHPVersion); !allowed {
				return "", fmt.Errorf("site provisioning rejected: %s", reason)
			}
		}
	}

	family := s.serverOSFamily(ctx, in.ServerID)
	if isApache {
		outCheck, _, codeCheck, _ := client.Exec(ctx, "which apache2 2>/dev/null || which httpd 2>/dev/null")
		if codeCheck != 0 || len(strings.TrimSpace(string(outCheck))) == 0 {
			logger.Info("Apache HTTP Server not detected. Installing apache...")
			pkg, unit := "apache2", "apache2"
			if family == "rhel" {
				pkg, unit = "httpd", "httpd"
			}
			if err := installPackage(ctx, client, family, pkg, unit); err != nil {
				return "", err
			}
		}
		if family != "rhel" {
			logger.Info("Enabling required Apache modules (rewrite, proxy, proxy_fcgi, headers)...")
			if err := module.RunPrivileged(ctx, client, "a2enmod rewrite proxy proxy_fcgi headers"); err != nil {
				return "", err
			}
		}
	} else {
		outCheck, _, codeCheck, _ := client.Exec(ctx, "which nginx 2>/dev/null")
		if codeCheck != 0 || len(strings.TrimSpace(string(outCheck))) == 0 {
			logger.Info("Nginx not detected. Installing nginx...")
			if err := installPackage(ctx, client, family, "nginx", "nginx"); err != nil {
				return "", err
			}
		}
	}

	if in.PHPVersion != "" && in.SiteType != "static" && in.SiteType != "reverse_proxy" {
		verCompact := strings.ReplaceAll(in.PHPVersion, ".", "")
		fpmCheck := fmt.Sprintf("systemctl is-active --quiet php%s-fpm || systemctl is-active --quiet php%s-php-fpm || [ -S /run/php/php%s-fpm.sock ]",
			in.PHPVersion, verCompact, in.PHPVersion)
		_, _, codeFPM, _ := client.NewCommand(fpmCheck).WithSudo().Exec(ctx)
		if codeFPM != 0 {
			logger.Info(fmt.Sprintf("PHP-FPM %s not detected. Installing packages...", in.PHPVersion))
			if err := php.InstallVersionPackages(ctx, client, family, in.PHPVersion, nil, logger); err != nil {
				return "", fmt.Errorf("failed to install PHP-FPM %s: %w", in.PHPVersion, err)
			}
			_, _, codeAfter, _ := client.NewCommand(fpmCheck).WithSudo().Exec(ctx)
			if codeAfter != 0 {
				return "", fmt.Errorf("PHP-FPM %s is not running after installation; refusing to create a virtualhost with a dead socket", in.PHPVersion)
			}
		} else {
			logger.Info(fmt.Sprintf("PHP-FPM %s is already installed and running.", in.PHPVersion))
		}
	}

	logger.SetStep("Creating DocumentRoot directory", 25)
	docRoot := in.DocumentRoot
	if in.SiteType == "laravel" {
		docRoot = strings.TrimSuffix(docRoot, "/") + "/public"
		in.DocumentRoot = docRoot
	}
	if err := ssh.ValidateRemotePath(docRoot, false); err != nil {
		return "", err
	}

	logArgs := ssh.ShellEscape("/var/log/nginx")
	if isApache {
		logArgs = ssh.ShellEscape("/var/log/apache2") + " " + ssh.ShellEscape("/var/log/httpd")
	}

	mkdirCmd := fmt.Sprintf("mkdir -p %s %s", ssh.ShellEscape(docRoot), logArgs)
	_, stderr, code, err := client.NewCommand(mkdirCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return "", fmt.Errorf("failed to create directory %s: %s", docRoot, string(stderr))
	}
	logger.Info(fmt.Sprintf("Created DocumentRoot: %s", docRoot))

	logger.SetStep("Configuring filesystem permissions and ownership", 40)
	root := ssh.ShellEscape(docRoot)
	chownCmd := fmt.Sprintf("chown -R www-data:www-data %s || chown -R apache:apache %s || chown -R nginx:nginx %s; chmod -R 755 %s", root, root, root, root)
	if _, chownErr, chownCode, chownExecErr := client.NewCommand(chownCmd).WithSudo().Exec(ctx); chownExecErr != nil || chownCode != 0 {
		return "", fmt.Errorf("failed to set document root ownership: %s", chownErr)
	}

	logger.SetStep("Placing default welcome index file", 55)
	// Ensure checkEmptyCmd runs with sudo and does not overwrite non-empty directories.
	checkEmptyCmd := fmt.Sprintf("[ -z \"$(ls -A %s 2>/dev/null)\" ] && echo 'empty' || echo 'has_files'", ssh.ShellEscape(docRoot))
	outEmpty, _, _, _ := client.NewCommand(checkEmptyCmd).WithSudo().Exec(ctx)
	if strings.Contains(string(outEmpty), "empty") {
		fileName, fileContent := RenderDefaultIndex(in)
		indexPath := fmt.Sprintf("%s/%s", docRoot, fileName)
		_ = client.WriteFile(ctx, indexPath, []byte(fileContent), 0644)
		logger.Info(fmt.Sprintf("Created initial placeholder file: %s", indexPath))
	} else {
		logger.Info(fmt.Sprintf("Directory %s is not empty; preserving existing files without overwriting placeholder index", docRoot))
	}

	// Optional automated database and user creation in site plan.
	if in.CreateDatabase && in.DatabaseName != "" {
		logger.SetStep("Provisioning MySQL/MariaDB database and user", 60)
		dbName := in.DatabaseName
		dbUser := in.DatabaseUser
		if dbUser == "" {
			dbUser = dbName
		}
		dbPass := in.DatabasePassword
		if dbPass == "" {
			dbPass = uuid.New().String()[:12] + "!"
		}

		if err := ssh.ValidateIdent(dbName, "database name"); err == nil && ssh.ValidateIdent(dbUser, "database user") == nil {
			createDBScript := fmt.Sprintf(`mysql <<'SQL_EOF'
CREATE DATABASE IF NOT EXISTS `+"`%s`"+` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';
GRANT ALL PRIVILEGES ON `+"`%s`.*"+` TO '%s'@'localhost';
FLUSH PRIVILEGES;
SQL_EOF`, dbName, dbUser, dbPass, dbName, dbUser)

			if _, stderr, code, err := client.NewCommand(createDBScript).WithSudo().Exec(ctx); err != nil || code != 0 {
				logger.Warn(fmt.Sprintf("Failed to auto-create database '%s': %s (continuing site provisioning)", dbName, stderr))
			} else {
				logger.Info(fmt.Sprintf("Created database '%s' and granted privileges on '%s.*' to '%s'@'localhost'", dbName, dbName, dbUser))
			}
		} else {
			logger.Warn("Invalid database or username format; skipping automated database provisioning")
		}
	}

	logger.SetStep(fmt.Sprintf("Generating and installing %s VirtualHost configuration", engine.Name()), 70)
	if err := engine.InstallVirtualHost(ctx, client, in); err != nil {
		return "", fmt.Errorf("failed to install virtualhost: %w", err)
	}

	logger.SetStep(fmt.Sprintf("Testing %s configuration syntax", engine.Name()), 85)
	testOut, testCode, _ := engine.TestConfig(ctx, client)
	isTestOk := false
	if engine.Name() == "nginx" {
		isTestOk = (testCode == 0 && strings.Contains(testOut, "syntax is ok"))
	} else {
		isTestOk = (testCode == 0 || strings.Contains(strings.ToLower(testOut), "syntax ok"))
	}
	if !isTestOk {
		// Rollback virtualhost configuration.
		_ = engine.DisableVirtualHost(ctx, client, in.Domain)
		return "", fmt.Errorf("%s configuration syntax test failed:\n%s", engine.Name(), testOut)
	}
	logger.Info(fmt.Sprintf("%s syntax test: OK", engine.Name()))

	logger.SetStep(fmt.Sprintf("Reloading %s service", engine.Name()), 95)
	if err := engine.Reload(ctx, client); err != nil {
		return "", fmt.Errorf("web server reload failed, site was not marked active: %w", err)
	}

	// Local HTTP probe.
	curlCmd := fmt.Sprintf("curl -sI -o /dev/null -w '%%{http_code}' -H %s http://127.0.0.1 || true", ssh.ShellEscape("Host: "+in.Domain))
	outHTTP, _, _, _ := client.Exec(ctx, curlCmd)
	httpStatus := strings.TrimSpace(string(outHTTP))
	logger.Info(fmt.Sprintf("Local HTTP probe response code: %s", httpStatus))

	vhostPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", in.Domain)
	if isApache {
		vhostPath = fmt.Sprintf("/etc/apache2/sites-available/%s.conf", in.Domain)
	}

	// Persist in SQLite.
	siteRecord := &db.SiteRecord{
		ID:           uuid.New().String(),
		ServerID:     in.ServerID,
		Domain:       in.Domain,
		Aliases:      in.Aliases,
		DocumentRoot: docRoot,
		SiteType:     in.SiteType,
		PHPVersion:   in.PHPVersion,
		FPMPool:      in.FPMPool,
		WebServer:    in.WebServer,
		SSLEnabled:   in.SSLEnabled,
		SSLProvider:  "none",
		Status:       "active",
		Origin:       "managed",
		VhostPath:    vhostPath,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if in.SSLEnabled {
		siteRecord.SSLProvider = "letsencrypt"
	}

	if err := s.Database.SaveSite(ctx, siteRecord); err != nil {
		logger.Warn(fmt.Sprintf("Warning saving site to database: %v", err))
	}

	logger.SetStep("Website successfully provisioned and online", 100)
	return fmt.Sprintf("Website %s is active on %s (WebServer: %s, DocumentRoot: %s)", in.Domain, record.Host, in.WebServer, in.DocumentRoot), nil
}

// DiscoveredSiteRaw represents a parsed site configuration on the remote host.
type DiscoveredSiteRaw struct {
	Domain       string   `json:"domain"`
	Aliases      []string `json:"aliases"`
	DocumentRoot string   `json:"document_root"`
	SiteType     string   `json:"site_type"`
	PHPVersion   string   `json:"php_version"`
	WebServer    string   `json:"web_server"`
	SSLEnabled   bool     `json:"ssl_enabled"`
	Status       string   `json:"status"`
	VhostPath    string   `json:"vhost_path"`
	IsDefault    bool     `json:"is_default"`
}

// DiscoverSites connects to the server and inspects all existing Apache and Nginx virtual hosts.
func (s *Service) DiscoverSites(ctx context.Context, serverID string) ([]*db.SiteRecord, error) {
	client, record, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// Multi-tier discovery script: checks runtime (nginx -T, apache2ctl -S),
	// configuration directories (Debian/Ubuntu, RHEL/CentOS, panels), and /home /var/www web roots.
	discoveryScript := `export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
PY3=""
if $SUDO python3 -c 'import sys; sys.exit(0)' 2>/dev/null; then
    PY3="$SUDO python3"
elif python3 -c 'import sys; sys.exit(0)' 2>/dev/null; then
    PY3="python3"
fi
PY2=""
if $SUDO python -c 'import sys; sys.exit(0)' 2>/dev/null; then
    PY2="$SUDO python"
elif python -c 'import sys; sys.exit(0)' 2>/dev/null; then
    PY2="python"
fi

if [ -n "$PY3" ]; then
    $PY3 - << 'PYEOF'
import os, glob, re, json, subprocess

sites_map = {}

def normalize_domain(d):
    if not d:
        return ""
    d = d.strip().strip("'\"").rstrip(";")
    if ":" in d:
        d = d.split(":")[0]
    if d.startswith("*."):
        d = d[2:]
    d = d.strip()
    if not d or d in ["_", "localhost", "default", "default_server", "any", "none"] or d.startswith("~"):
        return ""
    if " " in d or "\t" in d or "/" in d:
        return ""
    return d.lower()

def clean_doc_root(path):
    if not path:
        return ""
    path = path.strip().strip("'\"").rstrip(";")
    path = path.strip()
    if len(path) > 1 and path.endswith("/"):
        path = path[:-1]
    return path

def detect_site_type(doc_root, content=""):
    content_lower = (content or "").lower()
    if doc_root and os.path.isdir(doc_root):
        if os.path.exists(os.path.join(doc_root, "wp-config.php")) or os.path.exists(os.path.join(doc_root, "..", "wp-config.php")):
            return "wordpress"
        if os.path.exists(os.path.join(doc_root, "artisan")) or os.path.exists(os.path.join(doc_root, "..", "artisan")):
            return "laravel"
        if os.path.exists(os.path.join(doc_root, "package.json")) and not os.path.exists(os.path.join(doc_root, "index.php")):
            return "nodejs"
        if any(os.path.exists(os.path.join(doc_root, f)) for f in ["index.php", "composer.json"]):
            return "php"
        if os.path.exists(os.path.join(doc_root, "index.html")) or os.path.exists(os.path.join(doc_root, "index.htm")):
            return "static"
    if "wp-content" in content_lower or "wordpress" in content_lower:
        return "wordpress"
    if "artisan" in content_lower or "laravel" in content_lower:
        return "laravel"
    if "fastcgi_pass" in content_lower or "php" in content_lower or "fcgi" in content_lower:
        return "php"
    if "proxy_pass" in content_lower and not doc_root:
        return "reverse_proxy"
    return "static"

def get_system_default_php():
    # 1. Check symlink /run/php/php-fpm.sock or /var/run/php/php-fpm.sock
    for p in ['/run/php/php-fpm.sock', '/var/run/php/php-fpm.sock']:
        try:
            if os.path.islink(p):
                target = os.readlink(p)
                m = re.search(r'php([0-9]+\.[0-9]+)', target)
                if m:
                    return m.group(1)
        except Exception:
            pass
    # 2. Check apache conf-enabled or mods-enabled
    for d in ['/etc/apache2/conf-enabled', '/etc/apache2/mods-enabled']:
        if os.path.isdir(d):
            try:
                for f in sorted(os.listdir(d), reverse=True):
                    m = re.search(r'php([0-9]+\.[0-9]+)', f)
                    if m:
                        return m.group(1)
            except Exception:
                pass
    # 3. Check active PHP-FPM sockets in /run/php/
    for v in ['8.4', '8.3', '8.2', '8.1', '8.0', '7.4', '7.3', '7.2']:
        if os.path.exists(f'/run/php/php{v}-fpm.sock') or os.path.exists(f'/var/run/php/php{v}-fpm.sock'):
            return v
    # 4. Check CLI php version
    try:
        p = subprocess.run(["php", "-r", "echo PHP_MAJOR_VERSION . '.' . PHP_MINOR_VERSION;"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=2)
        if p.returncode == 0 and p.stdout.strip():
            return p.stdout.strip()
    except Exception:
        pass
    return ""

def extract_php_version(content):
    if not content:
        return ""
    m = re.search(r'php[/-]?fpm[/-]?([0-9]+\.[0-9]+)|php([0-9]+\.[0-9]+)[/-]?fpm', content, re.IGNORECASE)
    if m:
        return m.group(1) or m.group(2)
    m2 = re.search(r'php([0-9]+\.[0-9]+)', content, re.IGNORECASE)
    if m2:
        return m2.group(1)
    m3 = re.search(r'127\.0\.0\.1:90([78][0-9])', content)
    if m3:
        p = m3.group(1)
        return p[0] + "." + p[1]
    if "php-fpm.sock" in content or "proxy_fcgi" in content or "fastcgi_pass" in content:
        return get_system_default_php()
    return ""

def read_conf_file(path):
    try:
        with open(path, "r", encoding="utf-8", errors="ignore") as fp:
            return fp.read()
    except Exception:
        return ""

def add_or_update_site(domain, aliases, doc_root, site_type, php_ver, web_server, ssl_enabled, status, vhost_path=""):
    domain = normalize_domain(domain)
    if not domain:
        return
    clean_aliases = []
    for a in (aliases or []):
        na = normalize_domain(a)
        if na and na != domain and na not in clean_aliases:
            clean_aliases.append(na)
    
    doc_root = clean_doc_root(doc_root)
    if not site_type or site_type == "static":
        site_type = detect_site_type(doc_root)

    if not php_ver and site_type in ("php", "wordpress", "laravel"):
        php_ver = get_system_default_php()

    if domain in sites_map:
        existing = sites_map[domain]
        for a in clean_aliases:
            if a not in existing["aliases"]:
                existing["aliases"].append(a)
        if not existing["document_root"] and doc_root:
            existing["document_root"] = doc_root
        if (existing["web_server"] == "nginx" and web_server == "apache") or (existing["web_server"] == "apache" and web_server == "nginx"):
            existing["web_server"] = "nginx+apache"
        elif web_server and existing["web_server"] == "unknown":
            existing["web_server"] = web_server
        if ssl_enabled:
            existing["ssl_enabled"] = True
        if not existing["php_version"] and php_ver:
            existing["php_version"] = php_ver
            if existing["site_type"] == "static":
                existing["site_type"] = "php"
        if status == "active":
            existing["status"] = "active"
        if vhost_path and not existing.get("vhost_path"):
            existing["vhost_path"] = vhost_path
    else:
        sites_map[domain] = {
            "domain": domain,
            "aliases": clean_aliases,
            "document_root": doc_root,
            "site_type": site_type,
            "php_version": php_ver or "",
            "web_server": web_server or "nginx",
            "ssl_enabled": bool(ssl_enabled),
            "status": status or "active",
            "vhost_path": vhost_path or "",
            "is_default": False
        }

def parse_nginx_server_block(block, is_enabled, src_path=""):
    sn_matches = re.findall(r'server_name\s+([^;]+);', block, re.DOTALL | re.IGNORECASE)
    names = []
    for sn in sn_matches:
        for item in sn.split():
            clean = normalize_domain(item)
            if clean and clean not in names:
                names.append(clean)
    if not names:
        return
    names.sort(key=lambda x: (x.startswith("www."), len(x)))
    main_domain = names[0]
    aliases = names[1:]

    doc_root = ""
    r_match = re.search(r'(?:^\s*|\s+)root\s+["\']?([^";\s#]+)["\']?;', block, re.MULTILINE | re.IGNORECASE)
    if r_match:
        doc_root = r_match.group(1).strip()
    else:
        a_match = re.search(r'(?:^\s*|\s+)alias\s+["\']?([^";\s#]+)["\']?;', block, re.MULTILINE | re.IGNORECASE)
        if a_match:
            doc_root = a_match.group(1).strip()

    php_ver = extract_php_version(block)
    st = detect_site_type(doc_root, block)
    ssl_enabled = bool(re.search(r'ssl_certificate|listen\s+[^;]*443|ssl\s+on;', block, re.IGNORECASE))

    add_or_update_site(main_domain, aliases, doc_root, st, php_ver, "nginx", ssl_enabled, "active" if is_enabled else "inactive", src_path)

def parse_nginx_config_content(content, is_enabled, src_path=""):
    parts = re.split(r'(?:^|\s+)server\s*\{', content, flags=re.MULTILINE | re.IGNORECASE)
    if len(parts) > 1:
        for block in parts[1:]:
            parse_nginx_server_block(block, is_enabled, src_path)
    else:
        parse_nginx_server_block(content, is_enabled, src_path)

def parse_apache_config_content(content, is_enabled, src_path=""):
    vhost_blocks = re.findall(r'<VirtualHost\s+[^>]*>(.*?)</VirtualHost>', content, flags=re.DOTALL | re.IGNORECASE)
    if not vhost_blocks:
        vhost_blocks = [content]

    for block in vhost_blocks:
        sn_match = re.search(r'ServerName\s+([^\s#]+)', block, re.IGNORECASE)
        if not sn_match:
            continue
        domain = normalize_domain(sn_match.group(1))
        if not domain:
            continue

        aliases = []
        sa_matches = re.findall(r'ServerAlias\s+(.+)', block, re.IGNORECASE)
        for sa in sa_matches:
            for item in sa.split():
                clean = normalize_domain(item)
                if clean and clean != domain and clean not in aliases:
                    aliases.append(clean)

        doc_root = ""
        dr_match = re.search(r'DocumentRoot\s+["\']?([^"\'\n\r#]+?)["\']?(?:\s|$)', block, re.IGNORECASE)
        if dr_match:
            doc_root = dr_match.group(1).strip()

        php_ver = extract_php_version(block)
        st = detect_site_type(doc_root, block)
        ssl_enabled = bool(re.search(r'SSLEngine\s+on|SSLCertificateFile|<VirtualHost\s+[^>]*:443', block, re.IGNORECASE))

        add_or_update_site(domain, aliases, doc_root, st, php_ver, "apache", ssl_enabled, "active" if is_enabled else "inactive", src_path)

# 1. RUNTIME NGINX DUMP (nginx -T)
nginx_dump = ""
try:
    nginx_cmd = None
    for nc in ["nginx", "/usr/sbin/nginx", "/usr/bin/nginx", "/usr/local/nginx/sbin/nginx", "/usr/local/sbin/nginx", "/opt/nginx/sbin/nginx"]:
        if subprocess.run(["which", nc], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0 or os.path.exists(nc):
            nginx_cmd = nc
            break
    if nginx_cmd:
        p = subprocess.run([nginx_cmd, "-T"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=5)
        out = p.stdout or ""
        if out:
            nginx_dump = out
            file_blocks = re.split(r'# configuration file ([^:]+):', out)
            for i in range(1, len(file_blocks), 2):
                parse_nginx_config_content(file_blocks[i+1], is_enabled=True, src_path=file_blocks[i].strip())
except Exception:
    pass

# 2. RUNTIME APACHE DUMP (apache2ctl -S or httpd -S)
try:
    cmd = None
    for c in ["apache2ctl", "apachectl", "httpd", "/usr/sbin/apache2ctl", "/usr/sbin/apachectl", "/usr/sbin/httpd"]:
        if subprocess.run(["which", c], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0 or os.path.exists(c):
            cmd = c
            break
    if cmd:
        p = subprocess.run([cmd, "-S"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=5)
        out = (p.stdout or "") + (p.stderr or "")
        for line in out.splitlines():
            m = re.search(r'namevhost\s+([^\s\(\)]+)\s+\(([^:]+):', line)
            if m:
                d = normalize_domain(m.group(1))
                c_file = m.group(2).strip()
                if d and os.path.isfile(c_file):
                    parse_apache_config_content(read_conf_file(c_file), is_enabled=True, src_path=c_file)
except Exception:
    pass

# 3. NGINX FILESYSTEM CONFIG DIRECTORIES
nginx_dirs = [
    ("/etc/nginx/sites-available", "/etc/nginx/sites-enabled"),
    ("/etc/nginx/sites-enabled", "/etc/nginx/sites-enabled"),
    ("/etc/nginx/conf.d", "/etc/nginx/conf.d"),
    ("/etc/nginx/vhosts", "/etc/nginx/vhosts"),
    ("/etc/nginx/fastpanel2-sites", "/etc/nginx/fastpanel2-sites"),
    ("/etc/nginx/fastpanel2-sites-available", "/etc/nginx/fastpanel2-sites-enabled"),
    ("/etc/nginx/plesk.conf.d/vhosts", "/etc/nginx/plesk.conf.d/vhosts"),
    ("/www/server/panel/vhost/nginx", "/www/server/panel/vhost/nginx"),
]
for p in glob.glob("/home/*/conf/web") + glob.glob("/home/*/conf/nginx") + glob.glob("/home/*/conf/web/*"):
    if os.path.isdir(p):
        nginx_dirs.append((p, p))

for avail_dir, enabled_dir in nginx_dirs:
    if not os.path.isdir(avail_dir):
        continue
    try:
        filenames = os.listdir(avail_dir)
    except Exception:
        continue
    for fname in filenames:
        fpath = os.path.join(avail_dir, fname)
        if not os.path.isfile(fpath):
            continue
        if fname.startswith((".", "#")) or fname.endswith((".dpkg-old", ".dpkg-dist", ".bak", ".save", "~", ".swp", ".rpmnew")):
            continue
        if fname in ["nginx.conf", "mime.types", "fastcgi.conf", "fastcgi_params", "proxy_params", "scgi_params", "uwsgi_params", "default.conf", "000-default.conf"]:
            continue
        
        is_enabled = True
        if avail_dir != enabled_dir and os.path.isdir(enabled_dir):
            is_enabled = os.path.exists(os.path.join(enabled_dir, fname)) or os.path.exists(os.path.join(enabled_dir, fname + ".conf"))

        content = read_conf_file(fpath)
        if content:
            parse_nginx_config_content(content, is_enabled, fpath)

# 4. APACHE FILESYSTEM CONFIG DIRECTORIES
apache_dirs = [
    ("/etc/apache2/sites-available", "/etc/apache2/sites-enabled"),
    ("/etc/apache2/sites-enabled", "/etc/apache2/sites-enabled"),
    ("/etc/apache2/conf-available", "/etc/apache2/conf-enabled"),
    ("/etc/apache2/conf.d", "/etc/apache2/conf.d"),
    ("/etc/apache2/vhosts.d", "/etc/apache2/vhosts.d"),
    ("/etc/httpd/conf.d", "/etc/httpd/conf.d"),
    ("/etc/httpd/conf.vhosts", "/etc/httpd/conf.vhosts"),
    ("/etc/httpd/sites-enabled", "/etc/httpd/sites-enabled"),
    ("/www/server/panel/vhost/apache", "/www/server/panel/vhost/apache"),
]
for p in glob.glob("/home/*/conf/web") + glob.glob("/home/*/conf/apache") + glob.glob("/home/*/conf/apache2") + glob.glob("/home/*/conf/web/*"):
    if os.path.isdir(p):
        apache_dirs.append((p, p))

for avail_dir, enabled_dir in apache_dirs:
    if not os.path.isdir(avail_dir):
        continue
    try:
        filenames = os.listdir(avail_dir)
    except Exception:
        continue
    for fname in filenames:
        fpath = os.path.join(avail_dir, fname)
        if not os.path.isfile(fpath):
            continue
        if fname.startswith((".", "#")) or fname.endswith((".dpkg-old", ".dpkg-dist", ".bak", ".save", "~", ".swp", ".rpmnew")):
            continue
        if fname in ["httpd.conf", "apache2.conf", "ports.conf", "magic", "envvars", "000-default.conf", "default-ssl.conf"]:
            continue

        is_enabled = True
        if avail_dir != enabled_dir and os.path.isdir(enabled_dir):
            is_enabled = os.path.exists(os.path.join(enabled_dir, fname)) or os.path.exists(os.path.join(enabled_dir, fname + ".conf"))

        content = read_conf_file(fpath)
        if content:
            parse_apache_config_content(content, is_enabled, fpath)

# 4b. DEFAULT VHOST DETECTION (servers without named vhosts)
def default_docroot_has_app(dr):
    if not dr or not os.path.isdir(dr):
        return False
    return any(os.path.exists(os.path.join(dr, f)) for f in ["index.php", "wp-config.php", "artisan", "composer.json"])

def emit_default_site(doc_root, web_server, vhost_path, ssl_enabled):
    if not vhost_path:
        return
    # Skip the distro placeholder when named sites exist; keep it when it hosts a real app
    if sites_map and not default_docroot_has_app(doc_root):
        return
    st = detect_site_type(doc_root)
    php_ver = get_system_default_php() if st in ("php", "wordpress", "laravel") else ""
    sites_map["_default"] = {
        "domain": "_default",
        "aliases": [],
        "document_root": doc_root,
        "site_type": st,
        "php_version": php_ver or "",
        "web_server": web_server,
        "ssl_enabled": bool(ssl_enabled),
        "status": "active",
        "vhost_path": vhost_path,
        "is_default": True
    }

def find_nginx_default():
    candidates = []
    if nginx_dump:
        file_blocks = re.split(r'# configuration file ([^:]+):', nginx_dump)
        for i in range(1, len(file_blocks), 2):
            fpath = file_blocks[i].strip()
            parts = re.split(r'(?:^|\s+)server\s*\{', file_blocks[i+1], flags=re.MULTILINE | re.IGNORECASE)
            for block in parts[1:]:
                has_name = False
                for sn in re.findall(r'server_name\s+([^;]+);', block, re.IGNORECASE):
                    for item in sn.split():
                        if normalize_domain(item):
                            has_name = True
                if has_name:
                    continue
                is_default = bool(re.search(r'listen\s+[^;]*default_server', block, re.IGNORECASE))
                if is_default or re.search(r'listen\s+[^;]*(:|\s)80\b', block, re.IGNORECASE):
                    candidates.append((fpath, block))
    for p in ["/etc/nginx/sites-enabled/default", "/etc/nginx/conf.d/default.conf"]:
        if os.path.isfile(p):
            candidates.append((os.path.realpath(p), read_conf_file(p)))
    for fpath, block in candidates:
        r_match = re.search(r'(?:^\s*|\s+)root\s+["\']?([^";\s#]+)["\']?;', block, re.MULTILINE | re.IGNORECASE)
        dr = clean_doc_root(r_match.group(1)) if r_match else ""
        ssl = bool(re.search(r'ssl_certificate|listen\s+[^;]*443', block, re.IGNORECASE))
        return fpath, dr, ssl
    return "", "", False

def find_apache_default():
    for p in ["/etc/apache2/sites-enabled/000-default.conf", "/etc/apache2/sites-enabled/default-ssl.conf"]:
        if os.path.isfile(p):
            content = read_conf_file(p)
            m = re.search(r'DocumentRoot\s+["\']?([^"\'\n\r#]+?)["\']?(?:\s|$)', content, re.IGNORECASE)
            dr = clean_doc_root(m.group(1)) if m else ""
            ssl = "ssl" in os.path.basename(p) or bool(re.search(r'SSLEngine\s+on', content, re.IGNORECASE))
            return os.path.realpath(p), dr, ssl
    return "", "", False

ng_default = find_nginx_default()
ap_default = find_apache_default()
if ng_default[0] and ap_default[0]:
    # Hybrid stack: PHP runs in the Apache backend
    emit_default_site(ap_default[1], "nginx+apache", ap_default[0], ap_default[2] or ng_default[2])
elif ap_default[0]:
    emit_default_site(ap_default[1], "apache", ap_default[0], ap_default[2])
elif ng_default[0]:
    emit_default_site(ng_default[1], "nginx", ng_default[0], ng_default[2])

# 5. FILESYSTEM WEB ROOT DISCOVERY (home, var/www, srv)
default_ws = "nginx"
if subprocess.run(["which", "apache2"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
    default_ws = "apache"
elif subprocess.run(["which", "httpd"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
    default_ws = "apache"
elif subprocess.run(["which", "nginx"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
    default_ws = "nginx"

candidate_dirs = []
search_globs = [
    "/home/*/web/*",
    "/home/*/web/*/public_html",
    "/home/*/*",
    "/home/*/public_html",
    "/home/*/www",
    "/home/*",
    "/var/www/*",
    "/var/www/html/*",
    "/srv/www/*",
    "/srv/*"
]
for pattern in search_globs:
    for d in glob.glob(pattern):
        if os.path.isdir(d):
            candidate_dirs.append(d)

for dpath in candidate_dirs:
    bname = os.path.basename(dpath)
    if bname.startswith(".") or bname in ["lost+found", "bin", "etc", "lib", "log", "tmp", "ssl", "backup", "backups", "html", "cgi-bin"]:
        continue
    
    has_web_files = False
    doc_root = dpath
    for sub in ["public_html", "public", "www", "web", "html"]:
        sub_path = os.path.join(dpath, sub)
        if os.path.isdir(sub_path):
            if any(os.path.exists(os.path.join(sub_path, f)) for f in ["index.php", "index.html", "wp-config.php", "artisan", "composer.json"]):
                doc_root = sub_path
                has_web_files = True
                break

    if not has_web_files:
        if any(os.path.exists(os.path.join(dpath, f)) for f in ["index.php", "index.html", "wp-config.php", "artisan", "composer.json", "package.json", ".htaccess"]):
            has_web_files = True

    if not has_web_files:
        continue

    cand_domain = ""
    if "." in bname and not bname.endswith((".log", ".txt", ".tar", ".gz", ".zip")):
        cand_domain = bname
    else:
        parent_name = os.path.basename(os.path.dirname(dpath))
        if "." in parent_name:
            cand_domain = parent_name
        elif parent_name and parent_name not in ["home", "var", "www", "srv", "web"]:
            cand_domain = parent_name + ".local"
        else:
            cand_domain = bname + ".local"

    cand_domain = normalize_domain(cand_domain)
    if not cand_domain:
        continue

    if cand_domain in sites_map:
        if not sites_map[cand_domain]["document_root"] or sites_map[cand_domain]["document_root"] == "/var/www/html":
            sites_map[cand_domain]["document_root"] = doc_root
    else:
        st = detect_site_type(doc_root)
        add_or_update_site(cand_domain, [], doc_root, st, "", default_ws, False, "active")

# Certificate presence probe: config parsing can miss SSL (shared 443 catch-all
# blocks, included ssl snippets), but an issued cert is ground truth.
for d, info in sites_map.items():
    if info.get("ssl_enabled"):
        continue
    for name in (d, "www." + d):
        if os.path.exists("/etc/letsencrypt/live/" + name + "/fullchain.pem"):
            info["ssl_enabled"] = True
            break

print(json.dumps(list(sites_map.values())))
PYEOF
elif [ -n "$PY2" ]; then
    $PY2 - << 'PYEOF'
import os, glob, re, json
sites_map = {}
for p in ["/etc/apache2/sites-available", "/etc/httpd/conf.d", "/etc/nginx/sites-available", "/etc/nginx/conf.d"]:
    if not os.path.isdir(p): continue
    for f in os.listdir(p):
        fp = os.path.join(p, f)
        if not os.path.isfile(fp): continue
        try:
            with open(fp, 'r') as fp_in: c = fp_in.read()
            m = re.search(r'(?:ServerName|server_name)\s+([^;\s#]+)', c, re.IGNORECASE)
            if m:
                d = m.group(1).strip()
                if d and d not in ["_", "localhost", "default"]:
                    dr = re.search(r'(?:DocumentRoot|root)\s+["\']?([^";\s#]+)', c, re.IGNORECASE)
                    sites_map[d] = {"domain": d, "aliases": [], "document_root": dr.group(1) if dr else "", "site_type": "php" if "php" in c.lower() else "static", "php_version": "", "web_server": "apache" if "apache" in p or "httpd" in p else "nginx", "ssl_enabled": "ssl" in c.lower(), "status": "active"}
        except Exception: pass
print(json.dumps(list(sites_map.values())))
PYEOF
else
    # Minimal pure bash JSON scanner fallback
    SCAN=$(cat << 'BAWEOF'
    echo "["
    FIRST=1
    for f in /etc/nginx/sites-available/* /etc/nginx/conf.d/* /etc/apache2/sites-available/* /etc/httpd/conf.d/*; do
        [ -f "$f" ] || continue
        case "$(basename "$f")" in
            *.dpkg*|*.bak|*.save|*~|default|000-default*) continue ;;
        esac
        DOMAIN=$(grep -E -i '^[[:space:]]*(server_name|ServerName)[[:space:]]+' "$f" 2>/dev/null | head -n1 | awk '{print $2}' | tr -d ';\"'"'")
        [ -n "$DOMAIN" ] || continue
        case "$DOMAIN" in
            _|"localhost"|"default"|default_server) continue ;;
        esac
        DOCROOT=$(grep -E -i '^[[:space:]]*(root|DocumentRoot)[[:space:]]+' "$f" 2>/dev/null | head -n1 | awk '{print $2}' | tr -d ';\"'"'")
        WS="nginx"
        echo "$f" | grep -q -E 'apache|httpd' && WS="apache"
        SSL="false"
        grep -q -i -E 'ssl_certificate|SSLEngine[[:space:]]+on|443' "$f" 2>/dev/null && SSL="true"
        [ "$FIRST" -eq 1 ] && FIRST=0 || echo ","
        printf '{"domain":"%s","aliases":[],"document_root":"%s","site_type":"php","php_version":"","web_server":"%s","ssl_enabled":%s,"status":"active"}' "$DOMAIN" "$DOCROOT" "$WS" "$SSL"
    done
    echo "]"
BAWEOF
)
    if $SUDO true 2>/dev/null; then
        printf '%s\n' "$SCAN" | $SUDO /bin/sh
    else
        printf '%s\n' "$SCAN" | /bin/sh
    fi
fi
`

	out, stderr, _, _ := client.NewCommand(discoveryScript).Exec(ctx)
	outStr := strings.TrimSpace(string(out))

	// Safely slice between first '[' and last ']' to ignore any MOTD, banner, or prompt noise.
	jsonStart := strings.Index(outStr, "[")
	jsonEnd := strings.LastIndex(outStr, "]")
	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		outStr = outStr[jsonStart : jsonEnd+1]
	}

	var rawSites []DiscoveredSiteRaw
	if err := json.Unmarshal([]byte(outStr), &rawSites); err != nil {
		// If python fails or returned empty, log stderr.
		if len(stderr) > 0 {
			return nil, fmt.Errorf("failed to discover sites on server: %s (stderr: %s)", err, string(stderr))
		}
	}

	// Fetch existing sites in database.
	existingList, _ := s.Database.ListSites(ctx, serverID)
	existingMap := make(map[string]*db.SiteRecord)
	for _, es := range existingList {
		existingMap[es.Domain] = es
	}

	for _, raw := range rawSites {
		domain := raw.Domain
		if raw.IsDefault {
			// Distro default vhost has no domain of its own; key it by the server host.
			if record == nil {
				continue
			}
			domain = record.Host
		}
		if domain == "" {
			continue
		}
		if existing, found := existingMap[domain]; found {
			// Update status & metadata.
			if raw.DocumentRoot != "" {
				existing.DocumentRoot = raw.DocumentRoot
			}
			if raw.SiteType != "" {
				existing.SiteType = raw.SiteType
			}
			if raw.PHPVersion != "" {
				existing.PHPVersion = raw.PHPVersion
			}
			if raw.WebServer != "" {
				existing.WebServer = raw.WebServer
			}
			if raw.VhostPath != "" {
				existing.VhostPath = raw.VhostPath
			}
			existing.Status = raw.Status
			existing.SSLEnabled = raw.SSLEnabled
			if raw.SSLEnabled && existing.SSLProvider == "none" {
				existing.SSLProvider = "custom"
			}
			if len(raw.Aliases) > 0 {
				existing.Aliases = raw.Aliases
			}
			if existing.Origin == "" {
				existing.Origin = "discovered"
			}
			existing.UpdatedAt = time.Now()
			_ = s.Database.SaveSite(ctx, existing)
		} else {
			// Insert newly discovered site (ADR-S4).
			newSite := &db.SiteRecord{
				ID:           uuid.New().String(),
				ServerID:     serverID,
				Domain:       domain,
				Aliases:      raw.Aliases,
				DocumentRoot: raw.DocumentRoot,
				SiteType:     raw.SiteType,
				PHPVersion:   raw.PHPVersion,
				WebServer:    raw.WebServer,
				SSLEnabled:   raw.SSLEnabled,
				SSLProvider:  "none",
				Status:       raw.Status,
				Origin:       "discovered",
				VhostPath:    raw.VhostPath,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			if raw.SSLEnabled {
				newSite.SSLProvider = "custom"
			}
			_ = s.Database.SaveSite(ctx, newSite)
		}
	}

	return s.Database.ListSites(ctx, serverID)
}
