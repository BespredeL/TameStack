package site

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/ssh"

	"github.com/google/uuid"
)

var (
	nginxRootRegex     = regexp.MustCompile(`(?m)^\s*root\s+[^;]+;`)
	apacheDocRootRegex = regexp.MustCompile(`(?m)^\s*DocumentRoot\s+\S+`)
	apacheDirRegex     = regexp.MustCompile(`(?m)<Directory\s+([^>]+)>`)
)

// UpdateDocumentRootInConfig modifies the document root directive in Nginx or Apache virtual hosts.
// When isApache is true, Apache DocumentRoot is preferred over nginx root.
func UpdateDocumentRootInConfig(vhostContent string, newDocRoot string, isApache bool) (string, error) {
	newDocRoot = strings.TrimSpace(newDocRoot)
	if newDocRoot == "" {
		return "", fmt.Errorf("document root cannot be empty")
	}

	applyApache := func() (string, bool) {
		if !apacheDocRootRegex.MatchString(vhostContent) {
			return "", false
		}
		replacement := fmt.Sprintf("    DocumentRoot %s", newDocRoot)
		res := apacheDocRootRegex.ReplaceAllString(vhostContent, replacement)
		res = apacheDirRegex.ReplaceAllString(res, fmt.Sprintf("<Directory %s>", newDocRoot))
		return res, true
	}
	applyNginx := func() (string, bool) {
		if !nginxRootRegex.MatchString(vhostContent) {
			return "", false
		}
		replacement := fmt.Sprintf("    root %s;", newDocRoot)
		return nginxRootRegex.ReplaceAllString(vhostContent, replacement), true
	}

	if isApache {
		if res, ok := applyApache(); ok {
			return res, nil
		}
		if res, ok := applyNginx(); ok {
			return res, nil
		}
	} else {
		if res, ok := applyNginx(); ok {
			return res, nil
		}
		if res, ok := applyApache(); ok {
			return res, nil
		}
	}

	return "", fmt.Errorf("could not locate root or DocumentRoot directive in virtual host")
}

// GetSiteConfigFile retrieves the remote virtual host configuration file and its contents.
func (s *Service) GetSiteConfigFile(ctx context.Context, siteID string) (*SiteConfigFileResult, error) {
	siteRecord, err := s.Database.GetSite(ctx, siteID)
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

	contentBytes, err := client.ReadFile(ctx, vhostPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read virtual host file %s: %w", vhostPath, err)
	}

	webserverType := "nginx"
	if isApache {
		webserverType = "apache"
	} else if isHybridWebServer(siteRecord.WebServer) {
		webserverType = "nginx" // front proxy for hybrid stacks
	}

	return &SiteConfigFileResult{
		SiteID:        siteID,
		Domain:        siteRecord.Domain,
		FilePath:      vhostPath,
		Content:       string(contentBytes),
		WebServerType: webserverType,
	}, nil
}

