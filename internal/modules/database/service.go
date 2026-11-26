package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

type Service struct {
	module.Base
	engine *tasks.Engine
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}

	engine.RegisterHandler("db_create", s.handleDBCreateTask)
	engine.RegisterHandler("db_user_create", s.handleDBUserCreateTask)
	engine.RegisterHandler("db_delete", s.handleDBDeleteTask)
	engine.RegisterHandler("db_backup", s.handleDBBackupTask)
	engine.RegisterHandler("db_restore", s.handleDBRestoreTask)
	engine.RegisterHandler("db_service_control", s.handleDBServiceControlTask)
	engine.RegisterHandler("db_user_delete", s.handleDeleteUserTask)
	engine.RegisterHandler("db_install", s.handleDBInstallTask)

	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string) (string, string, int, error) {
	cb := client.NewCommand(cmd)
	if strings.Contains(cmd, "sudo") || strings.Contains(cmd, "run_sudo") {
		cb = cb.WithSudo()
	}
	stdout, stderr, code, err := cb.Exec(ctx)
	return string(stdout), string(stderr), code, err
}

func (s *Service) getServerSecret(record *db.ServerRecord) string {
	if record == nil || record.EncryptedSecret == "" || s.Vault == nil {
		return ""
	}
	sec, _ := s.Vault.Decrypt(record.EncryptedSecret)
	return sec
}

// detectMySQLClientCmd intelligently finds the functional MySQL client command line.
func (s *Service) detectMySQLClientCmd(ctx context.Context, client *ssh.SSHClient, serverPassword ...string) string {
	defaultsFile := ""
	if len(serverPassword) > 0 && serverPassword[0] != "" {
		if p, err := client.InstallMySQLDefaults(ctx, serverPassword[0]); err == nil {
			defaultsFile = p
		}
	}

	script, err := renderScript("detect_mysql_client.sh", struct{ Defaults string }{ssh.ShellEscape(defaultsFile)})
	if err != nil {
		return "mysql"
	}

	out, _, code, _ := s.execCmd(ctx, client, script)
	if code == 0 && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return "mysql"
}

// detectMySQLServiceStatus checks systemd services and process list for MySQL/MariaDB.
func (s *Service) detectMySQLServiceStatus(ctx context.Context, client *ssh.SSHClient) string {
	script, err := renderScript("detect_mysql_service_status.sh", nil)
	if err != nil {
		return "inactive"
	}
	out, _, _, _ := s.execCmd(ctx, client, script)
	status := strings.TrimSpace(out)
	if status == "active" {
		return "active"
	}
	return "inactive"
}

