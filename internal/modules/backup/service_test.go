package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

func TestBackupServiceTaskEnqueue(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	cfg := &config.AppConfig{
		DataDir:        tmpDir,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
	}

	database, err := db.Open(filepath.Join(tmpDir, "test.sqlite"))
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Seed server for foreign key.
	_ = database.SaveServer(ctx, &db.ServerRecord{
		ID:              "srv-backup-1",
		Name:            "Backup Node 1",
		Host:            "127.0.0.1",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "dummy",
		SSHHostKey:      "SHA256:dummy",
		Mode:            "management",
	})

	saltPath := filepath.Join(tmpDir, "salt.bin")
	_ = os.WriteFile(saltPath, []byte("12345678901234567890123456789012"), 0600)

	vault, err := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	taskEngine := tasks.NewEngine(database, 2)
	pool := ssh.NewPool()

	srv := NewService(cfg, database, vault, pool, taskEngine)

	// 1. Enqueue Backup Create.
	taskCreate, err := srv.CreateBackup(ctx, CreateBackupRequest{
		ServerID:      "srv-backup-1",
		BackupType:    "site",
		TargetName:    "example.com",
		Destination:   "local",
		RetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("failed to enqueue create backup: %v", err)
	}
	if taskCreate.OperationType != "backup_create" {
		t.Errorf("expected backup_create, got %s", taskCreate.OperationType)
	}

	// 2. Enqueue Backup Restore.
	taskRestore, err := srv.RestoreBackup(ctx, RestoreBackupRequest{
		ServerID: "srv-backup-1",
		BackupID: "b1",
		FilePath: "/var/backups/TameStack/site_example.com_20261113.tar.gz",
	})
	if err != nil {
		t.Fatalf("failed to enqueue restore backup: %v", err)
	}
	if taskRestore.OperationType != "backup_restore" {
		t.Errorf("expected backup_restore, got %s", taskRestore.OperationType)
	}
}

// TestVerifyLocalArchiveIntegrity verifies that valid archives pass and corrupt archives fail.
func TestVerifyLocalArchiveIntegrity(t *testing.T) {
	tmpDir := testutil.Workspace(t)

	// 1. Create a valid small tar.gz.
	validPath := filepath.Join(tmpDir, "valid.tar.gz")
	f, err := os.Create(validPath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	// Write dummy data with proper gzip header.
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	content := []byte("hello world")
	hdr := &tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("failed to write tar body: %v", err)
	}
	tw.Close()
	gz.Close()
	f.Close()

	if err := VerifyLocalArchiveIntegrity(validPath); err != nil {
		t.Errorf("expected valid archive to pass, got error: %v", err)
	}

	// 2. Create corrupted / truncated archive.
	corruptPath := filepath.Join(tmpDir, "corrupt.tar.gz")
	rawBytes, _ := os.ReadFile(validPath)
	// Truncate to half.
	_ = os.WriteFile(corruptPath, rawBytes[:len(rawBytes)/2], 0644)

	if err := VerifyLocalArchiveIntegrity(corruptPath); err == nil {
		t.Errorf("expected truncated archive to fail integrity check, but it passed")
	}
}

// TestDetectDangerousPaths ensures critical system paths in archives are detected and rejected.
func TestDetectDangerousPaths(t *testing.T) {
	testEntries := []string{
		"var/www/example.com/index.php",
		"var/www/example.com/public/robots.txt",
		"etc/passwd",
		"/etc/shadow",
		"bin/bash",
		"/usr/bin/python3",
		"lib/modules/test.ko",
		"boot/vmlinuz",
		"../etc/passwd",
		"wp-content/themes/default/index.php",
	}

	dangerous := DetectDangerousPaths(testEntries)

	expectedDangerous := map[string]bool{
		"etc/passwd":                     true,
		"/etc/shadow":                    true,
		"bin/bash":                       true,
		"/usr/bin/python3":               true,
		"lib/modules/test.ko":            true,
		"boot/vmlinuz":                   true,
		"../etc/passwd (path traversal)": true,
	}

	if len(dangerous) != len(expectedDangerous) {
		t.Fatalf("expected %d dangerous paths, got %d: %v", len(expectedDangerous), len(dangerous), dangerous)
	}

	for _, d := range dangerous {
		if !expectedDangerous[d] {
			t.Errorf("unexpected dangerous path detected: %s", d)
		}
	}
}
