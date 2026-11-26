package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"

	"github.com/google/uuid"
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

	engine.RegisterHandler("backup_create", s.handleBackupCreateTask)
	engine.RegisterHandler("backup_restore", s.handleBackupRestoreTask)
	engine.RegisterHandler("backup_delete", s.handleDeleteBackupTask)

	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string) (string, string, int, error) {
	stdout, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	return string(stdout), string(stderr), code, err
}

// ListBackups returns all created archives for a server.
func (s *Service) ListBackups(ctx context.Context, serverID string) ([]*db.BackupRecord, error) {
	return s.Database.ListBackups(ctx, serverID)
}

// InspectArchive performs preflight inspection of an archive.
func (s *Service) InspectArchive(ctx context.Context, serverID, filePath string) (*ArchiveInspectionResult, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return InspectArchive(ctx, client, filePath)
}

// DownloadBackup downloads a remote archive over SFTP to a local path.
func (s *Service) DownloadBackup(ctx context.Context, serverID, backupID, localDestPath string, progress DownloadProgressReporter) (string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	backupRec, err := s.Database.GetBackup(ctx, backupID)
	if err != nil {
		return "", fmt.Errorf("backup record not found: %w", err)
	}

	checksum, err := DownloadArchiveFromRemote(ctx, client, backupRec.FilePath, localDestPath, progress)
	if err != nil {
		return "", err
	}

	// Mark backup as offsite and downloaded in database.
	_ = s.Database.MarkBackupOffsite(ctx, backupID, time.Now())
	_ = s.Database.MarkBackupVerified(ctx, backupID, true, checksum)

	return checksum, nil
}

// VerifyBackup checks remote archive integrity and updates verified status in database.
func (s *Service) VerifyBackup(ctx context.Context, serverID, backupID string) (bool, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return false, err
	}

	backupRec, err := s.Database.GetBackup(ctx, backupID)
	if err != nil {
		return false, fmt.Errorf("backup record not found: %w", err)
	}

	if err := CheckArchiveIntegrity(ctx, client, backupRec.FilePath); err != nil {
		return false, err
	}

	_ = s.Database.MarkBackupVerified(ctx, backupID, true, "")
	return true, nil
}

// CreateBackup enqueues a backup task.
func (s *Service) CreateBackup(ctx context.Context, req CreateBackupRequest) (*db.TaskRecord, error) {
	if req.TargetPath == "" {
		req.TargetPath = "/var/backups/TameStack"
	}
	if req.RetentionDays <= 0 {
		req.RetentionDays = 14
	}

	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"backup_create",
		fmt.Sprintf("Создание бэкапа (%s: %s)", req.BackupType, req.TargetName),
		string(payload),
	)
}

// RestoreBackup enqueues an archive restore task.
func (s *Service) RestoreBackup(ctx context.Context, req RestoreBackupRequest) (*db.TaskRecord, error) {
	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"backup_restore",
		fmt.Sprintf("Восстановление из архива: %s", path.Base(req.FilePath)),
		string(payload),
	)
}

// DeleteBackup removes archive from disk and database.
func (s *Service) DeleteBackup(ctx context.Context, serverID string, backupID string) error {
	target, err := s.Database.GetBackup(ctx, backupID)
	if err != nil {
		return err
	}

	if target != nil && target.FilePath != "" {
		if client, _, err := s.GetSSHClient(ctx, serverID); err == nil {
			s.execCmd(ctx, client, fmt.Sprintf("rm -f %s", ssh.ShellEscape(target.FilePath)))
		}
	}

	return s.Database.DeleteBackup(ctx, backupID)
}

func parseHomeDir(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "/") {
			return line
		}
	}
	return ""
}

// --- Task Handlers ---

