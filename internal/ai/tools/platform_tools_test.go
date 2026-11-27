package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tamestack/internal/db"
	"tamestack/internal/testutil"
)

func openTestToolsDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "tools_test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestSchemaBuilderRequiredNonNil(t *testing.T) {
	schema := newSchema().str("foo", "bar").build()
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	if strings.Contains(string(raw), `"required":null`) {
		t.Fatalf("expected non-null required array in schema, got: %s", string(raw))
	}
}

func TestRegisterPlatformToolsAndDescriptions(t *testing.T) {
	database := openTestToolsDB(t)
	r := NewRegistry(database, nil)
	RegisterPlatformTools(r, database, nil, nil, nil, nil)

	expected := []string{
		"inspect_site_app",
		"get_app_process_status",
		"list_system_users",
		"list_user_ssh_keys",
		"inspect_mail_server",
		"inspect_dns_zones",
	}

	for _, name := range expected {
		tool, ok := r.GetTool(name)
		if !ok {
			t.Fatalf("expected platform tool %q to be registered", name)
		}
		if tool.Definition.IsMutating {
			t.Errorf("platform tool %q should be non-mutating", name)
		}
		if _, hasEN := toolDescriptionsEN[name]; !hasEN {
			t.Errorf("platform tool %q missing English description in toolDescriptionsEN", name)
		}
	}
}

func TestResolveSiteID(t *testing.T) {
	ctx := context.Background()
	database := openTestToolsDB(t)

	srv := &db.ServerRecord{ID: "srv_1", Name: "Web1", Host: "10.0.0.2", Port: 22, Username: "root"}
	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("save server: %v", err)
	}
	siteRec := &db.SiteRecord{
		ID:           "site_1",
		ServerID:     "srv_1",
		Domain:       "Example.com",
		DocumentRoot: "/var/www/example",
		SiteType:     "php",
		WebServer:    "nginx",
	}
	if err := database.SaveSite(ctx, siteRec); err != nil {
		t.Fatalf("save site: %v", err)
	}

	if id, err := resolveSiteID(ctx, database, "srv_1", "site_1"); err != nil || id != "site_1" {
		t.Fatalf("expected site_1 by ID, got %q (%v)", id, err)
	}
	if id, err := resolveSiteID(ctx, database, "srv_1", "example.com"); err != nil || id != "site_1" {
		t.Fatalf("expected site_1 by domain, got %q (%v)", id, err)
	}
	if _, err := resolveSiteID(ctx, database, "srv_1", "missing.com"); err == nil {
		t.Fatal("expected error for missing site")
	}
}

func TestAnalyzeDiskUsageRejectsUnsafePath(t *testing.T) {
	ctx := context.Background()
	database := openTestToolsDB(t)

	srv := &db.ServerRecord{ID: "srv_1", Name: "Web1", Host: "10.0.0.2", Port: 22, Username: "root"}
	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("save server: %v", err)
	}

	r := NewRegistry(database, nil)
	RegisterInfraTools(r, database, nil, nil, nil, nil, nil, nil, nil)

	tool, ok := r.GetTool("analyze_disk_usage")
	if !ok {
		t.Fatal("analyze_disk_usage not registered")
	}
	_, err := tool.Handler(ctx, map[string]any{
		"server_id": "srv_1",
		"path":      "/var/www; rm -rf /",
	})
	if err == nil {
		t.Fatal("expected ValidateRemotePath to reject unsafe path")
	}
}
