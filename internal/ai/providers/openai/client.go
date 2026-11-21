package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/providers/httpx"
	"tamestack/internal/ai/providers/stream"
)

// Client implements domain.Provider for OpenAI API and compatible endpoints.
// It also serves as the base client for other OpenAI-compatible providers
// (e.g. llama.cpp), which embed it and override Ping/ListModels as needed.
type Client struct {
	BaseURL      string
	HTTPClient   *http.Client
	apiKey       string
	providerType domain.ProviderType
	label        string
}

// Config holds options for the OpenAI client.
type Config struct {
	BaseURL      string
	APIKey       string
	ProviderType domain.ProviderType
	HTTPClient   *http.Client
	// Label names the provider in chat/stream error messages; defaults to "openai".
	Label string
}

// NewClient creates a new OpenAI provider client.
func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	pType := cfg.ProviderType
	if pType == "" {
		pType = domain.ProviderOpenAI
	}
	if baseURL == "" {
		if pType == domain.ProviderOpenAICompatible {
			baseURL = "http://127.0.0.1:8000/v1"
		} else {
			baseURL = "https://api.openai.com/v1"
		}
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = httpx.NewClient()
	}
	label := cfg.Label
	if label == "" {
		label = "openai"
	}
	return &Client{
		BaseURL:      baseURL,
		apiKey:       cfg.APIKey,
		providerType: pType,
		HTTPClient:   httpClient,
		label:        label,
	}
}

// Type returns the provider type (domain.ProviderOpenAI or domain.ProviderOpenAICompatible).
func (c *Client) Type() domain.ProviderType {
	if c.providerType != "" {
		return c.providerType
	}
	return domain.ProviderOpenAI
}

// Ping verifies API key validity and connection to the OpenAI/compatible endpoint.
func (c *Client) Ping(ctx context.Context) error {
	modelsURL := c.BaseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}
	c.SetHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("openai/compatible is not reachable at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback check on /health if /models is protected or not exposed by local engines (Unsloth, vLLM).
		if c.providerType == domain.ProviderOpenAICompatible || !strings.Contains(c.BaseURL, "api.openai.com") {
			healthURL := strings.TrimSuffix(c.BaseURL, "/v1") + "/health"
			if hReq, hErr := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil); hErr == nil {
				c.SetHeaders(hReq)
				if hResp, hErr2 := c.HTTPClient.Do(hReq); hErr2 == nil {
					defer hResp.Body.Close()
					if hResp.StatusCode == http.StatusOK {
						return nil
					}
				}
			}
		}
		return httpx.StatusError(resp, "ping")
	}
	return nil
}

type openAIModelListResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// ListModels retrieves models from OpenAI API, filtering for chat-capable models.
func (c *Client) ListModels(ctx context.Context) ([]domain.ModelSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list models request: %w", err)
	}
	c.SetHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch openai models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, "openai models")
	}

	var listResp openAIModelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode openai models: %w", err)
	}

	isCompatible := c.providerType == domain.ProviderOpenAICompatible

	specs := make([]domain.ModelSpec, 0)
	for _, m := range listResp.Data {
		isGPT := strings.HasPrefix(m.ID, "gpt-4") || strings.HasPrefix(m.ID, "gpt-3.5") || strings.HasPrefix(m.ID, "o1") || strings.HasPrefix(m.ID, "o3")
		if isCompatible || isGPT {
			contextWindow := 32768
			inputCost := 0.0
			outputCost := 0.0
			isLocal := isCompatible

			if isGPT {
				contextWindow = 128000
				inputCost = 2.50
				outputCost = 10.00
				isLocal = false
				if strings.Contains(m.ID, "mini") {
					inputCost = 0.15
					outputCost = 0.60
				}
			}

			specs = append(specs, domain.ModelSpec{
				ID:                m.ID,
				Name:              m.ID,
				Provider:          c.Type(),
				ContextWindow:     contextWindow,
				SupportsTools:     true,
				SupportsStreaming: true,
				IsLocal:           isLocal,
				InputCostPerM:     inputCost,
				OutputCostPerM:    outputCost,
			})
		}
	}

	if len(specs) == 0 && isCompatible {
		specs = append(specs, domain.ModelSpec{
			ID:                "default",
			Name:              "Default Model (Unsloth / Local)",
			Provider:          c.Type(),
			ContextWindow:     32768,
			SupportsTools:     true,
			SupportsStreaming: true,
			IsLocal:           true,
			InputCostPerM:     0,
			OutputCostPerM:    0,
		})
	}
	return specs, nil
}

type openAIChatMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCalls  []openAIToolCallItem `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type openAIToolCallItem struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

type openAIChatRequest struct {
	Model         string              `json:"model"`
	Messages      []openAIChatMessage `json:"messages"`
	Stream        bool                `json:"stream"`
	StreamOptions map[string]any      `json:"stream_options,omitempty"`
	Temperature   float64             `json:"temperature,omitempty"`
	MaxTokens     int                 `json:"max_tokens,omitempty"`
	Tools         []any               `json:"tools,omitempty"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int               `json:"index"`
		Message      openAIChatMessage `json:"message"`
		FinishReason string            `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIChatStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string               `json:"role"`
			Content   string               `json:"content"`
			ToolCalls []openAIToolCallItem `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// SetHeaders applies the JSON content type and bearer auth to a request.
