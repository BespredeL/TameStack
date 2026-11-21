package site

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"tamestack/internal/ssh"
)

//go:embed templates/* scripts/*
var siteFS embed.FS

// TemplateData holds prepared parameters for virtual host and placeholder templates.
type TemplateData struct {
	Domain         string
	ServerNames    string
	Aliases        []string
	AliasesString  string
	DocumentRoot   string
	SiteType       string
	PHPVersion     string
	FPMSocket      string
	ProxyPassURL   string
	SSLEnabled     bool
	EnableLogs     bool
	RedirectHTTPS  bool
	HTTP2Supported bool
}

// prepareTemplateData transforms SiteInput into sanitized TemplateData.
func prepareTemplateData(in SiteInput) TemplateData {
	serverNames := in.Domain
	if len(in.Aliases) > 0 {
		serverNames = serverNames + " " + strings.Join(in.Aliases, " ")
	}

	docRoot := in.DocumentRoot
	if in.SiteType == "laravel" {
		docRoot = strings.TrimSuffix(docRoot, "/") + "/public"
	}

	fpmSocket := in.FPMPool
	if fpmSocket == "" && in.PHPVersion != "" {
		fpmSocket = fmt.Sprintf("unix:/run/php/php%s-fpm.sock", in.PHPVersion)
	} else if fpmSocket != "" && !strings.HasPrefix(fpmSocket, "unix:") && !strings.Contains(fpmSocket, ":") {
		if strings.HasPrefix(fpmSocket, "/") {
			fpmSocket = "unix:" + fpmSocket
		} else {
			fpmSocket = fmt.Sprintf("unix:/run/php/php%s-fpm-%s.sock", in.PHPVersion, fpmSocket)
		}
	}

	proxyURL := in.ProxyPassURL
	if proxyURL == "" {
		proxyURL = "http://127.0.0.1:3000"
	}

	enableLogs := !in.DisableLogs
	redirectHTTPS := in.RedirectHTTPS && in.SSLEnabled

	return TemplateData{
		Domain:         in.Domain,
		ServerNames:    serverNames,
		Aliases:        in.Aliases,
		AliasesString:  strings.Join(in.Aliases, " "),
		DocumentRoot:   docRoot,
		SiteType:       in.SiteType,
		PHPVersion:     in.PHPVersion,
		FPMSocket:      fpmSocket,
		ProxyPassURL:   proxyURL,
		SSLEnabled:     in.SSLEnabled,
		EnableLogs:     enableLogs,
		RedirectHTTPS:  redirectHTTPS,
		HTTP2Supported: in.HTTP2Supported,
	}
}

// WebServerEngine encapsulates web server specific provisioning, syntax verification,
// activation, deactivation, and lifecycle management.
type WebServerEngine interface {
	Name() string
	BuildVirtualHost(in SiteInput) (string, error)
	InstallVirtualHost(ctx context.Context, client *ssh.SSHClient, in SiteInput) error
	EnableVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error
	DisableVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error
	TestConfig(ctx context.Context, client *ssh.SSHClient) (output string, code int, err error)
	Reload(ctx context.Context, client *ssh.SSHClient) error
	RemoveVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error
}

// EngineRegistry manages and resolves WebServerEngine strategies.
type EngineRegistry struct {
	engines map[string]WebServerEngine
}

// NewEngineRegistry initializes default engines for Nginx and Apache.
func NewEngineRegistry() *EngineRegistry {
	r := &EngineRegistry{
		engines: make(map[string]WebServerEngine),
	}
	r.Register(&NginxEngine{})
	r.Register(&ApacheEngine{})
	return r
}

// Register registers a web server engine.
func (r *EngineRegistry) Register(engine WebServerEngine) {
	r.engines[strings.ToLower(engine.Name())] = engine
}

// GetEngine resolves a single engine for create/plan. Hybrid stacks prefer nginx (front).
func (r *EngineRegistry) GetEngine(webServer string) (WebServerEngine, error) {
	ws := strings.ToLower(strings.TrimSpace(webServer))
	if isHybridWebServer(ws) {
		if eng, ok := r.engines["nginx"]; ok {
			return eng, nil
		}
	}
	if prefersApache(ws) {
		if eng, ok := r.engines["apache"]; ok {
			return eng, nil
		}
	}
	if eng, ok := r.engines["nginx"]; ok {
		return eng, nil
	}
	if eng, ok := r.engines["apache"]; ok {
		return eng, nil
	}
	return nil, fmt.Errorf("unsupported web server engine: %s", webServer)
}

