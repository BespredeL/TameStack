package anthropic

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

// Client implements domain.Provider for Anthropic Messages API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Config holds options for the Anthropic client.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new Anthropic provider client.
func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = httpx.NewClient()
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
	}
}

// Type returns domain.ProviderAnthropic.
func (c *Client) Type() domain.ProviderType {
	return domain.ProviderAnthropic
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
}

// Ping verifies API key validity by attempting a 1-token test message.
func (c *Client) Ping(ctx context.Context) error {
	pingPayload := map[string]any{
		"model":      "claude-3-5-haiku-20241022",
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	}
	data, _ := json.Marshal(pingPayload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic is not reachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {
		// If 200 OK or 400 with valid auth, credentials are acknowledged.
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid anthropic api key")
	}

	return httpx.StatusError(resp, "anthropic ping")
}

// ListModels returns statically supported Anthropic models.
func (c *Client) ListModels(ctx context.Context) ([]domain.ModelSpec, error) {
	return []domain.ModelSpec{
		{
			ID:                "claude-3-5-sonnet-20241022",
			Name:              "Claude 3.5 Sonnet",
			Provider:          domain.ProviderAnthropic,
			ContextWindow:     200000,
			SupportsTools:     true,
			SupportsStreaming: true,
			IsLocal:           false,
			InputCostPerM:     3.00,
			OutputCostPerM:    15.00,
		},
		{
			ID:                "claude-3-5-haiku-20241022",
			Name:              "Claude 3.5 Haiku",
			Provider:          domain.ProviderAnthropic,
			ContextWindow:     200000,
			SupportsTools:     true,
			SupportsStreaming: true,
			IsLocal:           false,
			InputCostPerM:     0.80,
			OutputCostPerM:    4.00,
		},
		{
			ID:                "claude-3-opus-20240229",
			Name:              "Claude 3 Opus",
			Provider:          domain.ProviderAnthropic,
			ContextWindow:     200000,
			SupportsTools:     true,
			SupportsStreaming: true,
			IsLocal:           false,
			InputCostPerM:     15.00,
			OutputCostPerM:    75.00,
		},
	}, nil
}

type anthropicMessageItem struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string                 `json:"model"`
	Messages    []anthropicMessageItem `json:"messages"`
	System      string                 `json:"system,omitempty"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream"`
}

type anthropicResponse struct {
	ID         string `json:"id"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func prepareAnthropicPayload(req *domain.ChatRequest) anthropicRequest {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	model := req.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	var systemPrompt string
	messages := make([]anthropicMessageItem, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == domain.RoleSystem {
			systemPrompt = m.Content
			continue
		}
		role := "user"
		if m.Role == domain.RoleAssistant {
			role = "assistant"
		}
		messages = append(messages, anthropicMessageItem{
			Role:    role,
			Content: m.Content,
		})
	}

	return anthropicRequest{
		Model:       model,
		Messages:    messages,
		System:      systemPrompt,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}
}

// Chat executes a synchronous request to Anthropic.
func (c *Client) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	payload := prepareAnthropicPayload(req)
	payload.Stream = false

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute anthropic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, "anthropic chat")
	}

	var aResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&aResp); err != nil {
		return nil, fmt.Errorf("failed to decode anthropic response: %w", err)
	}

	text := ""
	for _, block := range aResp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return &domain.ChatResponse{
		ID:    aResp.ID,
		Model: aResp.Model,
		Message: domain.Message{
			Role:    domain.RoleAssistant,
			Content: text,
		},
		FinishReason: aResp.StopReason,
		Usage: domain.TokenUsage{
			PromptTokens:     aResp.Usage.InputTokens,
			CompletionTokens: aResp.Usage.OutputTokens,
			TotalTokens:      aResp.Usage.InputTokens + aResp.Usage.OutputTokens,
		},
	}, nil
}

// StreamChat executes an SSE streaming request to Anthropic.
func (c *Client) StreamChat(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamChunk, error) {
	payload := prepareAnthropicPayload(req)
	payload.Stream = true

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute anthropic stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		err := httpx.StatusError(resp, "anthropic stream")
		resp.Body.Close()
		return nil, err
	}

	outChan := make(chan domain.StreamChunk, 20)

	go func() {
		defer resp.Body.Close()
		defer close(outChan)
		defer stream.Recover(outChan, "anthropic")

		var inputTokens, outputTokens int
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			if err := ctx.Err(); err != nil {
				stream.TrySend(outChan, domain.StreamChunk{Error: err})
				return
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var rawMap map[string]any
			if err := json.Unmarshal([]byte(dataStr), &rawMap); err != nil {
				continue
			}

			eventType, _ := rawMap["type"].(string)
			switch eventType {
			case "message_start":
				if msg, ok := rawMap["message"].(map[string]any); ok {
					if usage, ok := msg["usage"].(map[string]any); ok {
						if inT, ok := usage["input_tokens"].(float64); ok {
							inputTokens = int(inT)
						}
					}
				}

			case "content_block_delta":
				if delta, ok := rawMap["delta"].(map[string]any); ok {
					if text, ok := delta["text"].(string); ok {
						if !stream.Send(ctx, outChan, domain.StreamChunk{ContentDelta: text}) {
							return
						}
					}
				}

			case "message_delta":
				var finishReason string
				if delta, ok := rawMap["delta"].(map[string]any); ok {
					finishReason, _ = delta["stop_reason"].(string)
				}
				if usage, ok := rawMap["usage"].(map[string]any); ok {
					if outT, ok := usage["output_tokens"].(float64); ok {
						outputTokens = int(outT)
					}
				}
				if !stream.Send(ctx, outChan, domain.StreamChunk{
					FinishReason: finishReason,
					Usage: &domain.TokenUsage{
						PromptTokens:     inputTokens,
						CompletionTokens: outputTokens,
						TotalTokens:      inputTokens + outputTokens,
					},
				}) {
					return
				}

			case "message_stop":
				return
			}
		}

		if err := scanner.Err(); err != nil {
			stream.TrySend(outChan, domain.StreamChunk{Error: fmt.Errorf("scanner error: %w", err)})
		}
	}()

	return outChan, nil
}
