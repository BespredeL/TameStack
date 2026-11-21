package db

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Migration represents a single versioned database migration.
type Migration struct {
	Version int
	Name    string
	Up      func(tx *sql.Tx) error
}

// BackupDatabase creates a timestamped or standard .backup copy of SQLite database file.
func BackupDatabase(dbPath string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil // nothing to backup on first start
	}

	// Committed frames can still sit in the -wal file, and copying the main file
	// alone would leave a backup that is missing the newest transactions.
	if err := checkpointDatabase(dbPath); err != nil {
		return fmt.Errorf("failed to fold write-ahead log into the database: %w", err)
	}

	backupPath := dbPath + ".backup"
	src, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open source db for backup: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create db backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy database to backup: %w", err)
	}

	return nil
}

// checkpointDatabase folds the write-ahead log into the main file through a
// short-lived connection. Open calls it before the application handle exists, so
// no other writer competes for the checkpoint.
func checkpointDatabase(dbPath string) error {
	conn, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath))
	if err != nil {
		return err
	}
	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		conn.Close()
		return err
	}
	return conn.Close()
}

// CheckIntegrity runs SQLite PRAGMA integrity_check.
func (d *DB) CheckIntegrity() (bool, error) {
	var result string
	err := d.QueryRow("PRAGMA integrity_check(1);").Scan(&result)
	if err != nil {
		return false, fmt.Errorf("integrity check failed: %w", err)
	}
	return result == "ok", nil
}

// RunMigrations executes all unapplied versioned migrations in order.
func (d *DB) RunMigrations() error {
	_, err := d.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to init schema_migrations: %w", err)
	}

	// 2. Check if this is a legacy database with pre-existing tables.
	var serversCount int
	if err := d.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='servers';").Scan(&serversCount); err != nil {
		return fmt.Errorf("failed to detect legacy schema: %w", err)
	}

	var appliedCount int
	if err := d.QueryRow("SELECT COUNT(*) FROM schema_migrations;").Scan(&appliedCount); err != nil {
		return fmt.Errorf("failed to read applied migrations: %w", err)
	}

	// If legacy database already has tables but 0 migrations recorded, baseline it.
	if serversCount > 0 && appliedCount == 0 {
		slog.Info("Baselining existing database into schema_migrations")
		baseline := []int{1, 2, 3}
		for _, v := range baseline {
			name := fmt.Sprintf("00%d_baseline", v)
			if _, err := d.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?);", v, name); err != nil {
				return fmt.Errorf("failed to baseline migration %d: %w", v, err)
			}
		}
	}

	migrations := getMigrationsList()

	for _, m := range migrations {
		var exists int
		if err := d.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?;", m.Version).Scan(&exists); err != nil {
			return fmt.Errorf("failed to check migration %d (%s): %w", m.Version, m.Name, err)
		}
		if exists > 0 {
			continue // already applied
		}

		slog.Info("Applying database migration",
			slog.Int("version", m.Version),
			slog.String("name", m.Name),
		)

		tx, err := d.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d (%s): %w", m.Version, m.Name, err)
		}

		if err := m.Up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to apply migration %d (%s): %w", m.Version, m.Name, err)
		}

		_, err = tx.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?);", m.Version, m.Name)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %d (%s): %w", m.Version, m.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d (%s): %w", m.Version, m.Name, err)
		}

		slog.Info("Successfully applied database migration",
			slog.Int("version", m.Version),
			slog.String("name", m.Name),
		)
	}

	return nil
}

