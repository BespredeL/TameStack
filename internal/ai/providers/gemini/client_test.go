package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tamestack/internal/ai/domain"
)

func TestGeminiProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key != "test-gemini-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if strings.Contains(r.URL.Path, "/models") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"models": [
					{
						"name": "models/gemini-1.5-flash",
						"displayName": "Gemini 1.5 Flash",
						"inputTokenLimit": 1000000,
						"supportedGenerationMethods": ["generateContent"]
					}
				]
			}`))
			return
		}

		if strings.Contains(r.URL.Path, "streamGenerateContent") {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"parts":[{"text":"Gemini flash response"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}` + "\n\n"))
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
		APIKey:     "test-gemini-key",
		HTTPClient: server.Client(),
	})

	ctx := context.Background()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("gemini ping failed: %v", err)
	}

	models, err := client.ListModels(ctx)
	if err != nil || len(models) != 1 || models[0].ID != "gemini-1.5-flash" {
		t.Fatalf("gemini list models failed: %v", err)
	}

	streamChan, err := client.StreamChat(ctx, &domain.ChatRequest{
		Model:    "gemini-1.5-flash",
		Messages: []domain.Message{{Role: domain.RoleUser, Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("gemini stream chat failed: %v", err)
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

	if content != "Gemini flash response" {
		t.Fatalf("unexpected content: %s", content)
	}
	if usage == nil || usage.TotalTokens != 8 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}
