package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	aiContext "tamestack/internal/ai/context"
	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/policies"
	"tamestack/internal/db"
)

// ToolHandler is a function that implements an AI tool's business logic.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// RegisteredTool holds metadata and execution handler for an AI tool.
type RegisteredTool struct {
	Definition domain.ToolDefinition
	Handler    ToolHandler
}

// Registry manages tool registration, execution, and audit logging.
type Registry struct {
	database  *db.DB
	sanitizer *aiContext.Sanitizer
	tools     map[string]RegisteredTool
	disabled  map[string]struct{}
	mu        sync.RWMutex
}

// NewRegistry initializes an AI Tool Registry.
func NewRegistry(database *db.DB, sanitizer *aiContext.Sanitizer) *Registry {
	if sanitizer == nil {
		sanitizer = aiContext.NewSanitizer()
	}
	return &Registry{
		database:  database,
		sanitizer: sanitizer,
		tools:     make(map[string]RegisteredTool),
		disabled:  make(map[string]struct{}),
	}
}

// Register adds a tool definition and handler to the registry.
func (r *Registry) Register(def domain.ToolDefinition, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[def.Name] = RegisteredTool{
		Definition: def,
		Handler:    handler,
	}
}

// GetTool retrieves a registered tool by its name.
func (r *Registry) GetTool(name string) (RegisteredTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns list of tool specifications formatted for LLM function calling (defaults to Russian).
func (r *Registry) Definitions() []domain.ToolDefinition {
	return r.DefinitionsLang("ru")
}

// DefinitionsLang returns list of tool specifications with descriptions localized for lang ("ru" or "en").
func (r *Registry) DefinitionsLang(lang string) []domain.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	isEN := strings.ToLower(strings.TrimSpace(lang)) == "en"
	defs := make([]domain.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		def := t.Definition
		if isEN {
			if enDesc, ok := toolDescriptionsEN[def.Name]; ok {
				def.Description = enDesc
			}
		}
		defs = append(defs, def)
	}
	return defs
}

// SetDisabledTools updates the list of disabled tools.
func (r *Registry) SetDisabledTools(disabled []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disabled = make(map[string]struct{}, len(disabled))
	for _, d := range disabled {
		name := strings.TrimSpace(d)
		if name != "" {
			r.disabled[name] = struct{}{}
		}
	}
}

// GetDisabledTools returns a copy of disabled tool names.
func (r *Registry) GetDisabledTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]string, 0, len(r.disabled))
	for name := range r.disabled {
		list = append(list, name)
	}
	return list
}

// IsDisabled checks if toolName is currently disabled.
func (r *Registry) IsDisabled(toolName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.disabled) == 0 {
		return false
	}
	_, ok := r.disabled[strings.TrimSpace(toolName)]
	return ok
}

// ActiveDefinitions returns list of tool specifications excluding any disabled tools (defaults to Russian).
func (r *Registry) ActiveDefinitions() []domain.ToolDefinition {
	return r.ActiveDefinitionsLang("ru")
}

// ActiveDefinitionsLang returns enabled tool specifications localized for lang ("ru" or "en").
func (r *Registry) ActiveDefinitionsLang(lang string) []domain.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	isEN := strings.ToLower(strings.TrimSpace(lang)) == "en"
	defs := make([]domain.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		if _, ok := r.disabled[t.Definition.Name]; !ok {
			def := t.Definition
			if isEN {
				if enDesc, ok := toolDescriptionsEN[def.Name]; ok {
					def.Description = enDesc
				}
			}
			defs = append(defs, def)
		}
	}
	return defs
}

// ToolExecutionResult represents the outcome of calling a tool.
type ToolExecutionResult struct {
	ToolName   string `json:"tool_name"`
	Success    bool   `json:"success"`
	Data       any    `json:"data,omitempty"`
	RawOutput  string `json:"raw_output,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// Execute runs a tool that the security gate considers safe to auto-run, and records the audit entry.
// Tools requiring human confirmation are refused; use ExecuteApproved for those.
func (r *Registry) Execute(ctx context.Context, convID string, messageID string, toolName string, argumentsJSON string) (*ToolExecutionResult, error) {
	return r.execute(ctx, convID, messageID, toolName, argumentsJSON, false)
}

// ExecuteApproved runs a tool after the user explicitly confirmed it through the approval flow.
func (r *Registry) ExecuteApproved(ctx context.Context, convID string, messageID string, toolName string, argumentsJSON string) (*ToolExecutionResult, error) {
	return r.execute(ctx, convID, messageID, toolName, argumentsJSON, true)
}

func (r *Registry) execute(ctx context.Context, convID string, messageID string, toolName string, argumentsJSON string, humanApproved bool) (*ToolExecutionResult, error) {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	tool, exists := r.GetTool(toolName)
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found in registry", toolName)
	}

	if r.IsDisabled(toolName) {
		return nil, fmt.Errorf("tool '%s' is disabled by user settings", toolName)
	}

	if !humanApproved && policies.NewSecurityGate().RequiresHumanApproval(tool.Definition) {
		return nil, fmt.Errorf("tool '%s' requires human approval before it can run", toolName)
	}

	var args map[string]any
	trimmed := strings.TrimSpace(argumentsJSON)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	if trimmed != "" && trimmed != "{}" {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			// Attempt to repair truncated or incomplete JSON from streamed chunks.
			repaired := trimmed
			if !strings.HasSuffix(repaired, "}") {
				if strings.Count(repaired, "\"")%2 != 0 {
					repaired += "\""
				}
				repaired += "}"
			}
			if err2 := json.Unmarshal([]byte(repaired), &args); err2 != nil {
				return nil, fmt.Errorf("invalid json arguments for tool '%s': %w", toolName, err)
			}
		}
	} else {
		args = make(map[string]any)
	}
	if args == nil {
		args = make(map[string]any)
	}

	start := time.Now()
	data, err := tool.Handler(ctx, args)
	durationMs := time.Since(start).Milliseconds()

	success := err == nil
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	rawOutput := ""
	if data != nil {
		dataBytes, _ := json.Marshal(data)
		rawOutput = r.sanitizer.Sanitize(string(dataBytes))
	}

	// Audit Logging
	if r.database != nil && convID != "" {
		audit := &db.AIToolAuditRecord{
			ID:                  "audit_" + uuid.New().String(),
			ConversationID:      convID,
			MessageID:           messageID,
			ToolName:            toolName,
			ArgumentsJSON:       r.sanitizer.Sanitize(argumentsJSON),
			RiskLevel:           string(tool.Definition.RiskLevel),
			Approved:            humanApproved,
			ExecutedAt:          start,
			ExecutionDurationMs: durationMs,
			Success:             success,
			ErrorMessage:        errMsg,
		}
		_ = r.database.SaveAIToolAudit(ctx, audit)
	}

	res := &ToolExecutionResult{
		ToolName:   toolName,
		Success:    success,
		Data:       data,
		RawOutput:  rawOutput,
		Error:      errMsg,
		DurationMs: durationMs,
	}

	return res, nil
}
