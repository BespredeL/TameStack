package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

var webshellRegexPattern = `eval\s*\(.*(base64_decode|gzinflate|str_rot13|gzuncompress|bzdecompress)|assert\s*\(\s*\$_(POST|GET|REQUEST|COOKIE)|(system|shell_exec|exec|passthru|popen|proc_open)\s*\(\s*\$_(POST|GET|REQUEST|COOKIE)|\$_POST\[.+\]\s*\(\s*\$_POST|(c99shell|r57shell|WSO_VERSION|FilesMan|ALFA_DATA|Mini\s*Shell|b374k)|(preg_replace|mb_ereg_replace)\s*\(.*[\x27\x22]/e[\x27\x22]`

var uploadDirNames = []string{
	"upload", "uploads", "wp-content/uploads", "images", "img", "media", "cache", "tmp", "storage/app/public",
}

var dangerousScriptExtensions = map[string]bool{
	".php": true, ".phtml": true, ".php5": true, ".php7": true, ".phar": true,
	".pht": true, ".sh": true, ".bash": true, ".py": true, ".pl": true, ".cgi": true,
}

// SnapshotManifestEntry represents one file record within a snapshot.
type SnapshotManifestEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mtime  int64  `json:"mtime"`
	SHA256 string `json:"sha256"`
}

