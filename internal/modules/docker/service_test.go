package docker

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

func TestDockerServiceTaskEnqueue(t *testing.T) {
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
		ID:              "srv-dock-1",
		Name:            "Docker Node 1",
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

	// 1. Enqueue Container Action (Restart).
	taskRestart, err := srv.ContainerAction(ctx, ContainerActionRequest{
		ServerID:    "srv-dock-1",
		ContainerID: "c123abc456",
		Action:      "restart",
	})
	if err != nil {
		t.Fatalf("failed to enqueue container restart: %v", err)
	}
	if taskRestart.OperationType != "docker_container_action" {
		t.Errorf("expected docker_container_action, got %s", taskRestart.OperationType)
	}

	// 2. Enqueue Pull Image.
	taskPull, err := srv.PullImage(ctx, PullImageRequest{
		ServerID: "srv-dock-1",
		Image:    "redis:7-alpine",
	})
	if err != nil {
		t.Fatalf("failed to enqueue image pull: %v", err)
	}
	if taskPull.OperationType != "docker_image_pull" {
		t.Errorf("expected docker_image_pull, got %s", taskPull.OperationType)
	}

	// 3. Enqueue Compose Action.
	taskCompose, err := srv.ComposeAction(ctx, ComposeActionRequest{
		ServerID:   "srv-dock-1",
		WorkingDir: "/opt/stacks/web",
		Action:     "up",
	})
	if err != nil {
		t.Fatalf("failed to enqueue compose up: %v", err)
	}
	if taskCompose.OperationType != "docker_compose_action" {
		t.Errorf("expected docker_compose_action, got %s", taskCompose.OperationType)
	}
}
