package server

import (
	"context"
	"path/filepath"
	"testing"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
	"tamestack/internal/testutil"
)

func TestSetAndGetServerMode(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "mode_test.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID:       "srv-mode-1",
		Name:     "Mode Test",
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
		AuthType: "password",
	}); err != nil {
		t.Fatalf("SaveServer failed: %v", err)
	}

	svc := &Service{
		database: database,
		engine:   tasks.NewEngine(database, 1),
	}

	mode, err := svc.GetServerMode(ctx, "srv-mode-1")
	if err != nil {
		t.Fatalf("GetServerMode failed: %v", err)
	}
	if mode != "audit" {
		t.Errorf("default mode = %q, want audit", mode)
	}

	if err := svc.SetServerMode(ctx, "srv-mode-1", "invalid"); err == nil {
		t.Errorf("expected error for invalid mode, got nil")
	}

	if err := svc.SetServerMode(ctx, "srv-mode-1", "management"); err != nil {
		t.Fatalf("SetServerMode(management) failed: %v", err)
	}

	mode, err = svc.GetServerMode(ctx, "srv-mode-1")
	if err != nil {
		t.Fatalf("GetServerMode after update failed: %v", err)
	}
	if mode != "management" {
		t.Errorf("updated mode = %q, want management", mode)
	}
}