// ScanFiles searches for recently changed and suspicious files.
func (s *Service) ScanFiles(ctx context.Context, req ScanFilesRequest) (*ScanFilesResult, error) {
	start := time.Now()
	cleanPath := path.Clean(strings.TrimSpace(req.BasePath))
	if cleanPath == "" || cleanPath == "." || !strings.HasPrefix(cleanPath, "/") {
		return nil, fmt.Errorf("укажите корректный абсолютный путь для сканирования (например, /var/www)")
	}
	if cleanPath == "/" {
		return nil, fmt.Errorf("сканирование всего корня файловой системы '/' запрещено в целях производительности")
	}

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return nil, err
	}

	exclusions, _ := s.database.ListFileSecurityExclusions(ctx, req.ServerID)

	var timeArg string
	switch req.TimeRange {
	case "1h":
		timeArg = "-mmin -60"
	case "6h":
		timeArg = "-mmin -360"
	case "24h":
		timeArg = "-mtime -1"
	case "3d":
		timeArg = "-mtime -3"
	case "7d":
		timeArg = "-mtime -7"
	case "14d":
		timeArg = "-mtime -14"
	case "30d":
		timeArg = "-mtime -30"
	case "all":
		timeArg = ""
	default:
		timeArg = "-mtime -1"
	}

	var typeArg string
	switch req.FileTypes {
	case "scripts":
		typeArg = `\( -name "*.php*" -o -name "*.phtml" -o -name "*.js" -o -name "*.sh" -o -name "*.py" -o -name "*.pl" -o -name "*.cgi" -o -name "*.bash" \)`
	case "configs":
		typeArg = `\( -name ".htaccess" -o -name ".user.ini" -o -name "*.conf" -o -name "*.ini" -o -name "*.env" \)`
	case "all":
		typeArg = ""
	default:
		typeArg = `\( -name "*.php*" -o -name "*.phtml" -o -name "*.js" -o -name "*.sh" -o -name "*.py" -o -name "*.pl" -o -name "*.cgi" -o -name "*.bash" \)`
	}

	limit := req.MaxResults
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	// Uses native GNU find -printf for zero subshell overhead.
	findCmd := fmt.Sprintf(
		`TARGET=%s
if [ ! -d "$TARGET" ]; then
  echo "DIR_NOT_FOUND"
  exit 2
fi
find "$TARGET" -xdev \( -path '*/vendor/*' -prune -o -path '*/node_modules/*' -prune -o -path '*/.git/*' -prune -o -path '*/cache/*' -prune \) -o -type f %s %s -size -10M -printf '%%p\t%%s\t%%m\t%%u:%%g\t%%Ts\n' 2>/dev/null | head -n %d`,
		ssh.ShellEscape(cleanPath), timeArg, typeArg, limit,
	)

	out, err := s.execCmd(ctx, client, findCmd)
	if err != nil {
		if strings.Contains(out, "DIR_NOT_FOUND") {
			return nil, fmt.Errorf("указанный каталог '%s' не существует на сервере", cleanPath)
		}
		return nil, fmt.Errorf("ошибка поиска файлов: %w", err)
	}

	scannedFiles := make([]ScannedFile, 0)
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "DIR_NOT_FOUND" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}
		filePath := parts[0]
		sz, _ := strconv.ParseInt(parts[1], 10, 64)
		perm := parts[2]
		owner := parts[3]
		mtUnix, _ := strconv.ParseInt(parts[4], 10, 64)

		modTimeStr := time.Unix(mtUnix, 0).Format("2006-01-02 15:04:05")

		isExcluded := isPathExcluded(filePath, exclusions)

		scannedFiles = append(scannedFiles, ScannedFile{
			Path:          filePath,
			Size:          sz,
			FormattedSize: formatByteSize(sz),
			ModTime:       modTimeStr,
			Permissions:   perm,
			Owner:         owner,
			ThreatLevel:   string(ThreatClean),
			Reasons:       []string{},
			Snippet:       "",
			IsExcluded:    isExcluded,
		})
	}

	threatCount := 0

	if req.DetectMalware && len(scannedFiles) > 0 {
		var grepCandidates []string
		candidatesMap := make(map[string]*ScannedFile)

		for i := range scannedFiles {
			file := &scannedFiles[i]
			ext := strings.ToLower(path.Ext(file.Path))
			baseName := strings.ToLower(path.Base(file.Path))

			// Heuristic 1: Dangerous script in upload / images directory.
			if isUnderUploadDir(file.Path) && dangerousScriptExtensions[ext] {
				file.ThreatLevel = string(ThreatSuspicious)
				file.Reasons = append(file.Reasons, "Исполняемый скрипт в каталоге пользовательских загрузок/кэша")
			}

			// Heuristic 2: Dangerous permissions 0777 or 0666.
			if strings.HasSuffix(file.Permissions, "777") || strings.HasSuffix(file.Permissions, "666") {
				if file.ThreatLevel == string(ThreatClean) {
					file.ThreatLevel = string(ThreatSuspicious)
				}
				file.Reasons = append(file.Reasons, fmt.Sprintf("Опасные права доступа (%s)", file.Permissions))
			}

			// Heuristic 3: Double extension like file.php.jpg or script.php.bak.
			if strings.Count(baseName, ".") >= 2 {
				parts := strings.Split(baseName, ".")
				for pIdx := 0; pIdx < len(parts)-1; pIdx++ {
					if dangerousScriptExtensions["."+parts[pIdx]] {
						if file.ThreatLevel == string(ThreatClean) {
							file.ThreatLevel = string(ThreatSuspicious)
						}
						file.Reasons = append(file.Reasons, "Двойное расширение (маскировка под неисполняемый файл)")
						break
					}
				}
			}

			if dangerousScriptExtensions[ext] || ext == ".htaccess" || ext == ".ini" || ext == ".js" || isUnderUploadDir(file.Path) {
				grepCandidates = append(grepCandidates, file.Path)
				candidatesMap[file.Path] = file
			}
		}

		// Load active custom heuristic signatures.
		var activeCustomSignatures []*db.FileSecuritySignatureRecord
		if dbSigs, err := s.database.ListFileSecuritySignatures(ctx, req.ServerID); err == nil {
			for _, cs := range dbSigs {
				if cs.Enabled && strings.TrimSpace(cs.Pattern) != "" {
					activeCustomSignatures = append(activeCustomSignatures, cs)
				}
			}
		}

		// Run targeted grep in batches of 100 files.
		if len(grepCandidates) > 0 {
			batchSize := 100
			for i := 0; i < len(grepCandidates); i += batchSize {
				end := i + batchSize
				if end > len(grepCandidates) {
					end = len(grepCandidates)
				}
				batch := grepCandidates[i:end]

				escapedPaths := make([]string, len(batch))
				for bIdx, p := range batch {
					escapedPaths[bIdx] = ssh.ShellEscape(p)
				}

				grepCmd := fmt.Sprintf(
					"grep -i -E -n -H -m 2 %s %s 2>/dev/null",
					ssh.ShellEscape(webshellRegexPattern),
					strings.Join(escapedPaths, " "),
				)

				grepOut, _ := s.execCmd(ctx, client, grepCmd)
				if grepOut != "" {
					gScanner := bufio.NewScanner(strings.NewReader(grepOut))
					for gScanner.Scan() {
						gLine := gScanner.Text()
						parts := strings.SplitN(gLine, ":", 3)
						if len(parts) >= 3 {
							matchedPath := parts[0]
							lineNum := parts[1]
							snippet := strings.TrimSpace(parts[2])
							if len(snippet) > 160 {
								snippet = snippet[:160] + "..."
							}

							if f, ok := candidatesMap[matchedPath]; ok {
								f.ThreatLevel = string(ThreatMalicious)
								f.Reasons = appendUnique(f.Reasons, fmt.Sprintf("Обнаружена вредоносная сигнатура (строка %s)", lineNum))
								if f.Snippet == "" {
									f.Snippet = snippet
								}
							}
						}
					}
				}

				for _, cs := range activeCustomSignatures {
					csGrepCmd := fmt.Sprintf(
						"grep -i -E -n -H -m 2 %s %s 2>/dev/null",
						ssh.ShellEscape(cs.Pattern),
						strings.Join(escapedPaths, " "),
					)
					csOut, _ := s.execCmd(ctx, client, csGrepCmd)
					if csOut != "" {
						csScanner := bufio.NewScanner(strings.NewReader(csOut))
						for csScanner.Scan() {
							csLine := csScanner.Text()
							parts := strings.SplitN(csLine, ":", 3)
							if len(parts) >= 3 {
								matchedPath := parts[0]
								lineNum := parts[1]
								snippet := strings.TrimSpace(parts[2])
								if len(snippet) > 160 {
									snippet = snippet[:160] + "..."
								}
								if f, ok := candidatesMap[matchedPath]; ok {
									if cs.ThreatLevel == string(ThreatMalicious) {
										f.ThreatLevel = string(ThreatMalicious)
									} else if f.ThreatLevel != string(ThreatMalicious) {
										f.ThreatLevel = string(ThreatSuspicious)
									}
									f.Reasons = appendUnique(f.Reasons, fmt.Sprintf("Сигнатура \"%s\" (строка %s)", cs.Name, lineNum))
									if f.Snippet == "" {
										f.Snippet = snippet
									}
								}
							}
						}
					}
				}

				// Check for hidden PHP tags inside image files.
				imgCheckCmd := fmt.Sprintf(
					"grep -l -m 1 -E '(<\\?php|<\\?=)' %s 2>/dev/null",
					strings.Join(escapedPaths, " "),
				)
				imgOut, _ := s.execCmd(ctx, client, imgCheckCmd)
				if imgOut != "" {
					for _, p := range strings.Split(strings.TrimSpace(imgOut), "\n") {
						p = strings.TrimSpace(p)
						if p == "" {
							continue
						}
						ext := strings.ToLower(path.Ext(p))
						if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".ico" || ext == ".webp" {
							if f, ok := candidatesMap[p]; ok {
								f.ThreatLevel = string(ThreatMalicious)
								f.Reasons = appendUnique(f.Reasons, "PHP-код внедрён внутрь графического файла")
							}
						}
					}
				}
			}
		}

		for _, f := range scannedFiles {
			if f.ThreatLevel != string(ThreatClean) {
				threatCount++
			}
		}
	}

	return &ScanFilesResult{
		Files:        scannedFiles,
		ScannedCount: len(scannedFiles),
		ThreatCount:  threatCount,
		DurationMs:   time.Since(start).Milliseconds(),
	}, nil
}

