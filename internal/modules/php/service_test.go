package php

import (
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

func TestPlanPHPInstall(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	saltPath := filepath.Join(tmpDir, "vault.salt")
	dbPath := filepath.Join(tmpDir, "test_php.db")

	cfg := &config.AppConfig{
		DataDir:        tmpDir,
		DatabasePath:   dbPath,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
		SaltPath:       saltPath,
	}

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	_ = os.WriteFile(saltPath, []byte("01234567890123456789012345678901"), 0600)
	vault, err := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	pool := ssh.NewPool()
	engine := tasks.NewEngine(database, 2)
	service := NewService(cfg, database, vault, pool, engine)

	ctx := context.Background()

	// 1. Insert a mock server and diagnostics.
	srv := &db.ServerRecord{
		ID:              "srv-php-test",
		Name:            "Ubuntu Web Server",
		Host:            "10.0.0.5",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		Mode:            "manage",
		EncryptedSecret: "fake_secret",
	}
	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	diag := &db.DiagnosticsRecord{
		ServerID: "srv-php-test",
		OSName:   "Ubuntu 24.04 LTS",
		OSFamily: "debian",
	}
	if err := database.SaveDiagnostics(ctx, diag); err != nil {
		t.Fatalf("failed to save diagnostics: %v", err)
	}

	// 2. Generate Plan.
	plan, err := service.PlanPHPInstall(ctx, "srv-php-test", "8.3", []string{"curl", "mbstring", "mysql", "redis"})
	if err != nil {
		t.Fatalf("failed to generate plan: %v", err)
	}

	if !plan.OSCompatible {
		t.Fatalf("expected OS compatible")
	}

	if len(plan.PackagesToInstall) < 6 {
		t.Fatalf("expected at least 6 packages to install, got %d: %v", len(plan.PackagesToInstall), plan.PackagesToInstall)
	}

	if len(plan.RepositoriesNeeded) == 0 {
		t.Fatalf("expected repository needed for multi-version PHP")
	}

	// 3. Test UpdatePHPVersion task submission.
	task, err := service.UpdatePHPVersion("srv-php-test", "8.3")
	if err != nil {
		t.Fatalf("failed to queue update task: %v", err)
	}
	if task == nil || task.OperationType != "php_update" {
		t.Fatalf("expected php_update task record, got: %+v", task)
	}
}

func TestCleanCandidateVersion(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"8.2.26-1+ubuntu22.04.1+deb.sury.org+1", "8.2.26"},
		{"8.3.14-1.el9.remi", "8.3.14"},
		{"8.1.2", "8.1.2"},
		{"", ""},
		{"none", "none"},
	}

	for _, c := range cases {
		got := cleanCandidateVersion(c.input)
		if got != c.expected {
			t.Errorf("cleanCandidateVersion(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestToggleFPMValidation(t *testing.T) {
	service := &Service{}
	ctx := context.Background()

	// Invalid versions must be rejected before any SSH or DB lookup
	invalidVersions := []string{"8.2; id", "8.2 && rm -rf", "8.2|cat", "../8.2", ""}
	for _, v := range invalidVersions {
		err := service.ToggleFPM(ctx, "srv-test", v, true)
		if err == nil {
			t.Fatalf("expected error for invalid version %q, got nil", v)
		}
	}

	// Test ToggleFPMPayload serialization/deserialization
	payloadJSON, err := tasks.MarshalPayload(ToggleFPMPayload{
		ServerID: "srv-test",
		Version:  "8.3",
		Enable:   true,
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	decoded, err := tasks.DecodePayload[ToggleFPMPayload](payloadJSON)
	if err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if decoded.Version != "8.3" || !decoded.Enable {
		t.Fatalf("unexpected decoded payload: %+v", decoded)
	}
}