// InspectDatabases inspects all known DBMS on the target server.
func (s *Service) InspectDatabases(ctx context.Context, serverID string) ([]DatabaseInstanceInfo, error) {
	client, record, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	var instances []DatabaseInstanceInfo

	mysqlInst := DatabaseInstanceInfo{
		Engine:      EngineMySQL,
		DisplayName: "MySQL / MariaDB",
		Port:        3306,
	}
	vOut, _, vCode, _ := s.execCmd(ctx, client, "mysql --version 2>/dev/null || mariadb --version 2>/dev/null")
	if vCode == 0 && len(vOut) > 0 {
		mysqlInst.Installed = true
		mysqlInst.Version = strings.TrimSpace(vOut)
		if strings.Contains(strings.ToLower(vOut), "mariadb") {
			mysqlInst.Engine = EngineMariaDB
			mysqlInst.DisplayName = "MariaDB"
		}

		mysqlInst.ServiceStatus = s.detectMySQLServiceStatus(ctx, client)

		// Memory
		mOut, _, _, _ := s.execCmd(ctx, client, "ps -o rss= -C mysqld,mariadbd 2>/dev/null | awk '{s+=$1} END {print s}'")
		if strings.TrimSpace(mOut) != "" {
			kb, _ := strconv.ParseInt(strings.TrimSpace(mOut), 10, 64)
			mysqlInst.MemoryBytes = kb * 1024
		}

		// Count DBs using detected client.
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		cOut, _, cCode, _ := s.execCmd(ctx, client, fmt.Sprintf("%s -N -e 'SELECT COUNT(*) FROM information_schema.schemata;' 2>/dev/null", mysqlCmd))
		if cCode == 0 && strings.TrimSpace(cOut) != "" {
			cnt, _ := strconv.Atoi(strings.TrimSpace(cOut))
			mysqlInst.DBCount = cnt
			// If we successfully queried DB count, the service is definitively active.
			if cnt > 0 {
				mysqlInst.ServiceStatus = "active"
			}
		}
		mysqlInst.ConfigPath = s.detectConfigPath(ctx, client, mysqlInst.Engine)
	}
	instances = append(instances, mysqlInst)

	pgInst := DatabaseInstanceInfo{
		Engine:      EnginePostgreSQL,
		DisplayName: "PostgreSQL",
		Port:        5432,
	}
	vOut, _, vCode, _ = s.execCmd(ctx, client, "psql --version 2>/dev/null")
	if vCode == 0 && len(vOut) > 0 {
		pgInst.Installed = true
		pgInst.Version = strings.TrimSpace(vOut)

		sOut, _, _, _ := s.execCmd(ctx, client, "systemctl is-active postgresql 2>/dev/null")
		pgInst.ServiceStatus = strings.TrimSpace(sOut)
		if pgInst.ServiceStatus == "" {
			pgInst.ServiceStatus = "inactive"
		}

		mOut, _, _, _ := s.execCmd(ctx, client, "ps -o rss= -C postgres 2>/dev/null | awk '{s+=$1} END {print s}'")
		if strings.TrimSpace(mOut) != "" {
			kb, _ := strconv.ParseInt(strings.TrimSpace(mOut), 10, 64)
			pgInst.MemoryBytes = kb * 1024
		}

		cOut, _, cCode, _ := s.execCmd(ctx, client, "sudo -u postgres psql -t -A -c 'SELECT COUNT(*) FROM pg_database WHERE datistemplate = false;' 2>/dev/null")
		if cCode == 0 {
			cnt, _ := strconv.Atoi(strings.TrimSpace(cOut))
			pgInst.DBCount = cnt
		}
		pgInst.ConfigPath = s.detectConfigPath(ctx, client, EnginePostgreSQL)
	}
	instances = append(instances, pgInst)

	redisInst := DatabaseInstanceInfo{
		Engine:      EngineRedis,
		DisplayName: "Redis In-Memory Store",
		Port:        6379,
	}
	vOut, _, vCode, _ = s.execCmd(ctx, client, "redis-server --version 2>/dev/null")
	if vCode == 0 && len(vOut) > 0 {
		redisInst.Installed = true
		redisInst.Version = strings.TrimSpace(vOut)

		sOut, _, _, _ := s.execCmd(ctx, client, "systemctl is-active redis-server 2>/dev/null || systemctl is-active redis 2>/dev/null")
		redisInst.ServiceStatus = strings.TrimSpace(sOut)
		if redisInst.ServiceStatus == "" {
			redisInst.ServiceStatus = "inactive"
		}

		mOut, _, _, _ := s.execCmd(ctx, client, "redis-cli info memory 2>/dev/null | grep 'used_memory:' | cut -d: -f2")
		if strings.TrimSpace(mOut) != "" {
			b, _ := strconv.ParseInt(strings.TrimSpace(mOut), 10, 64)
			redisInst.MemoryBytes = b
		}

		kOut, _, kCode, _ := s.execCmd(ctx, client, "redis-cli info keyspace 2>/dev/null | grep -c '^db'")
		if kCode == 0 {
			cnt, _ := strconv.Atoi(strings.TrimSpace(kOut))
			redisInst.DBCount = cnt
		}
		redisInst.ConfigPath = s.detectConfigPath(ctx, client, EngineRedis)
	}
	instances = append(instances, redisInst)

	return instances, nil
}

