package site

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	nginxFastcgiRegex = regexp.MustCompile(`fastcgi_pass\s+[^;]+;`)
	apacheFcgiRegex   = regexp.MustCompile(`(?s)<FilesMatch\s+\\\?\.php\$>.*?</FilesMatch>`)
	nginxIndexRegex   = regexp.MustCompile(`(?m)^(\s*)index\s+([^;]+);`)
)

// ReplaceFastCGISocketPath replaces FastCGI socket paths in an Nginx virtual host with a specific socket path.
func ReplaceFastCGISocketPath(vhostContent string, socketPath string) (string, error) {
	newDirective := fmt.Sprintf("fastcgi_pass unix:%s;", socketPath)
	if nginxFastcgiRegex.MatchString(vhostContent) {
		return nginxFastcgiRegex.ReplaceAllString(vhostContent, newDirective), nil
	}

	// If no fastcgi_pass exists yet, insert into location ~ \.php$ if present.
	if strings.Contains(vhostContent, "location ~ \\.php$") {
		locIdx := strings.Index(vhostContent, "location ~ \\.php$")
		openBrace := strings.Index(vhostContent[locIdx:], "{")
		if openBrace != -1 {
			insertPos := locIdx + openBrace + 1
			injected := "\n        include snippets/fastcgi-php.conf;\n        " + newDirective
			return vhostContent[:insertPos] + injected + vhostContent[insertPos:], nil
		}
	}

	// No PHP handling at all (e.g. distro default vhost): inject a full location block.
	if closeIdx := strings.LastIndex(vhostContent, "}"); closeIdx != -1 {
		block := fmt.Sprintf("\n    location ~ \\.php$ {\n        include fastcgi_params;\n        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;\n        %s\n    }\n", newDirective)
		updated := vhostContent[:closeIdx] + block + vhostContent[closeIdx:]
		if nginxIndexRegex.MatchString(updated) {
			updated = nginxIndexRegex.ReplaceAllStringFunc(updated, func(m string) string {
				if strings.Contains(m, "index.php") {
					return m
				}
				sub := nginxIndexRegex.FindStringSubmatch(m)
				return sub[1] + "index index.php " + sub[2] + ";"
			})
		} else {
			updated = strings.Replace(updated, block, "\n    index index.php index.html;\n"+block, 1)
		}
		return updated, nil
	}

	return "", fmt.Errorf("no fastcgi_pass directive or php location block found in virtual host")
}

// ReplaceFastCGISocketInConfig replaces FastCGI socket paths in an Nginx virtual host using default socket.
func ReplaceFastCGISocketInConfig(vhostContent string, targetPHPVersion string) (string, error) {
	return ReplaceFastCGISocketPath(vhostContent, fmt.Sprintf("/run/php/php%s-fpm.sock", targetPHPVersion))
}

// InjectApacheFCGIHandlerWithSocket replaces or injects SetHandler proxy_fcgi in an Apache virtual host with custom socket.
func InjectApacheFCGIHandlerWithSocket(vhostContent string, socketPath string) (string, error) {
	newHandler := fmt.Sprintf(`    <FilesMatch \.php$>
        SetHandler "proxy:unix:%s|fcgi://localhost"
    </FilesMatch>`, socketPath)

	if apacheFcgiRegex.MatchString(vhostContent) {
		return apacheFcgiRegex.ReplaceAllString(vhostContent, newHandler), nil
	}

	// Insert before </VirtualHost>
	closeTag := "</VirtualHost>"
	if idx := strings.Index(vhostContent, closeTag); idx != -1 {
		return vhostContent[:idx] + newHandler + "\n" + vhostContent[idx:], nil
	}

	return "", fmt.Errorf("no </VirtualHost> closing tag found in Apache virtual host")
}

// InjectApacheFCGIHandler replaces or injects SetHandler proxy_fcgi in an Apache virtual host,
// allowing coexistence without disabling mod_php for other sites.
func InjectApacheFCGIHandler(vhostContent string, targetPHPVersion string) (string, error) {
	return InjectApacheFCGIHandlerWithSocket(vhostContent, fmt.Sprintf("/run/php/php%s-fpm.sock", targetPHPVersion))
}

func resolveTargetSocket(phpVersion, poolName, customSocket string) string {
	if customSocket != "" {
		return customSocket
	}
	cleanVer := strings.TrimSpace(phpVersion)
	if poolName != "" && poolName != "www" {
		return fmt.Sprintf("/run/php/php%s-fpm-%s.sock", cleanVer, poolName)
	}
	return fmt.Sprintf("/run/php/php%s-fpm.sock", cleanVer)
}

