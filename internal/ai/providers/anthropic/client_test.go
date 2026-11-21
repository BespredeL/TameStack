package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"tamestack/internal/ai/domain"
)

func TestAnthropicProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test-anthropic-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/messages" {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)

			_, _ = w.Write([]byte(`data: {"type":"message_start","message":{"usage":{"input_tokens":12}}}` + "\n\n"))
			if ok {
				flusher.Flush()
			}

			_, _ = w.Write([]byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Claude sonnet response"}}` + "\n\n"))
			if ok {
				flusher.Flush()
			}

			_, _ = w.Write([]byte(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":6}}` + "\n\n"))
			if ok {
				flusher.Flush()
			}

			_, _ = w.Write([]byte(`data: {"type":"message_stop"}` + "\n\n"))
			if ok {
				flusher.Flush()
			}
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		APIKey:     "test-anthropic-key",
		HTTPClient: server.Client(),
	})

	ctx := context.Background()

	models, err := client.ListModels(ctx)
	if err != nil || len(models) < 1 {
		t.Fatalf("anthropic list models failed: %v", err)
	}

	streamChan, err := client.StreamChat(ctx, &domain.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []domain.Message{{Role: domain.RoleUser, Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("anthropic stream chat failed: %v", err)
	}

	var content string
	var usage *domain.TokenUsage
	for chunk := range streamChan {
		if chunk.Error != nil {
			t.Fatalf("unexpected chunk error: %v", chunk.Error)
		}
		content += chunk.ContentDelta
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}

	if content != "Claude sonnet response" {
		t.Fatalf("unexpected content: %s", content)
	}
	if usage == nil || usage.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}
