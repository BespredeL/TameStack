package chat

import (
	"context"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/tools"
	"tamestack/internal/db"
)

func TestChatServiceOfflineZeroProviders(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_offline.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	vault := &mockVault{}

	reg := tools.NewRegistry(database, nil)
	svc := NewService(database, vault, reg)

	ctx := context.Background()

	// 1. Create conversation with no configured providers.
	conv, err := svc.CreateConversation(ctx, "Test Offline Conv", nil, "ollama", "qwen2.5-coder:7b", "auto")
	if err != nil {
		t.Fatalf("failed to create conv: %v", err)
	}

	// 2. Stream message should gracefully return an error (no active provider), not crash.
	_, err = svc.StreamMessage(ctx, conv.ID, "Привет, сервер упал", func(chunk domain.StreamChunk) {})

	if err == nil {
		t.Fatalf("expected error when streaming with no providers configured, got nil")
	}

	// 3. Messages in conversation must remain intact.
	msgs, err := svc.ListMessages(ctx, conv.ID)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("expected at least the user message saved in DB, got 0")
	}
	if msgs[0].Role != "user" {
		t.Fatalf("expected user role, got %s", msgs[0].Role)
	}
}