// SaveSiteConfigFile saves the modified virtual host configuration with pre-flight tests and rollback.
func (s *Service) SaveSiteConfigFile(ctx context.Context, siteID string, content string) error {
	siteRecord, err := s.Database.GetSite(ctx, siteID)
	if err != nil || siteRecord == nil {
		return fmt.Errorf("site not found: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, siteRecord.ServerID)
	if err != nil {
		return err
	}

	vhostPath, isApache, err := s.resolveVhost(ctx, client, siteRecord)
	if err != nil {
		return err
	}

	// Read current content before overwriting to save in backup history.
	if currentBytes, readErr := client.ReadFile(ctx, vhostPath); readErr == nil && len(currentBytes) > 0 {
		hash := fmt.Sprintf("%x", sha256.Sum256(currentBytes))
		backupRecord := &db.SiteConfigBackupRecord{
			ID:          uuid.New().String(),
			ServerID:    siteRecord.ServerID,
			SiteID:      siteID,
			FilePath:    vhostPath,
			ContentHash: hash,
			Content:     string(currentBytes),
			CreatedAt:   time.Now(),
		}
		_ = s.Database.SaveSiteConfigBackup(ctx, backupRecord)
	}

	testCmd, reloadCmd := testAndReloadCmds(isApache)
	res, err := client.SafeWriteWithTest(ctx, vhostPath, []byte(content), testCmd, reloadCmd)
	if err != nil {
		return fmt.Errorf("configuration syntax test failed (restored to backup %s): %s", res.BackupPath, res.ErrorMessage)
	}

	siteRecord.UpdatedAt = time.Now()
	return s.Database.SaveSite(ctx, siteRecord)
}

// ListSiteConfigBackups returns version history of configuration files for a site.
func (s *Service) ListSiteConfigBackups(ctx context.Context, siteID string) ([]db.SiteConfigBackupRecord, error) {
	return s.Database.ListSiteConfigBackups(ctx, siteID, 50)
}

// RollbackSiteConfigFile restores a previous virtual host configuration from backup history.
func (s *Service) RollbackSiteConfigFile(ctx context.Context, backupID string) error {
	backupRecord, err := s.Database.GetSiteConfigBackup(ctx, backupID)
	if err != nil || backupRecord == nil {
		return fmt.Errorf("backup record %s not found: %w", backupID, err)
	}

	return s.SaveSiteConfigFile(ctx, backupRecord.SiteID, backupRecord.Content)
}

// UpdateSiteSettings updates DocumentRoot, aliases, and reflects changes in configuration.
func (s *Service) UpdateSiteSettings(ctx context.Context, siteID string, documentRoot string, aliases []string) error {
	siteRecord, err := s.Database.GetSite(ctx, siteID)
	if err != nil || siteRecord == nil {
		return fmt.Errorf("site not found: %w", err)
	}

	documentRoot = strings.TrimSpace(documentRoot)
	if documentRoot == "" {
		return fmt.Errorf("document root cannot be empty")
	}

	client, _, err := s.GetSSHClient(ctx, siteRecord.ServerID)
	if err != nil {
		return err
	}

	// Ensure target directory exists on server.
	_ = client.EnsureDirectory(ctx, documentRoot, 0755, "www-data:www-data")

	vhostPath, isApache, err := s.resolveVhost(ctx, client, siteRecord)
	if err == nil {
		contentBytes, err := client.ReadFile(ctx, vhostPath)
		if err == nil {
			newContent, errDoc := UpdateDocumentRootInConfig(string(contentBytes), documentRoot, isApache)
			if errDoc == nil {
				testCmd, reloadCmd := testAndReloadCmds(isApache)
				if _, writeErr := client.SafeWriteWithTest(ctx, vhostPath, []byte(newContent), testCmd, reloadCmd); writeErr != nil {
					return fmt.Errorf("failed to update document root in virtual host: %w", writeErr)
				}
			} else {
				return errDoc
			}
		} else {
			return fmt.Errorf("failed to read virtual host %s: %w", vhostPath, err)
		}
	} else {
		return err
	}

	siteRecord.DocumentRoot = documentRoot
	siteRecord.Aliases = aliases
	siteRecord.UpdatedAt = time.Now()
	return s.Database.SaveSite(ctx, siteRecord)
}

// ToggleSiteStatus enables or disables the virtual host with rollback on test failure.
func (s *Service) ToggleSiteStatus(ctx context.Context, siteID string, enable bool) error {
	siteRecord, err := s.Database.GetSite(ctx, siteID)
	if err != nil || siteRecord == nil {
		return fmt.Errorf("site not found: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, siteRecord.ServerID)
	if err != nil {
		return err
	}

	domain := siteRecord.Domain
	vhostPath, isApache, err := s.resolveVhost(ctx, client, siteRecord)
	if err != nil {
		return err
	}

	testCmd, reloadCmd := testAndReloadCmds(isApache)

	if isApache {
		siteName := apacheSiteName(vhostPath, domain)
		if strings.Contains(vhostPath, "/httpd/conf.d/") {
			disabledPath := vhostPath + ".disabled"
			if enable {
				if client.FileExists(ctx, disabledPath) {
					if _, stderr, code, err := client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(disabledPath), ssh.ShellEscape(vhostPath))).WithSudo().Exec(ctx); err != nil || code != 0 {
						return fmt.Errorf("failed to enable httpd vhost: %s %v", stderr, err)
					}
				}
			} else {
				if _, stderr, code, err := client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(vhostPath), ssh.ShellEscape(disabledPath))).WithSudo().Exec(ctx); err != nil || code != 0 {
					return fmt.Errorf("failed to disable httpd vhost: %s %v", stderr, err)
				}
			}
			if _, stderrTest, codeTest, errTest := client.NewCommand(testCmd).WithSudo().Exec(ctx); errTest != nil || codeTest != 0 {
				if enable {
					_, _, _, _ = client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(vhostPath), ssh.ShellEscape(disabledPath))).WithSudo().Exec(ctx)
				} else {
					_, _, _, _ = client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(disabledPath), ssh.ShellEscape(vhostPath))).WithSudo().Exec(ctx)
				}
				return fmt.Errorf("apache syntax test failed; state rolled back: %s", stderrTest)
			}
		} else {
			action := "a2ensite"
			rollbackAction := "a2dissite"
			if !enable {
				action = "a2dissite"
				rollbackAction = "a2ensite"
			}
			if _, stderr, code, err := client.NewCommand(fmt.Sprintf("%s %s", action, ssh.ShellEscape(siteName))).WithSudo().Exec(ctx); err != nil || code != 0 {
				return fmt.Errorf("%s failed: %s %v", action, stderr, err)
			}
			if _, stderrTest, codeTest, errTest := client.NewCommand(testCmd).WithSudo().Exec(ctx); errTest != nil || codeTest != 0 {
				_, _, _, _ = client.NewCommand(fmt.Sprintf("%s %s", rollbackAction, ssh.ShellEscape(siteName))).WithSudo().Exec(ctx)
				return fmt.Errorf("apache syntax test failed; state rolled back: %s", stderrTest)
			}
		}
		if _, stderr, code, err := client.NewCommand(reloadCmd).WithSudo().Exec(ctx); err != nil || code != 0 {
			return fmt.Errorf("apache reload failed: %s %v", stderr, err)
		}
	} else if strings.Contains(vhostPath, "/nginx/conf.d/") {
		disabledPath := vhostPath + ".disabled"
		if enable {
			if client.FileExists(ctx, disabledPath) {
				if _, stderr, code, err := client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(disabledPath), ssh.ShellEscape(vhostPath))).WithSudo().Exec(ctx); err != nil || code != 0 {
					return fmt.Errorf("failed to enable nginx conf.d vhost: %s %v", stderr, err)
				}
			}
		} else {
			if _, stderr, code, err := client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(vhostPath), ssh.ShellEscape(disabledPath))).WithSudo().Exec(ctx); err != nil || code != 0 {
				return fmt.Errorf("failed to disable nginx conf.d vhost: %s %v", stderr, err)
			}
		}
		if _, stderrTest, codeTest, errTest := client.NewCommand(testCmd).WithSudo().Exec(ctx); errTest != nil || codeTest != 0 {
			if enable {
				_, _, _, _ = client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(vhostPath), ssh.ShellEscape(disabledPath))).WithSudo().Exec(ctx)
			} else {
				_, _, _, _ = client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(disabledPath), ssh.ShellEscape(vhostPath))).WithSudo().Exec(ctx)
			}
			return fmt.Errorf("nginx syntax test failed; rolled back: %s", stderrTest)
		}
		if _, stderr, code, err := client.NewCommand(reloadCmd).WithSudo().Exec(ctx); err != nil || code != 0 {
			return fmt.Errorf("nginx reload failed: %s %v", stderr, err)
		}
	} else {
		enabledPath := nginxEnabledPath(vhostPath, domain)
		sourcePath := vhostPath
		if strings.Contains(vhostPath, "sites-enabled") {
			avail := strings.Replace(vhostPath, "sites-enabled", "sites-available", 1)
			if client.FileExists(ctx, avail) {
				sourcePath = avail
			}
		}
		if enable {
			if _, stderr, code, err := client.NewCommand(fmt.Sprintf("ln -sfn %s %s", ssh.ShellEscape(sourcePath), ssh.ShellEscape(enabledPath))).WithSudo().Exec(ctx); err != nil || code != 0 {
				return fmt.Errorf("failed to enable site: %s %v", stderr, err)
			}
		} else {
			if _, stderr, code, err := client.NewCommand(fmt.Sprintf("rm -f %s", ssh.ShellEscape(enabledPath))).WithSudo().Exec(ctx); err != nil || code != 0 {
				return fmt.Errorf("failed to disable site: %s %v", stderr, err)
			}
		}
		if _, stderrTest, codeTest, errTest := client.NewCommand(testCmd).WithSudo().Exec(ctx); errTest != nil || codeTest != 0 {
			if enable {
				_, _, _, _ = client.NewCommand(fmt.Sprintf("rm -f %s", ssh.ShellEscape(enabledPath))).WithSudo().Exec(ctx)
			} else {
				_, _, _, _ = client.NewCommand(fmt.Sprintf("ln -sfn %s %s", ssh.ShellEscape(sourcePath), ssh.ShellEscape(enabledPath))).WithSudo().Exec(ctx)
			}
			return fmt.Errorf("nginx syntax test failed; state rolled back: %s", stderrTest)
		}
		if _, stderr, code, err := client.NewCommand(reloadCmd).WithSudo().Exec(ctx); err != nil || code != 0 {
			return fmt.Errorf("nginx reload failed: %s %v", stderr, err)
		}
	}

	if enable {
		siteRecord.Status = "active"
	} else {
		siteRecord.Status = "disabled"
	}
	siteRecord.UpdatedAt = time.Now()
	return s.Database.SaveSite(ctx, siteRecord)
}