func getMigrationsList() []Migration {
	return []Migration{
		{
			Version: 1,
			Name:    "001_core_schema",
			Up: func(tx *sql.Tx) error {
				schema := `
				CREATE TABLE IF NOT EXISTS servers (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					host TEXT NOT NULL,
					port INTEGER NOT NULL DEFAULT 22,
					username TEXT NOT NULL,
					auth_type TEXT NOT NULL,
					encrypted_secret TEXT NOT NULL,
					ssh_host_key TEXT DEFAULT '',
					group_name TEXT DEFAULT 'Default',
					tags TEXT DEFAULT '[]',
					is_favorite INTEGER DEFAULT 0,
					status TEXT DEFAULT 'unknown',
					last_connected_at DATETIME,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				CREATE TABLE IF NOT EXISTS server_diagnostics (
					server_id TEXT PRIMARY KEY,
					os_name TEXT,
					os_version TEXT,
					os_family TEXT,
					kernel_version TEXT,
					arch TEXT,
					cpu_cores INTEGER DEFAULT 0,
					cpu_model TEXT,
					ram_total_bytes INTEGER DEFAULT 0,
					disk_total_bytes INTEGER DEFAULT 0,
					disk_used_bytes INTEGER DEFAULT 0,
					uptime_seconds INTEGER DEFAULT 0,
					ssh_version TEXT,
					installed_webservers TEXT DEFAULT '[]',
					installed_php_versions TEXT DEFAULT '[]',
					installed_db_versions TEXT DEFAULT '[]',
					docker_version TEXT,
					diagnosed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS tasks (
					id TEXT PRIMARY KEY,
					server_id TEXT,
					operation_type TEXT NOT NULL,
					title TEXT NOT NULL,
					status TEXT NOT NULL,
					current_step TEXT DEFAULT '',
					progress INTEGER DEFAULT 0,
					payload TEXT DEFAULT '',
					result TEXT DEFAULT '',
					error_message TEXT DEFAULT '',
					started_at DATETIME,
					completed_at DATETIME,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE SET NULL
				);

				CREATE TABLE IF NOT EXISTS task_logs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					task_id TEXT NOT NULL,
					level TEXT NOT NULL,
					message TEXT NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS sites (
					id TEXT PRIMARY KEY,
					server_id TEXT NOT NULL,
					domain TEXT NOT NULL,
					aliases TEXT DEFAULT '[]',
					document_root TEXT NOT NULL,
					site_type TEXT NOT NULL,
					php_version TEXT,
					fpm_pool TEXT,
					web_server TEXT NOT NULL,
					ssl_enabled INTEGER DEFAULT 0,
					ssl_provider TEXT DEFAULT 'none',
					status TEXT DEFAULT 'active',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS ssl_certificates (
					id TEXT PRIMARY KEY,
					server_id TEXT NOT NULL,
					site_id TEXT,
					primary_domain TEXT NOT NULL,
					san_domains TEXT DEFAULT '[]',
					cert_path TEXT NOT NULL,
					key_path TEXT NOT NULL,
					issuer TEXT DEFAULT 'Let''s Encrypt',
					valid_from DATETIME,
					valid_to DATETIME,
					days_remaining INTEGER DEFAULT 0,
					auto_renew INTEGER DEFAULT 1,
					status TEXT DEFAULT 'valid',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
					FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL
				);

				CREATE TABLE IF NOT EXISTS backups (
					id TEXT PRIMARY KEY,
					server_id TEXT NOT NULL,
					backup_type TEXT NOT NULL,
					target_name TEXT NOT NULL,
					destination TEXT NOT NULL,
					file_path TEXT NOT NULL,
					size_bytes INTEGER DEFAULT 0,
					status TEXT DEFAULT 'completed',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
				CREATE INDEX IF NOT EXISTS idx_task_logs_task ON task_logs(task_id);
				CREATE INDEX IF NOT EXISTS idx_sites_server ON sites(server_id);
				CREATE INDEX IF NOT EXISTS idx_ssl_server ON ssl_certificates(server_id);
				`
				_, err := tx.Exec(schema)
				return err
			},
		},
		{
			Version: 2,
			Name:    "002_ai_subsystem",
			Up: func(tx *sql.Tx) error {
				schema := `
				CREATE TABLE IF NOT EXISTS ai_provider_configs (
					id TEXT PRIMARY KEY,
					provider_type TEXT NOT NULL,
					name TEXT NOT NULL,
					base_url TEXT NOT NULL,
					api_key_encrypted TEXT DEFAULT '',
					is_enabled INTEGER DEFAULT 1,
					is_default INTEGER DEFAULT 0,
					default_model TEXT DEFAULT '',
					system_prompt TEXT DEFAULT '',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				CREATE TABLE IF NOT EXISTS ai_global_settings (
					key TEXT PRIMARY KEY,
					value TEXT NOT NULL,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				CREATE TABLE IF NOT EXISTS ai_conversations (
					id TEXT PRIMARY KEY,
					title TEXT NOT NULL,
					server_id TEXT,
					system_prompt TEXT DEFAULT '',
					selected_provider TEXT NOT NULL,
					selected_model TEXT NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE SET NULL
				);

				CREATE TABLE IF NOT EXISTS ai_messages (
					id TEXT PRIMARY KEY,
					conversation_id TEXT NOT NULL,
					role TEXT NOT NULL,
					content TEXT NOT NULL,
					tool_calls_json TEXT DEFAULT '[]',
					tool_call_id TEXT DEFAULT '',
					prompt_tokens INTEGER DEFAULT 0,
					completion_tokens INTEGER DEFAULT 0,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (conversation_id) REFERENCES ai_conversations(id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS ai_memory_facts (
					id TEXT PRIMARY KEY,
					server_id TEXT,
					category TEXT NOT NULL,
					fact_key TEXT NOT NULL,
					fact_value TEXT NOT NULL,
					source TEXT DEFAULT 'auto',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
				);

				CREATE VIRTUAL TABLE IF NOT EXISTS ai_memory_fts USING fts5(
					fact_id UNINDEXED,
					fact_text
				);

				CREATE TABLE IF NOT EXISTS ai_tool_audit (
					id TEXT PRIMARY KEY,
					conversation_id TEXT NOT NULL,
					message_id TEXT DEFAULT '',
					tool_name TEXT NOT NULL,
					arguments_json TEXT NOT NULL,
					risk_level TEXT NOT NULL,
					approved INTEGER DEFAULT 1,
					executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					execution_duration_ms INTEGER DEFAULT 0,
					success INTEGER DEFAULT 1,
					error_message TEXT DEFAULT ''
				);

				CREATE INDEX IF NOT EXISTS idx_ai_messages_conv ON ai_messages(conversation_id);
				`
				_, err := tx.Exec(schema)
				return err
			},
		},
		{
			Version: 3,
			Name:    "003_sudo_elevation",
			Up: func(tx *sql.Tx) error {
				var hasCol int
				rows, err := tx.Query("PRAGMA table_info(servers);")
				if err != nil {
					return err
				}
				defer rows.Close()

				for rows.Next() {
					var cid int
					var name, ctype string
					var notnull, pk int
					var dflt *string
					if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
						if name == "encrypted_sudo_secret" {
							hasCol = 1
							break
						}
					}
				}

				if hasCol == 0 {
					_, err = tx.Exec("ALTER TABLE servers ADD COLUMN encrypted_sudo_secret TEXT DEFAULT '';")
					return err
				}
				return nil
			},
		},
		{
			Version: 4,
			Name:    "004_vault_settings",
			Up: func(tx *sql.Tx) error {
				schema := `
				CREATE TABLE IF NOT EXISTS vault_settings (
					key TEXT PRIMARY KEY,
					value TEXT NOT NULL,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);
				INSERT OR IGNORE INTO vault_settings (key, value) VALUES ('has_master_password', 'false');
				INSERT OR IGNORE INTO vault_settings (key, value) VALUES ('auto_lock_minutes', '15');
				INSERT OR IGNORE INTO vault_settings (key, value) VALUES ('master_password_verifier', '');
				`
				_, err := tx.Exec(schema)
				return err
			},
		},
		{
			Version: 5,
			Name:    "005_key_passphrase",
			Up: func(tx *sql.Tx) error {
				var hasCol int
				rows, err := tx.Query("PRAGMA table_info(servers);")
				if err != nil {
					return err
				}
				defer rows.Close()

				for rows.Next() {
					var cid int
					var name, ctype string
					var notnull, pk int
					var dflt *string
					if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
						if name == "encrypted_key_passphrase" {
							hasCol = 1
							break
						}
					}
				}
				if hasCol == 0 {
					_, err = tx.Exec("ALTER TABLE servers ADD COLUMN encrypted_key_passphrase TEXT DEFAULT '';")
					return err
				}
				return nil
			},
		},
		{
			Version: 6,
			Name:    "006_site_file_snapshots",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS site_file_snapshots (
					id TEXT PRIMARY KEY,
					site_id TEXT NOT NULL,
					server_id TEXT NOT NULL,
					document_root TEXT NOT NULL,
					is_baseline INTEGER NOT NULL DEFAULT 0,
					file_count INTEGER NOT NULL DEFAULT 0,
					manifest_json TEXT NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);
				CREATE TABLE IF NOT EXISTS site_audit_findings (
					id TEXT PRIMARY KEY,
					snapshot_id TEXT NOT NULL,
					path TEXT NOT NULL,
					rule_id TEXT NOT NULL,
					summary TEXT NOT NULL
				);
				`)
				return err
			},
		},
		{
			Version: 7,
			Name:    "007_inventory_and_site_origin",
			Up: func(tx *sql.Tx) error {
				if err := addColumnIfNotExists(tx, "servers", "mode", "TEXT DEFAULT 'audit'"); err != nil {
					return err
				}
				if err := addColumnIfNotExists(tx, "sites", "origin", "TEXT DEFAULT 'unknown'"); err != nil {
					return err
				}
				if err := addColumnIfNotExists(tx, "sites", "vhost_path", "TEXT DEFAULT ''"); err != nil {
					return err
				}
				_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS server_inventory (
					id TEXT PRIMARY KEY,
					server_id TEXT NOT NULL,
					snapshot_json TEXT NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);
				UPDATE sites SET origin = 'unknown' WHERE origin IS NULL OR origin = '';
				UPDATE servers SET mode = 'audit' WHERE mode IS NULL OR mode = '';
				`)
				return err
			},
		},
		{
			Version: 8,
			Name:    "008_site_config_backups",
			Up: func(tx *sql.Tx) error {
				queries := []string{
					`CREATE TABLE IF NOT EXISTS site_config_backups (
						id TEXT PRIMARY KEY,
						server_id TEXT NOT NULL,
						site_id TEXT NOT NULL,
						file_path TEXT NOT NULL,
						content_hash TEXT NOT NULL,
						content TEXT NOT NULL,
						remote_backup_path TEXT,
						task_id TEXT,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP
					);`,
					`CREATE INDEX IF NOT EXISTS idx_site_config_backups_site ON site_config_backups(site_id, created_at DESC);`,
					`CREATE INDEX IF NOT EXISTS idx_site_config_backups_server ON site_config_backups(server_id, created_at DESC);`,
				}
				for _, q := range queries {
					if _, err := tx.Exec(q); err != nil {
						return fmt.Errorf("failed executing query in migration 8: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 9,
			Name:    "009_backup_offsite_and_verification",
			Up: func(tx *sql.Tx) error {
				if err := addColumnIfNotExists(tx, "backups", "offsite", "INTEGER DEFAULT 0"); err != nil {
					return err
				}
				if err := addColumnIfNotExists(tx, "backups", "downloaded_at", "DATETIME"); err != nil {
					return err
				}
				if err := addColumnIfNotExists(tx, "backups", "verified", "INTEGER DEFAULT 0"); err != nil {
					return err
				}
				if err := addColumnIfNotExists(tx, "backups", "checksum", "TEXT DEFAULT ''"); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Version: 10,
			Name:    "010_file_security_snapshots_and_exclusions",
			Up: func(tx *sql.Tx) error {
				queries := []string{
					`CREATE TABLE IF NOT EXISTS file_security_exclusions (
						id TEXT PRIMARY KEY,
						server_id TEXT NOT NULL,
						pattern TEXT NOT NULL,
						reason TEXT NOT NULL DEFAULT '',
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP
					);`,
					`CREATE INDEX IF NOT EXISTS idx_file_security_exclusions_server ON file_security_exclusions(server_id);`,
					`CREATE TABLE IF NOT EXISTS file_security_snapshots (
						id TEXT PRIMARY KEY,
						server_id TEXT NOT NULL,
						site_id TEXT NOT NULL DEFAULT '',
						name TEXT NOT NULL,
						base_path TEXT NOT NULL,
						files_count INTEGER NOT NULL DEFAULT 0,
						total_size_bytes INTEGER NOT NULL DEFAULT 0,
						manifest_json TEXT NOT NULL,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP
					);`,
					`CREATE INDEX IF NOT EXISTS idx_file_security_snapshots_server ON file_security_snapshots(server_id);`,
				}
				for _, q := range queries {
					if _, err := tx.Exec(q); err != nil {
						return fmt.Errorf("failed executing query in migration 10: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 11,
			Name:    "011_server_blueprints_and_provisioning",
			Up: func(tx *sql.Tx) error {
				queries := []string{
					`CREATE TABLE IF NOT EXISTS server_blueprints (
						id TEXT PRIMARY KEY,
						name TEXT NOT NULL,
						description TEXT NOT NULL DEFAULT '',
						profile TEXT NOT NULL DEFAULT 'custom',
						spec_json TEXT NOT NULL DEFAULT '{}',
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
					);`,
					`CREATE TABLE IF NOT EXISTS server_provisioning_runs (
						id TEXT PRIMARY KEY,
						server_id TEXT NOT NULL,
						blueprint_id TEXT NOT NULL DEFAULT '',
						profile TEXT NOT NULL DEFAULT 'custom',
						status TEXT NOT NULL DEFAULT 'running',
						plan_json TEXT NOT NULL DEFAULT '{}',
						result_json TEXT NOT NULL DEFAULT '{}',
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						completed_at DATETIME,
						FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
					);`,
					`CREATE INDEX IF NOT EXISTS idx_server_provisioning_runs_server ON server_provisioning_runs(server_id);`,
				}
				for _, q := range queries {
					if _, err := tx.Exec(q); err != nil {
						return fmt.Errorf("failed executing query in migration 11: %w", err)
					}
				}
				return nil
			},
		},
	}
}

func addColumnIfNotExists(tx *sql.Tx, table, column, colDef string) error {
	rows, err := tx.Query(fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			if name == column {
				return nil
			}
		}
	}
	_, err = tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table, column, colDef))
	return err
}

// GetVaultSetting retrieves a setting from vault_settings table.
func (d *DB) GetVaultSetting(key string) (string, error) {
	var val string
	err := d.QueryRow("SELECT value FROM vault_settings WHERE key = ?;", key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

// SetVaultSetting saves or updates a setting in vault_settings table.
func (d *DB) SetVaultSetting(key, val string) error {
	_, err := d.Exec(`
		INSERT INTO vault_settings (key, value, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP;
	`, key, val)
	return err
}

// GetAppliedMigrations returns list of completed migrations.
func (d *DB) GetAppliedMigrations() ([]string, error) {
	rows, err := d.Query("SELECT version || ': ' || name || ' (' || applied_at || ')' FROM schema_migrations ORDER BY version ASC;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			result = append(result, s)
		}
	}
	return result, nil
}
