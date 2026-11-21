package context

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/db"
)

// ServerSnapshot represents summarized context of a single server.
type ServerSnapshot struct {
	Server       *db.ServerRecord
	Diagnostics  *db.DiagnosticsRecord
	Sites        []*db.SiteRecord
	Certificates []*db.SSLCertificateRecord
}

// ServerCollector gathers and formats context about a server from the local database.
type ServerCollector struct {
	database  *db.DB
	sanitizer *Sanitizer
}

// NewServerCollector creates a new context collector.
func NewServerCollector(database *db.DB, sanitizer *Sanitizer) *ServerCollector {
	if sanitizer == nil {
		sanitizer = NewSanitizer()
	}
	return &ServerCollector{
		database:  database,
		sanitizer: sanitizer,
	}
}

// CollectServerSnapshot loads full metadata for a server.
func (c *ServerCollector) CollectServerSnapshot(ctx context.Context, serverID string) (*ServerSnapshot, error) {
	srv, err := c.database.GetServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch server: %w", err)
	}
	if srv == nil {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	diag, _ := c.database.GetDiagnostics(ctx, serverID)
	sites, _ := c.database.ListSites(ctx, serverID)
	certs, _ := c.database.ListSSLCertificates(ctx, serverID)

	return &ServerSnapshot{
		Server:       srv,
		Diagnostics:  diag,
		Sites:        sites,
		Certificates: certs,
	}, nil
}

// FormatContextMarkdown formats the server snapshot into a clear prompt section in Russian (default).
func (c *ServerCollector) FormatContextMarkdown(snapshot *ServerSnapshot) string {
	return c.FormatContextMarkdownLang(snapshot, "ru")
}

