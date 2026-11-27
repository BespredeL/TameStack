package chat

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
)

type mockVault struct{}

func (m *mockVault) Encrypt(plainText string) (string, error) {
	return "enc_" + base64.StdEncoding.EncodeToString([]byte(plainText)), nil
}

func (m *mockVault) Decrypt(cipherTextB64 string) (string, error) {
	if len(cipherTextB64) > 4 && cipherTextB64[:4] == "enc_" {
		data, err := base64.StdEncoding.DecodeString(cipherTextB64[4:])
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return cipherTextB64, nil
}

func TestChatService(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_chat.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	vault := &mockVault{}
	svc := NewService(database, vault)
	ctx := context.Background()

	// 1. Save and List Provider Configs.
	err = svc.SaveProviderConfig(ctx, ProviderConfigInput{
		ID:           "openai-test",
		ProviderType: "openai",
		Name:         "OpenAI Production",
		BaseURL:      "https://api.openai.com/v1",
		APIKey:       "sk-secret-test-key",
		IsEnabled:    true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatalf("failed to save provider config: %v", err)
	}

	configs, err := svc.ListProviderConfigs(ctx)
	if err != nil || len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d (err: %v)", len(configs), err)
	}
	if !configs[0].HasAPIKey || configs[0].Name != "OpenAI Production" {
		t.Fatalf("mismatched safe config: %+v", configs[0])
	}

	// Verify encryption in DB.
	rawConfig, err := database.GetAIProviderConfig(ctx, "openai-test")
	if err != nil || rawConfig == nil {
		t.Fatalf("failed to get raw config from db: %v", err)
	}
	if rawConfig.APIKeyEncrypted == "sk-secret-test-key" {
		t.Fatalf("raw API key was not encrypted in db!")
	}

	// 2. Conversation Management.
	conv, err := svc.CreateConversation(ctx, "Test Diagnostics", nil, "You are a test assistant.", "ollama", "qwen2.5-coder:7b")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	fetchedConv, err := svc.GetConversation(ctx, conv.ID)
	if err != nil || fetchedConv == nil {
		t.Fatalf("failed to get conversation: %v", err)
	}
	if fetchedConv.Title != "Test Diagnostics" {
		t.Fatalf("unexpected conversation title: %s", fetchedConv.Title)
	}

	listConvs, err := svc.ListConversations(ctx)
	if err != nil || len(listConvs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(listConvs))
	}

	// 3. Messages
	userMsg := &db.AIMessageRecord{
		ID:             "msg-1",
		ConversationID: conv.ID,
		Role:           string(domain.RoleUser),
		Content:        "Hello TameStack AI",
	}
	if err := database.SaveAIMessage(ctx, userMsg); err != nil {
		t.Fatalf("failed to save user message: %v", err)
	}

	msgs, err := svc.ListMessages(ctx, conv.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	// 4. Delete Conversation.
	if err := svc.DeleteConversation(ctx, conv.ID); err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	afterDelete, err := svc.GetConversation(ctx, conv.ID)
	if err != nil || afterDelete != nil {
		t.Fatalf("conversation should be deleted, got: %+v (err: %v)", afterDelete, err)
	}

	_ = database.Close()
}

func TestNormalizeToolCallsAssignsFallbackID(t *testing.T) {
	calls := normalizeToolCalls([]domain.ToolCall{
		{ID: "", Function: domain.FunctionCall{Name: "analyze_disk_usage", Arguments: ""}},
		{ID: "call_custom", Function: domain.FunctionCall{Name: "get_system_metrics", Arguments: `{"server_id":"1"}`}},
	})
	if len(calls) != 2 {
		t.Fatalf("expected 2 normalized calls, got %d", len(calls))
	}
	if calls[0].ID != "call_1" || calls[0].Function.Arguments != "{}" {
		t.Fatalf("unexpected first call: %+v", calls[0])
	}
	if calls[1].ID != "call_custom" {
		t.Fatalf("unexpected second call ID: %q", calls[1].ID)
	}
}
