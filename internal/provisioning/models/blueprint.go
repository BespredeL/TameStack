package models

import "time"

// Profile identifiers
const (
	ProfileMinimalPHP = "minimal_php"
	ProfileFullPHP    = "full_php"
	ProfileApachePHP  = "apache_php"
	ProfileLaravel    = "laravel"
	ProfileWordPress  = "wordpress"
	ProfileDockerHost = "docker_host"
	ProfileCustom     = "custom"
)

// WebserverSpec describes desired web server configuration.
type WebserverSpec struct {
	Type        string `json:"type"`         // "nginx", "apache", "both", "none"
	EnableHTTP2 bool   `json:"enable_http2"` // default true
}

// VersionSpec describes a specific PHP version to install.
type VersionSpec struct {
	Version    string   `json:"version"` // "8.1", "8.2", "8.3", "8.4"
	Handler    string   `json:"handler"` // "fpm", "mod_php", "proxy_fcgi"
	Extensions []string `json:"extensions"`
	IsDefault  bool     `json:"is_default"`
}

// PHPSpec describes desired PHP runtimes.
type PHPSpec struct {
	Versions []VersionSpec `json:"versions"`
}

// DatabaseSpec describes desired database engine.
type DatabaseSpec struct {
	Type     string `json:"type"`     // "mariadb", "mysql", "postgres", "none"
	RootPass string `json:"root_pass,omitempty"`
}

// CacheSpec describes caching service.
type CacheSpec struct {
	Type string `json:"type"` // "redis", "none"
}

// SecuritySpec describes firewall and hardening.
type SecuritySpec struct {
	Firewall     string `json:"firewall"` // "ufw", "firewalld", "none"
	Fail2ban     bool   `json:"fail2ban"`
	SSHHarden    bool   `json:"ssh_harden"`
	DisableRoot  bool   `json:"disable_root"`
	AllowSSHPort int    `json:"allow_ssh_port"` // current SSH port to keep open
}

// SSLSpec describes SSL certificate automation.
type SSLSpec struct {
	Provider string `json:"provider"` // "certbot", "none"
	Email    string `json:"email,omitempty"`
}

// ToolsSpec describes CLI runtime tools.
type ToolsSpec struct {
	Composer bool `json:"composer"`
	NodeJS   bool `json:"nodejs"`
	Git      bool `json:"git"`
}

// ContainersSpec describes container runtime.
type ContainersSpec struct {
	Docker  bool `json:"docker"`
	Compose bool `json:"compose"`
}

// Blueprint is the complete declarative server configuration.
type Blueprint struct {
	Profile    string         `json:"profile"`
	Webserver  WebserverSpec  `json:"webserver"`
	PHP        PHPSpec        `json:"php"`
	Database   DatabaseSpec   `json:"database"`
	Cache      CacheSpec      `json:"cache"`
	Security   SecuritySpec   `json:"security"`
	SSL        SSLSpec        `json:"ssl"`
	Tools      ToolsSpec      `json:"tools"`
	Containers ContainersSpec `json:"containers"`
}

// PlanItem describes a single concrete action in the provisioning plan.
type PlanItem struct {
	Category    string `json:"category"` // "package", "user", "dir", "service", "firewall", "config"
	Action      string `json:"action"`   // "install", "create", "enable", "allow", "configure"
	Target      string `json:"target"`
	Description string `json:"description"`
	Skip        bool   `json:"skip"` // true if already in desired state (idempotent)
}

// ProvisioningPlan represents the pre-flight plan shown to the user.
type ProvisioningPlan struct {
	ServerID           string     `json:"server_id"`
	Profile            string     `json:"profile"`
	OSDistro           string     `json:"os_distro"`
	OSFamily           string     `json:"os_family"`
	OSVersion          string     `json:"os_version"`
	Arch               string     `json:"arch"`
	Items              []PlanItem `json:"items"`
	PackagesToInstall  []string   `json:"packages_to_install"`
	RepositoriesNeeded []string   `json:"repositories_needed"`
	ServicesToStart    []string   `json:"services_to_start"`
	PortsToOpen        []int      `json:"ports_to_open"`
	SSHPortPreserved   int        `json:"ssh_port_preserved"`
	PotentialConflicts []string   `json:"potential_conflicts"`
	Warnings           []string   `json:"warnings"`
	EstimatedTimeMin   int        `json:"estimated_time_min"`
	IsCompatible       bool       `json:"is_compatible"`
}

// VerificationCheck records the outcome of a post-provisioning sanity check.
type VerificationCheck struct {
	Component string `json:"component"`
	Passed    bool   `json:"passed"`
	Message   string `json:"message"`
}

// ProvisioningResult summarizes the outcome of the executor run.
type ProvisioningResult struct {
	Success      bool                `json:"success"`
	Profile      string              `json:"profile"`
	TotalSteps   int                 `json:"total_steps"`
	AppliedSteps int                 `json:"applied_steps"`
	SkippedSteps int                 `json:"skipped_steps"`
	DurationSec  int                 `json:"duration_sec"`
	Checks       []VerificationCheck `json:"checks"`
	Errors       []string            `json:"errors,omitempty"`
	FinishedAt   time.Time           `json:"finished_at"`
}