func (c *Client) SetHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func convertToOpenAIMessages(msgs []domain.Message) []openAIChatMessage {
	out := make([]openAIChatMessage, 0, len(msgs))
	for _, m := range msgs {
		item := openAIChatMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		if len(m.ToolCalls) > 0 {
			item.ToolCalls = make([]openAIToolCallItem, 0, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				item.ToolCalls = append(item.ToolCalls, openAIToolCallItem{
					Index: i,
					ID:    tc.ID,
					Type:  "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}
		out = append(out, item)
	}
	return out
}

func convertToolsToOpenAI(tools []domain.ToolDefinition) []any {
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
	payload := openAIChatRequest{
		Model:       req.Model,
		Messages:    convertToOpenAIMessages(req.Messages),
		Stream:      false,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Tools:       convertToolsToOpenAI(req.Tools),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s chat request: %w", c.label, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	c.SetHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute %s request: %w", c.label, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, c.label+" chat")
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode %s response: %w", c.label, err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("%s returned empty choices", c.label)
	}

	choice := chatResp.Choices[0]
	toolCalls := make([]domain.ToolCall, 0, len(choice.Message.ToolCalls))
	for _, tc := range choice.Message.ToolCalls {
		args := tc.Function.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		toolCalls = append(toolCalls, domain.ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: domain.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: args,
			},
		})
	}

	return &domain.ChatResponse{
		ID:    chatResp.ID,
		Model: chatResp.Model,
		Message: domain.Message{
			Role:      domain.RoleAssistant,
			Content:   choice.Message.Content,
			ToolCalls: toolCalls,
		},
		FinishReason: choice.FinishReason,
		Usage: domain.TokenUsage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		},
	}, nil
}

// StreamChat executes an asynchronous streaming chat request via SSE.
func (c *Client) StreamChat(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamChunk, error) {
	payload := openAIChatRequest{
		Model:         req.Model,
		Messages:      convertToOpenAIMessages(req.Messages),
		Stream:        true,
		StreamOptions: map[string]any{"include_usage": true},
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		Tools:         convertToolsToOpenAI(req.Tools),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s stream request: %w", c.label, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	c.SetHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute %s stream request: %w", c.label, err)
	}

	if resp.StatusCode != http.StatusOK {
		err := httpx.StatusError(resp, c.label+" stream")
		resp.Body.Close()
		return nil, err
	}

	outChan := make(chan domain.StreamChunk, 20)

	go func() {
		defer resp.Body.Close()
		defer close(outChan)
		defer stream.Recover(outChan, c.label)

		type toolCallAcc struct {
			id        string
			name      string
			arguments strings.Builder
		}
		accumulatedTools := make(map[int]*toolCallAcc)
		var toolIndices []int

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if err := ctx.Err(); err != nil {
				stream.TrySend(outChan, domain.StreamChunk{Error: err})
				return
			}

			line := scanner.Text()
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataStr == "[DONE]" {
				break
			}

			var chunk openAIChatStreamChunk
			if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
				continue
			}

			sc := domain.StreamChunk{}
			if chunk.Usage != nil {
				sc.Usage = &domain.TokenUsage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}

			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				sc.ContentDelta = choice.Delta.Content
				sc.FinishReason = choice.FinishReason

				if len(choice.Delta.ToolCalls) > 0 {
					for _, tc := range choice.Delta.ToolCalls {
						acc, exists := accumulatedTools[tc.Index]
						if !exists {
							acc = &toolCallAcc{}
							accumulatedTools[tc.Index] = acc
							toolIndices = append(toolIndices, tc.Index)
						}
						if tc.ID != "" {
							acc.id = tc.ID
						}
						if tc.Function.Name != "" {
							acc.name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							acc.arguments.WriteString(tc.Function.Arguments)
						}
					}
				}
			}

			// Emit content chunks or token usage.
			if sc.ContentDelta != "" || sc.Usage != nil {
				if !stream.Send(ctx, outChan, sc) {
					return
				}
			}
		}

		// Emit all accumulated tool calls in their streamed order.
		for _, idx := range toolIndices {
			acc := accumulatedTools[idx]
			if acc != nil && acc.name != "" {
				args := acc.arguments.String()
				if strings.TrimSpace(args) == "" {
					args = "{}"
				}
				if !stream.Send(ctx, outChan, domain.StreamChunk{
					FinishReason: "tool_calls",
					ToolCallDelta: &domain.ToolCall{
						ID:   acc.id,
						Type: "function",
						Function: domain.FunctionCall{
							Name:      acc.name,
							Arguments: args,
						},
					},
				}) {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			stream.TrySend(outChan, domain.StreamChunk{Error: fmt.Errorf("scanner error: %w", err)})
		}
	}()

	return outChan, nil
}