// FormatContextMarkdownLang formats the server snapshot in the specified language ("ru" or "en").
func (c *ServerCollector) FormatContextMarkdownLang(snapshot *ServerSnapshot, lang string) string {
	if snapshot == nil || snapshot.Server == nil {
		return ""
	}

	isEN := strings.ToLower(strings.TrimSpace(lang)) == "en"
	var sb strings.Builder

	if isEN {
		sb.WriteString(fmt.Sprintf("### Active Server: %s (%s:%d)\n", snapshot.Server.Name, snapshot.Server.Host, snapshot.Server.Port))
		sb.WriteString(fmt.Sprintf("- **User**: %s | **Group**: %s | **Status**: %s\n", snapshot.Server.Username, snapshot.Server.GroupName, snapshot.Server.Status))
	} else {
		sb.WriteString(fmt.Sprintf("### Активный сервер: %s (%s:%d)\n", snapshot.Server.Name, snapshot.Server.Host, snapshot.Server.Port))
		sb.WriteString(fmt.Sprintf("- **Пользователь**: %s | **Группа**: %s | **Статус**: %s\n", snapshot.Server.Username, snapshot.Server.GroupName, snapshot.Server.Status))
	}

	if snapshot.Diagnostics != nil {
		d := snapshot.Diagnostics
		ramGB := float64(d.RAMTotalBytes) / (1024 * 1024 * 1024)
		diskTotalGB := float64(d.DiskTotalBytes) / (1024 * 1024 * 1024)
		diskUsedGB := float64(d.DiskUsedBytes) / (1024 * 1024 * 1024)

		if isEN {
			sb.WriteString(fmt.Sprintf("- **OS**: %s %s (%s, %s, kernel: %s)\n", d.OSName, d.OSVersion, d.OSFamily, d.Arch, d.KernelVersion))
			sb.WriteString(fmt.Sprintf("- **Resources**: CPU: %d cores (%s) | RAM: %.1f GB | Disk: %.1f/%.1f GB\n", d.CPUCores, d.CPUModel, ramGB, diskUsedGB, diskTotalGB))
			if len(d.InstalledWebServers) > 0 {
				sb.WriteString(fmt.Sprintf("- **Web Servers**: %s\n", strings.Join(d.InstalledWebServers, ", ")))
			}
			if len(d.InstalledPHPVersions) > 0 {
				sb.WriteString(fmt.Sprintf("- **PHP Versions**: %s\n", strings.Join(d.InstalledPHPVersions, ", ")))
			}
			if len(d.InstalledDBVersions) > 0 {
				sb.WriteString(fmt.Sprintf("- **Databases**: %s\n", strings.Join(d.InstalledDBVersions, ", ")))
			}
			if d.DockerVersion != "" {
				sb.WriteString(fmt.Sprintf("- **Docker**: %s\n", d.DockerVersion))
			}
		} else {
			sb.WriteString(fmt.Sprintf("- **ОС**: %s %s (%s, %s, ядро: %s)\n", d.OSName, d.OSVersion, d.OSFamily, d.Arch, d.KernelVersion))
			sb.WriteString(fmt.Sprintf("- **Ресурсы**: CPU: %d cores (%s) | RAM: %.1f GB | Диск: %.1f/%.1f GB\n", d.CPUCores, d.CPUModel, ramGB, diskUsedGB, diskTotalGB))
			if len(d.InstalledWebServers) > 0 {
				sb.WriteString(fmt.Sprintf("- **Веб-серверы**: %s\n", strings.Join(d.InstalledWebServers, ", ")))
			}
			if len(d.InstalledPHPVersions) > 0 {
				sb.WriteString(fmt.Sprintf("- **Версии PHP**: %s\n", strings.Join(d.InstalledPHPVersions, ", ")))
			}
			if len(d.InstalledDBVersions) > 0 {
				sb.WriteString(fmt.Sprintf("- **Базы данных**: %s\n", strings.Join(d.InstalledDBVersions, ", ")))
			}
			if d.DockerVersion != "" {
				sb.WriteString(fmt.Sprintf("- **Docker**: %s\n", d.DockerVersion))
			}
		}
	}

	if len(snapshot.Sites) > 0 {
		if isEN {
			sb.WriteString(fmt.Sprintf("\n#### Hosted Sites (%d):\n", len(snapshot.Sites)))
			for _, s := range snapshot.Sites {
				sslStatus := "without SSL"
				if s.SSLEnabled {
					sslStatus = "SSL enabled (" + s.SSLProvider + ")"
				}
				sb.WriteString(fmt.Sprintf("- **%s** (%s, root: %s, php: %s, %s)\n", s.Domain, s.WebServer, s.DocumentRoot, s.PHPVersion, sslStatus))
			}
		} else {
			sb.WriteString(fmt.Sprintf("\n#### Размещенные сайты (%d):\n", len(snapshot.Sites)))
			for _, s := range snapshot.Sites {
				sslStatus := "без SSL"
				if s.SSLEnabled {
					sslStatus = "SSL включен (" + s.SSLProvider + ")"
				}
				sb.WriteString(fmt.Sprintf("- **%s** (%s, root: %s, php: %s, %s)\n", s.Domain, s.WebServer, s.DocumentRoot, s.PHPVersion, sslStatus))
			}
		}
	}

	if len(snapshot.Certificates) > 0 {
		if isEN {
			sb.WriteString(fmt.Sprintf("\n#### SSL Certificates (%d):\n", len(snapshot.Certificates)))
			for _, cert := range snapshot.Certificates {
				sb.WriteString(fmt.Sprintf("- %s: %d days remaining (status: %s)\n", cert.PrimaryDomain, cert.DaysRemaining, cert.Status))
			}
		} else {
			sb.WriteString(fmt.Sprintf("\n#### SSL сертификаты (%d):\n", len(snapshot.Certificates)))
			for _, cert := range snapshot.Certificates {
				sb.WriteString(fmt.Sprintf("- %s: остаток %d дн. (статус: %s)\n", cert.PrimaryDomain, cert.DaysRemaining, cert.Status))
			}
		}
	}

	return c.sanitizer.Sanitize(sb.String())
}
