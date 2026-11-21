package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the sql.DB instance and provides helper repositories.
type DB struct {
	*sql.DB
	path string
}

// ServerRecord represents the stored server entity.
type ServerRecord struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	Host                   string     `json:"host"`
	Port                   int        `json:"port"`
	Username               string     `json:"username"`
	AuthType               string     `json:"auth_type"` // "password", "key", "key_passphrase"
	EncryptedSecret        string     `json:"-"`
	EncryptedSudoSecret    string     `json:"-"`
	EncryptedKeyPassphrase string     `json:"-"`
	SSHHostKey             string     `json:"ssh_host_key"`
	GroupName              string     `json:"group_name"`
	Tags                   []string   `json:"tags"`
	IsFavorite             bool       `json:"is_favorite"`
	Status                 string     `json:"status"` // "online", "offline", "unknown"
	Mode                   string     `json:"mode"`   // "audit", "management"
	LastConnectedAt        *time.Time `json:"last_connected_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// DiagnosticsRecord holds system telemetry.
type DiagnosticsRecord struct {
	ServerID             string    `json:"server_id"`
	OSName               string    `json:"os_name"`
	OSVersion            string    `json:"os_version"`
	OSFamily             string    `json:"os_family"`
	KernelVersion        string    `json:"kernel_version"`
	Arch                 string    `json:"arch"`
	CPUCores             int       `json:"cpu_cores"`
	CPUModel             string    `json:"cpu_model"`
	RAMTotalBytes        int64     `json:"ram_total_bytes"`
	DiskTotalBytes       int64     `json:"disk_total_bytes"`
	DiskUsedBytes        int64     `json:"disk_used_bytes"`
	UptimeSeconds        int64     `json:"uptime_seconds"`
	SSHVersion           string    `json:"ssh_version"`
	InstalledWebServers  []string  `json:"installed_webservers"`
	InstalledPHPVersions []string  `json:"installed_php_versions"`
	InstalledDBVersions  []string  `json:"installed_db_versions"`
	DockerVersion        string    `json:"docker_version"`
	DiagnosedAt          time.Time `json:"diagnosed_at"`
}

// TaskRecord represents an asynchronous job.
type TaskRecord struct {
	ID            string     `json:"id"`
	ServerID      string     `json:"server_id"`
	OperationType string     `json:"operation_type"`
	Title         string     `json:"title"`
	Status        string     `json:"status"` // "pending", "running", "completed", "failed", "cancelled"
	CurrentStep   string     `json:"current_step"`
	Progress      int        `json:"progress"` // 0..100
	Payload       string     `json:"payload"`
	Result        string     `json:"result"`
	ErrorMessage  string     `json:"error_message"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// TaskLogRecord is a single output line of a task.
