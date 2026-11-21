package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/providers/httpx"
	"tamestack/internal/ai/providers/stream"
)

// Client implements domain.Provider for local Ollama instances.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Config holds options for the Ollama client.
type Config struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new Ollama provider client.
func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = httpx.NewClient()
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// Type returns domain.ProviderOllama.
func (c *Client) Type() domain.ProviderType {
	return domain.ProviderOllama
}

// Ping checks if Ollama is running and responsive.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/version", nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama is not reachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama ping returned status %d", resp.StatusCode)
	}
	return nil
}

type ollamaTagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		Model      string `json:"model"`
		ModifiedAt string `json:"modified_at"`
		Size       int64  `json:"size"`
		Details    struct {
			Family            string `json:"family"`
			ParameterSize     string `json:"parameter_size"`
			QuantizationLevel string `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

// ListModels retrieves all locally installed models in Ollama.
func (c *Client) ListModels(ctx context.Context) ([]domain.ModelSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list models request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ollama models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, "ollama tags")
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama tags: %w", err)
	}

	specs := make([]domain.ModelSpec, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		// Detect typical capabilities.
		supportsTools := strings.Contains(strings.ToLower(m.Name), "qwen2.5") ||
			strings.Contains(strings.ToLower(m.Name), "llama3") ||
			strings.Contains(strings.ToLower(m.Name), "mistral")

		specs = append(specs, domain.ModelSpec{
			ID:                m.Name,
			Name:              fmt.Sprintf("%s (%s)", m.Name, m.Details.ParameterSize),
			Provider:          domain.ProviderOllama,
			ContextWindow:     8192,
			SupportsTools:     supportsTools,
			SupportsStreaming: true,
			IsLocal:           true,
		})
	}
	return specs, nil
}

type ollamaChatMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options,omitempty"`
	Tools    []any               `json:"tools,omitempty"`
}

type ollamaChatChunk struct {
	Model           string            `json:"model"`
	CreatedAt       string            `json:"created_at"`
	Message         ollamaChatMessage `json:"message"`
	Done            bool              `json:"done"`
	TotalDuration   int64             `json:"total_duration"`
	PromptEvalCount int               `json:"prompt_eval_count"`
	EvalCount       int               `json:"eval_count"`
}

func convertToOllamaMessages(msgs []domain.Message) []ollamaChatMessage {
	out := make([]ollamaChatMessage, 0, len(msgs))
	for _, m := range msgs {
		role := string(m.Role)
		if m.Role == domain.RoleTool {
			role = "tool"
		}
		item := ollamaChatMessage{
			Role:    role,
			Content: m.Content,
		}
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				if args == nil {
					args = make(map[string]any)
				}
				item.ToolCalls = append(item.ToolCalls, ollamaToolCall{
					Function: struct {
						Name      string         `json:"name"`
						Arguments map[string]any `json:"arguments"`
					}{
						Name:      tc.Function.Name,
						Arguments: args,
					},
				})
			}
		}
		out = append(out, item)
	}
	return out
}

func convertToolsToOllama(tools []domain.ToolDefinition) []any {
	if len(tools) == 0 {
		return nil
	}
	out := make([]any, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}
	return out
}

// Chat executes a synchronous non-streaming chat request.
func (c *Client) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	options := map[string]any{}
	if req.Temperature > 0 {
		options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		options["num_predict"] = req.MaxTokens
	}

	payload := ollamaChatRequest{
		Model:    req.Model,
		Messages: convertToOllamaMessages(req.Messages),
		Stream:   false,
		Options:  options,
		Tools:    convertToolsToOllama(req.Tools),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, "ollama chat")
	}

	var chatChunk ollamaChatChunk
	if err := json.NewDecoder(resp.Body).Decode(&chatChunk); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	toolCalls := make([]domain.ToolCall, 0, len(chatChunk.Message.ToolCalls))
	for i, tc := range chatChunk.Message.ToolCalls {
		argsJSON, _ := json.Marshal(tc.Function.Arguments)
		toolCalls = append(toolCalls, domain.ToolCall{
			ID:   fmt.Sprintf("call_ollama_%d", i),
			Type: "function",
			Function: domain.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: string(argsJSON),
			},
		})
	}

	return &domain.ChatResponse{
		ID:    fmt.Sprintf("ollama_%d", time.Now().UnixNano()),
		Model: chatChunk.Model,
		Message: domain.Message{
			Role:      domain.RoleAssistant,
			Content:   chatChunk.Message.Content,
			ToolCalls: toolCalls,
		},
		FinishReason: "stop",
		Usage: domain.TokenUsage{
			PromptTokens:     chatChunk.PromptEvalCount,
			CompletionTokens: chatChunk.EvalCount,
			TotalTokens:      chatChunk.PromptEvalCount + chatChunk.EvalCount,
		},
	}, nil
}

// StreamChat executes an asynchronous streaming chat request.
func (c *Client) StreamChat(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamChunk, error) {
	options := map[string]any{}
	if req.Temperature > 0 {
		options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		options["num_predict"] = req.MaxTokens
	}

	payload := ollamaChatRequest{
		Model:    req.Model,
		Messages: convertToOllamaMessages(req.Messages),
		Stream:   true,
		Options:  options,
		Tools:    convertToolsToOllama(req.Tools),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute ollama stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		err := httpx.StatusError(resp, "ollama stream")
		resp.Body.Close()
		return nil, err
	}

	outChan := make(chan domain.StreamChunk, 20)

	go func() {
		defer resp.Body.Close()
		defer close(outChan)
		defer stream.Recover(outChan, "ollama")

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if err := ctx.Err(); err != nil {
				stream.TrySend(outChan, domain.StreamChunk{Error: err})
				return
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunk ollamaChatChunk
			if err := json.Unmarshal(line, &chunk); err != nil {
				stream.TrySend(outChan, domain.StreamChunk{Error: fmt.Errorf("failed to unmarshal chunk: %w", err)})
				return
			}

			var toolCall *domain.ToolCall
			if len(chunk.Message.ToolCalls) > 0 {
				tc := chunk.Message.ToolCalls[0]
				argsJSON, _ := json.Marshal(tc.Function.Arguments)
				toolCall = &domain.ToolCall{
					ID:   fmt.Sprintf("call_ollama_%d", time.Now().UnixNano()),
					Type: "function",
					Function: domain.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: string(argsJSON),
					},
				}
			}

			sc := domain.StreamChunk{
				ContentDelta:  chunk.Message.Content,
				ToolCallDelta: toolCall,
			}

			if chunk.Done {
				sc.FinishReason = "stop"
				sc.Usage = &domain.TokenUsage{
					PromptTokens:     chunk.PromptEvalCount,
					CompletionTokens: chunk.EvalCount,
					TotalTokens:      chunk.PromptEvalCount + chunk.EvalCount,
				}
			}

			if !stream.Send(ctx, outChan, sc) {
				return
			}

			if chunk.Done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			stream.TrySend(outChan, domain.StreamChunk{Error: fmt.Errorf("scanner error: %w", err)})
		}
	}()

	return outChan, nil
}
