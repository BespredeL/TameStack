package chat

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"
	"time"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/tools"
	"tamestack/internal/db"
)

func TestDebugStreamServer(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8888/v1/models")
	if err != nil || resp.StatusCode != 200 {
		t.Skip("Local server at 8888 not reachable")
	}
	resp.Body.Close()

	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_ai.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db error: %v", err)
	}
	defer database.Close()

	vault := &mockVault{}
	reg := tools.NewRegistry(database, nil)

	// Register sample tool.
	reg.Register(domain.ToolDefinition{
		Name:        "get_server_status",
		Description: "Check current CPU, RAM, Disk metrics and uptime of a server.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server_id": map[string]any{"type": "string", "description": "Target server ID"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		fmt.Printf("\n>>> TOOL get_server_status EXECUTED with args: %+v <<<\n", args)
		return map[string]any{"status": "online", "cpu": 15.2, "ram_used_mb": 2048}, nil
	})

	chatSvc := NewService(database, vault, reg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = chatSvc.SaveProviderConfig(ctx, ProviderConfigInput{
		ID:           "test-local",
		ProviderType: "openai_compatible",
		Name:         "Local Test",
		BaseURL:      "http://127.0.0.1:8888/v1",
		APIKey:       "",
		DefaultModel: "unsloth/NVIDIA-Nemotron-3.5-Bolt-E4B-GGUF",
		IsEnabled:    true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatalf("save provider error: %v", err)
	}

	conv, err := chatSvc.CreateConversation(ctx, "Test Server Check", nil, "", "openai_compatible", "unsloth/NVIDIA-Nemotron-3.5-Bolt-E4B-GGUF")
	if err != nil {
		t.Fatalf("create conv error: %v", err)
	}

	fmt.Printf("\n=== RUNNING STREAM MESSAGE WITH SERVER QUESTION ===\n")
	var callCount int
	msg, err := chatSvc.StreamMessage(ctx, conv.ID, "Проверь статус сервера, пожалуйста.", func(chunk domain.StreamChunk) {
		callCount++
		if chunk.ContentDelta != "" {
			fmt.Print(chunk.ContentDelta)
		}
		if chunk.ToolCallDelta != nil {
			fmt.Printf("\n[TOOL CALL DELTA: %+v]\n", chunk.ToolCallDelta)
		}
	})

	fmt.Printf("\n=== FINISHED STREAM MESSAGE (chunks=%d) ===\n", callCount)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}
	fmt.Printf("Message ID: %s, Content: %s\n", msg.ID, msg.Content)

	dbMsgs, _ := chatSvc.ListMessages(ctx, conv.ID)
	fmt.Printf("Total DB messages: %d\n", len(dbMsgs))
	for _, m := range dbMsgs {
		fmt.Printf(" - [%s] %s: %s (tool_calls: %s)\n", m.ID, m.Role, m.Content, m.ToolCallsJSON)
	}
}