type TaskLogRecord struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// SiteRecord represents a hosted website or virtual host.
type SiteRecord struct {
	ID           string    `json:"id"`
	ServerID     string    `json:"server_id"`
	Domain       string    `json:"domain"`
	Aliases      []string  `json:"aliases"`
	DocumentRoot string    `json:"document_root"`
	SiteType     string    `json:"site_type"` // "php", "laravel", "wordpress", "static", "reverse_proxy"
	PHPVersion   string    `json:"php_version"`
	FPMPool      string    `json:"fpm_pool"`
	WebServer    string    `json:"web_server"` // "nginx", "apache", "nginx+apache"
	SSLEnabled   bool      `json:"ssl_enabled"`
	SSLProvider  string    `json:"ssl_provider"` // "letsencrypt", "custom", "none"
	Status       string    `json:"status"`       // "active", "disabled"
	Origin       string    `json:"origin"`       // "managed", "discovered", "unknown"
	VhostPath    string    `json:"vhost_path"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SSLCertificateRecord represents an installed SSL certificate.
type SSLCertificateRecord struct {
	ID            string     `json:"id"`
	ServerID      string     `json:"server_id"`
	SiteID        string     `json:"site_id"`
	PrimaryDomain string     `json:"primary_domain"`
	SanDomains    []string   `json:"san_domains"`
	CertPath      string     `json:"cert_path"`
	KeyPath       string     `json:"key_path"`
	Issuer        string     `json:"issuer"`
	ValidFrom     *time.Time `json:"valid_from"`
	ValidTo       *time.Time `json:"valid_to"`
	DaysRemaining int        `json:"days_remaining"`
	AutoRenew     bool       `json:"auto_renew"`
	Status        string     `json:"status"` // "valid", "expiring_soon", "expired"
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// BackupRecord represents an archived backup archive.
type BackupRecord struct {
	ID           string     `json:"id"`
	ServerID     string     `json:"server_id"`
	BackupType   string     `json:"backup_type"` // "site", "database", "full", "config"
	TargetName   string     `json:"target_name"` // domain or db name
	Destination  string     `json:"destination"` // "local", "sftp", "s3"
	FilePath     string     `json:"file_path"`
	SizeBytes    int64      `json:"size_bytes"`
	Status       string     `json:"status"`
	Offsite      bool       `json:"offsite"`       // true if downloaded or transferred offsite
	DownloadedAt *time.Time `json:"downloaded_at"` // when downloaded to local computer
	Verified     bool       `json:"verified"`      // integrity check passed
	Checksum     string     `json:"checksum"`      // sha256 checksum
	CreatedAt    time.Time  `json:"created_at"`
}

// Open initializes and runs migrations on SQLite database.
func Open(dbPath string) (*DB, error) {
	// Create backup of db file before running migrations.
	if err := BackupDatabase(dbPath); err != nil {
		slog.Warn("Failed to create pre-migration db backup", slog.String("error", err.Error()))
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-64000)", dbPath)
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	rawDB.SetMaxOpenConns(1) // SQLite single-writer safety

	database := &DB{DB: rawDB, path: dbPath}
	if err := database.RunMigrations(); err != nil {
		rawDB.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	ok, err := database.CheckIntegrity()
	if err != nil || !ok {
		slog.Warn("SQLite integrity check warning", slog.Any("error", err), slog.Bool("ok", ok))
	}

	return database, nil
}

func (d *DB) migrate() error {
	return d.RunMigrations()
}

// --- Server Repository Methods ---

func (d *DB) ListServers(ctx context.Context) ([]*ServerRecord, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, name, host, port, username, auth_type, encrypted_secret, COALESCE(encrypted_sudo_secret, ''), COALESCE(encrypted_key_passphrase, ''), ssh_host_key,
		       group_name, tags, is_favorite, status, COALESCE(mode, 'audit'), last_connected_at, created_at, updated_at
		FROM servers ORDER BY is_favorite DESC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*ServerRecord
	for rows.Next() {
		s := &ServerRecord{}
		var tagsJSON string
		var isFavInt int
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.EncryptedSecret, &s.EncryptedSudoSecret, &s.EncryptedKeyPassphrase, &s.SSHHostKey,
			&s.GroupName, &tagsJSON, &isFavInt, &s.Status, &s.Mode, &s.LastConnectedAt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.IsFavorite = isFavInt == 1
		_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
		if s.Tags == nil {
			s.Tags = []string{}
		}
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

func (d *DB) GetServer(ctx context.Context, id string) (*ServerRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, name, host, port, username, auth_type, encrypted_secret, COALESCE(encrypted_sudo_secret, ''), COALESCE(encrypted_key_passphrase, ''), ssh_host_key,
		       group_name, tags, is_favorite, status, COALESCE(mode, 'audit'), last_connected_at, created_at, updated_at
		FROM servers WHERE id = ?
	`, id)

	s := &ServerRecord{}
	var tagsJSON string
	var isFavInt int
	if err := row.Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.EncryptedSecret, &s.EncryptedSudoSecret, &s.EncryptedKeyPassphrase, &s.SSHHostKey,
		&s.GroupName, &tagsJSON, &isFavInt, &s.Status, &s.Mode, &s.LastConnectedAt, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	s.IsFavorite = isFavInt == 1
	_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
	if s.Tags == nil {
		s.Tags = []string{}
	}
	return s, nil
}

func (d *DB) SaveServer(ctx context.Context, s *ServerRecord) error {
	tagsJSON, err := json.Marshal(s.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}
	isFavInt := 0
	if s.IsFavorite {
		isFavInt = 1
	}
	if s.Mode == "" {
		s.Mode = "audit"
	}

	_, err = d.ExecContext(ctx, `
		INSERT INTO servers (id, name, host, port, username, auth_type, encrypted_secret, encrypted_sudo_secret, encrypted_key_passphrase, ssh_host_key, group_name, tags, is_favorite, status, mode, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			host = excluded.host,
			port = excluded.port,
			username = excluded.username,
			auth_type = excluded.auth_type,
			encrypted_secret = excluded.encrypted_secret,
			encrypted_sudo_secret = excluded.encrypted_sudo_secret,
			encrypted_key_passphrase = excluded.encrypted_key_passphrase,
			ssh_host_key = excluded.ssh_host_key,
			group_name = excluded.group_name,
			tags = excluded.tags,
			is_favorite = excluded.is_favorite,
			status = excluded.status,
			mode = excluded.mode,
			updated_at = CURRENT_TIMESTAMP
	`, s.ID, s.Name, s.Host, s.Port, s.Username, s.AuthType, s.EncryptedSecret, s.EncryptedSudoSecret, s.EncryptedKeyPassphrase, s.SSHHostKey, s.GroupName, string(tagsJSON), isFavInt, s.Status, s.Mode)

	return err
}

