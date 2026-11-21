package backup

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/ssh"

	"github.com/google/uuid"
)

// DefaultBackupDir mirrors the TargetPath default of handleBackupCreateTask.
const DefaultBackupDir = "/var/backups/TameStack"

// DiscoverBackupsResult summarizes a discovery run over one server.
type DiscoverBackupsResult struct {
	ScannedPaths   []string           `json:"scanned_paths"`
	Found          int                `json:"found"`
	AlreadyKnown   int                `json:"already_known"`
	Registered     []*db.BackupRecord `json:"registered"`
	IgnoredUnknown int                `json:"ignored_unknown"`
}

// discoveredName captures the fields encoded in an archive file name.
// The grammar is the unchanged template from handleBackupCreateTask:
// site_<target>_<YYYYMMDD>_<HHMMSS>.tar.gz, db_<target>_<...>.sql.gz,
// configs_<...>.tar.gz, full_backup_<...>.tar.gz.
type discoveredName struct {
	BackupType string
	TargetName string
	CreatedAt  time.Time
}

var discoveryPatterns = []struct {
	re         *regexp.Regexp
	backupType string
	hasTarget  bool
}{
	{regexp.MustCompile(`^site_(.+)_(\d{8}_\d{6})\.tar\.gz$`), "site", true},
	{regexp.MustCompile(`^db_(.+)_(\d{8}_\d{6})\.sql\.gz$`), "database", true},
	{regexp.MustCompile(`^configs_(\d{8}_\d{6})\.tar\.gz$`), "config", false},
	{regexp.MustCompile(`^full_backup_(\d{8}_\d{6})\.tar\.gz$`), "full", false},
}

// parseArchiveName decodes a TameStack archive file name. ok=false for any
// file that does not follow the template, so foreign archives are never
// claimed as TameStack backups.
func parseArchiveName(name string) (discoveredName, bool) {
	for _, p := range discoveryPatterns {
		m := p.re.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		ts := m[len(m)-1]
		created, err := time.Parse("20060102_150405", ts)
		if err != nil {
			return discoveredName{}, false
		}
		target := ""
		if p.hasTarget {
			target = m[1]
		}
		return discoveredName{BackupType: p.backupType, TargetName: target, CreatedAt: created}, true
	}
	return discoveredName{}, false
}

// parseFindOutput turns `find -printf '%s %p\n'` output into candidate records
// for the given server. Files whose path is already registered count as known;
// names outside the TameStack template are reported as ignored.
func parseFindOutput(output, serverID string, knownPaths map[string]bool) (records []*db.BackupRecord, alreadyKnown, unknown int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sizeStr, filePath, ok := strings.Cut(line, " ")
		if !ok || !strings.HasPrefix(filePath, "/") {
			continue
		}
		if knownPaths[filePath] {
			alreadyKnown++
			continue
		}
		info, matched := parseArchiveName(path.Base(filePath))
		if !matched {
			unknown++
			continue
		}
		size, _ := strconv.ParseInt(sizeStr, 10, 64)
		records = append(records, &db.BackupRecord{
			ID:          uuid.New().String(),
			ServerID:    serverID,
			BackupType:  info.BackupType,
			TargetName:  info.TargetName,
			Destination: "local",
			FilePath:    filePath,
			SizeBytes:   size,
			Status:      "completed",
			Verified:    false, // integrity of pre-existing archives is unproven
			CreatedAt:   info.CreatedAt,
		})
	}
	return records, alreadyKnown, unknown
}

// DiscoverBackups scans the remote backup directories for archives produced
// by any TameStack version and registers the ones missing from the database.
// Remote operations are read-only; writes go to the local database only.
func (s *Service) DiscoverBackups(ctx context.Context, serverID string, targetPath string) (*DiscoverBackupsResult, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if targetPath == "" {
		targetPath = DefaultBackupDir
	}
	if err := ssh.ValidateRemotePath(targetPath, false); err != nil {
		return nil, err
	}

	dirs := []string{targetPath}
	outHome, _, _, _ := client.NewCommand("echo $HOME").Exec(ctx)
	if home := parseHomeDir(string(outHome)); home != "" {
		fallback := path.Join(home, ".TameStack", "backups")
		if fallback != targetPath {
			dirs = append(dirs, fallback)
		}
	}

	existing, err := s.Database.ListBackups(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать список известных бэкапов: %w", err)
	}
	knownMap := make(map[string]bool, len(existing))
	for _, b := range existing {
		if b != nil && b.FilePath != "" {
			knownMap[b.FilePath] = true
		}
	}

	result := &DiscoverBackupsResult{ScannedPaths: dirs}
	for _, dir := range dirs {
		escaped := ssh.ShellEscape(dir)
		out, _, _, err := s.execCmd(ctx, client, fmt.Sprintf(
			"find %s -maxdepth 1 -type f -printf '%%s %%p\\n' 2>/dev/null", escaped))
		if err != nil {
			continue // directory missing or unreadable
		}
		candidates, known, unknown := parseFindOutput(out, serverID, knownMap)
		result.AlreadyKnown += known
		result.IgnoredUnknown += unknown
		for _, rec := range candidates {
			if err := s.Database.SaveBackup(ctx, rec); err != nil {
				return result, fmt.Errorf("не удалось зарегистрировать %s: %w", rec.FilePath, err)
			}
			knownMap[rec.FilePath] = true
			result.Found++
			result.Registered = append(result.Registered, rec)
		}
	}
	return result, nil
}
