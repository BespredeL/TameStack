package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tamestack/internal/db"
	"tamestack/internal/testutil"
)

func TestMigrationsLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "TameStack_db_migration_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_mig.db")

	// First open -> runs all migrations.
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open and migrate database: %v", err)
	}

	migrations, err := database.GetAppliedMigrations()
	if err != nil {
		t.Fatalf("Failed to get applied migrations: %v", err)
	}
	if len(migrations) != 11 {
		t.Errorf("Expected 11 applied migrations, got: %d", len(migrations))
	}

	// Test vault settings.
	if err := database.SetVaultSetting("has_master_password", "true"); err != nil {
		t.Fatalf("Failed to set vault setting: %v", err)
	}
	val, err := database.GetVaultSetting("has_master_password")
	if err != nil || val != "true" {
		t.Errorf("Failed to get vault setting: val=%s, err=%v", val, err)
	}

	// Check integrity
	ok, err := database.CheckIntegrity()
	if err != nil || !ok {
		t.Errorf("Database integrity check failed: ok=%v, err=%v", ok, err)
	}

	_ = database.Close()

	// Second open -> should be idempotent and create backup.
	database2, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to re-open database: %v", err)
	}
	defer database2.Close()

	backupFile := dbPath + ".backup"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Errorf("Expected db backup file to exist: %s", backupFile)
	}

	migrations2, err := database2.GetAppliedMigrations()
	if err != nil || len(migrations2) != 11 {
		t.Errorf("Expected 11 migrations on re-open, got: %d", len(migrations2))
	}
}

// Rows committed while the write-ahead log still holds them must survive into the
// pre-migration backup, otherwise the backup is missing the newest transactions.
func TestBackupDatabaseCapturesWriteAheadLog(t *testing.T) {
	dir := testutil.Workspace(t)
	dbPath := filepath.Join(dir, "hub.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID: "srv-wal", Name: "wal-box", Host: "10.0.0.2", Port: 22,
		Username: "root", AuthType: "password", EncryptedSecret: "x", Status: "unknown",
	}); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	if st, err := os.Stat(dbPath + "-wal"); err != nil || st.Size() == 0 {
		t.Fatalf("expected pending write-ahead log frames, stat=%v err=%v", st, err)
	}

	if err := db.BackupDatabase(dbPath); err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	backup, err := os.ReadFile(dbPath + ".backup")
	if err != nil {
		t.Fatalf("backup not readable: %v", err)
	}
	restoredPath := filepath.Join(dir, "from-backup.db")
	if err := os.WriteFile(restoredPath, backup, 0600); err != nil {
		t.Fatalf("failed to stage the backup: %v", err)
	}

	restored, err := db.Open(restoredPath)
	if err != nil {
		t.Fatalf("failed to open the backup copy: %v", err)
	}
	defer restored.Close()

	rec, err := restored.GetServer(ctx, "srv-wal")
	if err != nil || rec == nil {
		t.Fatalf("write-ahead log content missing from the backup: %+v err=%v", rec, err)
	}
}
