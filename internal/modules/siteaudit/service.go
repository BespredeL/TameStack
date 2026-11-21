package siteaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

const manifestScript = `
cd "$SITE_ROOT" || exit 1
find . -xdev \( -path ./vendor -o -path './vendor/*' -o -path ./node_modules -o -path './node_modules/*' \) -prune -o -type f -print |
while IFS= read -r f; do
  rel=${f#./}
  case "$rel" in *..*) continue ;; esac
  hash=$(sha256sum -- "$f" 2>/dev/null | awk '{print $1}')
  size=$(wc -c < "$f" | tr -d ' ')
  mode=$(stat -c %a "$f" 2>/dev/null || echo 0)
  owner=$(stat -c %U "$f" 2>/dev/null || echo unknown)
  mtime=$(stat -c %Y "$f" 2>/dev/null || echo 0)
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$rel" "$size" "$mode" "$owner" "$mtime" "$hash"
done
`

// Snapshot is a stored manifest of one site.
type Snapshot struct {
	ID           string    `json:"id"`
	SiteID       string    `json:"site_id"`
	ServerID     string    `json:"server_id"`
	DocumentRoot string    `json:"document_root"`
	IsBaseline   bool      `json:"is_baseline"`
	FileCount    int       `json:"file_count"`
	Added        []string  `json:"added"`
	Removed      []string  `json:"removed"`
	Changed      []string  `json:"changed"`
	Findings     []Finding `json:"findings"`
	CreatedAt    time.Time `json:"created_at"`
}

type Service struct {
	cfg      *config.AppConfig
	database *db.DB
	vault    *crypto.Vault
	pool     *ssh.Pool
	engine   *tasks.Engine
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{cfg: cfg, database: database, vault: vault, pool: pool, engine: engine}
	if engine != nil {
		engine.RegisterHandler("site_file_quarantine", s.handleQuarantine)
	}
	return s
}

func (s *Service) getSSH(ctx context.Context, serverID string) (*ssh.SSHClient, error) {
	client, _, err := module.GetSSHClient(ctx, s.cfg, s.database, s.vault, s.pool, serverID)
	return client, err
}

// Capture walks the document root, stores a manifest, and returns findings.
// The walk only reads. includeHeavy hashes vendor and node_modules too.
func (s *Service) Capture(ctx context.Context, siteID string, makeBaseline bool) (*Snapshot, error) {
	site, err := s.database.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if site.DocumentRoot == "" || site.DocumentRoot == "/" {
		return nil, fmt.Errorf("у сайта нет безопасного document root")
	}
	client, err := s.getSSH(ctx, site.ServerID)
	if err != nil {
		return nil, err
	}
	script := manifestScript
	out, stderr, code, err := client.NewCommand(script).WithSudo().WithEnv("SITE_ROOT", site.DocumentRoot).Exec(ctx)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("снимок не снят: %s %v", strings.TrimSpace(string(stderr)), err)
	}
	files := ParseManifest(string(out))
	baseFiles, _ := s.baselineFiles(ctx, site.ID)
	diff := CompareManifests(baseFiles, files)
	excerpts := s.readExcerpts(ctx, client, site.DocumentRoot, files, diff)
	findings := EvaluateRules(files, diff, excerpts)

	snap := &Snapshot{
		ID: uuid.New().String(), SiteID: site.ID, ServerID: site.ServerID,
		DocumentRoot: site.DocumentRoot, IsBaseline: makeBaseline || len(baseFiles) == 0,
		FileCount: len(files), Added: diff.Added, Removed: diff.Removed, Changed: diff.Changed,
		Findings: findings, CreatedAt: time.Now(),
	}
	if err := s.save(ctx, snap, files); err != nil {
		return nil, err
	}
	return snap, nil
}

func (s *Service) readExcerpts(ctx context.Context, client *ssh.SSHClient, root string, files []FileEntry, diff Diff) map[string]string {
	want := map[string]bool{}
	for _, p := range append(append([]string{}, diff.Added...), diff.Changed...) {
		want[p] = true
	}
	out := map[string]string{}
	n := 0
	for _, f := range files {
		if n >= 30 || !want[f.Path] {
			continue
		}
		ext := strings.ToLower(f.Path)
		if !strings.HasSuffix(ext, ".php") && !strings.HasSuffix(ext, ".phtml") && !strings.HasSuffix(ext, ".js") && !strings.HasSuffix(ext, ".htaccess") {
			continue
		}
		full := strings.TrimRight(root, "/") + "/" + f.Path
		cmd := fmt.Sprintf("head -c 4096 %s", ssh.ShellEscape(full))
		body, _, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
		if err != nil || code != 0 {
			continue
		}
		out[f.Path] = string(body)
		n++
	}
	return out
}

func (s *Service) save(ctx context.Context, snap *Snapshot, files []FileEntry) error {
	manifest, err := json.Marshal(files)
	if err != nil {
		return err
	}
	base := 0
	if snap.IsBaseline {
		base = 1
		_, _ = s.database.ExecContext(ctx, `UPDATE site_file_snapshots SET is_baseline = 0 WHERE site_id = ?`, snap.SiteID)
	}
	_, err = s.database.ExecContext(ctx, `
		INSERT INTO site_file_snapshots (id, site_id, server_id, document_root, is_baseline, file_count, manifest_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, snap.ID, snap.SiteID, snap.ServerID, snap.DocumentRoot, base, snap.FileCount, string(manifest), snap.CreatedAt)
	if err != nil {
		return err
	}
	for _, f := range snap.Findings {
		_, _ = s.database.ExecContext(ctx, `
			INSERT INTO site_audit_findings (id, snapshot_id, path, rule_id, summary) VALUES (?, ?, ?, ?, ?)
		`, uuid.New().String(), snap.ID, f.Path, f.RuleID, f.Summary)
	}
	return nil
}

func (s *Service) baselineFiles(ctx context.Context, siteID string) ([]FileEntry, error) {
	var raw string
	err := s.database.QueryRowContext(ctx, `
		SELECT manifest_json FROM site_file_snapshots
		WHERE site_id = ? AND is_baseline = 1 ORDER BY created_at DESC LIMIT 1
	`, siteID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var files []FileEntry
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return nil, err
	}
	return files, nil
}

// ListFindings returns findings of the latest snapshot.
func (s *Service) ListFindings(ctx context.Context, siteID string) ([]Finding, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT f.path, f.rule_id, f.summary
		FROM site_audit_findings f
		JOIN site_file_snapshots s ON s.id = f.snapshot_id
		WHERE s.site_id = ?
		ORDER BY s.created_at DESC, f.path ASC
		LIMIT 200
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.Path, &f.RuleID, &f.Summary); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