func (d *DB) SetServerMode(ctx context.Context, serverID string, mode string) error {
	if mode != "audit" && mode != "management" {
		mode = "audit"
	}
	_, err := d.ExecContext(ctx, `UPDATE servers SET mode = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, mode, serverID)
	return err
}

func (d *DB) UpdateServerSudoSecret(ctx context.Context, id string, encSudoSecret string) error {
	_, err := d.ExecContext(ctx, `UPDATE servers SET encrypted_sudo_secret = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, encSudoSecret, id)
	return err
}

func (d *DB) DeleteServer(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	return err
}

func (d *DB) UpdateServerStatus(ctx context.Context, id string, status string) error {
	now := time.Now()
	_, err := d.ExecContext(ctx, `
		UPDATE servers SET status = ?, last_connected_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, status, now, id)
	return err
}

// --- Diagnostics Methods ---

func (d *DB) SaveDiagnostics(ctx context.Context, diag *DiagnosticsRecord) error {
	webJSON, _ := json.Marshal(diag.InstalledWebServers)
	phpJSON, _ := json.Marshal(diag.InstalledPHPVersions)
	dbJSON, _ := json.Marshal(diag.InstalledDBVersions)

	_, err := d.ExecContext(ctx, `
		INSERT INTO server_diagnostics (
			server_id, os_name, os_version, os_family, kernel_version, arch,
			cpu_cores, cpu_model, ram_total_bytes, disk_total_bytes, disk_used_bytes,
			uptime_seconds, ssh_version, installed_webservers, installed_php_versions,
			installed_db_versions, docker_version, diagnosed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(server_id) DO UPDATE SET
			os_name = excluded.os_name,
			os_version = excluded.os_version,
			os_family = excluded.os_family,
			kernel_version = excluded.kernel_version,
			arch = excluded.arch,
			cpu_cores = excluded.cpu_cores,
			cpu_model = excluded.cpu_model,
			ram_total_bytes = excluded.ram_total_bytes,
			disk_total_bytes = excluded.disk_total_bytes,
			disk_used_bytes = excluded.disk_used_bytes,
			uptime_seconds = excluded.uptime_seconds,
			ssh_version = excluded.ssh_version,
			installed_webservers = excluded.installed_webservers,
			installed_php_versions = excluded.installed_php_versions,
			installed_db_versions = excluded.installed_db_versions,
			docker_version = excluded.docker_version,
			diagnosed_at = CURRENT_TIMESTAMP
	`, diag.ServerID, diag.OSName, diag.OSVersion, diag.OSFamily, diag.KernelVersion, diag.Arch,
		diag.CPUCores, diag.CPUModel, diag.RAMTotalBytes, diag.DiskTotalBytes, diag.DiskUsedBytes,
		diag.UptimeSeconds, diag.SSHVersion, string(webJSON), string(phpJSON), string(dbJSON), diag.DockerVersion)

	return err
}

func (d *DB) GetDiagnostics(ctx context.Context, serverID string) (*DiagnosticsRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT server_id, os_name, os_version, os_family, kernel_version, arch,
		       cpu_cores, cpu_model, ram_total_bytes, disk_total_bytes, disk_used_bytes,
		       uptime_seconds, ssh_version, installed_webservers, installed_php_versions,
		       installed_db_versions, docker_version, diagnosed_at
		FROM server_diagnostics WHERE server_id = ?
	`, serverID)

	diag := &DiagnosticsRecord{}
	var webJSON, phpJSON, dbJSON string
	if err := row.Scan(
		&diag.ServerID, &diag.OSName, &diag.OSVersion, &diag.OSFamily, &diag.KernelVersion, &diag.Arch,
		&diag.CPUCores, &diag.CPUModel, &diag.RAMTotalBytes, &diag.DiskTotalBytes, &diag.DiskUsedBytes,
		&diag.UptimeSeconds, &diag.SSHVersion, &webJSON, &phpJSON, &dbJSON, &diag.DockerVersion, &diag.DiagnosedAt,
	); err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(webJSON), &diag.InstalledWebServers)
	_ = json.Unmarshal([]byte(phpJSON), &diag.InstalledPHPVersions)
	_ = json.Unmarshal([]byte(dbJSON), &diag.InstalledDBVersions)
	return diag, nil
}

