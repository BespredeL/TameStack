package files

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	if engine != nil {
		engine.RegisterHandler("files_disk_cleanup", s.handleDiskCleanupTask)
	}
	return s
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ListDirectory lists files and folders inside remotePath.
// If direct SFTP read fails with permission denied, it automatically attempts elevation via sudo.
func (s *Service) ListDirectory(ctx context.Context, serverID string, remotePath string) (*DirectoryListing, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if remotePath == "" || remotePath == "." {
		remotePath = "/var/www"
	}
	remotePath = path.Clean(filepath.ToSlash(remotePath))

	sc, err := client.SFTP()
	if err != nil {
		return nil, fmt.Errorf("sftp init error: %w", err)
	}

	// Check if directory exists.
	stat, err := sc.Stat(remotePath)
	if err != nil || !stat.IsDir() {
		// Verify if it exists via sudo before falling back to /
		testDirCmd := fmt.Sprintf("if test -d %s; then echo 'EXISTS'; fi", ssh.ShellEscape(remotePath))
		out, _, _, _ := client.NewCommand(testDirCmd).WithSudo().Exec(ctx)
		if strings.TrimSpace(string(out)) != "EXISTS" {
			remotePath = "/"
		}
	}

	fileInfos, err := sc.ReadDir(remotePath)
	if err != nil {
		// Fallback to sudo directory listing.
		return s.listDirectoryWithSudo(ctx, client, remotePath)
	}

	entries := []FileEntry{}
	for _, fi := range fileInfos {
		fullPath := path.Join(remotePath, fi.Name())
		entries = append(entries, FileEntry{
			Name:        fi.Name(),
			Path:        fullPath,
			Size:        fi.Size(),
			HumanSize:   formatBytes(fi.Size()),
			IsDir:       fi.IsDir(),
			Permissions: fi.Mode().String(),
			ModTime:     fi.ModTime(),
			Extension:   strings.ToLower(filepath.Ext(fi.Name())),
		})
	}

	// Sort directories first, then alphabetically.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parentPath := path.Dir(remotePath)
	if parentPath == remotePath {
		parentPath = ""
	}

	return &DirectoryListing{
		CurrentPath: remotePath,
		ParentPath:  parentPath,
		TotalItems:  len(entries),
		Entries:     entries,
	}, nil
}

