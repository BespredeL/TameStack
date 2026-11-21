package llamacpp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"tamestack/internal/ai/domain"
)

func TestLlamaCppProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok"}`))

		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data": [{"id": "mistral-7b-instruct"}]}`))

		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			_, _ = w.Write([]byte(`data: {"id":"chat-1","choices":[{"index":0,"delta":{"content":"Llama response"},"finish_reason":null}]}` + "\n\n"))
			if ok {
				flusher.Flush()
			}
			_, _ = w.Write([]byte(`data: [DONE]` + "\n\n"))
			if ok {
				flusher.Flush()
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

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("llamacpp ping failed: %v", err)
	}

	models, err := client.ListModels(ctx)
	if err != nil || len(models) != 1 || models[0].ID != "mistral-7b-instruct" {
		t.Fatalf("llamacpp list models failed: %v", err)
	}

	streamChan, err := client.StreamChat(ctx, &domain.ChatRequest{
		Model:    "mistral-7b-instruct",
		Messages: []domain.Message{{Role: domain.RoleUser, Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("llamacpp stream chat failed: %v", err)
	}

	var content string
	for chunk := range streamChan {
		if chunk.Error != nil {
			t.Fatalf("unexpected chunk error: %v", chunk.Error)
		}
		content += chunk.ContentDelta
	}

	if content != "Llama response" {
		t.Fatalf("unexpected content: %s", content)
	}
}