// DeleteScannedFile deletes a file after verifying path safety.
func (s *Service) DeleteScannedFile(ctx context.Context, serverID, filePath string) error {
	cleanPath := path.Clean(strings.TrimSpace(filePath))
	if err := validateSafeFilePath(cleanPath); err != nil {
		return err
	}

	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("rm -f %s", ssh.ShellEscape(cleanPath))
	_, err = s.execCmd(ctx, client, cmd)
	if err != nil {
		return fmt.Errorf("не удалось удалить файл: %w", err)
	}
	return nil
}

// ReadScannedFile reads file content for viewing/editing (up to 2MB).
func (s *Service) ReadScannedFile(ctx context.Context, serverID, filePath string) (string, error) {
	cleanPath := path.Clean(strings.TrimSpace(filePath))
	if err := validateSafeFilePath(cleanPath); err != nil {
		return "", err
	}

	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	cmd := fmt.Sprintf("head -c 2097152 %s", ssh.ShellEscape(cleanPath))
	out, err := s.execCmd(ctx, client, cmd)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	return out, nil
}

// SaveScannedFile writes modified file content back with automatic backup.
func (s *Service) SaveScannedFile(ctx context.Context, serverID, filePath, content string) error {
	cleanPath := path.Clean(strings.TrimSpace(filePath))
	if err := validateSafeFilePath(cleanPath); err != nil {
		return err
	}

	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return err
	}

	backupCmd := fmt.Sprintf("cp -p %s %s.bak.%d 2>/dev/null || true", ssh.ShellEscape(cleanPath), ssh.ShellEscape(cleanPath), time.Now().Unix())
	_, _ = s.execCmd(ctx, client, backupCmd)

	if err := client.WriteFile(ctx, cleanPath, []byte(content), os.FileMode(0644)); err != nil {
		return fmt.Errorf("ошибка сохранения файла: %w", err)
	}
	return nil
}