// EnginesForSite returns every engine that owns a vhost for this site metadata.
func (r *EngineRegistry) EnginesForSite(webServer string) []WebServerEngine {
	ws := strings.ToLower(strings.TrimSpace(webServer))
	var out []WebServerEngine
	wantNginx := strings.Contains(ws, "nginx") || ws == "" || ws == "unknown" || (!strings.Contains(ws, "apache") && !strings.Contains(ws, "nginx"))
	wantApache := strings.Contains(ws, "apache")
	if wantNginx {
		if eng, ok := r.engines["nginx"]; ok {
			out = append(out, eng)
		}
	}
	if wantApache {
		if eng, ok := r.engines["apache"]; ok {
			out = append(out, eng)
		}
	}
	if len(out) == 0 {
		if eng, ok := r.engines["nginx"]; ok {
			out = append(out, eng)
		}
	}
	return out
}

// --- Nginx Engine Implementation ---

// NginxEngine implements WebServerEngine for Nginx.
type NginxEngine struct{}

func (e *NginxEngine) Name() string {
	return "nginx"
}

func (e *NginxEngine) BuildVirtualHost(in SiteInput) (string, error) {
	tmplContent, err := siteFS.ReadFile("templates/nginx_vhost.conf.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read nginx template: %w", err)
	}

	tmpl, err := template.New("nginx_vhost").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse nginx template: %w", err)
	}

	data := prepareTemplateData(in)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute nginx template: %w", err)
	}

	return buf.String(), nil
}

func (e *NginxEngine) InstallVirtualHost(ctx context.Context, client *ssh.SSHClient, in SiteInput) error {
	vhostConfig, err := e.BuildVirtualHost(in)
	if err != nil {
		return err
	}

	availPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", in.Domain)
	enablePath := fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", in.Domain)

	if err := client.EnsureDirectory(ctx, "/etc/nginx/sites-available", 0755, ""); err != nil {
		return err
	}
	if err := client.EnsureDirectory(ctx, "/etc/nginx/sites-enabled", 0755, ""); err != nil {
		return err
	}

	if err := client.WriteFile(ctx, availPath, []byte(vhostConfig), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", availPath, err)
	}

	symlinkCmd := fmt.Sprintf("ln -sfn %s %s", ssh.ShellEscape(availPath), ssh.ShellEscape(enablePath))
	out, stderr, code, err := client.NewCommand(symlinkCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to enable nginx site: %s %s (%w)", string(out), string(stderr), err)
	}

	return nil
}

func (e *NginxEngine) EnableVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error {
	availPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", domain)
	enablePath := fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", domain)
	symlinkCmd := fmt.Sprintf("ln -sfn %s %s", ssh.ShellEscape(availPath), ssh.ShellEscape(enablePath))
	_, _, _, err := client.NewCommand(symlinkCmd).WithSudo().Exec(ctx)
	return err
}

func (e *NginxEngine) DisableVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error {
	enablePath := fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", domain)
	return client.RemovePath(ctx, enablePath, false)
}

func (e *NginxEngine) TestConfig(ctx context.Context, client *ssh.SSHClient) (output string, code int, err error) {
	out, stderr, code, err := client.NewCommand("nginx -t 2>&1").WithSudo().Exec(ctx)
	combined := strings.TrimSpace(string(out) + "\n" + string(stderr))
	return combined, code, err
}

func (e *NginxEngine) Reload(ctx context.Context, client *ssh.SSHClient) error {
	_, _, _, err := client.NewCommand("systemctl reload nginx").WithSudo().Exec(ctx)
	return err
}

func (e *NginxEngine) RemoveVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error {
	cfgName := fmt.Sprintf("%s.conf", domain)
	cmd := fmt.Sprintf("rm -f /etc/nginx/sites-enabled/%s /etc/nginx/sites-available/%s /etc/nginx/conf.d/%s /etc/nginx/sites-enabled/%s /etc/nginx/sites-available/%s",
		ssh.ShellEscape(cfgName), ssh.ShellEscape(cfgName), ssh.ShellEscape(cfgName), ssh.ShellEscape(domain), ssh.ShellEscape(domain))
	_, _, _, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	_ = e.Reload(ctx, client)
	return err
}

// --- Apache Engine Implementation ---

// ApacheEngine implements WebServerEngine for Apache 2.4 / httpd.
type ApacheEngine struct{}

func (e *ApacheEngine) Name() string {
	return "apache"
}

func (e *ApacheEngine) BuildVirtualHost(in SiteInput) (string, error) {
	tmplContent, err := siteFS.ReadFile("templates/apache_vhost.conf.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read apache template: %w", err)
	}

	tmpl, err := template.New("apache_vhost").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse apache template: %w", err)
	}

	data := prepareTemplateData(in)
	if strings.HasPrefix(data.FPMSocket, "unix:") {
		data.FPMSocket = strings.TrimPrefix(data.FPMSocket, "unix:")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute apache template: %w", err)
	}

	return buf.String(), nil
}

