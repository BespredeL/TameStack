package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

var (
	wpVersionRegex = regexp.MustCompile(`\$wp_version\s*=\s*'([^']+)'`)
	appKeyRegex    = regexp.MustCompile(`(?m)^APP_KEY=([^\s]+)`)
)

// ResolveProjectRoot returns the parent directory if DocumentRoot points to /public, /public_html, /web or /html.
func ResolveProjectRoot(docRoot string) string {
	clean := filepath.ToSlash(filepath.Clean(docRoot))
	suffixes := []string{"/public", "/public_html", "/web", "/html"}
	for _, s := range suffixes {
		if strings.HasSuffix(clean, s) {
			return strings.TrimSuffix(clean, s)
		}
	}
	return clean
}

type composerJSON struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
	Extra      struct {
		Laravel struct {
			DontDiscover []string `json:"dont-discover"`
		} `json:"laravel"`
	} `json:"extra"`
}

// ParseComposerJSON analyzes composer.json contents.
func ParseComposerJSON(data []byte) (version string, reqPHP string, isLaravel bool, isSymfony bool) {
	var c composerJSON
	if err := json.Unmarshal(data, &c); err != nil {
		return "", "", false, false
	}

	if php, ok := c.Require["php"]; ok {
		reqPHP = php
	}

	if laravelVer, ok := c.Require["laravel/framework"]; ok {
		isLaravel = true
		version = laravelVer
	} else if _, ok := c.Require["laravel/lumen-framework"]; ok {
		isLaravel = true
		version = c.Require["laravel/lumen-framework"]
	}

	if symfonyVer, ok := c.Require["symfony/framework-bundle"]; ok {
		isSymfony = true
		if version == "" {
			version = symfonyVer
		}
	}

	return version, reqPHP, isLaravel, isSymfony
}

// DetectWordPress inspects wp-config.php and wp-includes/version.php.
func DetectWordPress(ctx context.Context, client *ssh.SSHClient, docRoot string) (bool, string) {
	wpConfig := filepath.ToSlash(filepath.Join(docRoot, "wp-config.php"))
	if !client.FileExists(ctx, wpConfig) {
		// check one level above.
		parentWpConfig := filepath.ToSlash(filepath.Join(filepath.Dir(docRoot), "wp-config.php"))
		if !client.FileExists(ctx, parentWpConfig) {
			return false, ""
		}
	}

	versionFile := filepath.ToSlash(filepath.Join(docRoot, "wp-includes", "version.php"))
	out, err := client.ReadFile(ctx, versionFile)
	if err == nil {
		matches := wpVersionRegex.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			return true, matches[1]
		}
	}

	return true, "unknown"
}

// CheckStoragePermissions validates writable access to storage and bootstrap/cache.
func CheckStoragePermissions(ctx context.Context, client *ssh.SSHClient, projectRoot string) (storageOk, cacheOk bool) {
	storageDir := filepath.ToSlash(filepath.Join(projectRoot, "storage"))
	cacheDir := filepath.ToSlash(filepath.Join(projectRoot, "bootstrap", "cache"))

	checkDir := func(dir string) bool {
		cmd := fmt.Sprintf("[ -d %s ] && [ -w %s ] && echo 'ok' || echo 'no'", ssh.ShellEscape(dir), ssh.ShellEscape(dir))
		out, _, _, _ := client.NewCommand(cmd).WithSudo().Exec(ctx)
		return strings.Contains(string(out), "ok")
	}

	return checkDir(storageDir), checkDir(cacheDir)
}

// CheckEnvAppKey tests if .env exists and has a non-empty APP_KEY.
func CheckEnvAppKey(ctx context.Context, client *ssh.SSHClient, projectRoot string) (hasEnv, hasAppKey bool) {
	envFile := filepath.ToSlash(filepath.Join(projectRoot, ".env"))
	if !client.FileExists(ctx, envFile) {
		return false, false
	}
	out, err := client.ReadFile(ctx, envFile)
	if err != nil {
		return true, false
	}
	matches := appKeyRegex.FindStringSubmatch(string(out))
	if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" && strings.TrimSpace(matches[1]) != "base64:" {
		return true, true
	}
	return true, false
}

// CheckRuntimes probes node, npm, python3 and go binaries without installing.
func CheckRuntimes(ctx context.Context, client *ssh.SSHClient) AppRuntimeInfo {
	info := AppRuntimeInfo{}

	runCmd := func(cmd string) string {
		out, _, code, _ := client.Exec(ctx, cmd)
		if code == 0 {
			return strings.TrimSpace(string(out))
		}
		return ""
	}

	if nodeOut := runCmd("node -v 2>/dev/null"); nodeOut != "" {
		info.NodeInstalled = true
		info.NodeVersion = strings.TrimPrefix(nodeOut, "v")
	}
	if npmOut := runCmd("npm -v 2>/dev/null"); npmOut != "" {
		info.NPMInstalled = true
		info.NPMVersion = npmOut
	}
	if pyOut := runCmd("python3 --version 2>/dev/null || python --version 2>/dev/null"); pyOut != "" {
		info.PythonInstalled = true
		info.PythonVersion = strings.TrimPrefix(strings.TrimPrefix(pyOut, "Python "), "python ")
	}
	if goOut := runCmd("go version 2>/dev/null"); goOut != "" {
		info.GoInstalled = true
		fields := strings.Fields(goOut)
		if len(fields) >= 3 {
			info.GoVersion = strings.TrimPrefix(fields[2], "go")
		} else {
			info.GoVersion = goOut
		}
	}

	return info
}

