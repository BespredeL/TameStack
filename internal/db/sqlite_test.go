package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_TameStack.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// 1. Create Server.
	srv := &ServerRecord{
		ID:              "srv-1",
		Name:            "Production Web 1",
		Host:            "192.168.1.100",
		Port:            22,
		Username:        "root",
		AuthType:        "key",
		EncryptedSecret: "encrypted_key_data",
		SSHHostKey:      "SHA256:abc123testfingerprint",
		GroupName:       "Production",
		Tags:            []string{"web", "prod"},
		IsFavorite:      true,
		Status:          "online",
	}

	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	// 2. Fetch Server.
	fetched, err := database.GetServer(ctx, "srv-1")
	if err != nil {
		t.Fatalf("failed to get server: %v", err)
	}
	if fetched.Name != srv.Name || fetched.Host != srv.Host || len(fetched.Tags) != 2 {
		t.Fatalf("mismatched server data: %+v", fetched)
	}

	// 3. Diagnostics
	diag := &DiagnosticsRecord{
		ServerID:             "srv-1",
		OSName:               "Ubuntu",
		OSVersion:            "24.04 LTS",
		OSFamily:             "debian",
		KernelVersion:        "6.8.0-generic",
		Arch:                 "x86_64",
		CPUCores:             4,
		CPUModel:             "AMD EPYC",
		RAMTotalBytes:        8 * 1024 * 1024 * 1024,
		DiskTotalBytes:       100 * 1024 * 1024 * 1024,
		DiskUsedBytes:        20 * 1024 * 1024 * 1024,
		UptimeSeconds:        123456,
		SSHVersion:           "OpenSSH_9.6p1",
		InstalledWebServers:  []string{"nginx"},
		InstalledPHPVersions: []string{"8.2", "8.3"},
		InstalledDBVersions:  []string{"mysql"},
		DockerVersion:        "26.1.0",
	}

	if err := database.SaveDiagnostics(ctx, diag); err != nil {
		t.Fatalf("failed to save diagnostics: %v", err)
	}

	fetchedDiag, err := database.GetDiagnostics(ctx, "srv-1")
	if err != nil {
		t.Fatalf("failed to get diagnostics: %v", err)
	}
	if fetchedDiag.OSName != "Ubuntu" || len(fetchedDiag.InstalledPHPVersions) != 2 {
		t.Fatalf("mismatched diagnostics data: %+v", fetchedDiag)
	}

	// 4. Tasks and Logs.
	task := &TaskRecord{
		ID:            "task-1",
		ServerID:      "srv-1",
		OperationType: "system_diag",
		Title:         "System Diagnostics",
		Status:        "running",
		CurrentStep:   "Checking packages",
		Progress:      50,
		CreatedAt:     time.Now(),
	}

	if err := database.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	if err := database.AppendTaskLog(ctx, "task-1", "info", "Starting scan"); err != nil {
		t.Fatalf("failed to append log: %v", err)
	}

	logs, err := database.GetTaskLogs(ctx, "task-1")
	if err != nil {
		t.Fatalf("failed to get task logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Message != "Starting scan" {
		t.Fatalf("unexpected logs: %+v", logs)
	}

	_ = database.Close()
}

func TestFileSecuritySignaturesCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_signatures.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	sig := &FileSecuritySignatureRecord{
		ID:          "sig-1",
		ServerID:    "",
		Name:        "Crypto Miner",
		Pattern:     "(stratum\\+tcp|xmrig)",
		ThreatLevel: "malicious",
		Description: "Detects miner pools and binaries",
		Enabled:     true,
	}

	if err := database.SaveFileSecuritySignature(ctx, sig); err != nil {
		t.Fatalf("failed to save signature: %v", err)
	}

	list, err := database.ListFileSecuritySignatures(ctx, "srv-1")
	if err != nil {
		t.Fatalf("failed to list signatures: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Crypto Miner" || !list[0].Enabled {
		t.Fatalf("unexpected signature list: %+v", list)
	}

	if err := database.ToggleFileSecuritySignature(ctx, "sig-1", false); err != nil {
		t.Fatalf("failed to toggle signature: %v", err)
	}

	got, err := database.GetFileSecuritySignature(ctx, "sig-1")
	if err != nil {
		t.Fatalf("failed to get signature: %v", err)
	}
	if got.Enabled {
		t.Fatal("expected signature to be disabled after toggle")
	}

	if err := database.DeleteFileSecuritySignature(ctx, "sig-1"); err != nil {
		t.Fatalf("failed to delete signature: %v", err)
	}

	listAfter, err := database.ListFileSecuritySignatures(ctx, "srv-1")
	if err != nil {
		t.Fatalf("failed to list signatures after delete: %v", err)
	}
	if len(listAfter) != 0 {
		t.Fatalf("expected 0 signatures after delete, got %d", len(listAfter))
	}
}
