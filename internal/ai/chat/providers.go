package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/providers/anthropic"
	"tamestack/internal/ai/providers/gemini"
	"tamestack/internal/ai/providers/llamacpp"
	"tamestack/internal/ai/providers/ollama"
	"tamestack/internal/ai/providers/openai"
	"tamestack/internal/db"
)

// ProviderConfigDTO is a safe view of an AI provider configuration for the UI.
type ProviderConfigDTO struct {
	ID           string    `json:"id"`
	ProviderType string    `json:"provider_type"`
	Name         string    `json:"name"`
	BaseURL      string    `json:"base_url"`
	HasAPIKey    bool      `json:"has_api_key"`
	IsEnabled    bool      `json:"is_enabled"`
	IsDefault    bool      `json:"is_default"`
	DefaultModel string    `json:"default_model"`
	SystemPrompt string    `json:"system_prompt"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProviderConfigInput is sent from the frontend to create or update a provider config.
type ProviderConfigInput struct {
	ID           string `json:"id"`
	ProviderType string `json:"provider_type"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key,omitempty"` // Plain text to encrypt
	IsEnabled    bool   `json:"is_enabled"`
	IsDefault    bool   `json:"is_default"`
	DefaultModel string `json:"default_model"`
	SystemPrompt string `json:"system_prompt"`
}

// ListProviderConfigs returns safe representations of all configured providers.
func (s *Service) ListProviderConfigs(ctx context.Context) ([]ProviderConfigDTO, error) {
	records, err := s.database.ListAIProviderConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list ai provider configs: %w", err)
	}

	result := make([]ProviderConfigDTO, 0, len(records))
	for _, r := range records {
		result = append(result, ProviderConfigDTO{
			ID:           r.ID,
			ProviderType: r.ProviderType,
			Name:         r.Name,
			BaseURL:      r.BaseURL,
			HasAPIKey:    r.APIKeyEncrypted != "",
			IsEnabled:    r.IsEnabled,
			IsDefault:    r.IsDefault,
			DefaultModel: r.DefaultModel,
			SystemPrompt: r.SystemPrompt,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
		})
	}
	return result, nil
}

// SaveProviderConfig encrypts the API key and persists the provider configuration.
func (s *Service) SaveProviderConfig(ctx context.Context, input ProviderConfigInput) error {
	id := input.ID
	if id == "" {
		id = "prov_" + uuid.New().String()
	}

	existing, _ := s.database.GetAIProviderConfig(ctx, id)
	encryptedKey := ""
	if existing != nil {
		encryptedKey = existing.APIKeyEncrypted
	}

	// If a new API key was provided, encrypt it.
	if input.APIKey != "" {
		if s.vault != nil {
			enc, err := s.vault.Encrypt(input.APIKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt provider api key: %w", err)
			}
			encryptedKey = enc
		} else {
			encryptedKey = input.APIKey // fallback if vault is nil (testing)
		}
	}

	rec := &db.AIProviderConfigRecord{
		ID:              id,
		ProviderType:    input.ProviderType,
		Name:            input.Name,
		BaseURL:         input.BaseURL,
		APIKeyEncrypted: encryptedKey,
		IsEnabled:       input.IsEnabled,
		IsDefault:       input.IsDefault,
		DefaultModel:    input.DefaultModel,
		SystemPrompt:    input.SystemPrompt,
	}

	if err := s.database.SaveAIProviderConfig(ctx, rec); err != nil {
		return fmt.Errorf("failed to save provider config in database: %w", err)
	}

	// If this provider is marked as default, also update global default settings.
	if input.IsDefault && s.database != nil {
		_ = s.database.SetAIGlobalSetting(ctx, "default_provider", input.ProviderType)
		if input.DefaultModel != "" {
			_ = s.database.SetAIGlobalSetting(ctx, "default_model", input.DefaultModel)
		}
		if input.SystemPrompt != "" {
			_ = s.database.SetAIGlobalSetting(ctx, "system_prompt", input.SystemPrompt)
		}
	}

	return nil
}

// DeleteProviderConfig removes a provider configuration.
func (s *Service) DeleteProviderConfig(ctx context.Context, id string) error {
	return s.database.DeleteAIProviderConfig(ctx, id)
}

// GetProvider instantiates the requested provider client using its saved configuration.
func (s *Service) GetProvider(ctx context.Context, pType domain.ProviderType) (domain.Provider, error) {
	normalizedType := domain.ProviderType(strings.ToLower(strings.ReplaceAll(string(pType), "-", "_")))
	if normalizedType == "unsloth" || normalizedType == "openaicompatible" {
		pType = domain.ProviderOpenAICompatible
	}

	var rec *db.AIProviderConfigRecord
	if s.database != nil {
		r, err := s.database.GetAIProviderConfigByType(ctx, string(pType))
		if err != nil {
			return nil, fmt.Errorf("failed to get provider config: %w", err)
		}
		rec = r
	}

	baseURL := ""
	apiKey := ""

	if rec != nil {
		baseURL = rec.BaseURL
		if rec.APIKeyEncrypted != "" && s.vault != nil {
			dec, err := s.vault.Decrypt(rec.APIKeyEncrypted)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt api key for %s: %w", pType, err)
			}
			apiKey = dec
		}
	}

	switch pType {
	case domain.ProviderOllama:
		if baseURL == "" {
			baseURL = "http://127.0.0.1:11434"
		}
		return ollama.NewClient(ollama.Config{BaseURL: baseURL}), nil

	case domain.ProviderLlamaCpp:
		if baseURL == "" {
			baseURL = "http://127.0.0.1:8080"
		}
		return llamacpp.NewClient(llamacpp.Config{BaseURL: baseURL, APIKey: apiKey}), nil

	case domain.ProviderOpenAI:
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return openai.NewClient(openai.Config{BaseURL: baseURL, APIKey: apiKey, ProviderType: domain.ProviderOpenAI}), nil

	case domain.ProviderOpenAICompatible:
		if baseURL == "" {
			baseURL = "http://127.0.0.1:8000/v1"
		}
		return openai.NewClient(openai.Config{BaseURL: baseURL, APIKey: apiKey, ProviderType: domain.ProviderOpenAICompatible}), nil

	case domain.ProviderGemini:
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com/v1beta"
		}
		return gemini.NewClient(gemini.Config{BaseURL: baseURL, APIKey: apiKey}), nil

	case domain.ProviderAnthropic:
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		return anthropic.NewClient(anthropic.Config{BaseURL: baseURL, APIKey: apiKey}), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", pType)
	}
}

// TestProvider checks connectivity to a provider.
func (s *Service) TestProvider(ctx context.Context, pType domain.ProviderType) error {
	prov, err := s.GetProvider(ctx, pType)
	if err != nil {
		return err
	}
	return prov.Ping(ctx)
}

// ListModels returns available models from the specified provider.
func (s *Service) ListModels(ctx context.Context, pType domain.ProviderType) ([]domain.ModelSpec, error) {
	prov, err := s.GetProvider(ctx, pType)
	if err != nil {
		return nil, err
	}
	return prov.ListModels(ctx)
}