// ListDatabases returns all logical databases on the selected engine.
func (s *Service) ListDatabases(ctx context.Context, serverID string, engine EngineType) ([]DatabaseItem, error) {
	if engine == "" {
		engine = EngineMySQL
	}
	client, record, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	var list []DatabaseItem

	if engine == EngineMySQL || engine == EngineMariaDB {
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))

		// First attempt: detailed query with sizes & charsets (single line, robust escaping).
		cmd := fmt.Sprintf(`%s -N -e "SELECT s.schema_name, COALESCE(SUM(t.data_length + t.index_length), 0) AS size_bytes, COUNT(t.table_name) AS tbl_count, s.default_character_set_name, s.default_collation_name FROM information_schema.schemata s LEFT JOIN information_schema.tables t ON s.schema_name = t.table_schema GROUP BY s.schema_name, s.default_character_set_name, s.default_collation_name;"`, mysqlCmd)

		stdout, stderr, code, _ := s.execCmd(ctx, client, cmd)

		// Fallback: simple SHOW DATABASES if information_schema query fails or returns empty.
		if code != 0 || strings.TrimSpace(stdout) == "" {
			fallbackCmd := fmt.Sprintf(`%s -N -e "SHOW DATABASES;"`, mysqlCmd)
			fbOut, fbStderr, fbCode, fbErr := s.execCmd(ctx, client, fallbackCmd)
			if fbCode == 0 && strings.TrimSpace(fbOut) != "" {
				for _, line := range strings.Split(fbOut, "\n") {
					dbName := strings.TrimSpace(line)
					if dbName != "" {
						list = append(list, DatabaseItem{
							Name:        dbName,
							Engine:      engine,
							SizeBytes:   0,
							TablesCount: 0,
							Charset:     "utf8mb4",
							Collation:   "utf8mb4_general_ci",
						})
					}
				}
				return list, nil
			}
			if strings.Contains(stderr, "command not found") || strings.Contains(fbStderr, "command not found") || code == 127 || fbCode == 127 {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to query MySQL databases (code %d): %s (fallback stderr: %s, err: %v)", code, strings.TrimSpace(stderr), strings.TrimSpace(fbStderr), fbErr)
		}

		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				var size int64 = 0
				var tblCount int = 0
				charset := "utf8mb4"
				collation := "utf8mb4_general_ci"

				if len(parts) >= 2 {
					size, _ = strconv.ParseInt(parts[1], 10, 64)
				}
				if len(parts) >= 3 {
					tblCount, _ = strconv.Atoi(parts[2])
				}
				if len(parts) >= 4 {
					charset = parts[3]
				}
				if len(parts) >= 5 {
					collation = parts[4]
				}

				list = append(list, DatabaseItem{
					Name:        parts[0],
					Engine:      engine,
					SizeBytes:   size,
					TablesCount: tblCount,
					Charset:     charset,
					Collation:   collation,
				})
			}
		}
	} else if engine == EnginePostgreSQL {
		cmd := `sudo -u postgres psql -t -A -F"|" -c "
			SELECT 
				datname, 
				pg_database_size(datname) AS size_bytes,
				pg_encoding_to_char(encoding) AS encoding,
				datcollate
			FROM pg_database 
			WHERE datistemplate = false;
		" 2>/dev/null`
		stdout, stderr, code, err := s.execCmd(ctx, client, cmd)
		if err != nil || code != 0 {
			if strings.Contains(stderr, "command not found") || strings.Contains(stderr, "not found") || code == 127 {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to query PostgreSQL databases: %s %v", stderr, err)
		}

		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				size, _ := strconv.ParseInt(parts[1], 10, 64)
				list = append(list, DatabaseItem{
					Name:        parts[0],
					Engine:      engine,
					SizeBytes:   size,
					TablesCount: 0,
					Charset:     parts[2],
					Collation:   parts[3],
				})
			}
		}
	} else if engine == EngineRedis {
		cmd := "redis-cli info keyspace 2>/dev/null"
		stdout, _, code, _ := s.execCmd(ctx, client, cmd)
		if code == 0 {
			lines := strings.Split(stdout, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "db") {
					parts := strings.Split(line, ":")
					dbName := parts[0]
					list = append(list, DatabaseItem{
						Name:      dbName,
						Engine:    engine,
						Charset:   "binary",
						Collation: "none",
					})
				}
			}
		}
	}

	return list, nil
}