// PlanSiteRuntimeAssignment previews the runtime update and tests configuration compatibility.
func (s *Service) PlanSiteRuntimeAssignment(ctx context.Context, in SiteRuntimeInput) (*SiteRuntimePlan, error) {
	siteRecord, err := s.Database.GetSite(ctx, in.SiteID)
	if err != nil || siteRecord == nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, siteRecord.ServerID)
	if err != nil {
		return nil, err
	}

	vhostPath, isApache, err := s.resolveVhost(ctx, client, siteRecord)
	if err != nil {
		return nil, err
	}

	data, err := client.ReadFile(ctx, vhostPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vhost %s: %w", vhostPath, err)
	}

	targetSocket := resolveTargetSocket(in.PHPVersion, in.PoolName, in.CustomListenSocket)
	wsName := "nginx"
	if isApache {
		wsName = "apache"
	}

	plan := &SiteRuntimePlan{
		SiteID:            in.SiteID,
		Domain:            siteRecord.Domain,
		CurrentPHPVersion: siteRecord.PHPVersion,
		TargetPHPVersion:  in.PHPVersion,
		Handler:           in.Handler,
		PoolName:          in.PoolName,
		TargetSocket:      targetSocket,
		Webserver:         wsName,
		VhostFile:         vhostPath,
		IsCompatible:      true,
	}

	if in.Handler == "" {
		plan.Handler = "fpm"
	}

	var newConfig string
	if isApache {
		newConfig, err = InjectApacheFCGIHandlerWithSocket(string(data), targetSocket)
	} else {
		newConfig, err = ReplaceFastCGISocketPath(string(data), targetSocket)
	}

	if err != nil {
		plan.IsCompatible = false
		plan.PreflightWarnings = append(plan.PreflightWarnings, fmt.Sprintf("Failed to formulate new virtual host: %v", err))
	} else {
		plan.DiffPreview = newConfig
	}

	// Verify if target PHP socket exists on server
	if !client.FileExists(ctx, targetSocket) {
		plan.PreflightWarnings = append(plan.PreflightWarnings, fmt.Sprintf("Target socket %s is not currently active on the server. Ensure the PHP-FPM service/pool is running.", targetSocket))
	}

	return plan, nil
}

// AssignSiteRuntime safely switches a site's PHP version and FPM pool using ConfigGuardian.
func (s *Service) AssignSiteRuntime(ctx context.Context, in SiteRuntimeInput) error {
	siteRecord, err := s.Database.GetSite(ctx, in.SiteID)
	if err != nil || siteRecord == nil {
		return fmt.Errorf("site not found: %w", err)
	}

	if siteRecord.Origin == "discovered" {
		return fmt.Errorf("сайт '%s' обнаружен на сервере снаружи (origin: discovered). Для изменения конфигурации и смены PHP примите сайт под управление", siteRecord.Domain)
	}

	client, _, err := s.GetSSHClient(ctx, siteRecord.ServerID)
	if err != nil {
		return err
	}

	vhostPath, isApache, err := s.resolveVhost(ctx, client, siteRecord)
	if err != nil {
		return err
	}

	data, err := client.ReadFile(ctx, vhostPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", vhostPath, err)
	}

	targetSocket := resolveTargetSocket(in.PHPVersion, in.PoolName, in.CustomListenSocket)
	var newConfig string
	testCmd, reloadCmd := testAndReloadCmds(isApache)

	if isApache {
		_, _, _, _ = client.NewCommand("a2enmod proxy proxy_fcgi 2>/dev/null || true").WithSudo().Exec(ctx)
		newConfig, err = InjectApacheFCGIHandlerWithSocket(string(data), targetSocket)
		if err != nil {
			return err
		}
	} else {
		newConfig, err = ReplaceFastCGISocketPath(string(data), targetSocket)
		if err != nil {
			return err
		}
	}

	res, err := client.SafeWriteWithTest(ctx, vhostPath, []byte(newConfig), testCmd, reloadCmd)
	if err != nil {
		return fmt.Errorf("runtime update failed (restored to backup %s): %s", res.BackupPath, res.ErrorMessage)
	}

	siteRecord.PHPVersion = in.PHPVersion
	siteRecord.FPMPool = in.PoolName
	return s.Database.SaveSite(ctx, siteRecord)
}

// SwitchSitePHPVersion safely switches a site's PHP version on the remote server
// using ConfigGuardian for atomic syntax validation and rollback (backwards-compatibility wrapper).
func (s *Service) SwitchSitePHPVersion(ctx context.Context, siteID string, targetPHPVersion string) error {
	return s.AssignSiteRuntime(ctx, SiteRuntimeInput{
		SiteID:     siteID,
		PHPVersion: targetPHPVersion,
		Handler:    "fpm",
	})
}
