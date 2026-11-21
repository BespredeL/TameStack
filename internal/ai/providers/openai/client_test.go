package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tamestack/internal/ai/domain"
)

func TestOpenAIProvider(t *testing.T) {
	const expectedToken = "sk-test-secret-key-12345"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		switch r.URL.Path {
		case "/models":
			resp := openAIModelListResponse{
				Data: []struct {
					ID      string `json:"id"`
					OwnedBy string `json:"owned_by"`
				}{
					{ID: "gpt-4o", OwnedBy: "system"},
					{ID: "gpt-4o-mini", OwnedBy: "system"},
					{ID: "whisper-1", OwnedBy: "system"}, // Should be filtered out
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case "/chat/completions":
			var req openAIChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			if req.Stream {
				w.Header().Set("Content-Type", "text/event-stream")
				flusher, ok := w.(http.Flusher)

				// SSE Chunk 1
				_, _ = w.Write([]byte(`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Apache"},"finish_reason":null}]}` + "\n\n"))
				if ok {
					flusher.Flush()
				}

				// SSE Chunk 2 with tool call
				_, _ = w.Write([]byte(`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_99","function":{"name":"restart_apache","arguments":"{}"}}]},"finish_reason":null}]}` + "\n\n"))
				if ok {
					flusher.Flush()
				}

				// SSE Chunk 3 with usage & finish
				_, _ = w.Write([]byte(`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}` + "\n\n"))
				if ok {
					flusher.Flush()
				}

				// Done
				_, _ = w.Write([]byte(`data: [DONE]` + "\n\n"))
				if ok {
					flusher.Flush()
				}
			} else {
				w.Header().Set("Content-Type", "application/json")
				resp := openAIChatResponse{
					ID:    "chatcmpl-sync-1",
					Model: req.Model,
					Choices: []struct {
						Index        int               `json:"index"`
						Message      openAIChatMessage `json:"message"`
						FinishReason string            `json:"finish_reason"`
					}{
						{
							Index: 0,
							Message: openAIChatMessage{
								Role:    "assistant",
								Content: "Configuration is valid.",
							},
							FinishReason: "stop",
						},
					},
					Usage: struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
						TotalTokens      int `json:"total_tokens"`
					}{
						PromptTokens:     15,
						CompletionTokens: 5,
						TotalTokens:      20,
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		APIKey:     expectedToken,
		HTTPClient: server.Client(),
	})

	ctx := context.Background()

	// 1. Ping
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("openai ping failed: %v", err)
	}

	// 2. List Models
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("openai list models failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 chat models (filtered), got %d: %+v", len(models), models)
	}
	if models[0].ID != "gpt-4o" || models[1].ID != "gpt-4o-mini" {
		t.Fatalf("unexpected model ids: %+v", models)
	}

	// 3. Synchronous Chat
	chatResp, err := client.Chat(ctx, &domain.ChatRequest{
		Model: "gpt-4o",
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "Check syntax"},
		},
	})
	if err != nil {
		t.Fatalf("openai chat failed: %v", err)
	}
	if chatResp.Message.Content != "Configuration is valid." {
		t.Fatalf("unexpected content: %s", chatResp.Message.Content)
	}
	if chatResp.Usage.TotalTokens != 20 {
		t.Fatalf("expected 20 total tokens, got %d", chatResp.Usage.TotalTokens)
	}

	// 4. Stream Chat
	streamChan, err := client.StreamChat(ctx, &domain.ChatRequest{
		Model: "gpt-4o",
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "Stream with tool"},
		},
	})
	if err != nil {
		t.Fatalf("openai stream chat failed: %v", err)
	}

	var contentAcc string
	var hasToolCall bool
	var finalUsage *domain.TokenUsage

	for chunk := range streamChan {
		if chunk.Error != nil {
			t.Fatalf("stream chunk error: %v", chunk.Error)
		}
		contentAcc += chunk.ContentDelta
		if chunk.ToolCallDelta != nil && chunk.ToolCallDelta.Function.Name == "restart_apache" {
			hasToolCall = true
		}
		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}
	}

	if contentAcc != "Apache" {
		t.Fatalf("unexpected content accumulation: %s", contentAcc)
	}
	if !hasToolCall {
		t.Fatalf("expected tool call chunk not received")
	}
	if finalUsage == nil || finalUsage.TotalTokens != 30 {
		t.Fatalf("unexpected final usage: %+v", finalUsage)
	}
}

func TestOpenAICompatibleProvider_Unsloth(t *testing.T) {
	expectedToken := "unsloth-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+expectedToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "unsloth/Llama-3.2-3B-Instruct", "owned_by": "unsloth"},
					{"id": "custom-fine-tuned-model", "owned_by": "user"},
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:      server.URL + "/v1",
		APIKey:       expectedToken,
		ProviderType: domain.ProviderOpenAICompatible,
		HTTPClient:   server.Client(),
	})

	if client.Type() != domain.ProviderOpenAICompatible {
		t.Fatalf("expected provider type %s, got %s", domain.ProviderOpenAICompatible, client.Type())
	}

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 unsloth models, got %d: %+v", len(models), models)
	}
	if models[0].ID != "unsloth/Llama-3.2-3B-Instruct" {
		t.Fatalf("unexpected model ID: %s", models[0].ID)
	}
	if !models[0].IsLocal {
		t.Fatalf("expected model to be marked as local")
	}
}

func TestOpenAIStreamingToolCallAccumulation(t *testing.T) {
	// Simulate Unsloth / vLLM / OpenAI streaming fragmented tool call chunks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)

		// Chunk 1: Tool call declaration with ID and Name, but empty Arguments
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_metric_1","type":"function","function":{"name":"get_system_metrics","arguments":""}}]},"finish_reason":null}]}` + "\n\n"))
		if ok {
			flusher.Flush()
		}

		// Chunk 2: Fragment of JSON arguments with empty name and ID
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"server_id\":"}}]},"finish_reason":null}]}` + "\n\n"))
		if ok {
			flusher.Flush()
		}

		// Chunk 3: Final fragment of JSON arguments
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"srv-demo-99\"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n"))
		if ok {
			flusher.Flush()
		}

		// Done
		_, _ = w.Write([]byte(`data: [DONE]` + "\n\n"))
		if ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	streamChan, err := client.StreamChat(context.Background(), &domain.ChatRequest{
		Model: "unsloth-model",
	})
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var toolCalls []*domain.ToolCall
	for chunk := range streamChan {
		if chunk.Error != nil {
			t.Fatalf("unexpected chunk error: %v", chunk.Error)
		}
		if chunk.ToolCallDelta != nil {
			toolCalls = append(toolCalls, chunk.ToolCallDelta)
		}
	}

	// Must be exactly 1 tool call accumulated, with no empty tool name chunks
	if len(toolCalls) != 1 {
		t.Fatalf("expected exactly 1 accumulated tool call, got %d", len(toolCalls))
	}
	tc := toolCalls[0]
	if tc.Function.Name != "get_system_metrics" {
		t.Fatalf("expected tool name 'get_system_metrics', got '%s'", tc.Function.Name)
	}
	if tc.ID != "call_metric_1" {
		t.Fatalf("expected tool ID 'call_metric_1', got '%s'", tc.ID)
	}
	expectedArgs := `{"server_id":"srv-demo-99"}`
	if tc.Function.Arguments != expectedArgs {
		t.Fatalf("expected arguments '%s', got '%s'", expectedArgs, tc.Function.Arguments)
	}
}