func (s *Service) listDirectoryWithSudo(ctx context.Context, client *ssh.SSHClient, remotePath string) (*DirectoryListing, error) {
	script := fmt.Sprintf(`DIR=%s
if ! run_sudo test -d "$DIR"; then
    echo "DIR_NOT_FOUND"
    exit 1
fi

if command -v python3 >/dev/null 2>&1; then
    run_sudo python3 -c '
import os, sys, json, stat

d = sys.argv[1]
items = []
try:
    for name in os.listdir(d):
        if name in [".", ".."]:
            continue
        p = os.path.join(d, name)
        try:
            st = os.lstat(p)
            is_dir = stat.S_ISDIR(st.st_mode)
            ext = os.path.splitext(name)[1].lower() if not is_dir else ""
            items.append({
                "name": name,
                "path": p,
                "size": st.st_size,
                "is_dir": is_dir,
                "permissions": stat.filemode(st.st_mode),
                "mod_time": int(st.st_mtime),
                "extension": ext
            })
        except Exception:
            pass
    print("===JSON_START===" + json.dumps(items) + "===JSON_END===")
except Exception as e:
    sys.exit(1)
' "$DIR"
    exit 0
fi

echo "===STAT_START==="
run_sudo stat -c '%%F|%%s|%%Y|%%a|%%n' "$DIR"/* "$DIR"/.* 2>/dev/null
echo "===STAT_END==="
`, ssh.ShellEscape(remotePath))

	out, stderr, code, err := client.NewCommand(script).WithSudo().Exec(ctx)
	outStr := string(out)
	if err != nil || code != 0 || strings.Contains(outStr, "DIR_NOT_FOUND") {
		return nil, fmt.Errorf("failed to read remote directory %s (code %d): %s %s", remotePath, code, outStr, string(stderr))
	}

	entries := []FileEntry{}
	if strings.Contains(outStr, "===JSON_START===") && strings.Contains(outStr, "===JSON_END===") {
		start := strings.Index(outStr, "===JSON_START===") + len("===JSON_START===")
		end := strings.Index(outStr, "===JSON_END===")
		jsonStr := outStr[start:end]

		type pyEntry struct {
			Name        string `json:"name"`
			Path        string `json:"path"`
			Size        int64  `json:"size"`
			IsDir       bool   `json:"is_dir"`
			Permissions string `json:"permissions"`
			ModTime     int64  `json:"mod_time"`
			Extension   string `json:"extension"`
		}
		var pyItems []pyEntry
		if err := json.Unmarshal([]byte(jsonStr), &pyItems); err == nil {
			for _, item := range pyItems {
				entries = append(entries, FileEntry{
					Name:        item.Name,
					Path:        item.Path,
					Size:        item.Size,
					HumanSize:   formatBytes(item.Size),
					IsDir:       item.IsDir,
					Permissions: item.Permissions,
					ModTime:     time.Unix(item.ModTime, 0),
					Extension:   item.Extension,
				})
			}
		}
	} else if strings.Contains(outStr, "===STAT_START===") {
		lines := strings.Split(outStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "===") {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 5 {
				fileType := parts[0]
				size, _ := strconv.ParseInt(parts[1], 10, 64)
				mtimeUnix, _ := strconv.ParseInt(parts[2], 10, 64)
				perm := parts[3]
				fullPath := parts[4]
				name := path.Base(fullPath)
				if name == "." || name == ".." || fullPath == remotePath || fullPath == remotePath+"/*" {
					continue
				}
				isDir := strings.Contains(strings.ToLower(fileType), "directory")
				ext := ""
				if !isDir {
					ext = strings.ToLower(filepath.Ext(name))
				}
				entries = append(entries, FileEntry{
					Name:        name,
					Path:        fullPath,
					Size:        size,
					HumanSize:   formatBytes(size),
					IsDir:       isDir,
					Permissions: perm,
					ModTime:     time.Unix(mtimeUnix, 0),
					Extension:   ext,
				})
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parentPath := path.Dir(remotePath)
	if parentPath == remotePath {
		parentPath = ""
	}

	return &DirectoryListing{
		CurrentPath: remotePath,
		ParentPath:  parentPath,
		TotalItems:  len(entries),
		Entries:     entries,
	}, nil
}

// ReadFile retrieves contents of a text file up to 5MB.
// Automatically falls back to sudo elevation if direct SFTP access is restricted.
func (s *Service) ReadFile(ctx context.Context, serverID string, remotePath string) (*FileContentResult, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	remotePath = path.Clean(filepath.ToSlash(remotePath))
	sc, err := client.SFTP()
	if err != nil {
		return nil, err
	}

	var fileSize int64
	var filePerm string
	var isDir bool

	fi, err := sc.Stat(remotePath)
	if err == nil {
		fileSize = fi.Size()
		filePerm = fi.Mode().String()
		isDir = fi.IsDir()
	} else {
		// Check file metadata via sudo.
		statCmd := fmt.Sprintf("stat -c '%%s|%%F|%%a' %s 2>/dev/null", ssh.ShellEscape(remotePath))
		out, _, code, _ := client.NewCommand(statCmd).WithSudo().Exec(ctx)
		outStr := strings.TrimSpace(string(out))
		if code == 0 && outStr != "" {
			parts := strings.Split(outStr, "|")
			if len(parts) >= 3 {
				fileSize, _ = strconv.ParseInt(parts[0], 10, 64)
				isDir = strings.Contains(strings.ToLower(parts[1]), "directory")
				filePerm = parts[2]
			}
		} else {
			return nil, fmt.Errorf("file not found: %w", err)
		}
	}

	if isDir {
		return nil, fmt.Errorf("%s is a directory", remotePath)
	}

	if fileSize > 5*1024*1024 {
		return nil, fmt.Errorf("file size exceeds 5MB limit for inline editor")
	}

	data, err := client.ReadFile(ctx, remotePath)
	if err != nil {
		return nil, err
	}

	isBin := !utf8.Valid(data)
	content := ""
	if !isBin {
		content = string(data)
	}

	return &FileContentResult{
		Path:        remotePath,
		Content:     content,
		SizeBytes:   fileSize,
		IsBinary:    isBin,
		Permissions: filePerm,
	}, nil
}

// SaveFile overwrites or saves a remote text file with optional timestamped backup.
func (s *Service) SaveFile(ctx context.Context, req SaveFileRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	req.Path = path.Clean(filepath.ToSlash(req.Path))

	var backupPath string
	if req.CreateBackup || strings.Contains(req.Path, "/etc/nginx") || strings.Contains(req.Path, "/etc/apache2") || strings.Contains(req.Path, "/etc/httpd") {
		timestamp := time.Now().Format("20060102_150405")
		backupPath = fmt.Sprintf("%s.bak_%s", req.Path, timestamp)
		backupCmd := fmt.Sprintf("cp %s %s 2>/dev/null || true", ssh.ShellEscape(req.Path), ssh.ShellEscape(backupPath))
		_, _, _, _ = client.NewCommand(backupCmd).WithSudo().Exec(ctx)
	}

	sc, err := client.SFTP()
	if err != nil {
		return err
	}

	// Preserve existing permissions if available.
	mode := os.FileMode(0644)
	if fi, err := sc.Stat(req.Path); err == nil {
		mode = fi.Mode()
	}

	if err := client.WriteFile(ctx, req.Path, []byte(req.Content), mode); err != nil {
		return err
	}

	// Automated rollback if syntax validation fails.
	return validateAndRollbackOnFailure(ctx, client, req.Path, backupPath)
}

func validateAndRollbackOnFailure(ctx context.Context, client *ssh.SSHClient, filePath, backupPath string) error {
	var testCmd string
	if strings.Contains(filePath, "/etc/nginx") {
		testCmd = "nginx -t 2>&1"
	} else if strings.Contains(filePath, "/etc/apache2") || strings.Contains(filePath, "/etc/httpd") {
		testCmd = "apache2ctl configtest 2>&1 || apachectl configtest 2>&1 || httpd -t 2>&1"
	}

	if testCmd == "" {
		return nil
	}

	out, _, code, _ := client.NewCommand(testCmd).WithSudo().Exec(ctx)
	outStr := string(out)
	if code != 0 || (strings.Contains(filePath, "/etc/nginx") && !strings.Contains(outStr, "syntax is ok")) ||
		((strings.Contains(filePath, "/etc/apache2") || strings.Contains(filePath, "/etc/httpd")) && !strings.Contains(strings.ToLower(outStr), "syntax ok")) {

		if backupPath != "" {
			rollbackCmd := fmt.Sprintf("cp -f %s %s", ssh.ShellEscape(backupPath), ssh.ShellEscape(filePath))
			_, _, _, _ = client.NewCommand(rollbackCmd).WithSudo().Exec(ctx)
		}

		return fmt.Errorf("configuration syntax validation failed; reverted to backup: %s", strings.TrimSpace(outStr))
	}

	return nil
}

// CreateEntry creates a new folder or empty file.
func (s *Service) CreateEntry(ctx context.Context, req CreateEntryRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	cleanPath := path.Clean(filepath.ToSlash(req.Path))

	if req.IsDir {
		return client.EnsureDirectory(ctx, cleanPath, 0755, "")
	}

	// Create empty file (client.WriteFile has sudo elevation fallback).
	return client.WriteFile(ctx, cleanPath, []byte(""), 0644)
}

// DeleteEntry removes file or directory safely.
func (s *Service) DeleteEntry(ctx context.Context, req DeleteEntryRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	cleanPath := path.Clean(filepath.ToSlash(req.Path))
	// Prevent accidental catastrophic root deletions.
	if cleanPath == "/" || cleanPath == "" || cleanPath == "/etc" || cleanPath == "/bin" || cleanPath == "/usr" || cleanPath == "/var" {
		return fmt.Errorf("refusing to delete protected root directory: %s", cleanPath)
	}

	return client.RemovePath(ctx, cleanPath, true)
}

// RenameEntry renames or moves a file or folder.
func (s *Service) RenameEntry(ctx context.Context, req RenameEntryRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	oldP := path.Clean(filepath.ToSlash(req.OldPath))
	newP := path.Clean(filepath.ToSlash(req.NewPath))

	sc, err := client.SFTP()
	if err == nil {
		if err := sc.Rename(oldP, newP); err == nil {
			return nil
		}
	}

	cmd := fmt.Sprintf("mv %s %s", ssh.ShellEscape(oldP), ssh.ShellEscape(newP))
	out, stderr, code, execErr := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if execErr != nil || code != 0 {
		return fmt.Errorf("failed to rename %s to %s: %s %s", oldP, newP, string(out), string(stderr))
	}
	return nil
}

// Chmod modifies entry permissions.
func (s *Service) Chmod(ctx context.Context, req ChmodRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	cleanPath := path.Clean(filepath.ToSlash(req.Path))
	if err := ssh.ValidateChmodMode(req.Mode); err != nil {
		return err
	}
	if err := ssh.ValidateRemotePath(cleanPath, true); err != nil {
		return err
	}
	cmd := fmt.Sprintf("chmod %s %s", ssh.ShellEscape(req.Mode), ssh.ShellEscape(cleanPath))
	out, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("chmod failed: %s %s", string(out), string(stderr))
	}
	return nil
}

// AnalyzeDiskUsage analyzes disk usage for a server and specific directory path.
func (s *Service) AnalyzeDiskUsage(ctx context.Context, serverID string, targetPath string) (*DiskAnalysisReport, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if targetPath == "" || targetPath == "." {
		targetPath = "/"
	}
	targetPath = path.Clean(filepath.ToSlash(targetPath))
	if err := ssh.ValidateRemotePath(targetPath, true); err != nil {
		return nil, fmt.Errorf("недопустимый путь для анализа: %w", err)
	}

	report := &DiskAnalysisReport{
		ServerID:       serverID,
		AnalyzedPath:   targetPath,
		Partitions:     []DiskPartition{},
		TopDirectories: []DiskUsageItem{},
		LargestFiles:   []LargestFileItem{},
		CleanupOptions: []CleanupCategory{},
		AnalyzedAt:     time.Now(),
	}

	// 1. Filesystem partitions (df -B1)
	dfCmd := "run_sudo df -B1 -x tmpfs -x devtmpfs -x squashfs -x overlay 2>/dev/null || df -B1 2>/dev/null || df -k -P 2>/dev/null"
	outDF, _, _, _ := client.NewCommand(dfCmd).WithSudo().Exec(ctx)
	report.Partitions = parseDFPartitions(string(outDF))

	// 2. Immediate subdirectories size breakdown (du -k --max-depth=1)
	duCmd := fmt.Sprintf("run_sudo du -k --max-depth=1 %s 2>/dev/null", ssh.ShellEscape(targetPath))
	outDu, _, _, _ := client.NewCommand(duCmd).WithSudo().Exec(ctx)
	dirs, totalPathBytes := parseDuOutput(string(outDu), targetPath)
	report.TopDirectories = dirs
	report.TotalPathBytes = totalPathBytes
	report.HumanTotalPath = formatBytes(totalPathBytes)

	// 3. Largest files under targetPath
	findCmd := fmt.Sprintf("run_sudo find %s -xdev -not -path '*/proc/*' -not -path '*/sys/*' -not -path '*/dev/*' -not -path '*/run/*' -type f -size +1M -printf '%%s|%%T@|%%p\\n' 2>/dev/null | sort -rn | head -n 30", ssh.ShellEscape(targetPath))
	outFind, _, codeFind, _ := client.NewCommand(findCmd).WithSudo().Exec(ctx)
	if codeFind == 0 && len(strings.TrimSpace(string(outFind))) > 0 {
		report.LargestFiles = parseFindLargestFiles(string(outFind))
	} else {
		// Fallback for systems without GNU find -printf
		fallbackCmd := fmt.Sprintf("run_sudo find %s -xdev -not -path '*/proc/*' -not -path '*/sys/*' -not -path '*/dev/*' -not -path '*/run/*' -type f -size +1M -exec ls -l {} + 2>/dev/null | awk '{print $5\"|\"$9}' | sort -rn | head -n 30", ssh.ShellEscape(targetPath))
		outFallback, _, _, _ := client.NewCommand(fallbackCmd).WithSudo().Exec(ctx)
		report.LargestFiles = parseFindFallback(string(outFallback))
	}

	// 4. Safe Cleanup categories detection
	report.CleanupOptions = s.detectCleanupOptions(ctx, client)

	return report, nil
}

func (s *Service) detectCleanupOptions(ctx context.Context, client *ssh.SSHClient) []CleanupCategory {
	var categories []CleanupCategory

	// Batch probe cleanup sizes
	probeScript := `
echo "===JOURNAL==="
journalctl --disk-usage 2>/dev/null || echo "0B"
echo "===APT_CACHE==="
(du -sb /var/cache/apt/archives 2>/dev/null || du -sk /var/cache/dnf 2>/dev/null || du -sk /var/cache/yum 2>/dev/null || echo "0") | tail -n 1 | awk '{print $1}'
echo "===OLD_LOGS==="
find /var/log -type f \( -name "*.gz" -o -name "*.[0-9]" -o -name "*.old" \) -exec du -cb {} + 2>/dev/null | tail -n 1 | awk '{print $1}'
echo "===TEMP_FILES==="
find /tmp /var/tmp -xdev -type f -atime +3 -exec du -cb {} + 2>/dev/null | tail -n 1 | awk '{print $1}'
echo "===DOCKER==="
if command -v docker >/dev/null 2>&1; then
    docker system df 2>/dev/null | grep -E "Images|Containers|Build Cache" || echo "NONE"
else
    echo "NONE"
fi
`
	out, _, _, _ := client.NewCommand(probeScript).WithSudo().Exec(ctx)
	outStr := string(out)

	// Journal logs
	journalSize := int64(0)
	if strings.Contains(outStr, "===JOURNAL===") {
		sec := extractSection(outStr, "===JOURNAL===", "===APT_CACHE===")
		re := regexp.MustCompile(`([0-9.]+)\s*([KMGTP]?B)`)
		match := re.FindStringSubmatch(sec)
		if len(match) >= 3 {
			journalSize = parseByteSizeUnit(match[1] + match[2])
		}
	}
	categories = append(categories, CleanupCategory{
		ID:          "journal_logs",
		Title:       "Системные журналы (journalctl)",
		Description: "Ограничивает размер архивированных системных логов до 100 МБ.",
		SizeBytes:   journalSize,
		HumanSize:   formatBytes(journalSize),
		Safe:        true,
		Available:   journalSize > 10*1024*1024,
	})

	// APT / DNF package manager cache
	aptSize := int64(0)
	if strings.Contains(outStr, "===APT_CACHE===") {
		sec := strings.TrimSpace(extractSection(outStr, "===APT_CACHE===", "===OLD_LOGS==="))
		aptSize, _ = strconv.ParseInt(sec, 10, 64)
	}
	categories = append(categories, CleanupCategory{
		ID:          "apt_cache",
		Title:       "Кэш пакетов (APT/DNF/YUM)",
		Description: "Очищает загруженные .deb/.rpm архивы из кэша пакетов.",
		SizeBytes:   aptSize,
		HumanSize:   formatBytes(aptSize),
		Safe:        true,
		Available:   aptSize > 5*1024*1024,
	})

	// Old rotated logs
	oldLogsSize := int64(0)
	if strings.Contains(outStr, "===OLD_LOGS===") {
		sec := strings.TrimSpace(extractSection(outStr, "===OLD_LOGS===", "===TEMP_FILES==="))
		oldLogsSize, _ = strconv.ParseInt(sec, 10, 64)
	}
	categories = append(categories, CleanupCategory{
		ID:          "old_logs",
		Title:       "Ротированные архивы логов (/var/log/*.gz)",
		Description: "Удаляет сжатые старые архивы логов и ротированные файлы журналов.",
		SizeBytes:   oldLogsSize,
		HumanSize:   formatBytes(oldLogsSize),
		Safe:        true,
		Available:   oldLogsSize > 5*1024*1024,
	})

	// Temp files
	tempSize := int64(0)
	if strings.Contains(outStr, "===TEMP_FILES===") {
		sec := strings.TrimSpace(extractSection(outStr, "===TEMP_FILES===", "===DOCKER==="))
		tempSize, _ = strconv.ParseInt(sec, 10, 64)
	}
	categories = append(categories, CleanupCategory{
		ID:          "temp_files",
		Title:       "Временные файлы старше 3 дней (/tmp, /var/tmp)",
		Description: "Безопасно очищает неиспользуемые временные файлы старше 3 суток.",
		SizeBytes:   tempSize,
		HumanSize:   formatBytes(tempSize),
		Safe:        true,
		Available:   tempSize > 5*1024*1024,
	})

	// Docker cache
	dockerAvailable := false
	if strings.Contains(outStr, "===DOCKER===") {
		sec := extractSection(outStr, "===DOCKER===", "")
		if !strings.Contains(sec, "NONE") && len(strings.TrimSpace(sec)) > 0 {
			dockerAvailable = true
		}
	}
	categories = append(categories, CleanupCategory{
		ID:          "docker_cache",
		Title:       "Неиспользуемые ресурсы Docker",
		Description: "Удаляет остановленные контейнеры, неиспользуемые сети и образы без тегов (dangling).",
		SizeBytes:   0,
		HumanSize:   "Оценка по факту",
		Safe:        true,
		Available:   dockerAvailable,
	})

	return categories
}

func extractSection(src, startMarker, endMarker string) string {
	idxStart := strings.Index(src, startMarker)
	if idxStart == -1 {
		return ""
	}
	content := src[idxStart+len(startMarker):]
	if endMarker != "" {
		idxEnd := strings.Index(content, endMarker)
		if idxEnd != -1 {
			content = content[:idxEnd]
		}
	}
	return strings.TrimSpace(content)
}

func parseByteSizeUnit(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0
	}
	multiplier := int64(1)
	if strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "K") {
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "KB"), "K")
	} else if strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "MB"), "M")
	} else if strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "GB"), "G")
	} else if strings.HasSuffix(s, "TB") || strings.HasSuffix(s, "T") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "TB"), "T")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int64(val * float64(multiplier))
}

