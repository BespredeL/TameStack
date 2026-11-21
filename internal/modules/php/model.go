package php

// PHPVersionInfo describes an installed or installable PHP release.
type PHPVersionInfo struct {
	Version           string             `json:"version"` // "8.1", "8.2", "8.3", "8.4"
	PatchVersion      string             `json:"patch_version"`
	UpdateAvailable   bool               `json:"update_available"`
	CandidateVersion  string             `json:"candidate_version"`
	IsInstalled       bool               `json:"is_installed"`
	IsAvailable       bool               `json:"is_available"`
	UnavailableReason string             `json:"unavailable_reason"`
	IsDefaultCLI      bool               `json:"is_default_cli"`
	DefaultCLIPath    string             `json:"default_cli_path,omitempty"`
	FPMStatus         string             `json:"fpm_status"` // "active", "inactive", "failed", "not_installed"
	FPMService        string             `json:"fpm_service"`
	SocketPath        string             `json:"socket_path"`
	IniPath           string             `json:"ini_path"`
	AvailableHandlers []string           `json:"available_handlers"` // ["fpm", "cli", "mod_php", "proxy_fcgi"]
	Extensions        []PHPExtensionInfo `json:"extensions"`
}

// PHPExtensionInfo represents a specific PHP extension.
type PHPExtensionInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	IsInstalled bool   `json:"is_installed"`
	IsEnabled   bool   `json:"is_enabled"`
	IsCore      bool   `json:"is_core"`
	Category    string `json:"category,omitempty"` // "Database", "Security", "String", "Image", "Cache", "Core"
	Description string `json:"description"`
}

// FpmPoolConfig represents a PHP-FPM worker pool configuration.
type FpmPoolConfig struct {
	PoolName           string            `json:"pool_name"`
	IsEnabled          bool              `json:"is_enabled"`
	User               string            `json:"user"`
	Group              string            `json:"group"`
	Listen             string            `json:"listen"` // socket or port
	ListenOwner        string            `json:"listen_owner,omitempty"`
	ListenGroup        string            `json:"listen_group,omitempty"`
	ListenMode         string            `json:"listen_mode,omitempty"`
	PmMode             string            `json:"pm_mode"` // dynamic, ondemand, static
	MaxChildren        int               `json:"max_children"`
	StartServers       int               `json:"start_servers"`
	MinSpareServers    int               `json:"min_spare_servers"`
	MaxSpareServers    int               `json:"max_spare_servers"`
	PmMaxRequests      int               `json:"pm_max_requests,omitempty"`
	ProcessIdleTimeout string            `json:"process_idle_timeout,omitempty"`
	Slowlog            bool              `json:"slowlog"`
	SlowlogTimeout     string            `json:"slowlog_timeout,omitempty"`
	AccessLog          string            `json:"access_log,omitempty"`
	MemoryLimit        string            `json:"memory_limit,omitempty"`
	MaxExecutionTime   int               `json:"max_execution_time,omitempty"`
	UploadMaxFilesize  string            `json:"upload_max_filesize,omitempty"`
	PostMaxSize        string            `json:"post_max_size,omitempty"`
	PhpValues          map[string]string `json:"php_values,omitempty"`
	PhpAdminValues     map[string]string `json:"php_admin_values,omitempty"`
}

// CreatePoolInput contains parameters for creating or updating an isolated PHP-FPM pool.
type CreatePoolInput struct {
	ServerID           string            `json:"server_id"`
	Version            string            `json:"version"`   // e.g. "8.2", "8.3"
	PoolName           string            `json:"pool_name"` // e.g. "mysite"
	IsEnabled          bool              `json:"is_enabled"`
	User               string            `json:"user"`  // e.g. "www-data"
	Group              string            `json:"group"` // e.g. "www-data"
	ListenSocket       string            `json:"listen_socket"`
	ListenOwner        string            `json:"listen_owner,omitempty"`
	ListenGroup        string            `json:"listen_group,omitempty"`
	ListenMode         string            `json:"listen_mode,omitempty"`
	PmMode             string            `json:"pm_mode"` // "dynamic", "ondemand", "static"
	MaxChildren        int               `json:"max_children"`
	StartServers       int               `json:"start_servers"`
	MinSpareServers    int               `json:"min_spare_servers"`
	MaxSpareServers    int               `json:"max_spare_servers"`
	PmMaxRequests      int               `json:"pm_max_requests,omitempty"`
	ProcessIdleTimeout string            `json:"process_idle_timeout,omitempty"`
	MemoryLimit        string            `json:"memory_limit,omitempty"`
	MaxExecutionTime   int               `json:"max_execution_time,omitempty"`
	UploadMaxFilesize  string            `json:"upload_max_filesize,omitempty"`
	PostMaxSize        string            `json:"post_max_size,omitempty"`
	Slowlog            bool              `json:"slowlog"`
	SlowlogTimeout     string            `json:"slowlog_timeout,omitempty"`
	AccessLog          string            `json:"access_log,omitempty"`
	PhpValues          map[string]string `json:"php_values,omitempty"`
	PhpAdminValues     map[string]string `json:"php_admin_values,omitempty"`
}

