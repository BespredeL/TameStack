package database

import (
	"context"
	"os"
	"path/filepath"
	"tamestack/internal/testutil"
	"strings"
	"testing"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

func setupDatabaseService(t *testing.T) (*Service, *db.DB) {
	t.Helper()

	tmpDir := testutil.Workspace(t)
	cfg := &config.AppConfig{
		DataDir:        tmpDir,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
	}

	database, err := db.Open(filepath.Join(tmpDir, "test.sqlite"))
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	ctx := context.Background()

	// Seed server for foreign key.
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID:              "srv-1",
		Name:            "Test Server",
		Host:            "127.0.0.1",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "dummy",
		SSHHostKey:      "SHA256:dummy",
		Mode:            "management",
	}); err != nil {
		t.Fatalf("failed to seed server: %v", err)
	}

	saltPath := filepath.Join(tmpDir, "salt.bin")
	_ = os.WriteFile(saltPath, []byte("12345678901234567890123456789012"), 0600)

	vault, err := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	taskEngine := tasks.NewEngine(database, 2)
	pool := ssh.NewPool()

	return NewService(cfg, database, vault, pool, taskEngine), database
}

func TestDatabaseServiceTaskEnqueue(t *testing.T) {
	srv, database := setupDatabaseService(t)
	defer database.Close()

	ctx := context.Background()

	// 1. Enqueue Create Database.
	task, err := srv.CreateDatabase(ctx, CreateDatabaseRequest{
		ServerID: "srv-1",
		Engine:   EngineMySQL,
		Name:     "app_production",
	})
	if err != nil {
		t.Fatalf("failed to enqueue create database: %v", err)
	}
	if task.OperationType != "db_create" {
		t.Errorf("expected task operation type db_create, got %s", task.OperationType)
	}

	// 2. Enqueue Create User.
	taskUser, err := srv.CreateUser(ctx, CreateUserRequest{
		ServerID:  "srv-1",
		Engine:    EngineMySQL,
		Username:  "app_user",
		Password:  "SecretPass123!",
		Databases: []string{"app_production"},
	})
	if err != nil {
		t.Fatalf("failed to enqueue create user: %v", err)
	}
	if taskUser.OperationType != "db_user_create" {
		t.Errorf("expected task operation type db_user_create, got %s", taskUser.OperationType)
	}
	if strings.Contains(taskUser.Payload, "SecretPass123!") {
		t.Fatalf("database password stored in task payload: %s", taskUser.Payload)
	}

	// 3. Enqueue Backup.
	taskBackup, err := srv.BackupDatabase(ctx, BackupDatabaseRequest{
		ServerID:     "srv-1",
		Engine:       EnginePostgreSQL,
		DatabaseName: "finance_db",
	})
	if err != nil {
		t.Fatalf("failed to enqueue backup: %v", err)
	}
	if taskBackup.OperationType != "db_backup" {
		t.Errorf("expected task operation type db_backup, got %s", taskBackup.OperationType)
	}
}

// TestDatabaseServiceRejectsHostileIdentifiers asserts identifiers and hosts are validated before
// any SSH connection or task is created, so a payload cannot reach a remote shell or SQL string.
func TestDatabaseServiceRejectsHostileIdentifiers(t *testing.T) {
	srv, database := setupDatabaseService(t)
	defer database.Close()

	ctx := context.Background()

	createUserCases := []struct {
		username string
		host     string
	}{
		{"a' OR 1=1 --", "localhost"},
		{"$(id)", "localhost"},
		{"app`touch /tmp/x`", "localhost"},
		{"app; DROP USER root", "localhost"},
		{"app_user", "localhost; id"},
		{"app_user", "$(id)"},
		{"app_user", "host'"},
	}
	for _, tc := range createUserCases {
		if _, err := srv.CreateUser(ctx, CreateUserRequest{
			ServerID: "srv-1",
			Engine:   EngineMySQL,
			Username: tc.username,
			Password: "SecretPass123!",
			Host:     tc.host,
		}); err == nil {
			t.Errorf("CreateUser accepted username=%q host=%q", tc.username, tc.host)
		}
	}

	for _, engine := range []EngineType{EngineMySQL, EnginePostgreSQL} {
		for _, username := range []string{"a' OR 1=1 --", "app'; DROP USER root;--", "$(id)", "app`x`"} {
			if err := srv.DeleteUser(ctx, "srv-1", engine, username, "%"); err == nil {
				t.Errorf("DeleteUser accepted %s username=%q", engine, username)
			}
		}
		if err := srv.DeleteUser(ctx, "srv-1", engine, "app_user", "localhost; id"); err == nil {
			t.Errorf("DeleteUser accepted %s with a hostile host", engine)
		}
	}

	if err := srv.DeleteUser(ctx, "srv-1", EngineRedis, "app_user", "%"); err == nil {
		t.Error("DeleteUser accepted an unsupported engine instead of failing")
	}

	if _, err := srv.CreateDatabase(ctx, CreateDatabaseRequest{
		ServerID: "srv-1",
		Engine:   EngineMySQL,
		Name:     "app'; DROP DATABASE mysql;--",
	}); err == nil {
		t.Error("CreateDatabase accepted a hostile database name")
	}

	if _, err := srv.CreateDatabase(ctx, CreateDatabaseRequest{
		ServerID: "srv-1",
		Engine:   EngineMySQL,
		Name:     "app_production",
		Charset:  "utf8mb4; DROP USER root",
	}); err == nil {
		t.Error("CreateDatabase accepted a hostile charset")
	}

	if _, err := srv.DeleteDatabase(ctx, "srv-1", EngineMySQL, "app`id`"); err == nil {
		t.Error("DeleteDatabase accepted a hostile database name")
	}

	if _, err := srv.BackupDatabase(ctx, BackupDatabaseRequest{
		ServerID:     "srv-1",
		Engine:       EngineMySQL,
		DatabaseName: "app_production",
		TargetPath:   "/var",
	}); err == nil {
		t.Error("BackupDatabase accepted a path that is too broad")
	}

	if _, err := srv.RestoreDatabase(ctx, RestoreDatabaseRequest{
		ServerID:     "srv-1",
		Engine:       EngineMySQL,
		DatabaseName: "app_production",
		DumpFilePath: "/var/backups/../../root/.ssh/id_rsa",
	}); err == nil {
		t.Error("RestoreDatabase accepted a traversing dump path")
	}

	if _, err := srv.ServiceControl(ctx, "srv-1", EngineMySQL, "restart; id"); err == nil {
		t.Error("ServiceControl accepted a hostile systemctl action")
	}
}