func (s *Service) ListFileExclusions(ctx context.Context, serverID string) ([]FileSecurityExclusion, error) {
	records, err := s.database.ListFileSecurityExclusions(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]FileSecurityExclusion, 0, len(records))
	for _, r := range records {
		out = append(out, FileSecurityExclusion{
			ID:        r.ID,
			ServerID:  r.ServerID,
			Pattern:   r.Pattern,
			Reason:    r.Reason,
			CreatedAt: r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return out, nil
}

func (s *Service) AddFileExclusion(ctx context.Context, serverID, pattern, reason string) (*FileSecurityExclusion, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("маска исключения не может быть пустой")
	}
	record := &db.FileSecurityExclusionRecord{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		Pattern:   pattern,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	if err := s.database.SaveFileSecurityExclusion(ctx, record); err != nil {
		return nil, err
	}
	return &FileSecurityExclusion{
		ID:        record.ID,
		ServerID:  record.ServerID,
		Pattern:   record.Pattern,
		Reason:    record.Reason,
		CreatedAt: record.CreatedAt.Format("2006-01-02 15:04"),
	}, nil
}

func (s *Service) DeleteFileExclusion(ctx context.Context, exclusionID string) error {
	return s.database.DeleteFileSecurityExclusion(ctx, exclusionID)
}

func (s *Service) ListFileSignatures(ctx context.Context, serverID string) ([]FileSecuritySignature, error) {
	records, err := s.database.ListFileSecuritySignatures(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]FileSecuritySignature, 0, len(records))
	for _, r := range records {
		out = append(out, FileSecuritySignature{
			ID:          r.ID,
			ServerID:    r.ServerID,
			Name:        r.Name,
			Pattern:     r.Pattern,
			ThreatLevel: r.ThreatLevel,
			Description: r.Description,
			Enabled:     r.Enabled,
			CreatedAt:   r.CreatedAt.Format("2006-01-02 15:04"),
			UpdatedAt:   r.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}
	return out, nil
}

func (s *Service) SaveFileSignature(ctx context.Context, sig FileSecuritySignature) (*FileSecuritySignature, error) {
	name := strings.TrimSpace(sig.Name)
	if name == "" {
		return nil, fmt.Errorf("название сигнатуры не может быть пустым")
	}
	pattern := strings.TrimSpace(sig.Pattern)
	if pattern == "" {
		return nil, fmt.Errorf("регулярное выражение сигнатуры не может быть пустым")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return nil, fmt.Errorf("некорректное регулярное выражение: %w", err)
	}
	threatLevel := sig.ThreatLevel
	if threatLevel != string(ThreatSuspicious) && threatLevel != string(ThreatMalicious) {
		threatLevel = string(ThreatMalicious)
	}
	id := strings.TrimSpace(sig.ID)
	if id == "" {
		id = uuid.New().String()
	}
	rec := &db.FileSecuritySignatureRecord{
		ID:          id,
		ServerID:    strings.TrimSpace(sig.ServerID),
		Name:        name,
		Pattern:     pattern,
		ThreatLevel: threatLevel,
		Description: strings.TrimSpace(sig.Description),
		Enabled:     sig.Enabled,
	}
	if err := s.database.SaveFileSecuritySignature(ctx, rec); err != nil {
		return nil, err
	}
	return &FileSecuritySignature{
		ID:          rec.ID,
		ServerID:    rec.ServerID,
		Name:        rec.Name,
		Pattern:     rec.Pattern,
		ThreatLevel: rec.ThreatLevel,
		Description: rec.Description,
		Enabled:     rec.Enabled,
		CreatedAt:   rec.CreatedAt.Format("2006-01-02 15:04"),
		UpdatedAt:   rec.UpdatedAt.Format("2006-01-02 15:04"),
	}, nil
}

func (s *Service) DeleteFileSignature(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("ID сигнатуры не может быть пустым")
	}
	return s.database.DeleteFileSecuritySignature(ctx, id)
}

func (s *Service) ToggleFileSignature(ctx context.Context, id string, enabled bool) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("ID сигнатуры не может быть пустым")
	}
	return s.database.ToggleFileSecuritySignature(ctx, id, enabled)
}