// ListUsers returns all database users for the specified engine.
func (s *Service) ListUsers(ctx context.Context, serverID string, engine EngineType) ([]DatabaseUser, error) {
	client, record, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	var users []DatabaseUser

	if engine == EngineMySQL || engine == EngineMariaDB {
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		cmd := fmt.Sprintf(`%s -N -e "SELECT User, Host FROM mysql.user ORDER BY User;"`, mysqlCmd)
		stdout, _, code, _ := s.execCmd(ctx, client, cmd)

		if code != 0 || strings.TrimSpace(stdout) == "" {
			// Fallback to mysql.db table.
			fallbackCmd := fmt.Sprintf(`%s -N -e "SELECT DISTINCT User, Host FROM mysql.db;"`, mysqlCmd)
			fbOut, fbErrStr, fbCode, _ := s.execCmd(ctx, client, fallbackCmd)
			if fbCode == 0 && strings.TrimSpace(fbOut) != "" {
				stdout = fbOut
			} else if code == 127 || fbCode == 127 || strings.Contains(fbErrStr, "command not found") {
				return nil, nil
			}
		}

		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				username := parts[0]
				host := "%"
				if len(parts) >= 2 {
					host = parts[1]
				}
				if username == "" {
					username = "(anonymous)"
				}
				users = append(users, DatabaseUser{
					Username: username,
					Host:     host,
					Engine:   engine,
				})
			}
		}
	} else if engine == EnginePostgreSQL {
		cmd := `sudo -u postgres psql -t -A -c "SELECT usename FROM pg_user ORDER BY usename;" 2>/dev/null`
		stdout, stderr, code, err := s.execCmd(ctx, client, cmd)
		if err != nil || code != 0 {
			if strings.Contains(stderr, "command not found") || strings.Contains(stderr, "not found") || code == 127 {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to query PostgreSQL users: %v", err)
		}
		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				users = append(users, DatabaseUser{
					Username: line,
					Host:     "localhost",
					Engine:   engine,
				})
			}
		}
	}

	return users, nil
}

// Enqueue tasks
func (s *Service) CreateDatabase(ctx context.Context, req CreateDatabaseRequest) (*db.TaskRecord, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("database name is required")
	}
	if err := ssh.ValidateIdent(req.Name, "database name"); err != nil {
		return nil, err
	}
	if req.Charset != "" {
		if err := ssh.ValidateCharset(req.Charset); err != nil {
			return nil, err
		}
	}
	if req.Collation != "" {
		if err := ssh.ValidateCharset(req.Collation); err != nil {
			return nil, err
		}
	}
	if req.Charset == "" {
		if req.Engine == EnginePostgreSQL {
			req.Charset = "UTF8"
		} else {
			req.Charset = "utf8mb4"
			req.Collation = "utf8mb4_unicode_ci"
		}
	}

	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"db_create",
		fmt.Sprintf("Create %s database: %s", req.Engine, req.Name),
		string(payload),
	)
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*db.TaskRecord, error) {
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("username and password are required")
	}
	if req.Host == "" {
		req.Host = "localhost"
	}

	if err := ssh.ValidateIdent(req.Username, "database user"); err != nil {
		return nil, err
	}
	if err := ssh.ValidateHostToken(req.Host); err != nil {
		return nil, err
	}
	encPass, err := s.Vault.Encrypt(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt database password: %w", err)
	}
	stored := req
	stored.Password = "vault:" + encPass
	payload, _ := json.Marshal(stored)
	return s.engine.Submit(
		req.ServerID,
		"db_user_create",
		fmt.Sprintf("Create %s user: %s@%s", req.Engine, req.Username, req.Host),
		string(payload),
	)
}

