package db

import (
	"context"
	"path/filepath"
	"testing"

	"tamestack/internal/testutil"
)

func openCleanupTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := Open(filepath.Join(testutil.Workspace(t), "cleanup.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestDeleteSiteRemovesOrphanedRecords(t *testing.T) {
	database := openCleanupTestDB(t)
	ctx := context.Background()

	srv := &ServerRecord{
		ID: "srv-1", Name: "Test", Host: "127.0.0.1", Port: 22,
		Username: "root", AuthType: "key", EncryptedSecret: "x", SSHHostKey: "SHA256:test",
	}
	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}
	for _, id := range []string{"site-1", "site-2"} {
		site := &SiteRecord{
			ID: id, ServerID: "srv-1", Domain: id + ".example.com",
			DocumentRoot: "/var/www/" + id, SiteType: "static", WebServer: "nginx", Status: "active",
		}
		if err := database.SaveSite(ctx, site); err != nil {
			t.Fatalf("failed to save site %s: %v", id, err)
		}
		backup := &SiteConfigBackupRecord{
			ID: "backup-" + id, ServerID: "srv-1", SiteID: id,
			FilePath: "/etc/nginx/sites-available/" + id, ContentHash: "hash", Content: "server {}",
		}
		if err := database.SaveSiteConfigBackup(ctx, backup); err != nil {
			t.Fatalf("failed to save config backup for %s: %v", id, err)
		}
		if _, err := database.ExecContext(ctx, `
			INSERT INTO site_file_snapshots (id, site_id, server_id, document_root, manifest_json)
			VALUES (?, ?, 'srv-1', ?, '{}')`, "snap-"+id, id, "/var/www/"+id); err != nil {
			t.Fatalf("failed to insert file snapshot for %s: %v", id, err)
		}
		if _, err := database.ExecContext(ctx, `
			INSERT INTO site_audit_findings (id, snapshot_id, path, rule_id, summary)
			VALUES (?, ?, '/var/www/x.php', 'rule-1', 'finding')`, "find-"+id, "snap-"+id); err != nil {
			t.Fatalf("failed to insert audit finding for %s: %v", id, err)
		}
	}

	if err := database.DeleteSite(ctx, "site-1"); err != nil {
		t.Fatalf("failed to delete site: %v", err)
	}

	sites, err := database.ListSites(ctx, "srv-1")
	if err != nil || len(sites) != 1 || sites[0].ID != "site-2" {
		t.Fatalf("expected only site-2 to remain, got %+v (err: %v)", sites, err)
	}

	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := database.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("count query failed: %v", err)
		}
		return n
	}

	if n := count(`SELECT COUNT(*) FROM site_config_backups WHERE site_id = 'site-1'`); n != 0 {
		t.Fatalf("expected config backups of deleted site to be removed, got %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM site_config_backups WHERE site_id = 'site-2'`); n != 1 {
		t.Fatalf("expected other site's config backup to survive, got %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM site_file_snapshots WHERE site_id = 'site-1'`); n != 0 {
		t.Fatalf("expected file snapshots of deleted site to be removed, got %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM site_audit_findings WHERE snapshot_id = 'snap-site-1'`); n != 0 {
		t.Fatalf("expected audit findings of deleted site to be removed, got %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM site_file_snapshots WHERE site_id = 'site-2'`); n != 1 {
		t.Fatalf("expected other site's snapshot to survive, got %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM site_audit_findings WHERE snapshot_id = 'snap-site-2'`); n != 1 {
		t.Fatalf("expected other site's finding to survive, got %d", n)
	}
}

func TestDeleteAIMemoryFactsScopedAndAtomic(t *testing.T) {
	database := openCleanupTestDB(t)
	ctx := context.Background()

	srv1, srv2 := "srv-1", "srv-2"
	for _, id := range []string{srv1, srv2} {
		srv := &ServerRecord{
			ID: id, Name: id, Host: "127.0.0.1", Port: 22,
			Username: "root", AuthType: "key", EncryptedSecret: "x", SSHHostKey: "SHA256:test",
		}
		if err := database.SaveServer(ctx, srv); err != nil {
			t.Fatalf("failed to save server %s: %v", id, err)
		}
	}
	facts := []*AIMemoryFactRecord{
		{ID: "fact-1", ServerID: &srv1, Category: "web", FactKey: "nginx_port", FactValue: "Nginx listens on port 8080", Source: "manual"},
		{ID: "fact-2", ServerID: &srv2, Category: "web", FactKey: "apache_port", FactValue: "Apache listens on port 8081", Source: "manual"},
		{ID: "fact-3", Category: "preference", FactKey: "editor", FactValue: "User prefers nano over vim", Source: "manual"},
	}
	for _, f := range facts {
		if err := database.SaveAIMemoryFact(ctx, f); err != nil {
			t.Fatalf("failed to save fact %s: %v", f.ID, err)
		}
	}

	if err := database.DeleteAIMemoryFacts(ctx, &srv1); err != nil {
		t.Fatalf("failed to delete scoped facts: %v", err)
	}
	if n, err := database.CountAIMemoryFacts(ctx); err != nil || n != 2 {
		t.Fatalf("expected 2 facts after scoped delete, got %d (err: %v)", n, err)
	}
	var ftsCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_memory_fts WHERE fact_id = 'fact-1'`).Scan(&ftsCount); err != nil {
		t.Fatalf("failed to count fts rows: %v", err)
	}
	if ftsCount != 0 {
		t.Fatalf("expected fts rows of deleted fact to be removed, got %d", ftsCount)
	}
	if res, err := database.SearchAIMemory(ctx, "Apache", nil, 5); err != nil || len(res) != 1 {
		t.Fatalf("expected other server's fact to stay searchable, got %+v (err: %v)", res, err)
	}

	if err := database.DeleteAIMemoryFacts(ctx, nil); err != nil {
		t.Fatalf("failed to delete all facts: %v", err)
	}
	if n, err := database.CountAIMemoryFacts(ctx); err != nil || n != 0 {
		t.Fatalf("expected 0 facts after full delete, got %d (err: %v)", n, err)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_memory_fts`).Scan(&ftsCount); err != nil {
		t.Fatalf("failed to count fts rows: %v", err)
	}
	if ftsCount != 0 {
		t.Fatalf("expected empty fts index after full delete, got %d rows", ftsCount)
	}
}