func (s *Service) handleBackupCreateTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, _, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req CreateBackupRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	if err := ssh.ValidateRemotePath(req.TargetPath, false); err != nil {
		return "", err
	}
	if req.TargetName != "" {
		if err := ssh.ValidateToken(req.TargetName, "backup target"); err != nil {
			return "", err
		}
	}

	logger.Info(fmt.Sprintf("Подготовка бэкапа %s для '%s'...", req.BackupType, req.TargetName))
	logger.SetStep("preparing_directories", 10)

	if strings.HasPrefix(req.TargetPath, "~") {
		outHome, _, _, _ := client.NewCommand("echo $HOME").Exec(ctx)
		homeDir := parseHomeDir(string(outHome))
		if homeDir != "" {
			req.TargetPath = strings.Replace(req.TargetPath, "~", homeDir, 1)
		}
	}

	dest := ssh.ShellEscape(req.TargetPath)
	if _, stderr, code, err := s.execCmd(ctx, client, fmt.Sprintf("mkdir -p %s && chmod 755 %s", dest, dest)); err != nil || code != 0 {
		logger.Warn(fmt.Sprintf("Не удалось создать целевой каталог %s (%s). Пробуем создать резервный каталог в домашней директории...", req.TargetPath, strings.TrimSpace(stderr)))
		outHome, _, _, _ := client.NewCommand("echo $HOME").Exec(ctx)
		homeDir := parseHomeDir(string(outHome))
		if homeDir == "" {
			outPwd, _, _, _ := client.NewCommand("pwd").Exec(ctx)
			homeDir = parseHomeDir(string(outPwd))
		}
		if homeDir != "" {
			fallbackPath := path.Join(homeDir, ".TameStack", "backups")
			fallbackDest := ssh.ShellEscape(fallbackPath)
			if _, fallbackErr, fallbackCode, _ := client.NewCommand(fmt.Sprintf("mkdir -p %s && chmod 755 %s", fallbackDest, fallbackDest)).Exec(ctx); fallbackCode == 0 {
				req.TargetPath = fallbackPath
				dest = fallbackDest
				logger.Info(fmt.Sprintf("Каталог бэкапов переключен на: %s", req.TargetPath))
			} else {
				return "", fmt.Errorf("failed to prepare backup directory: %s (fallback also failed: %s)", stderr, fallbackErr)
			}
		} else {
			return "", fmt.Errorf("failed to prepare backup directory: %s %v", stderr, err)
		}
	}

	// Check free disk space before archive creation.
	logger.SetStep("checking_disk_space", 15)
	outDF, _, _, _ := s.execCmd(ctx, client, fmt.Sprintf("df -k %s 2>/dev/null | tail -n 1 | awk '{print $4}'", dest))
	freeKB, _ := strconv.ParseInt(strings.TrimSpace(outDF), 10, 64)
	if freeKB > 0 && freeKB < 100*1024 { // Less than 100MB
		return "", fmt.Errorf("недостаточно свободного места на целевом разделе для бэкапа: свободно %d МБ, требуется минимум 100 МБ", freeKB/1024)
	}

	timestamp := time.Now().Format("20060102_150405")
	var archiveFile string
	var cmd string

	switch req.BackupType {
	case "site":
		archiveFile = fmt.Sprintf("%s/site_%s_%s.tar.gz", req.TargetPath, req.TargetName, timestamp)
		logger.SetStep("archiving_website_files", 30)

		if req.TargetName == "all" || req.TargetName == "." {
			cmd = fmt.Sprintf("tar -czf %s -C /var/www .", ssh.ShellEscape(archiveFile))
			break
		}

		// Candidate site directories, most trusted first: DocumentRoot recorded
		// in the app DB, then the /var/www/<domain> convention, then TargetName
		// as a full path (previous behavior).
		candidates := []string{"/var/www/" + req.TargetName, req.TargetName}
		if sites, sErr := s.Database.ListSites(ctx, task.ServerID); sErr == nil {
			for _, st := range sites {
				if st != nil && st.Domain == req.TargetName && st.DocumentRoot != "" {
					candidates = append([]string{st.DocumentRoot}, candidates...)
					break
				}
			}
		}
		quoted := make([]string, 0, len(candidates))
		for _, c := range candidates {
			quoted = append(quoted, ssh.ShellEscape(c))
		}
		// TargetName is ValidateToken'd above, so it is safe inside the message.
		findDir := "dir=\"\"; for c in " + strings.Join(quoted, " ") + "; do " +
			"if [ -d \"$c\" ]; then dir=\"$c\"; break; fi; done; " +
			"if [ -z \"$dir\" ]; then echo \"Site directory for '" + req.TargetName + "' not found (checked DocumentRoot and /var/www)\" >&2; exit 1; fi; " +
			"dirBase=$(basename \"$dir\"); dirParent=$(dirname \"$dir\"); "

		// Check if we should dump a single site database.
		dbDumpName := req.DatabaseName
		if dbDumpName == "" && req.IncludeDatabase {
			cleanTarget := strings.ReplaceAll(req.TargetName, ".", "_")
			cleanTarget = strings.ReplaceAll(cleanTarget, "-", "_")
			dbDumpName = cleanTarget
		}

		if dbDumpName != "" {
			safeDB := ssh.ShellEscape(dbDumpName)
			dumpPath := fmt.Sprintf("/tmp/%s_%s.sql", dbDumpName, timestamp)
			safeDump := ssh.ShellEscape(dumpPath)
			dumpErrPath := fmt.Sprintf("/tmp/%s_%s.sql.err", dbDumpName, timestamp)
			safeDumpErr := ssh.ShellEscape(dumpErrPath)
			dumpBase := fmt.Sprintf("%s_%s.sql", dbDumpName, timestamp)

			if req.DatabaseName != "" {
				// Explicit database requested: fail the task if mysqldump fails so the user doesn't get an incomplete backup silently.
				cmd = fmt.Sprintf("if ! mysqldump --single-transaction --quick %s > %s 2>%s; then err=$(cat %s 2>/dev/null); rm -f %s %s; echo \"mysqldump failed for %s: $err\" >&2; exit 1; fi; rm -f %s; ",
					safeDB, safeDump, safeDumpErr, safeDumpErr, safeDump, safeDumpErr, dbDumpName, safeDumpErr)
			} else {
				// Auto-inferred database from domain: log warning on failure, still archive site files.
				cmd = fmt.Sprintf("if ! mysqldump --single-transaction --quick %s > %s 2>%s; then echo \"Warning: mysqldump for %s failed: $(cat %s 2>/dev/null)\" >&2; rm -f %s; fi; rm -f %s; ",
					safeDB, safeDump, safeDumpErr, dbDumpName, safeDumpErr, safeDump, safeDumpErr)
			}
			cmd = cmd + findDir +
				fmt.Sprintf(
					"if [ -s %s ]; then "+
						"tar -czf %s -C \"$dirParent\" \"$dirBase\" -C /tmp %s; "+
						"else "+
						"tar -czf %s -C \"$dirParent\" \"$dirBase\"; "+
						"fi; "+
						"rm -f %s",
					safeDump,
					ssh.ShellEscape(archiveFile), ssh.ShellEscape(dumpBase),
					ssh.ShellEscape(archiveFile),
					safeDump,
				)
			logger.Info(fmt.Sprintf("В архив включены файлы сайта и отдельный дамп базы данных '%s'.", dbDumpName))
		} else {
			cmd = findDir + fmt.Sprintf("tar -czf %s -C \"$dirParent\" \"$dirBase\"", ssh.ShellEscape(archiveFile))
		}

	case "database":
		archiveFile = fmt.Sprintf("%s/db_%s_%s.sql.gz", req.TargetPath, req.TargetName, timestamp)
		logger.SetStep("dumping_database", 30)
		if req.TargetName == "all" || req.TargetName == "" {
			cmd = fmt.Sprintf("mysqldump --all-databases --single-transaction --quick | gzip > %s", ssh.ShellEscape(archiveFile))
		} else {
			cmd = fmt.Sprintf("mysqldump --single-transaction --quick %s | gzip > %s", ssh.ShellEscape(req.TargetName), ssh.ShellEscape(archiveFile))
		}

	case "config":
		archiveFile = fmt.Sprintf("%s/configs_%s.tar.gz", req.TargetPath, timestamp)
		logger.SetStep("archiving_system_configs", 30)
		cmd = fmt.Sprintf(`TARGETS=""
for p in /etc/nginx /etc/apache2 /etc/httpd /etc/php /etc/mysql /etc/redis /etc/memcached; do
    if [ -e "$p" ]; then
        TARGETS="$TARGETS $p"
    fi
done
if [ -z "$TARGETS" ]; then
    echo "No standard configuration directories found to back up" >&2
    exit 1
fi
tar -czf %s $TARGETS`, ssh.ShellEscape(archiveFile))

	case "full":
		archiveFile = fmt.Sprintf("%s/full_backup_%s.tar.gz", req.TargetPath, timestamp)
		dumpFile := fmt.Sprintf("%s/all_databases_%s.sql", req.TargetPath, timestamp)
		logger.SetStep("generating_full_backup", 30)
		cmd = fmt.Sprintf(`DUMP_FILE=%s
ARCHIVE_FILE=%s
TARGET_DIR=%s
if command -v mysqldump >/dev/null 2>&1; then
    mysqldump --all-databases > "$DUMP_FILE" 2>/dev/null || rm -f "$DUMP_FILE"
fi
DIRS=""
for d in var/www etc/nginx etc/apache2 etc/httpd etc/php etc/mysql; do
    if [ -e "/$d" ]; then
        DIRS="$DIRS $d"
    fi
done
if [ -f "$DUMP_FILE" ]; then
    dumpBase=$(basename "$DUMP_FILE")
    tar -czf "$ARCHIVE_FILE" -C / $DIRS -C "$TARGET_DIR" "$dumpBase"
    rm -f "$DUMP_FILE"
else
    if [ -z "$DIRS" ]; then
        echo "No data or configuration directories found for full backup" >&2
        exit 1
    fi
    tar -czf "$ARCHIVE_FILE" -C / $DIRS
fi`, ssh.ShellEscape(dumpFile), ssh.ShellEscape(archiveFile), ssh.ShellEscape(req.TargetPath))

	default:
		return "", fmt.Errorf("unsupported backup type: %s", req.BackupType)
	}

	logger.Info(fmt.Sprintf("Выполнение команды создания бэкапа: %s", cmd))
	_, stderr, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		logger.Error(fmt.Sprintf("Backup failed: %s %v", stderr, err))
		return "", fmt.Errorf("backup failed: %s", stderr)
	}

	// Verify file size.
	sizeOut, _, _, _ := s.execCmd(ctx, client, fmt.Sprintf("stat -c %%s %s 2>/dev/null", ssh.ShellEscape(archiveFile)))
	sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(sizeOut), 10, 64)

	// Ensure readable permissions on the created archive.
	_, _, _, _ = s.execCmd(ctx, client, fmt.Sprintf("chmod 644 %s", ssh.ShellEscape(archiveFile)))

	// Verify archive integrity immediately after creation.
	logger.SetStep("verifying_archive_integrity", 60)
	if err := CheckArchiveIntegrity(ctx, client, archiveFile); err != nil {
		s.execCmd(ctx, client, fmt.Sprintf("rm -f %s", ssh.ShellEscape(archiveFile)))
		logger.Error(fmt.Sprintf("Созданный архив не прошел проверку целостности: %v", err))
		return "", fmt.Errorf("созданный архив поврежден или не может быть прочитан: %w", err)
	}
	logger.Info("Проверка целостности архива успешно пройдена (tar/gzip check OK).")

	logger.Info(fmt.Sprintf("Архив успешно создан: %s (размер: %d байт)", archiveFile, sizeBytes))
	logger.SetStep("applying_retention_policy", 80)

	// Safe retention policy.
	if req.RetentionDays > 0 {
		existingBackups, _ := s.Database.ListBackups(ctx, task.ServerID)
		targetCount := 0
		for _, b := range existingBackups {
			if b.TargetName == req.TargetName && b.BackupType == req.BackupType {
				targetCount++
			}
		}
		if targetCount <= 1 {
			logger.Info("Ротация пропущена: на сервере осталась единственная копия для этой цели, она защищена от удаления.")
		} else {
			cleanCmd := fmt.Sprintf("find %s -type f -mtime +%d -delete", ssh.ShellEscape(req.TargetPath), req.RetentionDays)
			s.execCmd(ctx, client, cleanCmd)
			logger.Info(fmt.Sprintf("Применена политика хранения: удалены копии старше %d дней.", req.RetentionDays))
		}
	}

	record := &db.BackupRecord{
		ID:          uuid.New().String(),
		ServerID:    task.ServerID,
		BackupType:  req.BackupType,
		TargetName:  req.TargetName,
		Destination: req.Destination,
		FilePath:    archiveFile,
		SizeBytes:   sizeBytes,
		Status:      "completed",
		Offsite:     false, // local-only until downloaded or sent offsite
		Verified:    true,
		CreatedAt:   time.Now(),
	}
	_ = s.Database.SaveBackup(ctx, record)

	logger.SetStep("completed", 100)
	logger.Info(fmt.Sprintf("Создание бэкапа %s успешно завершено.", path.Base(archiveFile)))
	return fmt.Sprintf("Backup created: %s (%d bytes)", archiveFile, sizeBytes), nil
}