func (s *Service) DeleteDatabase(ctx context.Context, serverID string, engine EngineType, dbName string) (*db.TaskRecord, error) {
	if err := ssh.ValidateIdent(dbName, "database name"); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"engine": string(engine),
		"name":   dbName,
	})

	return s.engine.Submit(
		serverID,
		"db_delete",
		fmt.Sprintf("Drop %s database: %s", engine, dbName),
		string(payload),
	)
}

// DeleteUser drops a database user from MySQL/MariaDB or PostgreSQL.
func (s *Service) DeleteUser(ctx context.Context, serverID string, engine EngineType, username, host string) error {
	switch engine {
	case EngineMySQL, EngineMariaDB, EnginePostgreSQL:
	default:
		return fmt.Errorf("unsupported engine for user deletion: %s", engine)
	}

	if err := ssh.ValidateIdent(username, "database user"); err != nil {
		return err
	}
	if host == "" {
		host = "%"
	}
	if err := ssh.ValidateHostToken(host); err != nil {
		return err
	}

	client, record, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	switch engine {
	case EngineMySQL, EngineMariaDB:
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		query := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s'; FLUSH PRIVILEGES;", username, host)
		out, stderr, code, err := client.ExecStdin(ctx, client.SudoHeader()+mysqlCmd+"\n", strings.NewReader(query))
		if err != nil || code != 0 {
			return fmt.Errorf("failed to drop MySQL user '%s'@'%s': %s %s", username, host, out, stderr)
		}
		return nil

	case EnginePostgreSQL:
		query := fmt.Sprintf("DROP USER IF EXISTS \"%s\";", username)
		out, stderr, code, err := client.ExecStdin(ctx, client.SudoHeader()+"$SUDO -u postgres psql -v ON_ERROR_STOP=1\n", strings.NewReader(query))
		if err != nil || code != 0 {
			return fmt.Errorf("failed to drop PostgreSQL user '%s': %s %s", username, out, stderr)
		}
		return nil

	default:
		return fmt.Errorf("unsupported engine for user deletion: %s", engine)
	}
}

func (s *Service) BackupDatabase(ctx context.Context, req BackupDatabaseRequest) (*db.TaskRecord, error) {
	if req.DatabaseName == "" {
		return nil, fmt.Errorf("database name is required")
	}
	if err := ssh.ValidateIdent(req.DatabaseName, "database name"); err != nil {
		return nil, err
	}
	if req.TargetPath == "" {
		req.TargetPath = "/var/backups/TameStack"
	}
	if err := ssh.ValidateRemotePath(req.TargetPath, false); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"db_backup",
		fmt.Sprintf("Dump %s database: %s", req.Engine, req.DatabaseName),
		string(payload),
	)
}

func (s *Service) RestoreDatabase(ctx context.Context, req RestoreDatabaseRequest) (*db.TaskRecord, error) {
	if req.DatabaseName == "" || req.DumpFilePath == "" {
		return nil, fmt.Errorf("database name and dump file path are required")
	}
	if err := ssh.ValidateIdent(req.DatabaseName, "database name"); err != nil {
		return nil, err
	}
	if err := ssh.ValidateRemotePath(req.DumpFilePath, false); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"db_restore",
		fmt.Sprintf("Restore %s database %s from %s", req.Engine, req.DatabaseName, req.DumpFilePath),
		string(payload),
	)
}

func (s *Service) ServiceControl(ctx context.Context, serverID string, engine EngineType, action string) (*db.TaskRecord, error) {
	if err := ssh.ValidateSystemctlAction(action); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"engine": string(engine),
		"action": action,
	})

	return s.engine.Submit(
		serverID,
		"db_service_control",
		fmt.Sprintf("%s %s service", strings.Title(action), engine),
		string(payload),
	)
}

// --- Task Handlers ---

