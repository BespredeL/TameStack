package security

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Number    int    `json:"number"`
	Port      string `json:"port"`
	Protocol  string `json:"protocol"` // "tcp", "udp", "any"
	Action    string `json:"action"`   // "ALLOW", "DENY", "REJECT", "LIMIT"
	FromIP    string `json:"from_ip"`
	Direction string `json:"direction"` // "IN", "OUT"
	Comment   string `json:"comment"`
}

// FirewallStatus is the firewall status and configuration.
type FirewallStatus struct {
	Type            string         `json:"type"`             // "ufw", "firewalld", "none"
	IsActive        bool           `json:"is_active"`        // true when enabled
	DefaultIncoming string         `json:"default_incoming"` // "deny", "allow", "reject"
	DefaultOutgoing string         `json:"default_outgoing"` // "allow", "deny"
	Rules           []FirewallRule `json:"rules"`
}

// AddFirewallRuleRequest is a request to add a firewall rule.
type AddFirewallRuleRequest struct {
	ServerID string `json:"server_id"`
	Port     string `json:"port"`     // e.g. "80", "443", "8000:8010"
	Protocol string `json:"protocol"` // "tcp", "udp", "any"
	Action   string `json:"action"`   // "ALLOW", "DENY", "REJECT", "LIMIT"
	FromIP   string `json:"from_ip"`  // default "any"
	Comment  string `json:"comment"`
}

// UpdateFirewallRuleRequest is a request to update an existing firewall rule.
type UpdateFirewallRuleRequest struct {
	ServerID string `json:"server_id"`
	RuleNum  int    `json:"rule_num"`
	OldPort  string `json:"old_port"`
	OldProto string `json:"old_proto"`
	Port     string `json:"port"`     // e.g. "80", "443", "8000:8010"
	Protocol string `json:"protocol"` // "tcp", "udp", "any"
	Action   string `json:"action"`   // "ALLOW", "DENY", "REJECT", "LIMIT"
	FromIP   string `json:"from_ip"`  // default "any"
	Comment  string `json:"comment"`
}

// Fail2banJail describes a fail2ban jail (both active runtime and configured).
type Fail2banJail struct {
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Port            string   `json:"port"`
	Filter          string   `json:"filter"`
	LogPath         string   `json:"log_path"`
	MaxRetry        int      `json:"max_retry"`
	FindTime        string   `json:"find_time"`
	BanTime         string   `json:"ban_time"`
	Action          string   `json:"action,omitempty"`
	IgnoreIP        string   `json:"ignoreip,omitempty"`
	CurrentlyFailed int      `json:"currently_failed"`
	TotalFailed     int      `json:"total_failed"`
	CurrentlyBanned int      `json:"currently_banned"`
	TotalBanned     int      `json:"total_banned"`
	BannedIPList    []string `json:"banned_ip_list"`
	ConfigFile      string   `json:"config_file,omitempty"`
	IsManaged       bool     `json:"is_managed"`
}

// Fail2banStatus is the overall status of the fail2ban service.
type Fail2banStatus struct {
	IsInstalled      bool           `json:"is_installed"`
	IsActive         bool           `json:"is_active"`
	Version          string         `json:"version"`
	Jails            []Fail2banJail `json:"jails"`
	AvailableFilters []string       `json:"available_filters,omitempty"`
}

// SaveFail2banJailRequest is a request to create or update a fail2ban jail.
type SaveFail2banJailRequest struct {
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Port     string `json:"port"`
	Filter   string `json:"filter"`
	LogPath  string `json:"log_path"`
	MaxRetry int    `json:"max_retry"`
	FindTime string `json:"find_time"`
	BanTime  string `json:"ban_time"`
	Action   string `json:"action,omitempty"`
	IgnoreIP string `json:"ignoreip,omitempty"`
}

// DeleteFail2banJailRequest is a request to delete a fail2ban jail.
type DeleteFail2banJailRequest struct {
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
}

// ToggleFail2banJailRequest is a request to enable or disable a fail2ban jail.
type ToggleFail2banJailRequest struct {
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
}

// UnbanIPRequest is a request to unban an IP address in fail2ban.
type UnbanIPRequest struct {
	ServerID string `json:"server_id"`
	Jail     string `json:"jail"`
	IP       string `json:"ip"`
}

// BanIPRequest is a request to manually ban an IP address in fail2ban.
type BanIPRequest struct {
	ServerID string `json:"server_id"`
	Jail     string `json:"jail"`
	IP       string `json:"ip"`
}

// ListeningPort describes an open listening port.
type ListeningPort struct {
	Protocol     string `json:"protocol"`      // "tcp", "udp"
	LocalAddress string `json:"local_address"` // e.g. "0.0.0.0", "127.0.0.1", "::"
	Port         int    `json:"port"`
	Process      string `json:"process"` // e.g. "nginx", "sshd", "mysqld"
	PID          int    `json:"pid"`
	IsPublic     bool   `json:"is_public"` // true when listening on 0.0.0.0 or ::
}

// SecurityCheckItem is a single security audit check item.
type SecurityCheckItem struct {
	Key            string `json:"key"`
	Title          string `json:"title"`
	Category       string `json:"category"` // "ssh", "firewall", "updates", "network", "services"
	Status         string `json:"status"`   // "pass", "warning", "danger"
	Details        string `json:"details"`
	Recommendation string `json:"recommendation"`
	CanAutoFix     bool   `json:"can_auto_fix"`
}

