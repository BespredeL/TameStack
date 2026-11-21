package site

// SiteInput contains creation and configuration parameters for a website.
type SiteInput struct {
	ServerID     string   `json:"server_id"`
	Domain       string   `json:"domain"`
	Aliases      []string `json:"aliases"`
	DocumentRoot string   `json:"document_root"`
	SiteType     string   `json:"site_type"` // "php", "laravel", "wordpress", "static", "reverse_proxy"
	PHPVersion   string   `json:"php_version"`
	FPMPool      string   `json:"fpm_pool"`
	WebServer    string   `json:"web_server"` // "nginx", "apache", "nginx+apache"
	ProxyPassURL string   `json:"proxy_pass_url"`
	SSLEnabled   bool     `json:"ssl_enabled"`

	// Stage 4
	CreateDatabase   bool   `json:"create_database"`
	DatabaseName     string `json:"database_name"`
	DatabaseUser     string `json:"database_user"`
	DatabasePassword string `json:"database_password"`
	EnableLogs       bool   `json:"enable_logs"`
	DisableLogs      bool   `json:"disable_logs"`
	RedirectHTTPS    bool   `json:"redirect_https"`

	// Stage 5
	HTTP2Supported bool `json:"http2_supported"`
}

// SiteDNSCheckResult holds DNS resolution findings.
type SiteDNSCheckResult struct {
	Domain      string   `json:"domain"`
	ResolvedIPs []string `json:"resolved_ips"`
	ServerIP    string   `json:"server_ip"`
	Match       bool     `json:"match"`
	Message     string   `json:"message"`
}

// CreateSitePlan details planned files, directories, and pre-flight checks.
type CreateSitePlan struct {
	Domain            string   `json:"domain"`
	ServerHost        string   `json:"server_host"`
	DocumentRoot      string   `json:"document_root"`
	ConfigFile        string   `json:"config_file"`
	AccessLogFile     string   `json:"access_log_file"`
	ErrorLogFile      string   `json:"error_log_file"`
	PHPVersion        string   `json:"php_version"`
	FPMSocket         string   `json:"fpm_socket"`
	WebserverToReload string   `json:"webserver_to_reload"`
	DNSMatches        bool     `json:"dns_matches"`
	PreflightWarnings []string `json:"preflight_warnings"`

	// Stage 4
	OwnerUser            string   `json:"owner_user"` // e.g. "www-data:www-data"
	PackagesToInstall    []string `json:"packages_to_install"`
	ExistingSiteConflict bool     `json:"existing_site_conflict"`
	ConflictMessage      string   `json:"conflict_message"`
	DatabasePlanned      bool     `json:"database_planned"`
	DatabaseName         string   `json:"database_name"`
	DatabaseUser         string   `json:"database_user"`
}

// SiteConfigFileResult contains remote virtual host configuration file details.
type SiteConfigFileResult struct {
	SiteID        string `json:"site_id"`
	Domain        string `json:"domain"`
	FilePath      string `json:"file_path"`
	Content       string `json:"content"`
	WebServerType string `json:"webserver_type"` // "nginx" or "apache"
}

// SiteDeletePlan represents the pre-deletion plan and warnings.
type SiteDeletePlan struct {
	SiteID         string `json:"site_id"`
	Domain         string `json:"domain"`
	Origin         string `json:"origin"`
	WebServer      string `json:"web_server"`
	DocumentRoot   string `json:"document_root"`
	DeleteFiles    bool   `json:"delete_files"`
	IsDiscovered   bool   `json:"is_discovered"`
	WarningMessage string `json:"warning_message"`
	Description    string `json:"description"`
	FilesWarning   string `json:"files_warning"`
}

// SiteRuntimeInput contains parameters for switching a site's PHP runtime and pool.
type SiteRuntimeInput struct {
	SiteID             string `json:"site_id"`
	PHPVersion         string `json:"php_version"`
	Handler            string `json:"handler"` // "fpm", "mod_php", "proxy_fcgi"
	PoolName           string `json:"pool_name"`
	CustomListenSocket string `json:"custom_listen_socket"`
}

// SiteRuntimePlan previews runtime changes and compatibility before applying.
type SiteRuntimePlan struct {
	SiteID            string   `json:"site_id"`
	Domain            string   `json:"domain"`
	CurrentPHPVersion string   `json:"current_php_version"`
	TargetPHPVersion  string   `json:"target_php_version"`
	Handler           string   `json:"handler"`
	PoolName          string   `json:"pool_name"`
	TargetSocket      string   `json:"target_socket"`
	Webserver         string   `json:"webserver"`
	VhostFile         string   `json:"vhost_file"`
	IsCompatible      bool     `json:"is_compatible"`
	MissingExtensions []string `json:"missing_extensions"`
	PreflightWarnings []string `json:"preflight_warnings"`
	DiffPreview       string   `json:"diff_preview"`
}
