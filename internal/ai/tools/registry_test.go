package tools

import (
	"context"
	"path/filepath"
	"strings"
	"tamestack/internal/testutil"
	"testing"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
)

func TestToolRegistry(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_tools.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	registry := NewRegistry(database, nil)

	// Register dummy tool.
	registry.Register(domain.ToolDefinition{
		Name:        "calculate_sum",
		Description: "Adds two numbers",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		a, _ := args["a"].(float64)
		b, _ := args["b"].(float64)
		return map[string]any{"sum": a + b}, nil
	})

	// 1. Check definitions.
	defs := registry.Definitions()
	if len(defs) != 1 || defs[0].Name != "calculate_sum" {
		t.Fatalf("unexpected definitions: %+v", defs)
	}

	// 2. Execute successfully.
	convID := "conv-test-123"
	res, err := registry.Execute(ctx, convID, "msg-1", "calculate_sum", `{"a": 10, "b": 32}`)
	if err != nil {
		t.Fatalf("tool execution failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected tool success, got failure: %s", res.Error)
	}

	// 3. Verify audit record in SQLite.
	audits, err := database.ListAIToolAudits(ctx, convID)
	if err != nil || len(audits) != 1 {
		t.Fatalf("expected 1 audit record, got %d (err: %v)", len(audits), err)
	}
	if audits[0].ToolName != "calculate_sum" {
		t.Fatalf("unexpected audit entry: %+v", audits[0])
	}
	if audits[0].Approved {
		t.Fatalf("auto-run read-only tool must not be recorded as human-approved: %+v", audits[0])
	}

	// 4. Execute nonexistent tool.
	_, err = registry.Execute(ctx, convID, "msg-2", "non_existent_tool", `{}`)
	if err == nil {
		t.Fatalf("expected error executing nonexistent tool")
	}

	// 5. Malformed JSON.
	_, err = registry.Execute(ctx, convID, "msg-3", "calculate_sum", `not-a-json`)
	if err == nil {
		t.Fatalf("expected error on malformed json args")
	}
}

func TestApprovalGateIsEnforcedInExecute(t *testing.T) {
	database, err := db.Open(filepath.Join(testutil.Workspace(t), "approval_gate.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	registry := NewRegistry(database, nil)

	ran := false
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		ran = true
		return map[string]any{"ok": true}, nil
	}
	registry.Register(domain.ToolDefinition{
		Name:        "drop_database",
		Description: "mutating tool",
		RiskLevel:   domain.RiskHigh,
		IsMutating:  true,
	}, handler)

	convID := "conv-gate"
	if _, err := registry.Execute(ctx, convID, "msg-1", "drop_database", `{}`); err == nil {
		t.Fatal("expected unapproved mutating tool to be refused")
	}
	if ran {
		t.Fatal("refused tool still ran its handler")
	}

	res, err := registry.ExecuteApproved(ctx, convID, "msg-2", "drop_database", `{}`)
	if err != nil || !res.Success {
		t.Fatalf("approved execution failed: res=%+v err=%v", res, err)
	}

	audits, err := database.ListAIToolAudits(ctx, convID)
	if err != nil || len(audits) != 1 {
		t.Fatalf("expected only the approved run to be audited, got %d (err: %v)", len(audits), err)
	}
	if !audits[0].Approved {
		t.Fatalf("approved run must be recorded as approved: %+v", audits[0])
	}
}

func TestRegistry_DisabledTools(t *testing.T) {
	registry := NewRegistry(nil, nil)

	registry.Register(domain.ToolDefinition{
		Name:        "tool_a",
		Description: "A",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
	}, func(ctx context.Context, args map[string]any) (any, error) {
		return "ok_a", nil
	})
	registry.Register(domain.ToolDefinition{
		Name:        "tool_b",
		Description: "B",
		RiskLevel:   domain.RiskSafe,
		IsMutating:  false,
	}, func(ctx context.Context, args map[string]any) (any, error) {
		return "ok_b", nil
	})

	if len(registry.ActiveDefinitions()) != 2 {
		t.Fatalf("expected 2 active definitions, got %d", len(registry.ActiveDefinitions()))
	}

	registry.SetDisabledTools([]string{"tool_b"})
	if !registry.IsDisabled("tool_b") {
		t.Fatalf("expected tool_b to be disabled")
	}
	if registry.IsDisabled("tool_a") {
		t.Fatalf("expected tool_a to be enabled")
	}

	active := registry.ActiveDefinitions()
	if len(active) != 1 || active[0].Name != "tool_a" {
		t.Fatalf("expected only tool_a in active definitions, got %+v", active)
	}

	// Execution of disabled tool must fail.
	_, err := registry.Execute(context.Background(), "c1", "m1", "tool_b", `{}`)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled error for tool_b, got %v", err)
	}

	// Execution of enabled tool must succeed.
	res, err := registry.Execute(context.Background(), "c1", "m1", "tool_a", `{}`)
	if err != nil || !res.Success {
		t.Fatalf("expected tool_a execution success, got res=%v, err=%v", res, err)
	}
}

func TestRegistry_LocalizedToolDefinitions(t *testing.T) {
	database, _ := db.Open(filepath.Join(testutil.Workspace(t), "local_tools.db"))
	defer database.Close()

	registry := NewRegistry(database, nil)
	registry.Register(domain.ToolDefinition{
		Name:        "list_databases",
		Description: "Возвращает список баз данных",
	}, func(ctx context.Context, args map[string]any) (any, error) {
		return "ok", nil
	})

	ruDefs := registry.DefinitionsLang("ru")
	if len(ruDefs) != 1 || ruDefs[0].Description != "Возвращает список баз данных" {
		t.Fatalf("unexpected RU description: %+v", ruDefs)
	}

	enDefs := registry.DefinitionsLang("en")
	if len(enDefs) != 1 || !strings.Contains(enDefs[0].Description, "Returns the list of databases") {
		t.Fatalf("unexpected EN description: %+v", enDefs)
	}
}
