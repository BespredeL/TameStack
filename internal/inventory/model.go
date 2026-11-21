package inventory

import (
	"time"
)

type SystemUnit struct {
	Unit        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
}

type ListeningSocket struct {
	Proto   string `json:"proto"`
	Local   string `json:"local"`
	Port    int    `json:"port"`
	Process string `json:"process"`
	Pid     int    `json:"pid"`
}

type OSUser struct {
	Username string `json:"username"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	Home     string `json:"home"`
	Shell    string `json:"shell"`
	IsSystem bool   `json:"is_system"`
}

type MountPoint struct {
	Filesystem string `json:"filesystem"`
	Mount      string `json:"mount"`
	Type       string `json:"type"`
	TotalBytes int64  `json:"total_bytes"`
	FreeBytes  int64  `json:"free_bytes"`
}

type CronEntry struct {
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	User     string `json:"user"`
	Source   string `json:"source"`
}

type ContainerInfo struct {
	ID     string `json:"id"`
	Names  string `json:"names"`
	Image  string `json:"image"`
	Status string `json:"status"`
	Ports  string `json:"ports"`
}

type VerifiedLink struct {
	SiteDomain string `json:"site_domain"`
	VhostPath  string `json:"vhost_path"`
	FPMSocket  string `json:"fpm_socket"`
	PoolName   string `json:"pool_name"`
	CertPath   string `json:"cert_path"`
	Status     string `json:"status"` // "proven", "unproven"
	Note       string `json:"note"`
}

// DatabaseFacts records an installed DBMS and the logical databases found in it.
type DatabaseFacts struct {
	Engine    string   `json:"engine"`
	Version   string   `json:"version"`
	Databases []string `json:"databases"`
}

// CurrentUserDetail captures the authenticated SSH user's privileges.
type CurrentUserDetail struct {
	Username string   `json:"username"`
	UID      int      `json:"uid"`
	GID      int      `json:"gid"`
	Groups   []string `json:"groups"`
	HasSudo  bool     `json:"has_sudo"`
}

// OSInfo provides detailed operating system facts.
type OSInfo struct {
	Distro         string            `json:"distro"`
	Version        string            `json:"version"`
	Codename       string            `json:"codename"`
	Architecture   string            `json:"architecture"`
	Kernel         string            `json:"kernel"`
	InitSystem     string            `json:"init_system"`
	PackageManager string            `json:"package_manager"`
	CurrentUser    CurrentUserDetail `json:"current_user"`
}

// WebServerInstance represents a single detected web server installation/process.
type WebServerInstance struct {
	ID               string   `json:"id"`
	Engine           string   `json:"engine"` // "nginx", "apache2", "caddy", "openresty", "litespeed"
	Name             string   `json:"name"`
	BinaryPath       string   `json:"binary_path"`
	Version          string   `json:"version"`
	InstallType      string   `json:"install_type"` // "source", "apt", "rpm", "docker", "unknown"
	Status           string   `json:"status"`       // "active", "inactive", "failed", "stopped"
	PIDs             []int    `json:"pids"`
	ListeningPorts   []int    `json:"listening_ports"`
	ListeningSockets []string `json:"listening_sockets"`
	ConfigPath       string   `json:"config_path"`
	PrefixDir        string   `json:"prefix_dir"`
	VhostIncludes    []string `json:"vhost_includes"`
	SystemdUnit      string   `json:"systemd_unit"`
	SystemdOverrides []string `json:"systemd_overrides"`
	Modules          []string `json:"modules"`
	IsManageable     bool     `json:"is_manageable"`
	TestCommand      string   `json:"test_command"`
	ReloadCommand    string   `json:"reload_command"`
	Limitations      []string `json:"limitations"`
}

// FpmPoolFact holds runtime PHP-FPM pool details.
type FpmPoolFact struct {
	PoolName    string `json:"pool_name"`
	Listen      string `json:"listen"`
	User        string `json:"user"`
	Group       string `json:"group"`
	PmMode      string `json:"pm_mode"`
	MaxChildren int    `json:"max_children"`
	ConfigPath  string `json:"config_path"`
}

// PHPRuntimeInstance represents an installed PHP binary, FPM daemon, or Apache mod_php.
type PHPRuntimeInstance struct {
	ID           string        `json:"id"`
	Version      string        `json:"version"` // e.g. "8.1.20" or "8.1"
	MajorMinor   string        `json:"major_minor"`
	Kind         string        `json:"kind"` // "cli", "fpm", "mod_php", "cgi", "docker", "orphaned_unit"
	BinaryPath   string        `json:"binary_path"`
	Status       string        `json:"status"` // "active", "inactive", "failed", "orphaned"
	IsOrphaned   bool          `json:"is_orphaned"`
	IsManageable bool          `json:"is_manageable"`
	Sockets      []string      `json:"sockets"`
	TCPListeners []string      `json:"tcp_listeners"`
	ConfigDir    string        `json:"config_dir"`
	IniPath      string        `json:"ini_path"`
	SystemdUnit  string        `json:"systemd_unit"`
	Modules      []string      `json:"modules"`
	ActivePools  []FpmPoolFact `json:"active_pools"`
	Limitations  []string      `json:"limitations"`
}

// DatabaseInstance represents a discovered database engine on the system.
type DatabaseInstance struct {
	Engine      string   `json:"engine"` // "mysql", "mariadb", "postgresql", "redis", "mongodb"
	Version     string   `json:"version"`
	BinaryPath  string   `json:"binary_path"`
	ConfigPath  string   `json:"config_path"`
	Status      string   `json:"status"`
	Ports       []int    `json:"ports"`
	Sockets     []string `json:"sockets"`
	SystemdUnit string   `json:"systemd_unit"`
	InstallType string   `json:"install_type"`
}

// RepoDiagnostic checks an APT source line or entry.
type RepoDiagnostic struct {
	URL              string   `json:"url"`
	Codename         string   `json:"codename"`
	Component        string   `json:"component"`
	FilePath         string   `json:"file_path"`
	IsMismatch       bool     `json:"is_mismatch"`
	ExpectedCodename string   `json:"expected_codename"`
	MissingKeys      []string `json:"missing_keys"`
	IsExpired        bool     `json:"is_expired"`
	Status           string   `json:"status"` // "ok", "warning", "error"
	Message          string   `json:"message"`
}

// APTDiagnostics summarizes repository and package manager health.
type APTDiagnostics struct {
	Sources            []RepoDiagnostic `json:"sources"`
	MissingGPGKeys     []string         `json:"missing_gpg_keys"`
	CodenameMismatches []string         `json:"codename_mismatches"`
	BrokenPackages     []string         `json:"broken_packages"`
	DuplicateSources   []string         `json:"duplicate_sources"`
	HasErrors          bool             `json:"has_errors"`
	Summary            string           `json:"summary"`
}

// SiteTopologyChain details the full end-to-end request route for a site.
type SiteTopologyChain struct {
	Domain              string   `json:"domain"`
	VhostPath           string   `json:"vhost_path"`
	WebServerInstanceID string   `json:"web_server_instance_id"`
	WebServerEngine     string   `json:"web_server_engine"`
	ListenPorts         []int    `json:"listen_ports"`
	SSLEnabled          bool     `json:"ssl_enabled"`
	SSLCertPath         string   `json:"ssl_cert_path"`
	BackendType         string   `json:"backend_type"` // "direct", "reverse_proxy", "mod_php", "fastcgi_fpm", "static", "unknown"
	BackendTarget       string   `json:"backend_target"`
	PHPVersion          string   `json:"php_version"`
	PHPRuntimeID        string   `json:"php_runtime_id"`
	Confidence          string   `json:"confidence"` // "proven", "inferred", "unproven"
	Warnings            []string `json:"warnings"`
}

// ServerTopology groups all correlated infrastructure views.
type ServerTopology struct {
	OS          OSInfo               `json:"os"`
	WebServers  []WebServerInstance  `json:"web_servers"`
	PHPRuntimes []PHPRuntimeInstance `json:"php_runtimes"`
	Databases   []DatabaseInstance   `json:"databases"`
	APT         APTDiagnostics       `json:"apt"`
	SiteChains  []SiteTopologyChain  `json:"site_chains"`
}

type ServerInventorySnapshot struct {
	ID          string                `json:"id"`
	ServerID    string                `json:"server_id"`
	Units       []SystemUnit          `json:"units"`
	Sockets     []ListeningSocket     `json:"sockets"`
	Users       []OSUser              `json:"users"`
	Mounts      []MountPoint          `json:"mounts"`
	CronJobs    []CronEntry           `json:"cron_jobs"`
	Containers  []ContainerInfo       `json:"containers"`
	Databases   []DatabaseFacts       `json:"databases"`
	Links       []VerifiedLink        `json:"links"`
	OS          OSInfo                `json:"os"`
	WebServers  []WebServerInstance   `json:"web_servers"`
	PHPRuntimes []PHPRuntimeInstance  `json:"php_runtimes"`
	DBInstances []DatabaseInstance    `json:"db_instances"`
	APT         APTDiagnostics        `json:"apt"`
	SiteChains  []SiteTopologyChain   `json:"site_chains"`
	CreatedAt   time.Time             `json:"created_at"`
}