// PHPSettings holds structured directives from php.ini.
type PHPSettings struct {
	UploadMaxFilesize       string `json:"upload_max_filesize"`
	PostMaxSize             string `json:"post_max_size"`
	MemoryLimit             string `json:"memory_limit"`
	MaxExecutionTime        int    `json:"max_execution_time"`
	MaxInputTime            int    `json:"max_input_time"`
	MaxInputVars            int    `json:"max_input_vars"`
	DateTimezone            string `json:"date_timezone"`
	ErrorReporting          string `json:"error_reporting"`
	DisplayErrors           bool   `json:"display_errors"`
	LogErrors               bool   `json:"log_errors"`
	OpcacheEnable           bool   `json:"opcache_enable"`
	OpcacheMemory           int    `json:"opcache_memory_consumption"`
	OpcacheMaxFiles         int    `json:"opcache_max_accelerated_files"`
	OpcacheRevalidateFreq   int    `json:"opcache_revalidate_freq"`
	ShortOpenTag            bool   `json:"short_open_tag"`
	ExposePHP               bool   `json:"expose_php"`
}

// PHPIniBackupItem describes a timestamped backup of php.ini.
type PHPIniBackupItem struct {
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	Timestamp int64     `json:"timestamp"`
	CreatedAt string    `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
}

// PHPAppRequirement checks required vs missing extensions for an application type.
type PHPAppRequirement struct {
	AppType             string   `json:"app_type"` // "laravel", "wordpress", "drupal", "symfony"
	IsCompatible        bool     `json:"is_compatible"`
	RequiredExtensions  []string `json:"required_extensions"`
	MissingExtensions   []string `json:"missing_extensions"`
	InstalledExtensions []string `json:"installed_extensions"`
	Recommendations     []string `json:"recommendations"`
}

// FPMServiceOverview provides detailed service status and runtime metrics.
type FPMServiceOverview struct {
	Version      string   `json:"version"`
	ServiceName  string   `json:"service_name"`
	Status       string   `json:"status"` // "active", "inactive", "failed"
	MainPID      int      `json:"main_pid"`
	ActivePools  int      `json:"active_pools"`
	TotalWorkers int      `json:"total_workers"`
	Sockets      []string `json:"sockets"`
	ConfigFile   string   `json:"config_file"`
	ConfigValid  bool     `json:"config_valid"`
	ConfigOutput string   `json:"config_output"`
}

// InstallPHPPlan details packages, repositories, and compatibility checks before installation.
type InstallPHPPlan struct {
	TargetVersion       string   `json:"target_version"`
	OSFamily            string   `json:"os_family"`
	OSDistro            string   `json:"os_distro"`
	OSCompatible        bool     `json:"os_compatible"`
	RepositoriesNeeded  []string `json:"repositories_needed"`
	PackagesToInstall   []string `json:"packages_to_install"`
	FPMServiceToStart   string   `json:"fpm_service_to_start"`
	EstimatedDownloadMB int      `json:"estimated_download_mb"`
	Message             string   `json:"message"`
}

// StandardExtensions is the curated list of common extensions.
var StandardExtensions = []struct {
	Name        string
	Description string
}{
	{"curl", "cURL HTTP client library"},
	{"mbstring", "Multibyte string support for UTF-8"},
	{"mysql", "MySQL & MariaDB PDO driver"},
	{"xml", "XML parsing & DOM tools"},
	{"zip", "ZIP archive handling"},
	{"gd", "GD image generation and resizing"},
	{"intl", "Internationalization functions (ICU)"},
	{"imagick", "ImageMagick advanced image processing"},
	{"redis", "High-performance Redis client driver"},
	{"igbinary", "Compact binary serializer"},
	{"soap", "SOAP protocol client and server"},
	{"imap", "IMAP, POP3, and NNTP email library"},
	{"opcache", "Zend bytecode opcode cache for speed"},
	{"bcmath", "Arbitrary precision mathematics"},
	{"sockets", "Low-level network sockets API"},
	{"fileinfo", "MIME type and file signature detection"},
}