func (s *Service) handleDBCreateTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, record, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req CreateDatabaseRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidateIdent(req.Name, "database name"); err != nil {
		return "", err
	}
	if req.Charset != "" {
		if err := ssh.ValidateCharset(req.Charset); err != nil {
			return "", err
		}
	}
	if req.Collation != "" {
		if err := ssh.ValidateCharset(req.Collation); err != nil {
			return "", err
		}
	}

	logger.Info(fmt.Sprintf("Creating %s database '%s'...", req.Engine, req.Name))
	logger.SetStep("executing_create_query", 30)

	if req.Engine == EngineMySQL || req.Engine == EngineMariaDB {
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		colSql := ""
		if req.Collation != "" {
			colSql = fmt.Sprintf("COLLATE %s", req.Collation)
		}
		sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s %s;", req.Name, req.Charset, colSql)
		cmd := fmt.Sprintf("%s -e %s", mysqlCmd, ssh.ShellEscape(sql))
		_, stderr, code, err := s.execCmd(ctx, client, cmd)
		if err != nil || code != 0 {
			logger.Error(fmt.Sprintf("Failed to create MySQL database: %s %v", stderr, err))
			return "", fmt.Errorf("MySQL create database error: %s", stderr)
		}
	} else if req.Engine == EnginePostgreSQL {
		sql := fmt.Sprintf("CREATE DATABASE \"%s\" ENCODING '%s';", req.Name, req.Charset)
		cmd := fmt.Sprintf("$SUDO -u postgres psql -c %s", ssh.ShellEscape(sql))
		_, stderr, code, err := s.execCmd(ctx, client, cmd)
		if err != nil || code != 0 {
			logger.Error(fmt.Sprintf("Failed to create PostgreSQL database: %s %v", stderr, err))
			return "", fmt.Errorf("PostgreSQL create database error: %s", stderr)
		}
	} else {
		return "", fmt.Errorf("unsupported engine: %s", req.Engine)
	}

	logger.SetStep("completed", 100)
	logger.Info(fmt.Sprintf("Database '%s' created successfully.", req.Name))
	return fmt.Sprintf("Database '%s' created successfully.", req.Name), nil
}

func (s *Service) handleDBUserCreateTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, record, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req CreateUserRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if strings.HasPrefix(req.Password, "vault:") {
		plain, err := s.Vault.Decrypt(strings.TrimPrefix(req.Password, "vault:"))
		if err != nil {
			return "", fmt.Errorf("failed to decrypt database password: %w", err)
		}
		req.Password = plain
	}
	if err := ssh.ValidateIdent(req.Username, "database user"); err != nil {
		return "", err
	}
	if req.Host == "" {
		req.Host = "localhost"
	}
	if err := ssh.ValidateHostToken(req.Host); err != nil {
		return "", err
	}
	for _, dbName := range req.Databases {
		if err := ssh.ValidateIdent(dbName, "database name"); err != nil {
			return "", err
		}
	}
	if strings.ContainsAny(req.Password, "\r\n") {
		return "", fmt.Errorf("database password contains a newline")
	}

	logger.Info(fmt.Sprintf("Creating %s user '%s'@'%s'...", req.Engine, req.Username, req.Host))
	logger.SetStep("creating_user", 30)

	if req.Engine == EngineMySQL || req.Engine == EngineMariaDB {
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		escapedPass := strings.ReplaceAll(req.Password, "'", "''")
		createSql := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';\nFLUSH PRIVILEGES;\n", req.Username, req.Host, escapedPass)
		for _, dbName := range req.Databases {
			createSql += fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s';\n", dbName, req.Username, req.Host)
		}
		cmd := client.SudoHeader() + mysqlCmd + "\n"
		_, stderr, code, err := client.ExecStdin(ctx, cmd, strings.NewReader(createSql))
		if err != nil || code != 0 {
			return "", fmt.Errorf("create user error: %s %v", stderr, err)
		}
	} else if req.Engine == EnginePostgreSQL {
		escapedPass := strings.ReplaceAll(req.Password, "'", "''")
		createSql := fmt.Sprintf("CREATE USER \"%s\" WITH ENCRYPTED PASSWORD '%s';\n", req.Username, escapedPass)
		for _, dbName := range req.Databases {
			createSql += fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\";\n", dbName, req.Username)
		}
		stdin := client.SudoHeader() + "$SUDO -u postgres psql -v ON_ERROR_STOP=1\n"
		_, stderr, code, err := client.ExecStdin(ctx, stdin, strings.NewReader(createSql))
		if err != nil || code != 0 {
			return "", fmt.Errorf("create postgres user error: %s %v", stderr, err)
		}
		for _, dbName := range req.Databases {
			logger.Info(fmt.Sprintf("Granted all privileges on \"%s\" to \"%s\"", dbName, req.Username))
		}
	}

	logger.SetStep("completed", 100)
	logger.Info(fmt.Sprintf("User '%s' configured successfully.", req.Username))
	return fmt.Sprintf("User '%s' created successfully.", req.Username), nil
}