// --- Task Repository Methods ---

func (d *DB) SaveTask(ctx context.Context, task *TaskRecord) error {
	var serverIDVal interface{}
	if task.ServerID != "" {
		serverIDVal = task.ServerID
	}

	_, err := d.ExecContext(ctx, `
		INSERT INTO tasks (id, server_id, operation_type, title, status, current_step, progress, payload, result, error_message, started_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			current_step = excluded.current_step,
			progress = excluded.progress,
			result = excluded.result,
			error_message = excluded.error_message,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at
	`, task.ID, serverIDVal, task.OperationType, task.Title, task.Status, task.CurrentStep, task.Progress,
		task.Payload, task.Result, task.ErrorMessage, task.StartedAt, task.CompletedAt, task.CreatedAt)
	return err
}

func (d *DB) GetTask(ctx context.Context, id string) (*TaskRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, COALESCE(server_id, ''), operation_type, title, status, current_step, progress, payload, result, error_message, started_at, completed_at, created_at
		FROM tasks WHERE id = ?
	`, id)

	t := &TaskRecord{}
	if err := row.Scan(&t.ID, &t.ServerID, &t.OperationType, &t.Title, &t.Status, &t.CurrentStep, &t.Progress,
		&t.Payload, &t.Result, &t.ErrorMessage, &t.StartedAt, &t.CompletedAt, &t.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (d *DB) ListTasks(ctx context.Context, limit int) ([]*TaskRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.QueryContext(ctx, `
		SELECT id, COALESCE(server_id, ''), operation_type, title, status, current_step, progress, payload, result, error_message, started_at, completed_at, created_at
		FROM tasks ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*TaskRecord
	for rows.Next() {
		t := &TaskRecord{}
		if err := rows.Scan(
			&t.ID, &t.ServerID, &t.OperationType, &t.Title, &t.Status, &t.CurrentStep, &t.Progress,
			&t.Payload, &t.Result, &t.ErrorMessage, &t.StartedAt, &t.CompletedAt, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (d *DB) AppendTaskLog(ctx context.Context, taskID string, level string, message string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO task_logs (task_id, level, message, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, taskID, level, message)
	return err
}

func (d *DB) GetTaskLogs(ctx context.Context, taskID string) ([]*TaskLogRecord, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, task_id, level, message, created_at FROM task_logs WHERE task_id = ? ORDER BY id ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*TaskLogRecord
	for rows.Next() {
		l := &TaskLogRecord{}
		if err := rows.Scan(&l.ID, &l.TaskID, &l.Level, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- Site Repository Methods ---

func (d *DB) SaveSite(ctx context.Context, site *SiteRecord) error {
	aliasesJSON, _ := json.Marshal(site.Aliases)
	sslInt := 0
	if site.SSLEnabled {
		sslInt = 1
	}
	if site.Origin == "" {
		site.Origin = "unknown"
	}

	_, err := d.ExecContext(ctx, `
		INSERT INTO sites (id, server_id, domain, aliases, document_root, site_type, php_version, fpm_pool, web_server, ssl_enabled, ssl_provider, status, origin, vhost_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			domain = excluded.domain,
			aliases = excluded.aliases,
			document_root = excluded.document_root,
			site_type = excluded.site_type,
			php_version = excluded.php_version,
			fpm_pool = excluded.fpm_pool,
			web_server = excluded.web_server,
			ssl_enabled = excluded.ssl_enabled,
			ssl_provider = excluded.ssl_provider,
			status = excluded.status,
			origin = COALESCE(NULLIF(excluded.origin, ''), sites.origin, 'unknown'),
			vhost_path = COALESCE(NULLIF(excluded.vhost_path, ''), sites.vhost_path, ''),
			updated_at = CURRENT_TIMESTAMP
	`, site.ID, site.ServerID, site.Domain, string(aliasesJSON), site.DocumentRoot, site.SiteType,
		site.PHPVersion, site.FPMPool, site.WebServer, sslInt, site.SSLProvider, site.Status, site.Origin, site.VhostPath)
	return err
}

func (d *DB) ListSites(ctx context.Context, serverID string) ([]*SiteRecord, error) {
	var rows *sql.Rows
	var err error
	if serverID != "" {
		rows, err = d.QueryContext(ctx, `
			SELECT id, server_id, domain, aliases, document_root, site_type, php_version, fpm_pool, web_server, ssl_enabled, ssl_provider, status, COALESCE(origin, 'unknown'), COALESCE(vhost_path, ''), created_at, updated_at
			FROM sites WHERE server_id = ? ORDER BY domain ASC
		`, serverID)
	} else {
		rows, err = d.QueryContext(ctx, `
			SELECT id, server_id, domain, aliases, document_root, site_type, php_version, fpm_pool, web_server, ssl_enabled, ssl_provider, status, COALESCE(origin, 'unknown'), COALESCE(vhost_path, ''), created_at, updated_at
			FROM sites ORDER BY domain ASC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []*SiteRecord
	for rows.Next() {
		s := &SiteRecord{}
		var aliasesJSON string
		var sslInt int
		if err := rows.Scan(
			&s.ID, &s.ServerID, &s.Domain, &aliasesJSON, &s.DocumentRoot, &s.SiteType,
			&s.PHPVersion, &s.FPMPool, &s.WebServer, &sslInt, &s.SSLProvider, &s.Status,
			&s.Origin, &s.VhostPath,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.SSLEnabled = sslInt == 1
		_ = json.Unmarshal([]byte(aliasesJSON), &s.Aliases)
		if s.Aliases == nil {
			s.Aliases = []string{}
		}
		sites = append(sites, s)
	}
	return sites, rows.Err()
}

func (d *DB) GetSite(ctx context.Context, id string) (*SiteRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, server_id, domain, aliases, document_root, site_type, php_version, fpm_pool, web_server, ssl_enabled, ssl_provider, status, COALESCE(origin, 'unknown'), COALESCE(vhost_path, ''), created_at, updated_at
		FROM sites WHERE id = ?
	`, id)

	s := &SiteRecord{}
	var aliasesJSON string
	var sslInt int
	if err := row.Scan(
		&s.ID, &s.ServerID, &s.Domain, &aliasesJSON, &s.DocumentRoot, &s.SiteType,
		&s.PHPVersion, &s.FPMPool, &s.WebServer, &sslInt, &s.SSLProvider, &s.Status,
		&s.Origin, &s.VhostPath,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	s.SSLEnabled = sslInt == 1
	_ = json.Unmarshal([]byte(aliasesJSON), &s.Aliases)
	if s.Aliases == nil {
		s.Aliases = []string{}
	}
	return s, nil
}

func (d *DB) DeleteSite(ctx context.Context, id string) error {
	// ssl_certificates.site_id is cleared by its ON DELETE SET NULL foreign key;
	// the tables below have no FK and must be cleaned up explicitly.
	stmts := []string{
		`DELETE FROM site_config_backups WHERE site_id = ?`,
		`DELETE FROM site_audit_findings WHERE snapshot_id IN (SELECT id FROM site_file_snapshots WHERE site_id = ?)`,
		`DELETE FROM site_file_snapshots WHERE site_id = ?`,
		`DELETE FROM sites WHERE id = ?`,
	}
	return d.withTx(ctx, func(tx *sql.Tx) error {
		for _, q := range stmts {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return fmt.Errorf("failed to delete site %s: %w", id, err)
			}
		}
		return nil
	})
}

// SiteConfigBackupRecord tracks version history of site virtual host configs.
type SiteConfigBackupRecord struct {
	ID               string    `json:"id"`
	ServerID         string    `json:"server_id"`
	SiteID           string    `json:"site_id"`
	FilePath         string    `json:"file_path"`
	ContentHash      string    `json:"content_hash"`
	Content          string    `json:"content"`
	RemoteBackupPath string    `json:"remote_backup_path"`
	TaskID           string    `json:"task_id"`
	CreatedAt        time.Time `json:"created_at"`
}

func (d *DB) SaveSiteConfigBackup(ctx context.Context, b *SiteConfigBackupRecord) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO site_config_backups (id, server_id, site_id, file_path, content_hash, content, remote_backup_path, task_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, b.ID, b.ServerID, b.SiteID, b.FilePath, b.ContentHash, b.Content, b.RemoteBackupPath, b.TaskID, b.CreatedAt)
	return err
}

func (d *DB) ListSiteConfigBackups(ctx context.Context, siteID string, limit int) ([]SiteConfigBackupRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.QueryContext(ctx, `
		SELECT id, server_id, site_id, file_path, content_hash, content, remote_backup_path, task_id, created_at
		FROM site_config_backups
		WHERE site_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []SiteConfigBackupRecord
	for rows.Next() {
		var r SiteConfigBackupRecord
		var remoteBackup, taskID sql.NullString
		if err := rows.Scan(&r.ID, &r.ServerID, &r.SiteID, &r.FilePath, &r.ContentHash, &r.Content, &remoteBackup, &taskID, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.RemoteBackupPath = remoteBackup.String
		r.TaskID = taskID.String
		records = append(records, r)
	}
	return records, rows.Err()
}

func (d *DB) GetSiteConfigBackup(ctx context.Context, id string) (*SiteConfigBackupRecord, error) {
	var r SiteConfigBackupRecord
	var remoteBackup, taskID sql.NullString
	err := d.QueryRowContext(ctx, `
		SELECT id, server_id, site_id, file_path, content_hash, content, remote_backup_path, task_id, created_at
		FROM site_config_backups
		WHERE id = ?
	`, id).Scan(&r.ID, &r.ServerID, &r.SiteID, &r.FilePath, &r.ContentHash, &r.Content, &remoteBackup, &taskID, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	r.RemoteBackupPath = remoteBackup.String
	r.TaskID = taskID.String
	return &r, nil
}

func (d *DB) SaveInventorySnapshot(ctx context.Context, id, serverID, snapshotJSON string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO server_inventory (id, server_id, snapshot_json, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, id, serverID, snapshotJSON)
	return err
}

func (d *DB) GetLatestInventorySnapshot(ctx context.Context, serverID string) (string, error) {
	var snap string
	err := d.QueryRowContext(ctx, `
		SELECT snapshot_json FROM server_inventory WHERE server_id = ? ORDER BY created_at DESC LIMIT 1
	`, serverID).Scan(&snap)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return snap, nil
}

func (d *DB) UpdateSiteStatus(ctx context.Context, id string, status string) error {
	_, err := d.ExecContext(ctx, `UPDATE sites SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

// --- SSL Certificate Repository Methods ---

func (d *DB) SaveSSLCertificate(ctx context.Context, cert *SSLCertificateRecord) error {
	sanJSON, _ := json.Marshal(cert.SanDomains)
	autoRenewInt := 0
	if cert.AutoRenew {
		autoRenewInt = 1
	}

	var siteIDVal interface{}
	if cert.SiteID != "" {
		siteIDVal = cert.SiteID
	}

	_, err := d.ExecContext(ctx, `
		INSERT INTO ssl_certificates (
			id, server_id, site_id, primary_domain, san_domains, cert_path, key_path,
			issuer, valid_from, valid_to, days_remaining, auto_renew, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			site_id = excluded.site_id,
			primary_domain = excluded.primary_domain,
			san_domains = excluded.san_domains,
			cert_path = excluded.cert_path,
			key_path = excluded.key_path,
			issuer = excluded.issuer,
			valid_from = excluded.valid_from,
			valid_to = excluded.valid_to,
			days_remaining = excluded.days_remaining,
			auto_renew = excluded.auto_renew,
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP
	`, cert.ID, cert.ServerID, siteIDVal, cert.PrimaryDomain, string(sanJSON), cert.CertPath, cert.KeyPath,
		cert.Issuer, cert.ValidFrom, cert.ValidTo, cert.DaysRemaining, autoRenewInt, cert.Status)

	return err
}

func (d *DB) ListSSLCertificates(ctx context.Context, serverID string) ([]*SSLCertificateRecord, error) {
	var rows *sql.Rows
	var err error
	if serverID != "" {
		rows, err = d.QueryContext(ctx, `
			SELECT id, server_id, COALESCE(site_id, ''), primary_domain, san_domains, cert_path, key_path,
			       issuer, valid_from, valid_to, days_remaining, auto_renew, status, created_at, updated_at
			FROM ssl_certificates WHERE server_id = ? ORDER BY primary_domain ASC
		`, serverID)
	} else {
		rows, err = d.QueryContext(ctx, `
			SELECT id, server_id, COALESCE(site_id, ''), primary_domain, san_domains, cert_path, key_path,
			       issuer, valid_from, valid_to, days_remaining, auto_renew, status, created_at, updated_at
			FROM ssl_certificates ORDER BY primary_domain ASC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*SSLCertificateRecord
	for rows.Next() {
		c := &SSLCertificateRecord{}
		var sanJSON string
		var autoRenewInt int
		if err := rows.Scan(
			&c.ID, &c.ServerID, &c.SiteID, &c.PrimaryDomain, &sanJSON, &c.CertPath, &c.KeyPath,
			&c.Issuer, &c.ValidFrom, &c.ValidTo, &c.DaysRemaining, &autoRenewInt, &c.Status,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c.AutoRenew = autoRenewInt == 1
		_ = json.Unmarshal([]byte(sanJSON), &c.SanDomains)
		if c.SanDomains == nil {
			c.SanDomains = []string{}
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

func (d *DB) GetSSLCertificate(ctx context.Context, id string) (*SSLCertificateRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, server_id, COALESCE(site_id, ''), primary_domain, san_domains, cert_path, key_path,
		       issuer, valid_from, valid_to, days_remaining, auto_renew, status, created_at, updated_at
		FROM ssl_certificates WHERE id = ?
	`, id)

	c := &SSLCertificateRecord{}
	var sanJSON string
	var autoRenewInt int
	if err := row.Scan(
		&c.ID, &c.ServerID, &c.SiteID, &c.PrimaryDomain, &sanJSON, &c.CertPath, &c.KeyPath,
		&c.Issuer, &c.ValidFrom, &c.ValidTo, &c.DaysRemaining, &autoRenewInt, &c.Status,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	c.AutoRenew = autoRenewInt == 1
	_ = json.Unmarshal([]byte(sanJSON), &c.SanDomains)
	if c.SanDomains == nil {
		c.SanDomains = []string{}
	}
	return c, nil
}

func (d *DB) DeleteSSLCertificate(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM ssl_certificates WHERE id = ?`, id)
	return err
}

// --- Backups Operations ---

func (d *DB) SaveBackup(ctx context.Context, b *BackupRecord) error {
	query := `
		INSERT INTO backups (
			id, server_id, backup_type, target_name, destination, file_path, size_bytes, status, offsite, downloaded_at, verified, checksum, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			size_bytes = excluded.size_bytes,
			offsite = excluded.offsite,
			downloaded_at = excluded.downloaded_at,
			verified = excluded.verified,
			checksum = excluded.checksum;
	`
	_, err := d.ExecContext(ctx, query,
		b.ID, b.ServerID, b.BackupType, b.TargetName, b.Destination, b.FilePath, b.SizeBytes, b.Status,
		b.Offsite, b.DownloadedAt, b.Verified, b.Checksum, b.CreatedAt,
	)
	return err
}

func (d *DB) GetBackup(ctx context.Context, id string) (*BackupRecord, error) {
	b := &BackupRecord{}
	err := d.QueryRowContext(ctx, `
		SELECT id, server_id, backup_type, target_name, destination, file_path, size_bytes, status,
		       COALESCE(offsite, 0), downloaded_at, COALESCE(verified, 0), COALESCE(checksum, ''), created_at
		FROM backups
		WHERE id = ?
	`, id).Scan(
		&b.ID, &b.ServerID, &b.BackupType, &b.TargetName, &b.Destination, &b.FilePath, &b.SizeBytes, &b.Status,
		&b.Offsite, &b.DownloadedAt, &b.Verified, &b.Checksum, &b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (d *DB) MarkBackupOffsite(ctx context.Context, id string, downloadedAt time.Time) error {
	_, err := d.ExecContext(ctx, `
		UPDATE backups SET offsite = 1, downloaded_at = ? WHERE id = ?
	`, downloadedAt, id)
	return err
}

func (d *DB) MarkBackupVerified(ctx context.Context, id string, verified bool, checksum string) error {
	verVal := 0
	if verified {
		verVal = 1
	}
	_, err := d.ExecContext(ctx, `
		UPDATE backups SET verified = ?, checksum = ? WHERE id = ?
	`, verVal, checksum, id)
	return err
}

func (d *DB) ListBackups(ctx context.Context, serverID string) ([]*BackupRecord, error) {
	var rows *sql.Rows
	var err error
	if serverID != "" {
		rows, err = d.QueryContext(ctx, `
			SELECT id, server_id, backup_type, target_name, destination, file_path, size_bytes, status,
			       COALESCE(offsite, 0), downloaded_at, COALESCE(verified, 0), COALESCE(checksum, ''), created_at
			FROM backups
			WHERE server_id = ?
			ORDER BY created_at DESC
		`, serverID)
	} else {
		rows, err = d.QueryContext(ctx, `
			SELECT id, server_id, backup_type, target_name, destination, file_path, size_bytes, status,
			       COALESCE(offsite, 0), downloaded_at, COALESCE(verified, 0), COALESCE(checksum, ''), created_at
			FROM backups
			ORDER BY created_at DESC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*BackupRecord
	for rows.Next() {
		b := &BackupRecord{}
		if err := rows.Scan(
			&b.ID, &b.ServerID, &b.BackupType, &b.TargetName, &b.Destination, &b.FilePath, &b.SizeBytes, &b.Status,
			&b.Offsite, &b.DownloadedAt, &b.Verified, &b.Checksum, &b.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}

func (d *DB) DeleteBackup(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM backups WHERE id = ?`, id)
	return err
}

// --- Provisioning and Blueprints ---

type ServerBlueprintRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Profile     string    `json:"profile"`
	SpecJSON    string    `json:"spec_json"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProvisioningRunRecord struct {
	ID          string     `json:"id"`
	ServerID    string     `json:"server_id"`
	BlueprintID string     `json:"blueprint_id"`
	Profile     string     `json:"profile"`
	Status      string     `json:"status"` // "running", "completed", "failed"
	PlanJSON    string     `json:"plan_json"`
	ResultJSON  string     `json:"result_json"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func (d *DB) SaveServerBlueprint(ctx context.Context, b *ServerBlueprintRecord) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	b.UpdatedAt = time.Now()
	query := `
		INSERT INTO server_blueprints (id, name, description, profile, spec_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			profile = excluded.profile,
			spec_json = excluded.spec_json,
			updated_at = excluded.updated_at
	`
	_, err := d.ExecContext(ctx, query, b.ID, b.Name, b.Description, b.Profile, b.SpecJSON, b.CreatedAt, b.UpdatedAt)
	return err
}

func (d *DB) GetServerBlueprint(ctx context.Context, id string) (*ServerBlueprintRecord, error) {
	row := d.QueryRowContext(ctx, `SELECT id, name, description, profile, spec_json, created_at, updated_at FROM server_blueprints WHERE id = ?`, id)
	b := &ServerBlueprintRecord{}
	if err := row.Scan(&b.ID, &b.Name, &b.Description, &b.Profile, &b.SpecJSON, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	return b, nil
}

func (d *DB) ListServerBlueprints(ctx context.Context) ([]*ServerBlueprintRecord, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, name, description, profile, spec_json, created_at, updated_at FROM server_blueprints ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*ServerBlueprintRecord
	for rows.Next() {
		b := &ServerBlueprintRecord{}
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.Profile, &b.SpecJSON, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}

func (d *DB) DeleteServerBlueprint(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM server_blueprints WHERE id = ?`, id)
	return err
}

func (d *DB) SaveProvisioningRun(ctx context.Context, r *ProvisioningRunRecord) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	query := `
		INSERT INTO server_provisioning_runs (id, server_id, blueprint_id, profile, status, plan_json, result_json, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			result_json = excluded.result_json,
			completed_at = excluded.completed_at
	`
	_, err := d.ExecContext(ctx, query, r.ID, r.ServerID, r.BlueprintID, r.Profile, r.Status, r.PlanJSON, r.ResultJSON, r.CreatedAt, r.CompletedAt)
	return err
}

func (d *DB) GetProvisioningRun(ctx context.Context, id string) (*ProvisioningRunRecord, error) {
	row := d.QueryRowContext(ctx, `SELECT id, server_id, blueprint_id, profile, status, plan_json, result_json, created_at, completed_at FROM server_provisioning_runs WHERE id = ?`, id)
	r := &ProvisioningRunRecord{}
	if err := row.Scan(&r.ID, &r.ServerID, &r.BlueprintID, &r.Profile, &r.Status, &r.PlanJSON, &r.ResultJSON, &r.CreatedAt, &r.CompletedAt); err != nil {
		return nil, err
	}
	return r, nil
}

func (d *DB) ListProvisioningRuns(ctx context.Context, serverID string, limit int) ([]*ProvisioningRunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows *sql.Rows
	var err error
	if serverID != "" {
		rows, err = d.QueryContext(ctx, `SELECT id, server_id, blueprint_id, profile, status, plan_json, result_json, created_at, completed_at FROM server_provisioning_runs WHERE server_id = ? ORDER BY created_at DESC LIMIT ?`, serverID, limit)
	} else {
		rows, err = d.QueryContext(ctx, `SELECT id, server_id, blueprint_id, profile, status, plan_json, result_json, created_at, completed_at FROM server_provisioning_runs ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*ProvisioningRunRecord
	for rows.Next() {
		r := &ProvisioningRunRecord{}
		if err := rows.Scan(&r.ID, &r.ServerID, &r.BlueprintID, &r.Profile, &r.Status, &r.PlanJSON, &r.ResultJSON, &r.CreatedAt, &r.CompletedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}