func (s *Service) CreateFileSnapshot(ctx context.Context, serverID, siteID, name, basePath string) (*FileSecuritySnapshot, error) {
	cleanPath := path.Clean(strings.TrimSpace(basePath))
	if cleanPath == "" || cleanPath == "." || !strings.HasPrefix(cleanPath, "/") || cleanPath == "/" {
		return nil, fmt.Errorf("укажите корректный каталог для создания снимка (например, /var/www/site)")
	}
	if name == "" {
		name = fmt.Sprintf("Снимок %s (%s)", path.Base(cleanPath), time.Now().Format("02.01 15:04"))
	}

	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// Fetch exclusions for this server to prune heavy non-code assets.
	exclusions, _ := s.database.ListFileSecurityExclusions(ctx, serverID)
	var exList []string
	for _, ex := range exclusions {
		pat := strings.TrimSpace(ex.Pattern)
		if pat != "" {
			exList = append(exList, pat)
		}
	}
	exJSON, _ := json.Marshal(exList)

	// High-speed streaming manifest generator with Python 3 (single-process) and GNU find fallback.
	script := fmt.Sprintf(`
TARGET=%s
EX_JSON=%s
if [ ! -d "$TARGET" ]; then
  echo "DIR_NOT_FOUND"
  exit 2
fi
if command -v python3 >/dev/null 2>&1; then
  python3 -c "
import os, sys, hashlib, stat, json, fnmatch

base = sys.argv[1]
try:
    ex_list = [x.strip() for x in json.loads(sys.argv[2]) if x.strip()]
except Exception:
    ex_list = []

prune_dirs = {'.git', 'node_modules', 'vendor', 'cache', '.svn', '.hg', 'sessions', 'tmp'}

for root, dirs, files in os.walk(base, topdown=True):
    new_dirs = []
    for d in dirs:
        if d in prune_dirs:
            continue
        full_d = os.path.join(root, d)
        rel_d = os.path.relpath(full_d, base).replace('\\\\', '/')
        if any(ex in full_d or ex == rel_d or ex == d or fnmatch.fnmatch(full_d, ex) or fnmatch.fnmatch(d, ex) for ex in ex_list):
            continue
        new_dirs.append(d)
    dirs[:] = new_dirs

    for f in files:
        full_f = os.path.join(root, f)
        rel_f = os.path.relpath(full_f, base).replace('\\\\', '/')
        if any(ex in full_f or ex == rel_f or ex == f or fnmatch.fnmatch(full_f, ex) or fnmatch.fnmatch(f, ex) for ex in ex_list):
            continue
        try:
            st = os.stat(full_f)
            if not stat.S_ISREG(st.st_mode):
                continue
            sz = st.st_size
            mt = int(st.st_mtime)
            h = hashlib.sha256()
            if sz > 15 * 1024 * 1024:
                with open(full_f, 'rb') as fp:
                    h.update(fp.read(65536))
                h.update(f'{sz}_{mt}'.encode())
            else:
                with open(full_f, 'rb') as fp:
                    while True:
                        buf = fp.read(65536)
                        if not buf:
                            break
                        h.update(buf)
            print(f'{rel_f}\t{sz}\t{mt}\t{h.hexdigest()}')
        except Exception:
            continue
" "$TARGET" "$EX_JSON"
else
  cd "$TARGET" || exit 1
  find . -xdev \( -path './vendor*' -prune -o -path './node_modules*' -prune -o -path './.git*' -prune -o -path './cache*' -prune \) -o -type f -size -15M -exec sha256sum {} + 2>/dev/null | while read -r h f; do
    rel=${f#./}
    sz=$(stat -c %%s "$f" 2>/dev/null || echo 0)
    mt=$(stat -c %%Y "$f" 2>/dev/null || echo 0)
    printf '%%s\t%%s\t%%s\t%%s\n' "$rel" "$sz" "$mt" "$h"
  done
fi
`, ssh.ShellEscape(cleanPath), ssh.ShellEscape(string(exJSON)))

	out, err := s.execCmd(ctx, client, script)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать снимок файлов: %w", err)
	}

	entries := make([]SnapshotManifestEntry, 0)
	var totalSize int64
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "DIR_NOT_FOUND" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		sz, _ := strconv.ParseInt(parts[1], 10, 64)
		mt, _ := strconv.ParseInt(parts[2], 10, 64)
		entries = append(entries, SnapshotManifestEntry{
			Path:   parts[0],
			Size:   sz,
			Mtime:  mt,
			SHA256: parts[3],
		})
		totalSize += sz
	}

	manifestBytes, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации манифеста: %w", err)
	}

	record := &db.FileSecuritySnapshotRecord{
		ID:             uuid.New().String(),
		ServerID:       serverID,
		SiteID:         siteID,
		Name:           name,
		BasePath:       cleanPath,
		FilesCount:     len(entries),
		TotalSizeBytes: totalSize,
		ManifestJSON:   string(manifestBytes),
		CreatedAt:      time.Now(),
	}

	if err := s.database.SaveFileSecuritySnapshot(ctx, record); err != nil {
		return nil, err
	}

	return &FileSecuritySnapshot{
		ID:             record.ID,
		ServerID:       record.ServerID,
		SiteID:         record.SiteID,
		Name:           record.Name,
		BasePath:       record.BasePath,
		FilesCount:     record.FilesCount,
		TotalSizeBytes: record.TotalSizeBytes,
		CreatedAt:      record.CreatedAt.Format("2006-01-02 15:04"),
	}, nil
}