func parseDFPartitions(output string) []DiskPartition {
	var partitions []DiskPartition
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Filesystem") || strings.HasPrefix(trimmed, "Файловая") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 6 {
			total, _ := strconv.ParseInt(fields[1], 10, 64)
			used, _ := strconv.ParseInt(fields[2], 10, 64)
			free, _ := strconv.ParseInt(fields[3], 10, 64)
			mount := fields[5]

			if total <= 0 {
				continue
			}

			// If blocks were reported in 1K rather than 1B
			if total < 100*1024*1024 {
				total *= 1024
				used *= 1024
				free *= 1024
			}

			percent := float64(used) / float64(total) * 100.0
			partitions = append(partitions, DiskPartition{
				Filesystem:  fields[0],
				MountPoint:  mount,
				TotalBytes:  total,
				UsedBytes:   used,
				FreeBytes:   free,
				UsedPercent: math.Round(percent*10) / 10,
				HumanTotal:  formatBytes(total),
				HumanUsed:   formatBytes(used),
				HumanFree:   formatBytes(free),
			})
		}
	}
	return partitions
}

func parseDuOutput(output string, targetPath string) ([]DiskUsageItem, int64) {
	var items []DiskUsageItem
	var totalPathBytes int64
	cleanTarget := path.Clean(filepath.ToSlash(targetPath))

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "\t", 2)
		if len(parts) < 2 {
			parts = strings.Fields(trimmed)
		}
		if len(parts) >= 2 {
			kb, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				continue
			}
			bytes := kb * 1024
			p := path.Clean(filepath.ToSlash(strings.TrimSpace(parts[1])))

			if p == cleanTarget {
				totalPathBytes = bytes
				continue
			}

			name := path.Base(p)
			items = append(items, DiskUsageItem{
				Path:      p,
				Name:      name,
				SizeBytes: bytes,
				HumanSize: formatBytes(bytes),
				IsDir:     true,
			})
		}
	}

	if totalPathBytes == 0 && len(items) > 0 {
		for _, it := range items {
			totalPathBytes += it.SizeBytes
		}
	}

	// Calculate percentages
	for i := range items {
		if totalPathBytes > 0 {
			items[i].Percent = math.Round((float64(items[i].SizeBytes)/float64(totalPathBytes)*100)*10) / 10
		}
	}

	// Sort descending by size
	sort.Slice(items, func(i, j int) bool {
		return items[i].SizeBytes > items[j].SizeBytes
	})

	if len(items) > 25 {
		items = items[:25]
	}

	return items, totalPathBytes
}

