package database

import "time"

type EngineType string

const (
	EngineMySQL      EngineType = "mysql"
	EngineMariaDB    EngineType = "mariadb"
	EnginePostgreSQL EngineType = "postgres"
	EngineRedis      EngineType = "redis"
)

// DatabaseInstanceInfo describes an installed DBMS on the target server.
type DatabaseInstanceInfo struct {
	Engine        EngineType `json:"engine"`
	DisplayName   string     `json:"display_name"`
	Installed     bool       `json:"installed"`
	Version       string     `json:"version"`
	ServiceStatus string     `json:"service_status"` // active, inactive, failed
	Port          int        `json:"port"`
	MemoryBytes   int64      `json:"memory_bytes"`
	DBCount       int        `json:"db_count"`
	ConfigPath    string     `json:"config_path"`
}

// DatabaseItem describes a single logical database inside an engine.
type DatabaseItem struct {
	Name        string     `json:"name"`
	Engine      EngineType `json:"engine"`
	SizeBytes   int64      `json:"size_bytes"`
	TablesCount int        `json:"tables_count"`
	Charset     string     `json:"charset"`
	Collation   string     `json:"collation"`
}

// DatabaseUser describes an existing database user account.
type DatabaseUser struct {
	Username  string     `json:"username"`
	Host      string     `json:"host"` // e.g. "localhost", "%"
	Engine    EngineType `json:"engine"`
	Databases []string   `json:"databases"`
}

// CreateDatabaseRequest payload for creating a new database.
type CreateDatabaseRequest struct {
	ServerID  string     `json:"server_id"`
	Engine    EngineType `json:"engine"`
	Name      string     `json:"name"`
	Charset   string     `json:"charset"`   // e.g. "utf8mb4" or "UTF8"
	Collation string     `json:"collation"` // e.g. "utf8mb4_unicode_ci"
}

// CreateUserRequest payload for creating a database user.
type CreateUserRequest struct {
	ServerID  string     `json:"server_id"`
	Engine    EngineType `json:"engine"`
	Username  string     `json:"username"`
	Password  string     `json:"password"`
	Host      string     `json:"host"` // default "%" or "localhost"
	Databases []string   `json:"databases"`
}

// BackupDatabaseRequest payload for dumping a database to a .sql or .sql.gz file.
type BackupDatabaseRequest struct {
	ServerID     string     `json:"server_id"`
	Engine       EngineType `json:"engine"`
	DatabaseName string     `json:"database_name"`
	TargetPath   string     `json:"target_path"` // directory on server or file
}

// RestoreDatabaseRequest payload for restoring a database from a dump.
type RestoreDatabaseRequest struct {
	ServerID     string     `json:"server_id"`
	Engine       EngineType `json:"engine"`
	DatabaseName string     `json:"database_name"`
	DumpFilePath string     `json:"dump_file_path"`
}

// DatabaseDumpMetadata records created backups.
type DatabaseDumpMetadata struct {
	ID           string     `json:"id"`
	ServerID     string     `json:"server_id"`
	DatabaseName string     `json:"database_name"`
	Engine       EngineType `json:"engine"`
	FilePath     string     `json:"file_path"`
	SizeBytes    int64      `json:"size_bytes"`
	CreatedAt    time.Time  `json:"created_at"`
}