func (s *Service) handleBackupRestoreTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, _, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req RestoreBackupRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	logger.Info(fmt.Sprintf("Запуск восстановления из архива %s...", req.FilePath))
	logger.SetStep("inspecting_archive_safety", 15)

	// Preflight inspection of archive contents.
	inspection, err := InspectArchive(ctx, client, req.FilePath)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать структуру архива: %w", err)
	}
	if !inspection.IntegrityPassed {
		return "", fmt.Errorf("архив повреждён или обрезан: %s", inspection.ErrorMessage)
	}

	// Verify dangerous system paths.
	if inspection.HasSystemPaths && !req.AllowSystemPaths {
		dangerousList := strings.Join(inspection.DangerousPaths, ", ")
		logger.Error(fmt.Sprintf("Отказ восстановления: обнаружены системные пути (%s)", dangerousList))
		return "", fmt.Errorf("восстановление заблокировано политикой безопасности TameStack: архив содержит критические системные пути (%s). Для распаковки требуется явное согласие 'Разрешить системные пути'", dangerousList)
	}

	if req.DryRun {
		logger.Info(fmt.Sprintf("Режим DryRun (проверка): архив корректен, содержит %d файлов. Восстановление не производилось.", inspection.TotalEntries))
		return fmt.Sprintf("Архив проверен (dry-run): %d файлов, целостность OK", inspection.TotalEntries), nil
	}

	logger.SetStep("restoring_archive", 40)
	var cmd string

	if strings.HasSuffix(req.FilePath, ".sql.gz") {
		dbName := req.TargetName
		if dbName != "" && dbName != "all" {
			if err := ssh.ValidateIdent(dbName, "database name"); err != nil {
				return "", err
			}
		}
		if dbName == "" || dbName == "all" {
			cmd = fmt.Sprintf("gunzip < %s | mysql", ssh.ShellEscape(req.FilePath))
		} else {
			cmd = fmt.Sprintf("gunzip < %s | mysql %s", ssh.ShellEscape(req.FilePath), ssh.ShellEscape(dbName))
		}
	} else if strings.HasSuffix(req.FilePath, ".tar.gz") || strings.HasSuffix(req.FilePath, ".tgz") {
		destDir := ""
		if req.TargetType == "site" && req.TargetName != "" {
			// Site target names are domains used to build the docroot path.
			if err := ssh.ValidateHostToken(req.TargetName); err != nil {
				return "", err
			}
			destDir = "/var/www/" + req.TargetName
			if req.TargetDirectory != "" {
				destDir = req.TargetDirectory
			}
		} else if req.TargetDirectory != "" {
			destDir = req.TargetDirectory
		}
		if destDir == "" {
			return "", fmt.Errorf("не указана целевая директория распаковки: архив не будет извлечён в /")
		}
		if err := ssh.ValidateRemotePath(destDir, false); err != nil {
			return "", err
		}
		destEscaped := ssh.ShellEscape(destDir)
		cmd = fmt.Sprintf("mkdir -p %s && tar -xzf %s -C %s", destEscaped, ssh.ShellEscape(req.FilePath), destEscaped)
	} else {
		return "", fmt.Errorf("неподдерживаемый формат архива: %s", req.FilePath)
	}

	stdout, stderr, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		return "", fmt.Errorf("ошибка выполнения команды восстановления: %s %s %v", stdout, stderr, err)
	}

	// If the archive contained a database dump alongside site files, import it into the database.
	if inspection.ContainsDBDump && inspection.DBDumpName != "" && req.TargetType == "site" {
		logger.SetStep("restoring_linked_database", 75)
		if base := path.Base(inspection.DBDumpName); base != inspection.DBDumpName || base == "." || base == "/" {
			return "", fmt.Errorf("имя дампа в архиве недопустимо: %s", inspection.DBDumpName)
		}
		destDir := "/var/www/" + req.TargetName
		if req.TargetDirectory != "" {
			destDir = req.TargetDirectory
		}
		dumpFilePath := path.Join(destDir, inspection.DBDumpName)
		if err := ssh.ValidateRemotePath(dumpFilePath, false); err != nil {
			return "", err
		}
		cleanDBName := strings.ReplaceAll(req.TargetName, ".", "_")
		cleanDBName = strings.ReplaceAll(cleanDBName, "-", "_")
		if err := ssh.ValidateIdent(cleanDBName, "database name"); err != nil {
			return "", err
		}

		importCmd := fmt.Sprintf(
			"if [ -f %s ]; then mysql %s < %s; rm -f %s; else echo 'dump file not found' >&2; exit 1; fi",
			ssh.ShellEscape(dumpFilePath), ssh.ShellEscape(cleanDBName), ssh.ShellEscape(dumpFilePath), ssh.ShellEscape(dumpFilePath),
		)
		stdout, stderr, code, err := s.execCmd(ctx, client, importCmd)
		if err != nil || code != 0 {
			logger.Warn(fmt.Sprintf("Дамп базы данных %s не применён: %s %s", cleanDBName, stdout, stderr))
		} else {
			logger.Info(fmt.Sprintf("Дамп базы данных %s успешно применен.", cleanDBName))
		}
	}

	logger.SetStep("completed", 100)
	logger.Info("Архив успешно восстановлен.")
	return "Restore completed successfully", nil
}
