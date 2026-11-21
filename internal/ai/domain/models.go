package domain

import (
	"context"
	"time"
)

// ProviderType identifies the AI backend service.
type ProviderType string

const (
	ProviderOllama           ProviderType = "ollama"
	ProviderLlamaCpp         ProviderType = "llamacpp"
	ProviderOpenAI           ProviderType = "openai"
	ProviderGemini           ProviderType = "gemini"
	ProviderAnthropic        ProviderType = "anthropic"
	ProviderOpenAICompatible ProviderType = "openai_compatible"
)

// Role defines the message author in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// RiskLevel defines the security classification of an operation.
type RiskLevel string

const (
	RiskSafe     RiskLevel = "safe"     // Read-only inspection, no server modifications
	RiskModerate RiskLevel = "moderate" // Safe restarts, non-destructive queries
	RiskHigh     RiskLevel = "high"     // Configuration edits, service reloads
	RiskCritical RiskLevel = "critical" // Deletions, service stops, migrations
)

// FunctionCall represents an invocation of a named tool with arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments
}

// ToolCall represents a tool call requested by the assistant.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // Defaults to "function"
	Function FunctionCall `json:"function"`
}

// Message is a single turn in a chat dialog.
type Message struct {
	ID         string     `json:"id,omitempty"`
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at,omitempty"`
}

// ToolDefinition describes a function callable by the model.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
	RiskLevel   RiskLevel      `json:"risk_level"`
	IsMutating  bool           `json:"is_mutating"`
	// ApprovalViaTask marks a proposal tool whose only side effect is creating a
	// requires-approval task in the Tasks engine; the mutation itself stays gated
	// behind the user's task approval, so the tool may auto-run in the chat loop.
	ApprovalViaTask bool `json:"approval_via_task,omitempty"`
}

// ModelSpec describes the capabilities and constraints of a model.
type ModelSpec struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Provider          ProviderType `json:"provider"`
	ContextWindow     int          `json:"context_window"`
	SupportsTools     bool         `json:"supports_tools"`
	SupportsStreaming bool         `json:"supports_streaming"`
	IsLocal           bool         `json:"is_local"`
	InputCostPerM     float64      `json:"input_cost_per_m,omitempty"`  // USD per million tokens
	OutputCostPerM    float64      `json:"output_cost_per_m,omitempty"` // USD per million tokens
}

// TokenUsage holds token consumption details.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatRequest encapsulates all parameters needed to query a provider.
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

// ChatResponse encapsulates the complete response from a provider.
type ChatResponse struct {
	ID           string     `json:"id"`
	Model        string     `json:"model"`
	Message      Message    `json:"message"`
	FinishReason string     `json:"finish_reason"`
	Usage        TokenUsage `json:"usage"`
}

// StreamChunk is emitted over a channel during streaming responses.
type StreamChunk struct {
	ContentDelta  string    `json:"content_delta,omitempty"`
	ToolCallDelta *ToolCall `json:"tool_call_delta,omitempty"`
	FinishReason  string    `json:"finish_reason,omitempty"`
	Usage         *TokenUsage
	Error         error
}

// Provider represents the universal contract for an LLM provider.
type Provider interface {
	// Type returns the provider identifier.
	Type() ProviderType

	// Ping checks connectivity and health of the provider.
	Ping(ctx context.Context) error

	// ListModels returns all models currently available through this provider.
	ListModels(ctx context.Context) ([]ModelSpec, error)

	// Chat performs a blocking synchronous chat request.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChat starts a streaming chat request, returning chunks over a channel.
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)
}