// SecurityAuditReport is the full server security audit report.
type SecurityAuditReport struct {
	ServerID             string              `json:"server_id"`
	Score                int                 `json:"score"` // 0-100
	SSHRuleRootLogin     string              `json:"ssh_rule_root_login"`
	SSHRulePasswordAuth  string              `json:"ssh_rule_password_auth"`
	SSHPort              int                 `json:"ssh_port"`
	PendingUpdatesCount  int                 `json:"pending_updates_count"`
	SecurityUpdatesCount int                 `json:"security_updates_count"`
	ListeningPorts       []ListeningPort     `json:"listening_ports"`
	Checks               []SecurityCheckItem `json:"checks"`
	LastAuditedAt        string              `json:"last_audited_at"`
}

// SiteEndpointCheck is the result of a site/endpoint availability check.
type SiteEndpointCheck struct {
	URL              string `json:"url"`
	StatusCode       int    `json:"status_code"`
	ResponseTimeMs   int64  `json:"response_time_ms"`
	IsUp             bool   `json:"is_up"`
	SSLDaysRemaining int    `json:"ssl_days_remaining"`
	ErrorMessage     string `json:"error_message"`
	CheckedAt        string `json:"checked_at"`
}

// RealtimeSystemMetrics is a live snapshot of the server's system metrics.
type RealtimeSystemMetrics struct {
	ServerID          string  `json:"server_id"`
	CPUPercent        float64 `json:"cpu_percent"`
	Load1             float64 `json:"load_1"`
	Load5             float64 `json:"load_5"`
	Load15            float64 `json:"load_15"`
	MemoryTotalMB     uint64  `json:"memory_total_mb"`
	MemoryUsedMB      uint64  `json:"memory_used_mb"`
	MemoryFreeMB      uint64  `json:"memory_free_mb"`
	MemoryUsedPercent float64 `json:"memory_used_percent"`
	SwapTotalMB       uint64  `json:"swap_total_mb"`
	SwapUsedMB        uint64  `json:"swap_used_mb"`
	DiskTotalGB       float64 `json:"disk_total_gb"`
	DiskUsedGB        float64 `json:"disk_used_gb"`
	DiskFreeGB        float64 `json:"disk_free_gb"`
	DiskUsedPercent   float64 `json:"disk_used_percent"`
	UptimeSeconds     uint64  `json:"uptime_seconds"`
	Timestamp         string  `json:"timestamp"`
}

const (
	ThreatClean      = "clean"
	ThreatSuspicious = "suspicious"
	ThreatMalicious  = "malicious"
)

// ScannedFile represents a file detected during security scan.
type ScannedFile struct {
	Path          string   `json:"path"`
	Size          int64    `json:"size"`
	FormattedSize string   `json:"formatted_size"`
	ModTime       string   `json:"mod_time"`
	Permissions   string   `json:"permissions"`
	Owner         string   `json:"owner"`
	ThreatLevel   string   `json:"threat_level"` // "clean", "suspicious", "malicious"
	Reasons       []string `json:"reasons"`
	Snippet       string   `json:"snippet"`
	IsExcluded    bool     `json:"is_excluded"`
}

// ScanFilesRequest is sent to inspect recently changed files.
type ScanFilesRequest struct {
	ServerID      string `json:"server_id"`
	BasePath      string `json:"base_path"`
	TimeRange     string `json:"time_range"` // "1h", "6h", "24h", "3d", "7d", "30d", "all"
	FileTypes     string `json:"file_types"` // "scripts", "configs", "all"
	DetectMalware bool   `json:"detect_malware"`
	MaxResults    int    `json:"max_results"`
}

// ScanFilesResult represents summary and detected files.
type ScanFilesResult struct {
	Files        []ScannedFile `json:"files"`
	ScannedCount int           `json:"scanned_count"`
	ThreatCount  int           `json:"threat_count"`
	DurationMs   int64         `json:"duration_ms"`
}

// FileSecurityExclusion is an excluded path or pattern.
type FileSecurityExclusion struct {
	ID        string `json:"id"`
	ServerID  string `json:"server_id"`
	Pattern   string `json:"pattern"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

// FileSecuritySnapshot stores baseline snapshot metadata.
type FileSecuritySnapshot struct {
	ID             string `json:"id"`
	ServerID       string `json:"server_id"`
	SiteID         string `json:"site_id"`
	Name           string `json:"name"`
	BasePath       string `json:"base_path"`
	FilesCount     int    `json:"files_count"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
	CreatedAt      string `json:"created_at"`
}

// SnapshotDiffResult represents differences between current files and baseline.
type SnapshotDiffResult struct {
	SnapshotID    string        `json:"snapshot_id"`
	SnapshotName  string        `json:"snapshot_name"`
	BasePath      string        `json:"base_path"`
	AddedFiles    []ScannedFile `json:"added_files"`
	ModifiedFiles []ScannedFile `json:"modified_files"`
	DeletedFiles  []string      `json:"deleted_files"`
	TotalAdded    int           `json:"total_added"`
	TotalModified int           `json:"total_modified"`
	TotalDeleted  int           `json:"total_deleted"`
	DurationMs    int64         `json:"duration_ms"`
}
