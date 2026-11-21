package apps

// AppFrameworkType represents a detected application framework.
type AppFrameworkType string

const (
	FrameworkLaravel   AppFrameworkType = "laravel"
	FrameworkWordPress AppFrameworkType = "wordpress"
	FrameworkSymfony   AppFrameworkType = "symfony"
	FrameworkNodeJS    AppFrameworkType = "nodejs"
	FrameworkPython    AppFrameworkType = "python"
	FrameworkGolang    AppFrameworkType = "golang"
	FrameworkUnknown   AppFrameworkType = "unknown"
)

// AppIssue describes an operational or security issue detected in the app.
type AppIssue struct {
	Severity  string `json:"severity"` // "error", "warning", "info"
	Code      string `json:"code"`     // e.g. "storage_permissions", "missing_app_key"
	Message   string `json:"message"`
	Fixable   bool   `json:"fixable"`
	FixAction string `json:"fix_action,omitempty"`
}

// AppRuntimeInfo describes installed languages/runtimes on the server.
type AppRuntimeInfo struct {
	NodeInstalled   bool   `json:"node_installed"`
	NodeVersion     string `json:"node_version,omitempty"`
	NPMInstalled    bool   `json:"npm_installed"`
	NPMVersion      string `json:"npm_version,omitempty"`
	PythonInstalled bool   `json:"python_installed"`
	PythonVersion   string `json:"python_version,omitempty"`
	GoInstalled     bool   `json:"go_installed"`
	GoVersion       string `json:"go_version,omitempty"`
}

// AppInspectResult is the comprehensive inspection report for a website application.
type AppInspectResult struct {
	SiteID                 string           `json:"site_id"`
	Domain                 string           `json:"domain"`
	Framework              AppFrameworkType `json:"framework"`
	FrameworkVersion       string           `json:"framework_version,omitempty"`
	RequiredPHP            string           `json:"required_php,omitempty"`
	HasArtisan             bool             `json:"has_artisan"`
	HasComposer            bool             `json:"has_composer"`
	HasWpConfig            bool             `json:"has_wp_config"`
	HasConsole             bool             `json:"has_console"`
	HasPackageJSON         bool             `json:"has_package_json"`
	StorageWritable        bool             `json:"storage_writable"`
	BootstrapCacheWritable bool             `json:"bootstrap_cache_writable"`
	HasEnvFile             bool             `json:"has_env_file"`
	HasAppKey              bool             `json:"has_app_key"`
	SchedulerActive        bool             `json:"scheduler_active"`
	QueueWorkerStatus      string           `json:"queue_worker_status"` // "running", "stopped", "not_installed"
	QueueWorkerUnit        string           `json:"queue_worker_unit,omitempty"`
	Issues                 []AppIssue       `json:"issues"`
	AvailableLogs          []string         `json:"available_logs"`
	Runtimes               AppRuntimeInfo   `json:"runtimes"`
}

// ArtisanCommandRequest carries an artisan command to execute.
type ArtisanCommandRequest struct {
	SiteID           string `json:"site_id"`
	Command          string `json:"command"`
	ConfirmDangerous bool   `json:"confirm_dangerous"`
}

// ArtisanCommandResult contains execution output.
type ArtisanCommandResult struct {
	Command    string `json:"command"`
	Output     string `json:"output"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
}

// EnvFileResult contains .env content.
type EnvFileResult struct {
	SiteID  string `json:"site_id"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// SaveEnvRequest payload for writing .env.
type SaveEnvRequest struct {
	SiteID  string `json:"site_id"`
	Content string `json:"content"`
}

// QueueWorkerConfig specifies queue worker configuration.
type QueueWorkerConfig struct {
	SiteID     string `json:"site_id"`
	Connection string `json:"connection,omitempty"`
	Queue      string `json:"queue,omitempty"`
	Sleep      int    `json:"sleep,omitempty"`
	Tries      int    `json:"tries,omitempty"`
	Timeout    int    `json:"timeout,omitempty"`
}
