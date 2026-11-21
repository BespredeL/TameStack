package router

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/ai/domain"
)

// RoutingPolicy defines rules for selecting models.
type RoutingPolicy string

const (
	PolicyLocalOnly    RoutingPolicy = "local_only"    // Strictly local, cloud forbidden (maximum privacy)
	PolicyLocalFirst   RoutingPolicy = "local_first"   // Prefer local, fallback to cloud if local unavailable
	PolicyCloudComplex RoutingPolicy = "cloud_complex" // Local for simple, cloud for multi-step diagnosis
	PolicyAuto         RoutingPolicy = "auto"          // Automatic balance between cost, privacy and complexity
)

// TaskComplexity assesses difficulty of the prompt.
type TaskComplexity string

const (
	ComplexitySimple   TaskComplexity = "simple"   // Command explanations, short definitions, basic status
	ComplexityModerate TaskComplexity = "moderate" // Single config verification, log filtering
	ComplexityComplex  TaskComplexity = "complex"  // Architecture planning, multi-step migration, SQL tuning
)

// RouterConfig holds settings for the intelligent router.
type RouterConfig struct {
	Policy         RoutingPolicy `json:"policy"`
	FallbackModel  string        `json:"fallback_model"`
	FallbackType   domain.ProviderType
	MaxCostPerCall float64 `json:"max_cost_per_call"`
}

// Router chooses the best model and provider for a given user request.
type Router struct {
	config    RouterConfig
	providers map[domain.ProviderType]domain.Provider
}

// NewRouter creates a new intelligent Model Router.
func NewRouter(cfg RouterConfig, providers map[domain.ProviderType]domain.Provider) *Router {
	if cfg.Policy == "" {
		cfg.Policy = PolicyAuto
	}
	return &Router{
		config:    cfg,
		providers: providers,
	}
}

// AnalyzeComplexity evaluates the difficulty level of a request based on heuristics.
func AnalyzeComplexity(prompt string, hasTools bool) TaskComplexity {
	lower := strings.ToLower(prompt)
	words := len(strings.Fields(lower))

	complexKeywords := []string{
		"архитектур", "миграци", "оптимизаци", "deadlock", "тюнинг", "репликаци",
		"кластер", "performance", "architecture", "audit", "диагностик", "investigate",
	}

	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			return ComplexityComplex
		}
	}

	if words > 150 || (hasTools && words > 50) {
		return ComplexityComplex
	}
	if words > 40 || hasTools {
		return ComplexityModerate
	}
	return ComplexitySimple
}

// RouteDecision is the output recommendation of the router.
type RouteDecision struct {
	ProviderType  domain.ProviderType `json:"provider_type"`
	Model         string              `json:"model"`
	Reason        string              `json:"reason"`
	EstimatedCost float64             `json:"estimated_cost"`
	IsFallback    bool                `json:"is_fallback"`
}

// SelectModel resolves the optimal provider and model for the prompt.
func (r *Router) SelectModel(ctx context.Context, prompt string, tools []domain.ToolDefinition) (*RouteDecision, error) {
	complexity := AnalyzeComplexity(prompt, len(tools) > 0)
	hasTools := len(tools) > 0

	// 1. PolicyLocalOnly - strictly local models.
	if r.config.Policy == PolicyLocalOnly {
		return r.selectLocal(ctx, hasTools, "Политика строгой конфиденциальности (Local Only)")
	}

	// 2. PolicyLocalFirst
	if r.config.Policy == PolicyLocalFirst {
		dec, err := r.selectLocal(ctx, hasTools, "Политика Local First")
		if err == nil {
			return dec, nil
		}
		// Fallback to cloud if local fails.
		return r.selectCloud(ctx, complexity, hasTools, "Резервное облако (локальный инференс недоступен)")
	}

	// 3. PolicyCloudComplex & PolicyAuto.
	if complexity == ComplexitySimple {
		dec, err := r.selectLocal(ctx, hasTools, "Простой запрос перенаправлен на быструю локальную модель")
		if err == nil {
			return dec, nil
		}
	}

	// For complex queries or when local is unavailable, select capable cloud model.
	return r.selectCloud(ctx, complexity, hasTools, fmt.Sprintf("Запрос сложности '%s' направлен в производительную модель", complexity))
}

