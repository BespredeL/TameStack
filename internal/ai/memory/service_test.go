package memory

import (
	"context"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"

	aiContext "tamestack/internal/ai/context"
	"tamestack/internal/db"
)

func TestMemoryService(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_memory.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	svc := NewService(database, nil)
	srvID := "srv-1"

	// Insert server record to satisfy foreign key constraint.
	_ = database.SaveServer(ctx, &db.ServerRecord{
		ID:       srvID,
		Name:     "Test Node",
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
		Status:   "online",
	})

	// 1. Save Fact.
	fact, err := svc.SaveFact(ctx, &srvID, "web", "nginx_fastcgi", "Fastcgi pass configured to unix:/run/php/php8.2-fpm.sock", "manual")
	if err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}
	if fact.FactKey != "nginx_fastcgi" {
		t.Fatalf("unexpected fact key: %s", fact.FactKey)
	}

	// 2. List Facts.
	facts, err := svc.ListFacts(ctx, &srvID)
	if err != nil || len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d (err: %v)", len(facts), err)
	}

	// 3. FTS Search.
	matched, err := svc.SearchFacts(ctx, "fastcgi", &srvID, 5)
	if err != nil || len(matched) != 1 {
		t.Fatalf("expected 1 match for 'fastcgi', got %d (err: %v)", len(matched), err)
	}

	// 4. Auto Extract Server Facts.
	snapshot := &aiContext.ServerSnapshot{
		Server: &db.ServerRecord{ID: srvID, Name: "Node 1"},
		Diagnostics: &db.DiagnosticsRecord{
			OSName:              "Debian",
			OSVersion:           "12 Bookworm",
			Arch:                "x86_64",
			KernelVersion:       "6.1.0",
			InstalledWebServers: []string{"nginx"},
			InstalledDBVersions: []string{"mariadb"},
		},
		Sites: []*db.SiteRecord{
			{Domain: "example.com", WebServer: "nginx", PHPVersion: "8.2"},
		},
	}

	if err := svc.ExtractServerFacts(ctx, snapshot); err != nil {
		t.Fatalf("extract server facts failed: %v", err)
	}

	allFacts, err := svc.ListFacts(ctx, &srvID)
	if err != nil || len(allFacts) < 4 {
		t.Fatalf("expected at least 4 facts after extraction, got %d", len(allFacts))
	}
}