func parseFindLargestFiles(output string) []LargestFileItem {
	var items []LargestFileItem
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "|", 3)
		if len(parts) >= 3 {
			size, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				continue
			}
			mtimeFloat, _ := strconv.ParseFloat(parts[1], 64)
			filePath := parts[2]
			items = append(items, LargestFileItem{
				Path:      filePath,
				Name:      path.Base(filePath),
				SizeBytes: size,
				HumanSize: formatBytes(size),
				ModTime:   time.Unix(int64(mtimeFloat), 0),
			})
		}
	}
	return items
}

func parseFindFallback(output string) []LargestFileItem {
	var items []LargestFileItem
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "|", 2)
		if len(parts) >= 2 {
			size, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			if err != nil {
				continue
			}
			filePath := strings.TrimSpace(parts[1])
			items = append(items, LargestFileItem{
				Path:      filePath,
				Name:      path.Base(filePath),
				SizeBytes: size,
				HumanSize: formatBytes(size),
				ModTime:   time.Now(),
			})
		}
	}
	return items
}

// CleanDiskCategory queues an asynchronous cleanup task for a specific category.
func (s *Service) CleanDiskCategory(ctx context.Context, serverID string, categoryID string) (*db.TaskRecord, error) {
	if s.engine == nil {
		return nil, fmt.Errorf("task engine not initialized")
	}

	categoryTitles := map[string]string{
		"journal_logs": "Системные журналы (journalctl)",
		"apt_cache":    "Кэш пакетов (APT/DNF)",
		"old_logs":     "Ротированные архивы логов",
		"temp_files":   "Временные файлы (/tmp)",
		"docker_cache": "Неиспользуемые ресурсы Docker",
	}

	catName := categoryTitles[categoryID]
	if catName == "" {
		catName = categoryID
	}

	title := fmt.Sprintf("Очистка диска: %s", catName)
	payload, _ := json.Marshal(CleanDiskRequest{
		ServerID:   serverID,
		CategoryID: categoryID,
	})

	return s.engine.Submit(serverID, "files_disk_cleanup", title, string(payload))
}