func (s *Service) handleDBDeleteTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, record, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var payload struct {
		Engine string `json:"engine"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidateIdent(payload.Name, "database name"); err != nil {
		return "", err
	}

	logger.Warn(fmt.Sprintf("Dropping %s database '%s'...", payload.Engine, payload.Name))
	logger.SetStep("dropping_database", 50)

	if payload.Engine == string(EngineMySQL) || payload.Engine == string(EngineMariaDB) {
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", payload.Name)
		cmd := fmt.Sprintf("%s -e %s", mysqlCmd, ssh.ShellEscape(sql))
		_, stderr, code, err := s.execCmd(ctx, client, cmd)
		if err != nil || code != 0 {
			return "", fmt.Errorf("drop MySQL database error: %s %v", strings.TrimSpace(stderr), err)
		}
	} else if payload.Engine == string(EnginePostgreSQL) {
		sql := fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\";", payload.Name)
		cmd := fmt.Sprintf("$SUDO -u postgres psql -c %s", ssh.ShellEscape(sql))
		_, stderr, code, err := s.execCmd(ctx, client, cmd)
		if err != nil || code != 0 {
			return "", fmt.Errorf("drop PostgreSQL database error: %s %v", strings.TrimSpace(stderr), err)
		}
	} else {
		return "", fmt.Errorf("unsupported engine: %s", payload.Engine)
	}

	logger.SetStep("completed", 100)
	logger.Info(fmt.Sprintf("Database '%s' dropped.", payload.Name))
	return fmt.Sprintf("Database '%s' dropped successfully.", payload.Name), nil
}

func (s *Service) handleDBBackupTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, record, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req BackupDatabaseRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidateIdent(req.DatabaseName, "database name"); err != nil {
		return "", err
	}
	if req.TargetPath == "" {
		req.TargetPath = "/var/backups/TameStack"
	}
	if err := ssh.ValidateRemotePath(req.TargetPath, false); err != nil {
		return "", err
	}
	dest := ssh.ShellEscape(req.TargetPath)
	if _, stderr, code, err := s.execCmd(ctx, client, fmt.Sprintf("mkdir -p %s && chmod 700 %s", dest, dest)); err != nil || code != 0 {
		return "", fmt.Errorf("failed to prepare backup directory: %s %v", stderr, err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupFile string
	var cmd string

	logger.Info(fmt.Sprintf("Starting dump of %s database '%s'...", req.Engine, req.DatabaseName))
	logger.SetStep("executing_dump", 30)

	if req.Engine == EngineMySQL || req.Engine == EngineMariaDB {
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		dumpBin := "mysqldump"
		if strings.Contains(mysqlCmd, "debian.cnf") {
			if strings.HasPrefix(mysqlCmd, "sudo") {
				dumpBin = "sudo mysqldump --defaults-file=/etc/mysql/debian.cnf"
			} else {
				dumpBin = "mysqldump --defaults-file=/etc/mysql/debian.cnf"
			}
		} else if strings.HasPrefix(mysqlCmd, "sudo") {
			dumpBin = "sudo mysqldump"
		}
		backupFile = fmt.Sprintf("%s/%s_%s.sql.gz", req.TargetPath, req.DatabaseName, timestamp)
		cmd = fmt.Sprintf("%s --single-transaction --quick --routines --triggers %s | gzip > %s", dumpBin, ssh.ShellEscape(req.DatabaseName), ssh.ShellEscape(backupFile))
	} else if req.Engine == EnginePostgreSQL {
		backupFile = fmt.Sprintf("%s/%s_%s.dump", req.TargetPath, req.DatabaseName, timestamp)
		cmd = fmt.Sprintf("sudo -u postgres pg_dump -Fc %s > %s", ssh.ShellEscape(req.DatabaseName), ssh.ShellEscape(backupFile))
	} else {
		return "", fmt.Errorf("backups for engine %s are not supported", req.Engine)
	}

	_, _, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		return "", fmt.Errorf("database backup failed: %v", err)
	}

	sizeOut, _, _, _ := s.execCmd(ctx, client, fmt.Sprintf("stat -c %%s %s 2>/dev/null", ssh.ShellEscape(backupFile)))
	logger.Info(fmt.Sprintf("Backup created at %s (Size: %s bytes)", backupFile, strings.TrimSpace(sizeOut)))
	logger.SetStep("completed", 100)

	return fmt.Sprintf("Backup created: %s", backupFile), nil
}

func (s *Service) handleDBRestoreTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, record, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req RestoreDatabaseRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidateIdent(req.DatabaseName, "database name"); err != nil {
		return "", err
	}
	if err := ssh.ValidateRemotePath(req.DumpFilePath, false); err != nil {
		return "", err
	}

	logger.Info(fmt.Sprintf("Restoring %s database '%s' from %s...", req.Engine, req.DatabaseName, req.DumpFilePath))
	logger.SetStep("restoring_dump", 30)

	var cmd string
	if req.Engine == EngineMySQL || req.Engine == EngineMariaDB {
		mysqlCmd := s.detectMySQLClientCmd(ctx, client, s.getServerSecret(record))
		if strings.HasSuffix(req.DumpFilePath, ".gz") {
			cmd = fmt.Sprintf("gunzip < %s | %s %s", ssh.ShellEscape(req.DumpFilePath), mysqlCmd, ssh.ShellEscape(req.DatabaseName))
		} else {
			cmd = fmt.Sprintf("%s %s < %s", mysqlCmd, ssh.ShellEscape(req.DatabaseName), ssh.ShellEscape(req.DumpFilePath))
		}
	} else if req.Engine == EnginePostgreSQL {
		cmd = fmt.Sprintf("sudo -u postgres pg_restore --clean --if-exists -d %s %s", ssh.ShellEscape(req.DatabaseName), ssh.ShellEscape(req.DumpFilePath))
	} else {
		return "", fmt.Errorf("restores for engine %s are not supported", req.Engine)
	}

	_, _, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		return "", fmt.Errorf("restore error: %v", err)
	}

	logger.SetStep("completed", 100)
	logger.Info(fmt.Sprintf("Database '%s' restored successfully.", req.DatabaseName))
	return fmt.Sprintf("Database '%s' restored successfully.", req.DatabaseName), nil
}

func (s *Service) handleDBServiceControlTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, _, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var payload struct {
		Engine string `json:"engine"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	logger.Info(fmt.Sprintf("Executing service %s for %s...", payload.Action, payload.Engine))

	cmd, err := serviceControlScript(EngineType(payload.Engine), payload.Action)
	if err != nil {
		return "", err
	}

	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return "", fmt.Errorf("service control failed (code %d): %s %s", code, strings.TrimSpace(string(out)), strings.TrimSpace(string(stderr)))
	}

	logger.SetStep("completed", 100)
	logger.Info(fmt.Sprintf("Service %s %s completed successfully.", payload.Engine, payload.Action))
	return fmt.Sprintf("Service %s %s completed.", payload.Engine, payload.Action), nil
}