func (s *Service) ListFileSnapshots(ctx context.Context, serverID string) ([]FileSecuritySnapshot, error) {
	records, err := s.database.ListFileSecuritySnapshots(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]FileSecuritySnapshot, 0, len(records))
	for _, r := range records {
		out = append(out, FileSecuritySnapshot{
			ID:             r.ID,
			ServerID:       r.ServerID,
			SiteID:         r.SiteID,
			Name:           r.Name,
			BasePath:       r.BasePath,
			FilesCount:     r.FilesCount,
			TotalSizeBytes: r.TotalSizeBytes,
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return out, nil
}

func (s *Service) DeleteFileSnapshot(ctx context.Context, snapshotID string) error {
	return s.database.DeleteFileSecuritySnapshot(ctx, snapshotID)
}

func (s *Service) CompareWithSnapshot(ctx context.Context, snapshotID string) (*SnapshotDiffResult, error) {
	start := time.Now()
	snapshot, err := s.database.GetFileSecuritySnapshot(ctx, snapshotID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, fmt.Errorf("снимок не найден")
	}

	var baseEntries []SnapshotManifestEntry
	if err := json.Unmarshal([]byte(snapshot.ManifestJSON), &baseEntries); err != nil {
		return nil, fmt.Errorf("ошибка чтения манифеста базового снимка: %w", err)
	}

	baseMap := make(map[string]SnapshotManifestEntry)
	for _, e := range baseEntries {
		baseMap[e.Path] = e
	}

	client, _, err := s.getSSHClient(ctx, snapshot.ServerID)
	if err != nil {
		return nil, err
	}

	exclusions, _ := s.database.ListFileSecurityExclusions(ctx, snapshot.ServerID)
	var exList []string
	for _, ex := range exclusions {
		pat := strings.TrimSpace(ex.Pattern)
		if pat != "" {
			exList = append(exList, pat)
		}
	}
	exJSON, _ := json.Marshal(exList)

	// High-speed current state capture with Python 3 and fallback.
	script := fmt.Sprintf(`
TARGET=%s
EX_JSON=%s
if [ ! -d "$TARGET" ]; then
  echo "DIR_NOT_FOUND"
  exit 2
fi
if command -v python3 >/dev/null 2>&1; then
  python3 -c "
import os, sys, hashlib, stat, pwd, grp, json, fnmatch

base = sys.argv[1]
try:
    ex_list = [x.strip() for x in json.loads(sys.argv[2]) if x.strip()]
except Exception:
    ex_list = []

prune_dirs = {'.git', 'node_modules', 'vendor', 'cache', '.svn', '.hg', 'sessions', 'tmp'}

for root, dirs, files in os.walk(base, topdown=True):
    new_dirs = []
    for d in dirs:
        if d in prune_dirs:
            continue
        full_d = os.path.join(root, d)
        rel_d = os.path.relpath(full_d, base).replace('\\\\', '/')
        if any(ex in full_d or ex == rel_d or ex == d or fnmatch.fnmatch(full_d, ex) or fnmatch.fnmatch(d, ex) for ex in ex_list):
            continue
        new_dirs.append(d)
    dirs[:] = new_dirs

    for f in files:
        full_f = os.path.join(root, f)
        rel_f = os.path.relpath(full_f, base).replace('\\\\', '/')
        if any(ex in full_f or ex == rel_f or ex == f or fnmatch.fnmatch(full_f, ex) or fnmatch.fnmatch(f, ex) for ex in ex_list):
            continue
        try:
            st = os.stat(full_f)
            if not stat.S_ISREG(st.st_mode):
                continue
            sz = st.st_size
            mt = int(st.st_mtime)
            perm = oct(stat.S_IMODE(st.st_mode))[-4:]
            try:
                own = f'{pwd.getpwuid(st.st_uid).pw_name}:{grp.getgrgid(st.st_gid).gr_name}'
            except Exception:
                own = f'{st.st_uid}:{st.st_gid}'
            h = hashlib.sha256()
            if sz > 15 * 1024 * 1024:
                with open(full_f, 'rb') as fp:
                    h.update(fp.read(65536))
                h.update(f'{sz}_{mt}'.encode())
            else:
                with open(full_f, 'rb') as fp:
                    while True:
                        buf = fp.read(65536)
                        if not buf:
                            break
                        h.update(buf)
            print(f'{rel_f}\t{sz}\t{perm}\t{own}\t{mt}\t{h.hexdigest()}')
        except Exception:
            continue
" "$TARGET" "$EX_JSON"
else
  cd "$TARGET" || exit 1
  find . -xdev \( -path './vendor*' -prune -o -path './node_modules*' -prune -o -path './.git*' -prune -o -path './cache*' -prune \) -o -type f -size -15M -exec sha256sum {} + 2>/dev/null | while read -r h f; do
    rel=${f#./}
    sz=$(stat -c %%s "$f" 2>/dev/null || echo 0)
    perm=$(stat -c %%a "$f" 2>/dev/null || echo 0644)
    own=$(stat -c '%%U:%%G' "$f" 2>/dev/null || echo root:root)
    mt=$(stat -c %%Y "$f" 2>/dev/null || echo 0)
    printf '%%s\t%%s\t%%s\t%%s\t%%s\t%%s\n' "$rel" "$sz" "$perm" "$own" "$mt" "$h"
  done
fi
`, ssh.ShellEscape(snapshot.BasePath), ssh.ShellEscape(string(exJSON)))

	out, err := s.execCmd(ctx, client, script)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить текущее состояние каталога: %w", err)
	}

	currentSeen := make(map[string]bool)
	addedFiles := make([]ScannedFile, 0)
	modifiedFiles := make([]ScannedFile, 0)
	deletedFiles := make([]string, 0)

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "DIR_NOT_FOUND" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}
		relPath := parts[0]
		sz, _ := strconv.ParseInt(parts[1], 10, 64)
		perm := parts[2]
		owner := parts[3]
		mtUnix, _ := strconv.ParseInt(parts[4], 10, 64)
		currentHash := parts[5]

		currentSeen[relPath] = true
		fullPath := path.Join(snapshot.BasePath, relPath)
		modTimeStr := time.Unix(mtUnix, 0).Format("2006-01-02 15:04:05")

		baseEntry, existsInBase := baseMap[relPath]
		if !existsInBase {
			f := ScannedFile{
				Path:          fullPath,
				Size:          sz,
				FormattedSize: formatByteSize(sz),
				ModTime:       modTimeStr,
				Permissions:   perm,
				Owner:         owner,
				ThreatLevel:   string(ThreatClean),
				Reasons:       []string{"Новый файл (отсутствовал в эталонном снимке)"},
			}
			addedFiles = append(addedFiles, f)
		} else if baseEntry.SHA256 != currentHash || baseEntry.Size != sz {
			f := ScannedFile{
				Path:          fullPath,
				Size:          sz,
				FormattedSize: formatByteSize(sz),
				ModTime:       modTimeStr,
				Permissions:   perm,
				Owner:         owner,
				ThreatLevel:   string(ThreatClean),
				Reasons:       []string{"Содержимое файла изменено с момента снимка"},
			}
			modifiedFiles = append(modifiedFiles, f)
		}
	}

	for baseRel := range baseMap {
		if !currentSeen[baseRel] {
			deletedFiles = append(deletedFiles, path.Join(snapshot.BasePath, baseRel))
		}
	}

	// Run quick malware analysis on Added and Modified files.
	changedFiles := append(addedFiles, modifiedFiles...)
	if len(changedFiles) > 0 {
		var grepPaths []string
		pathMap := make(map[string]*ScannedFile)

		for i := range addedFiles {
			f := &addedFiles[i]
			ext := strings.ToLower(path.Ext(f.Path))
			if dangerousScriptExtensions[ext] || isUnderUploadDir(f.Path) {
				f.ThreatLevel = string(ThreatSuspicious)
				f.Reasons = appendUnique(f.Reasons, "Новый скрипт добавлен после фиксации базового снимка")
			}
			grepPaths = append(grepPaths, f.Path)
			pathMap[f.Path] = f
		}
		for i := range modifiedFiles {
			f := &modifiedFiles[i]
			ext := strings.ToLower(path.Ext(f.Path))
			if dangerousScriptExtensions[ext] {
				f.ThreatLevel = string(ThreatSuspicious)
			}
			grepPaths = append(grepPaths, f.Path)
			pathMap[f.Path] = f
		}

		var activeCustomSignatures []*db.FileSecuritySignatureRecord
		if dbSigs, err := s.database.ListFileSecuritySignatures(ctx, snapshot.ServerID); err == nil {
			for _, cs := range dbSigs {
				if cs.Enabled && strings.TrimSpace(cs.Pattern) != "" {
					activeCustomSignatures = append(activeCustomSignatures, cs)
				}
			}
		}

		if len(grepPaths) > 0 && len(grepPaths) <= 200 {
			escapedPaths := make([]string, len(grepPaths))
			for idx, p := range grepPaths {
				escapedPaths[idx] = ssh.ShellEscape(p)
			}
			grepCmd := fmt.Sprintf(
				"grep -i -E -n -H -m 2 %s %s 2>/dev/null",
				ssh.ShellEscape(webshellRegexPattern),
				strings.Join(escapedPaths, " "),
			)
			grepOut, _ := s.execCmd(ctx, client, grepCmd)
			if grepOut != "" {
				gScanner := bufio.NewScanner(strings.NewReader(grepOut))
				for gScanner.Scan() {
					parts := strings.SplitN(gScanner.Text(), ":", 3)
					if len(parts) >= 3 {
						matchedPath := parts[0]
						lineNum := parts[1]
						snippet := strings.TrimSpace(parts[2])
						if len(snippet) > 160 {
							snippet = snippet[:160] + "..."
						}
						if f, ok := pathMap[matchedPath]; ok {
							f.ThreatLevel = string(ThreatMalicious)
							f.Reasons = appendUnique(f.Reasons, fmt.Sprintf("Обнаружена вредоносная сигнатура (строка %s)", lineNum))
							if f.Snippet == "" {
								f.Snippet = snippet
							}
						}
					}
				}
			}

			for _, cs := range activeCustomSignatures {
				csGrepCmd := fmt.Sprintf(
					"grep -i -E -n -H -m 2 %s %s 2>/dev/null",
					ssh.ShellEscape(cs.Pattern),
					strings.Join(escapedPaths, " "),
				)
				csOut, _ := s.execCmd(ctx, client, csGrepCmd)
				if csOut != "" {
					csScanner := bufio.NewScanner(strings.NewReader(csOut))
					for csScanner.Scan() {
						parts := strings.SplitN(csScanner.Text(), ":", 3)
						if len(parts) >= 3 {
							matchedPath := parts[0]
							lineNum := parts[1]
							snippet := strings.TrimSpace(parts[2])
							if len(snippet) > 160 {
								snippet = snippet[:160] + "..."
							}
							if f, ok := pathMap[matchedPath]; ok {
								if cs.ThreatLevel == string(ThreatMalicious) {
									f.ThreatLevel = string(ThreatMalicious)
								} else if f.ThreatLevel != string(ThreatMalicious) {
									f.ThreatLevel = string(ThreatSuspicious)
								}
								f.Reasons = appendUnique(f.Reasons, fmt.Sprintf("Сигнатура \"%s\" (строка %s)", cs.Name, lineNum))
								if f.Snippet == "" {
									f.Snippet = snippet
								}
							}
						}
					}
				}
			}
		}
	}

	return &SnapshotDiffResult{
		SnapshotID:    snapshot.ID,
		SnapshotName:  snapshot.Name,
		BasePath:      snapshot.BasePath,
		AddedFiles:    addedFiles,
		ModifiedFiles: modifiedFiles,
		DeletedFiles:  deletedFiles,
		TotalAdded:    len(addedFiles),
		TotalModified: len(modifiedFiles),
		TotalDeleted:  len(deletedFiles),
		DurationMs:    time.Since(start).Milliseconds(),
	}, nil
}

