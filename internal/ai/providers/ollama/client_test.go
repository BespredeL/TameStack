package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tamestack/internal/ai/domain"
)

func TestOllamaProvider(t *testing.T) {
	// Mock HTTP server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version": "0.4.1"}`))

		case "/api/tags":
			resp := ollamaTagsResponse{
				Models: []struct {
					Name       string `json:"name"`
					Model      string `json:"model"`
					ModifiedAt string `json:"modified_at"`
					Size       int64  `json:"size"`
					Details    struct {
						Family            string `json:"family"`
						ParameterSize     string `json:"parameter_size"`
						QuantizationLevel string `json:"quantization_level"`
					} `json:"details"`
				}{
					{
						Name: "qwen2.5-coder:7b",
						Details: struct {
							Family            string `json:"family"`
							ParameterSize     string `json:"parameter_size"`
							QuantizationLevel string `json:"quantization_level"`
						}{
							Family:        "qwen2",
							ParameterSize: "7B",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case "/api/chat":
			var req ollamaChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			if req.Stream {
				w.Header().Set("Content-Type", "application/x-ndjson")
				flusher, ok := w.(http.Flusher)

				// Chunk 1
				_, _ = w.Write([]byte(`{"model":"qwen2.5-coder:7b","message":{"role":"assistant","content":"Nginx is "},"done":false}` + "\n"))
				if ok {
					flusher.Flush()
				}
				// Chunk 2 (Done).
				_, _ = w.Write([]byte(`{"model":"qwen2.5-coder:7b","message":{"role":"assistant","content":"active."},"done":true,"prompt_eval_count":10,"eval_count":5}` + "\n"))
				if ok {
					flusher.Flush()
				}
			} else {
				w.Header().Set("Content-Type", "application/json")
				chunk := ollamaChatChunk{
					Model: "qwen2.5-coder:7b",
					Message: ollamaChatMessage{
						Role:    "assistant",
						Content: "All systems nominal.",
					},
					Done:            true,
					PromptEvalCount: 8,
					EvalCount:       4,
				}
				_ = json.NewEncoder(w).Encode(chunk)
			}

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	ctx := context.Background()

	// 1. Ping
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// 2. List Models.
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(models) != 1 || models[0].ID != "qwen2.5-coder:7b" {
		t.Fatalf("unexpected models: %+v", models)
	}
	if !models[0].SupportsTools || !models[0].IsLocal {
		t.Fatalf("model spec attributes mismatch: %+v", models[0])
	}

	// 3. Synchronous Chat.
	chatResp, err := client.Chat(ctx, &domain.ChatRequest{
		Model: "qwen2.5-coder:7b",
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "Status check"},
		},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if chatResp.Message.Content != "All systems nominal." {
		t.Fatalf("unexpected response content: %s", chatResp.Message.Content)
	}
	if chatResp.Usage.TotalTokens != 12 {
		t.Fatalf("expected 12 total tokens, got %d", chatResp.Usage.TotalTokens)
	}

	// 4. Stream Chat.
	streamChan, err := client.StreamChat(ctx, &domain.ChatRequest{
		Model: "qwen2.5-coder:7b",
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "Stream check"},
		},
	})
	if err != nil {
		t.Fatalf("stream chat failed: %v", err)
	}

	var assembled string
	var finalDone bool
	for chunk := range streamChan {
		if chunk.Error != nil {
			t.Fatalf("stream chunk error: %v", chunk.Error)
		}
		assembled += chunk.ContentDelta
		if chunk.FinishReason == "stop" {
			finalDone = true
			if chunk.Usage == nil || chunk.Usage.TotalTokens != 15 {
				t.Fatalf("unexpected stream usage: %+v", chunk.Usage)
			}
		}
	}

	if assembled != "Nginx is active." || !finalDone {
		t.Fatalf("unexpected stream assembly: '%s', done: %v", assembled, finalDone)
	}
}