func (e *ApacheEngine) isDebian(ctx context.Context, client *ssh.SSHClient) bool {
	out, _, _, _ := client.NewCommand("[ -d /etc/apache2/sites-available ] && echo 'debian' || echo 'rhel'").Exec(ctx)
	return strings.Contains(string(out), "debian")
}

func (e *ApacheEngine) InstallVirtualHost(ctx context.Context, client *ssh.SSHClient, in SiteInput) error {
	vhostConfig, err := e.BuildVirtualHost(in)
	if err != nil {
		return err
	}

	if e.isDebian(ctx, client) {
		availPath := fmt.Sprintf("/etc/apache2/sites-available/%s.conf", in.Domain)
		if err := client.EnsureDirectory(ctx, "/etc/apache2/sites-available", 0755, ""); err != nil {
			return err
		}
		if err := client.WriteFile(ctx, availPath, []byte(vhostConfig), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", availPath, err)
		}
		cmd := fmt.Sprintf("a2ensite %s.conf", ssh.ShellEscape(in.Domain))
		_, _, _, _ = client.NewCommand(cmd).WithSudo().Exec(ctx)
	} else {
		confdPath := fmt.Sprintf("/etc/httpd/conf.d/%s.conf", in.Domain)
		if err := client.EnsureDirectory(ctx, "/etc/httpd/conf.d", 0755, ""); err != nil {
			return err
		}
		if err := client.WriteFile(ctx, confdPath, []byte(vhostConfig), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", confdPath, err)
		}
	}

	return nil
}

func (e *ApacheEngine) EnableVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error {
	if e.isDebian(ctx, client) {
		cmd := fmt.Sprintf("a2ensite %s.conf", ssh.ShellEscape(domain))
		_, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
		if err != nil || code != 0 {
			return fmt.Errorf("a2ensite failed: %s %v", stderr, err)
		}
		return nil
	}
	disabled := fmt.Sprintf("/etc/httpd/conf.d/%s.conf.disabled", domain)
	enabled := fmt.Sprintf("/etc/httpd/conf.d/%s.conf", domain)
	if client.FileExists(ctx, disabled) {
		_, stderr, code, err := client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(disabled), ssh.ShellEscape(enabled))).WithSudo().Exec(ctx)
		if err != nil || code != 0 {
			return fmt.Errorf("failed to enable httpd vhost: %s %v", stderr, err)
		}
	}
	return nil
}

func (e *ApacheEngine) DisableVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error {
	if e.isDebian(ctx, client) {
		cmd := fmt.Sprintf("a2dissite %s.conf", ssh.ShellEscape(domain))
		_, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
		if err != nil || code != 0 {
			return fmt.Errorf("a2dissite failed: %s %v", stderr, err)
		}
		return nil
	}
	enabled := fmt.Sprintf("/etc/httpd/conf.d/%s.conf", domain)
	disabled := enabled + ".disabled"
	if client.FileExists(ctx, enabled) {
		_, stderr, code, err := client.NewCommand(fmt.Sprintf("mv %s %s", ssh.ShellEscape(enabled), ssh.ShellEscape(disabled))).WithSudo().Exec(ctx)
		if err != nil || code != 0 {
			return fmt.Errorf("failed to disable httpd vhost: %s %v", stderr, err)
		}
	}
	return nil
}

func (e *ApacheEngine) TestConfig(ctx context.Context, client *ssh.SSHClient) (output string, code int, err error) {
	cmd := "apache2ctl configtest 2>&1 || apachectl configtest 2>&1 || httpd -t 2>&1"
	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	combined := strings.TrimSpace(string(out) + "\n" + string(stderr))
	return combined, code, err
}

func (e *ApacheEngine) Reload(ctx context.Context, client *ssh.SSHClient) error {
	cmd := "systemctl reload apache2 2>/dev/null || systemctl reload httpd 2>/dev/null"
	_, _, _, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	return err
}

func (e *ApacheEngine) RemoveVirtualHost(ctx context.Context, client *ssh.SSHClient, domain string) error {
	cfgName := fmt.Sprintf("%s.conf", domain)
	_ = e.DisableVirtualHost(ctx, client, domain)
	cmd := fmt.Sprintf("rm -f /etc/apache2/sites-enabled/%s /etc/apache2/sites-available/%s /etc/apache2/sites-enabled/%s /etc/apache2/sites-available/%s /etc/httpd/conf.d/%s",
		ssh.ShellEscape(cfgName), ssh.ShellEscape(cfgName), ssh.ShellEscape(domain), ssh.ShellEscape(domain), ssh.ShellEscape(cfgName))
	_, _, _, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	_ = e.Reload(ctx, client)
	return err
}