func validateSafeFilePath(p string) error {
	if p == "" || p == "." || !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return fmt.Errorf("недопустимый путь к файлу")
	}
	forbidden := []string{
		"/", "/etc", "/etc/passwd", "/etc/shadow", "/bin", "/sbin", "/usr", "/boot",
		"/lib", "/lib64", "/sys", "/proc", "/dev", "/root", "/var",
	}
	for _, f := range forbidden {
		if p == f {
			return fmt.Errorf("операции над системным каталогом '%s' заблокированы", p)
		}
	}
	return nil
}

func isUnderUploadDir(p string) bool {
	low := strings.ToLower(p)
	for _, dir := range uploadDirNames {
		if strings.Contains(low, "/"+dir+"/") || strings.HasSuffix(low, "/"+dir) {
			return true
		}
	}
	return false
}

func isPathExcluded(filePath string, exclusions []*db.FileSecurityExclusionRecord) bool {
	// Scanned paths are always POSIX-style, so matching must not depend on the
	// host OS separator (filepath.Match on Windows lets "*" cross "/").
	for _, exc := range exclusions {
		pattern := strings.TrimSpace(exc.Pattern)
		if pattern == "" {
			continue
		}
		if filePath == pattern {
			return true
		}
		if !strings.ContainsAny(pattern, "*?[") {
			cleanPattern := strings.TrimSuffix(pattern, "/")
			if filePath == cleanPattern ||
				strings.HasPrefix(filePath, cleanPattern+"/") ||
				strings.Contains(filePath, "/"+strings.Trim(cleanPattern, "/")+"/") ||
				strings.HasSuffix(filePath, "/"+cleanPattern) ||
				strings.Contains(filePath, pattern) {
				return true
			}
			continue
		}
		if strings.HasSuffix(pattern, "/*") {
			dir := strings.TrimSuffix(pattern, "/*")
			if filePath == dir || strings.HasPrefix(filePath, dir+"/") {
				return true
			}
			cleanDir := strings.Trim(dir, "/")
			if strings.Contains(filePath, "/"+cleanDir+"/") || strings.HasSuffix(filePath, "/"+cleanDir) {
				return true
			}
		}
		if strings.HasPrefix(pattern, "*/") && strings.HasSuffix(pattern, "/*") {
			mid := strings.TrimSuffix(strings.TrimPrefix(pattern, "*/"), "/*")
			if strings.Contains(filePath, "/"+mid+"/") || strings.HasSuffix(filePath, "/"+mid) {
				return true
			}
		}
		if strings.HasPrefix(pattern, "*.") && !strings.Contains(pattern[2:], "/") {
			if strings.HasSuffix(filePath, pattern[1:]) {
				return true
			}
		}
		if matched, _ := path.Match(pattern, filePath); matched {
			return true
		}
		segments := strings.Split(strings.Trim(filePath, "/"), "/")
		for i := range segments {
			if matched, _ := path.Match(pattern, strings.Join(segments[i:], "/")); matched {
				return true
			}
		}
	}
	return false
}

func formatByteSize(b int64) string {
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

func appendUnique(slice []string, val string) []string {
	for _, item := range slice {
		if item == val {
			return slice
		}
	}
	return append(slice, val)
}