func (s *Service) handleDiskCleanupTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req CleanDiskRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Подготовка к очистке", 15)

	var cmd string
	var categoryTitle string
	switch req.CategoryID {
	case "journal_logs":
		categoryTitle = "Системные журналы"
		cmd = "journalctl --vacuum-size=100M 2>&1"
	case "apt_cache":
		categoryTitle = "Кэш пакетов"
		cmd = "apt-get clean 2>&1 || yum clean all 2>&1 || dnf clean all 2>&1"
	case "old_logs":
		categoryTitle = "Ротированные архивы логов"
		cmd = "find /var/log -type f \\( -name \"*.gz\" -o -name \"*.[0-9]\" -o -name \"*.old\" \\) -delete 2>&1"
	case "temp_files":
		categoryTitle = "Временные файлы"
		cmd = "find /tmp /var/tmp -xdev -type f -atime +3 -delete 2>&1"
	case "docker_cache":
		categoryTitle = "Ресурсы Docker"
		cmd = "docker system prune -f 2>&1"
	default:
		return "", fmt.Errorf("неизвестная категория очистки: %s", req.CategoryID)
	}

	logger.Info(fmt.Sprintf("Выполнение команды: %s", cmd))
	logger.SetStep(fmt.Sprintf("Очистка: %s", categoryTitle), 50)

	out, stderr, code, execErr := client.NewCommand(cmd).WithSudo().Exec(ctx)
	outStr := strings.TrimSpace(string(out))
	if execErr != nil || code != 0 {
		logger.Warn(fmt.Sprintf("Предупреждение при выполнении команды: %v (%s)", execErr, string(stderr)))
	}

	logger.Info("Вывод операции:\n" + outStr)
	logger.SetStep("Очистка успешно завершена", 100)

	return fmt.Sprintf("Очистка '%s' завершена. Результат:\n%s", categoryTitle, outStr), nil
}
