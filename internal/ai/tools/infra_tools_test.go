package tools

import (
	"context"
	"path/filepath"
	"testing"

	"tamestack/internal/db"
	"tamestack/internal/testutil"
)

func TestInfraToolsRegistration(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "infra_test.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID:       "srv-infra-1",
		Name:     "Infra Test Srv",
		Host:     "10.0.0.1",
		Port:     22,
		Username: "root",
	}); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	r := NewRegistry(database, nil)
	// Register infra tools with nil service pointers to verify registration and tool schemas
	RegisterInfraTools(r, database, nil, nil, nil, nil, nil, nil, nil)

	expectedTools := []string{
		"inspect_docker_containers",
		"get_docker_container_logs",
		"inspect_ssl_certificates",
		"list_cron_jobs",
		"check_package_updates",
		"analyze_disk_usage",
		"list_system_processes",
		"list_backups",
	}

	for _, name := range expectedTools {
		tool, exists := r.GetTool(name)
		if !exists {
			t.Errorf("tool %s was not registered", name)
			continue
		}
		if tool.Definition.RiskLevel != "safe" {
			t.Errorf("expected tool %s to be safe, got %s", name, tool.Definition.RiskLevel)
		}
		if tool.Definition.IsMutating {
			t.Errorf("expected tool %s to be non-mutating", name)
		}
	}

	// Verify executing with uninitialized service returns descriptive error
	dockerTool, _ := r.GetTool("inspect_docker_containers")
	_, err = dockerTool.Handler(ctx, map[string]any{"server_id": "srv-infra-1"})
	if err == nil {
		t.Errorf("expected error when calling inspect_docker_containers with nil service")
	}

	psTool, _ := r.GetTool("list_system_processes")
	_, err = psTool.Handler(ctx, map[string]any{"server_id": "srv-infra-1"})
	if err == nil {
		t.Errorf("expected error when calling list_system_processes with nil service")
	}
}