func (r *Router) selectLocal(ctx context.Context, hasTools bool, reason string) (*RouteDecision, error) {
	// Try Ollama first.
	if p, ok := r.providers[domain.ProviderOllama]; ok && p != nil {
		if err := p.Ping(ctx); err == nil {
			models, mErr := p.ListModels(ctx)
			if mErr == nil && len(models) > 0 {
				chosen := models[0].ID
				for _, m := range models {
					if hasTools && m.SupportsTools {
						chosen = m.ID
						break
					}
				}
				return &RouteDecision{
					ProviderType:  domain.ProviderOllama,
					Model:         chosen,
					Reason:        reason,
					EstimatedCost: 0.0,
				}, nil
			}
		}
	}

	// Try llama.cpp next.
	if p, ok := r.providers[domain.ProviderLlamaCpp]; ok && p != nil {
		if err := p.Ping(ctx); err == nil {
			models, mErr := p.ListModels(ctx)
			if mErr == nil && len(models) > 0 {
				return &RouteDecision{
					ProviderType:  domain.ProviderLlamaCpp,
					Model:         models[0].ID,
					Reason:        reason + " (llama.cpp)",
					EstimatedCost: 0.0,
				}, nil
			}
		}
	}

	// Try OpenAI-compatible local server (Unsloth / vLLM / LocalAI).
	if p, ok := r.providers[domain.ProviderOpenAICompatible]; ok && p != nil {
		if err := p.Ping(ctx); err == nil {
			models, mErr := p.ListModels(ctx)
			if mErr == nil && len(models) > 0 {
				chosen := models[0].ID
				for _, m := range models {
					if hasTools && m.SupportsTools {
						chosen = m.ID
						break
					}
				}
				return &RouteDecision{
					ProviderType:  domain.ProviderOpenAICompatible,
					Model:         chosen,
					Reason:        reason + " (OpenAI-совместимый / Unsloth)",
					EstimatedCost: 0.0,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("локальные модели (Ollama, Unsloth/OpenAI-compatible, llama.cpp) недоступны или не запущены")
}

func (r *Router) selectCloud(ctx context.Context, complexity TaskComplexity, hasTools bool, reason string) (*RouteDecision, error) {
	// Try OpenAI-compatible custom endpoint first if explicitly configured.
	if p, ok := r.providers[domain.ProviderOpenAICompatible]; ok && p != nil {
		if err := p.Ping(ctx); err == nil {
			models, mErr := p.ListModels(ctx)
			if mErr == nil && len(models) > 0 {
				return &RouteDecision{
					ProviderType:  domain.ProviderOpenAICompatible,
					Model:         models[0].ID,
					Reason:        reason + " (OpenAI-совместимый / Unsloth)",
					EstimatedCost: 0.0,
				}, nil
			}
		}
	}

	if p, ok := r.providers[domain.ProviderOpenAI]; ok && p != nil {
		if err := p.Ping(ctx); err == nil {
			model := "gpt-4o-mini"
			if complexity == ComplexityComplex {
				model = "gpt-4o"
			}
			return &RouteDecision{
				ProviderType:  domain.ProviderOpenAI,
				Model:         model,
				Reason:        reason + " (OpenAI)",
				EstimatedCost: 0.002,
			}, nil
		}
	}

	if p, ok := r.providers[domain.ProviderGemini]; ok && p != nil {
		if err := p.Ping(ctx); err == nil {
			model := "gemini-1.5-flash"
			if complexity == ComplexityComplex {
				model = "gemini-1.5-pro"
			}
			return &RouteDecision{
				ProviderType:  domain.ProviderGemini,
				Model:         model,
				Reason:        reason + " (Gemini)",
				EstimatedCost: 0.001,
			}, nil
		}
	}

	if p, ok := r.providers[domain.ProviderAnthropic]; ok && p != nil {
		if err := p.Ping(ctx); err == nil {
			model := "claude-3-5-haiku-20241022"
			if complexity == ComplexityComplex {
				model = "claude-3-5-sonnet-20241022"
			}
			return &RouteDecision{
				ProviderType:  domain.ProviderAnthropic,
				Model:         model,
				Reason:        reason + " (Anthropic)",
				EstimatedCost: 0.003,
			}, nil
		}
	}

	// Fallback to local if all cloud fails.
	dec, err := r.selectLocal(ctx, hasTools, "Аварийный откат на локальный инференс (облачные провайдеры недоступны)")
	if err == nil {
		dec.IsFallback = true
		return dec, nil
	}

	return nil, fmt.Errorf("ни один сконфигурированный AI-провайдер не доступен")
}
