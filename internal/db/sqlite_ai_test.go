package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteAICRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ai.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// 1. Provider Configs.
	pConfig := &AIProviderConfigRecord{
		ID:              "prov-1",
		ProviderType:    "ollama",
		Name:            "Local Ollama",
		BaseURL:         "http://127.0.0.1:11434",
		APIKeyEncrypted: "",
		IsEnabled:       true,
		IsDefault:       true,
	}
	if err := database.SaveAIProviderConfig(ctx, pConfig); err != nil {
		t.Fatalf("failed to save ai provider config: %v", err)
	}

	fetchedProv, err := database.GetAIProviderConfig(ctx, "prov-1")
	if err != nil || fetchedProv == nil {
		t.Fatalf("failed to get ai provider config: %v", err)
	}
	if fetchedProv.Name != "Local Ollama" || !fetchedProv.IsDefault {
		t.Fatalf("mismatched provider config: %+v", fetchedProv)
	}

	byType, err := database.GetAIProviderConfigByType(ctx, "ollama")
	if err != nil || byType == nil || byType.ID != "prov-1" {
		t.Fatalf("failed to get ai provider config by type: %v", err)
	}

	// 2. Conversation
	conv := &AIConversationRecord{
		ID:               "conv-1",
		Title:            "Apache & Nginx diagnosis",
		SystemPrompt:     "You are a DevOps assistant.",
		SelectedProvider: "ollama",
		SelectedModel:    "qwen2.5-coder:7b",
	}
	if err := database.SaveAIConversation(ctx, conv); err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	fetchedConv, err := database.GetAIConversation(ctx, "conv-1")
	if err != nil || fetchedConv == nil {
		t.Fatalf("failed to get conversation: %v", err)
	}
	if fetchedConv.Title != conv.Title {
		t.Fatalf("mismatched conversation: %+v", fetchedConv)
	}

	// 3. Messages
	msg1 := &AIMessageRecord{
		ID:             "msg-1",
		ConversationID: "conv-1",
		Role:           "user",
		Content:        "Why is Apache failing to start on port 80?",
		CreatedAt:      time.Now(),
	}
	if err := database.SaveAIMessage(ctx, msg1); err != nil {
		t.Fatalf("failed to save message: %v", err)
	}

	msg2 := &AIMessageRecord{
		ID:               "msg-2",
		ConversationID:   "conv-1",
		Role:             "assistant",
		Content:          "Nginx is already bound to port 80.",
		PromptTokens:     25,
		CompletionTokens: 12,
		CreatedAt:        time.Now().Add(time.Second),
	}
	if err := database.SaveAIMessage(ctx, msg2); err != nil {
		t.Fatalf("failed to save message 2: %v", err)
	}

	msgs, err := database.ListAIMessages(ctx, "conv-1")
	if err != nil || len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d (err: %v)", len(msgs), err)
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected message roles: %+v", msgs)
	}

	// 4. Memory Facts & FTS.
	fact := &AIMemoryFactRecord{
		ID:        "fact-1",
		Category:  "web",
		FactKey:   "apache_port",
		FactValue: "Apache configured on port 8080 due to Nginx reverse proxy",
		Source:    "manual",
	}
	if err := database.SaveAIMemoryFact(ctx, fact); err != nil {
		t.Fatalf("failed to save memory fact: %v", err)
	}

	searchRes, err := database.SearchAIMemory(ctx, "proxy", nil, 5)
	if err != nil {
		t.Fatalf("failed to search fts memory: %v", err)
	}
	if len(searchRes) == 0 {
		t.Fatalf("expected FTS match for 'proxy', got 0 results")
	}
	if searchRes[0].FactKey != "apache_port" {
		t.Fatalf("unexpected search result: %+v", searchRes[0])
	}

	// 5. Tool Audit.
	audit := &AIToolAuditRecord{
		ID:                  "audit-1",
		ConversationID:      "conv-1",
		MessageID:           "msg-2",
		ToolName:            "list_services",
		ArgumentsJSON:       `{"server_id":"srv-1"}`,
		RiskLevel:           "safe",
		Approved:            true,
		ExecutedAt:          time.Now(),
		ExecutionDurationMs: 42,
		Success:             true,
	}
	if err := database.SaveAIToolAudit(ctx, audit); err != nil {
		t.Fatalf("failed to save tool audit: %v", err)
	}

	audits, err := database.ListAIToolAudits(ctx, "conv-1")
	if err != nil || len(audits) != 1 {
		t.Fatalf("expected 1 audit entry, got %d (err: %v)", len(audits), err)
	}
	if audits[0].ToolName != "list_services" || !audits[0].Success {
		t.Fatalf("unexpected audit entry: %+v", audits[0])
	}

	_ = database.Close()
}
