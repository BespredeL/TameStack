package router

import (
	"context"
	"testing"

	"tamestack/internal/ai/domain"
)

type mockProvider struct {
	pType   domain.ProviderType
	healthy bool
	models  []domain.ModelSpec
}

func (m *mockProvider) Type() domain.ProviderType { return m.pType }
func (m *mockProvider) Ping(ctx context.Context) error {
	if !m.healthy {
		return context.DeadlineExceeded
	}
	return nil
}
func (m *mockProvider) ListModels(ctx context.Context) ([]domain.ModelSpec, error) {
	return m.models, nil
}
func (m *mockProvider) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	return nil, nil
}
func (m *mockProvider) StreamChat(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamChunk, error) {
	return nil, nil
}

func TestModelRouter(t *testing.T) {
	ctx := context.Background()

	ollamaMock := &mockProvider{
		pType:   domain.ProviderOllama,
		healthy: true,
		models: []domain.ModelSpec{
			{ID: "qwen2.5-coder:7b", SupportsTools: true, IsLocal: true},
		},
	}

	openAIMock := &mockProvider{
		pType:   domain.ProviderOpenAI,
		healthy: true,
		models: []domain.ModelSpec{
			{ID: "gpt-4o", SupportsTools: true, IsLocal: false},
			{ID: "gpt-4o-mini", SupportsTools: true, IsLocal: false},
		},
	}

	providers := map[domain.ProviderType]domain.Provider{
		domain.ProviderOllama: ollamaMock,
		domain.ProviderOpenAI: openAIMock,
	}

	// 1. PolicyLocalOnly - must never choose cloud.
	routerLocalOnly := NewRouter(RouterConfig{Policy: PolicyLocalOnly}, providers)
	dec, err := routerLocalOnly.SelectModel(ctx, "Сложная архитектурная оптимизация кластера MySQL", nil)
	if err != nil {
		t.Fatalf("local only routing failed: %v", err)
	}
	if dec.ProviderType != domain.ProviderOllama {
		t.Fatalf("expected Ollama for local only, got %s", dec.ProviderType)
	}

	// 2. PolicyLocalOnly when local is down - must return error, NOT cloud!
	ollamaMock.healthy = false
	_, err = routerLocalOnly.SelectModel(ctx, "Команда ls -la", nil)
	if err == nil {
		t.Fatalf("expected error when local is down in PolicyLocalOnly, but got success")
	}
	ollamaMock.healthy = true

	// 3. Complexity & Auto Routing.
	routerAuto := NewRouter(RouterConfig{Policy: PolicyAuto}, providers)

	// Simple request -> local model.
	simpleDec, err := routerAuto.SelectModel(ctx, "Как посмотреть статус службы nginx?", nil)
	if err != nil {
		t.Fatalf("auto simple routing failed: %v", err)
	}
	if simpleDec.ProviderType != domain.ProviderOllama {
		t.Fatalf("expected Ollama for simple request, got %s", simpleDec.ProviderType)
	}

	// Complex request -> cloud model.
	complexDec, err := routerAuto.SelectModel(ctx, "Глубокая оптимизация архитектуры репликации и тюнинг буферного пула", nil)
	if err != nil {
		t.Fatalf("auto complex routing failed: %v", err)
	}
	if complexDec.ProviderType != domain.ProviderOpenAI || complexDec.Model != "gpt-4o" {
		t.Fatalf("expected OpenAI gpt-4o for complex request, got %+v", complexDec)
	}

	// 4. Token & Cost Estimator.
	tokens := EstimateTokens("Привет, TameStack AI!")
	if tokens < 3 || tokens > 15 {
		t.Fatalf("unexpected token estimate: %d", tokens)
	}

	cost := EstimateChatCost(domain.ModelSpec{
		InputCostPerM:  2.50,
		OutputCostPerM: 10.00,
	}, 1000, 500)
	// (1000/1e6)*2.50 + (500/1e6)*10.00 = 0.0025 + 0.0050 = 0.0075.
	if cost < 0.007 || cost > 0.008 {
		t.Fatalf("unexpected cost estimate: %f", cost)
	}
}
