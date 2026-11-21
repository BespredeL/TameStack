package backup

import "time"

// CreateBackupRequest payload for launching a backup task.
type CreateBackupRequest struct {
	ServerID        string `json:"server_id"`
	BackupType      string `json:"backup_type"`      // "site", "database", "config", "full"
	TargetName      string `json:"target_name"`      // domain or db name
	Destination     string `json:"destination"`      // "local", "sftp", "s3"
	TargetPath      string `json:"target_path"`      // directory on server, default /var/backups/TameStack
	RetentionDays   int    `json:"retention_days"`   // e.g. 7 or 14
	IncludeDatabase bool   `json:"include_database"` // for site backups: dump linked db
	DatabaseName    string `json:"database_name,omitempty"`
}

// RestoreBackupRequest payload for restoring an archive.
type RestoreBackupRequest struct {
	ServerID         string `json:"server_id"`
	BackupID         string `json:"backup_id"`
	FilePath         string `json:"file_path"`
	TargetType       string `json:"target_type"` // "site", "database", "config", "full"
	TargetName       string `json:"target_name"` // domain or db name
	TargetDirectory  string `json:"target_directory,omitempty"`
	AllowSystemPaths bool   `json:"allow_system_paths"` // explicit consent to extract into /etc, /usr, /bin
	DryRun           bool   `json:"dry_run"`
}

// ArchiveInspectionResult represents preflight inspection of a backup archive.
type ArchiveInspectionResult struct {
	FilePath        string    `json:"file_path"`
	ArchiveType     string    `json:"archive_type"` // "tar.gz", "sql.gz", "unknown"
	SizeBytes       int64     `json:"size_bytes"`
	TotalEntries    int       `json:"total_entries"`
	SampleEntries   []string  `json:"sample_entries"`
	HasSystemPaths  bool      `json:"has_system_paths"`
	DangerousPaths  []string  `json:"dangerous_paths"`
	SuggestedRoot   string    `json:"suggested_root"`
	ContainsDBDump  bool      `json:"contains_db_dump"`
	DBDumpName      string    `json:"db_dump_name,omitempty"`
	IntegrityPassed bool      `json:"integrity_passed"`
	InspectedAt     time.Time `json:"inspected_at"`
	ErrorMessage    string    `json:"error_message,omitempty"`
}