// InspectSiteApp conducts complete framework and health detection.
func InspectSiteApp(ctx context.Context, client *ssh.SSHClient, site *db.SiteRecord) (*AppInspectResult, error) {
	projectRoot := ResolveProjectRoot(site.DocumentRoot)

	res := &AppInspectResult{
		SiteID:            site.ID,
		Domain:            site.Domain,
		Framework:         FrameworkUnknown,
		Issues:            []AppIssue{},
		AvailableLogs:     []string{},
		QueueWorkerStatus: "not_installed",
	}

	res.Runtimes = CheckRuntimes(ctx, client)

	// WordPress: wp-config.php takes precedence over generic package.json.
	if isWP, wpVer := DetectWordPress(ctx, client, site.DocumentRoot); isWP {
		res.Framework = FrameworkWordPress
		res.HasWpConfig = true
		res.FrameworkVersion = wpVer
	}

	composerPath := filepath.ToSlash(filepath.Join(projectRoot, "composer.json"))
	res.HasComposer = client.FileExists(ctx, composerPath)

	artisanPath := filepath.ToSlash(filepath.Join(projectRoot, "artisan"))
	res.HasArtisan = client.FileExists(ctx, artisanPath)

	consolePath := filepath.ToSlash(filepath.Join(projectRoot, "bin", "console"))
	res.HasConsole = client.FileExists(ctx, consolePath)

	pkgPath := filepath.ToSlash(filepath.Join(projectRoot, "package.json"))
	res.HasPackageJSON = client.FileExists(ctx, pkgPath)

	if res.HasComposer {
		if compBytes, err := client.ReadFile(ctx, composerPath); err == nil {
			version, reqPHP, isLaravel, isSymfony := ParseComposerJSON(compBytes)
			res.RequiredPHP = reqPHP

			// A composer.json with laravel/framework + artisan is a full Laravel app.
			if isLaravel && res.HasArtisan {
				res.Framework = FrameworkLaravel
				res.FrameworkVersion = version
			} else if isSymfony && res.HasConsole {
				res.Framework = FrameworkSymfony
				res.FrameworkVersion = version
			} else if isLaravel && !res.HasArtisan {
				// composer.json without artisan is partial / composer_only.
				res.Issues = append(res.Issues, AppIssue{
					Severity: "warning",
					Code:     "artisan_missing",
					Message:  "Файл composer.json содержит laravel/framework, но бинарник artisan не найден в корне проекта.",
				})
			}
		}
	} else if res.HasArtisan {
		res.Framework = FrameworkLaravel
	} else if res.HasConsole {
		res.Framework = FrameworkSymfony
	} else if res.HasPackageJSON && res.Framework == FrameworkUnknown {
		res.Framework = FrameworkNodeJS
	}

	if res.Framework == FrameworkLaravel {
		res.StorageWritable, res.BootstrapCacheWritable = CheckStoragePermissions(ctx, client, projectRoot)
		if !res.StorageWritable || !res.BootstrapCacheWritable {
			res.Issues = append(res.Issues, AppIssue{
				Severity:  "error",
				Code:      "storage_permissions",
				Message:   "Каталоги storage/ или bootstrap/cache/ не имеют прав на запись веб-сервером.",
				Fixable:   true,
				FixAction: "fix_permissions",
			})
		}

		res.HasEnvFile, res.HasAppKey = CheckEnvAppKey(ctx, client, projectRoot)
		if !res.HasEnvFile {
			res.Issues = append(res.Issues, AppIssue{
				Severity: "error",
				Code:     "missing_env",
				Message:  "Файл .env не найден в корне проекта. Скопируйте .env.example или настройте конфигурацию.",
			})
		} else if !res.HasAppKey {
			res.Issues = append(res.Issues, AppIssue{
				Severity:  "error",
				Code:      "missing_app_key",
				Message:   "Ключ шифрования APP_KEY отсутствует или пуст в файле .env.",
				Fixable:   true,
				FixAction: "key_generate",
			})
		}

		laravelLog := filepath.ToSlash(filepath.Join(projectRoot, "storage", "logs", "laravel.log"))
		if client.FileExists(ctx, laravelLog) {
			res.AvailableLogs = append(res.AvailableLogs, laravelLog)
		}

		outCron, _, _, _ := client.NewCommand("crontab -l 2>/dev/null").WithSudo().Exec(ctx)
		if strings.Contains(string(outCron), "schedule:run") && strings.Contains(string(outCron), projectRoot) {
			res.SchedulerActive = true
		}

		unitName := fmt.Sprintf("sh-queue-%s.service", site.ID[:8])
		res.QueueWorkerUnit = unitName
		statusCmd := fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'inactive'", unitName)
		outUnit, _, _, _ := client.NewCommand(statusCmd).WithSudo().Exec(ctx)
		uStatus := strings.TrimSpace(string(outUnit))
		if uStatus == "active" {
			res.QueueWorkerStatus = "running"
		} else if uStatus == "failed" || uStatus == "inactive" {
			// check if unit file exists.
			unitPath := fmt.Sprintf("/etc/systemd/system/%s", unitName)
			if client.FileExists(ctx, unitPath) {
				res.QueueWorkerStatus = "stopped"
			} else {
				res.QueueWorkerStatus = "not_installed"
			}
		}
	}

	return res, nil
}
