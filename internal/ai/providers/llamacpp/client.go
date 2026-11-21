package llamacpp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/providers/httpx"
	"tamestack/internal/ai/providers/openai"
)

// Client implements domain.Provider for llama.cpp (llama-server). llama-server
// exposes an OpenAI-compatible API, so Chat and StreamChat reuse the openai base
// client; only Ping and ListModels have llama-specific behavior.
type Client struct {
	*openai.Client
}

// Config holds options for the llama.cpp client.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new llama.cpp provider client.
func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = httpx.NewClient()
	}
	return &Client{
		Client: openai.NewClient(openai.Config{
			BaseURL:      baseURL + "/v1",
			APIKey:       cfg.APIKey,
			ProviderType: domain.ProviderLlamaCpp,
			HTTPClient:   httpClient,
			Label:        "llama.cpp",
		}),
	}
}

// Type returns domain.ProviderLlamaCpp.
func (c *Client) Type() domain.ProviderType {
	return domain.ProviderLlamaCpp
}

// Ping checks if llama-server is healthy.
func (c *Client) Ping(ctx context.Context) error {
	root := strings.TrimSuffix(c.BaseURL, "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}
	c.SetHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		// Fallback check on /v1/models if /health isn't exposed.
		mReq, mErr := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
		if mErr != nil {
			return fmt.Errorf("llama.cpp is not reachable at %s: %w", root, err)
		}
		c.SetHeaders(mReq)
		mResp, mErr := c.HTTPClient.Do(mReq)
		if mErr != nil {
			return fmt.Errorf("llama.cpp is not reachable at %s: %w", root, err)
		}
		defer mResp.Body.Close()
		if mResp.StatusCode == http.StatusOK {
			return nil
		}
		return fmt.Errorf("llama.cpp ping returned status %d", mResp.StatusCode)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llama.cpp health returned status %d", resp.StatusCode)
	}
	return nil
}

type llamaModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListModels retrieves loaded models from llama-server.
func (c *Client) ListModels(ctx context.Context) ([]domain.ModelSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list models request: %w", err)
	}
	c.SetHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch llama.cpp models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, "llama.cpp models")
	}

	var listResp llamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode llama.cpp models: %w", err)
	}

	specs := make([]domain.ModelSpec, 0, len(listResp.Data))
	for _, m := range listResp.Data {
		specs = append(specs, domain.ModelSpec{
			ID:                m.ID,
			Name:              m.ID,
			Provider:          domain.ProviderLlamaCpp,
			ContextWindow:     8192,
			SupportsTools:     true,
			SupportsStreaming: true,
			IsLocal:           true,
		})
	}
	if len(specs) == 0 {
		// Provide default fallback spec if server returns empty list.
		specs = append(specs, domain.ModelSpec{
			ID:                "default",
			Name:              "llama.cpp loaded model",
			Provider:          domain.ProviderLlamaCpp,
			ContextWindow:     8192,
			SupportsTools:     true,
			SupportsStreaming: true,
			IsLocal:           true,
		})
	}
	return specs, nil
}
