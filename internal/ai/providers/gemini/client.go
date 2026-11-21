package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/providers/httpx"
	"tamestack/internal/ai/providers/stream"
)

// Client implements domain.Provider for Google Gemini API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Config holds options for the Gemini client.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new Google Gemini provider client.
func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
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

// Type returns domain.ProviderGemini.
func (c *Client) Type() domain.ProviderType {
	return domain.ProviderGemini
}

// Ping verifies API key validity and connection to Gemini API.
func (c *Client) Ping(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/models?key=%s", c.baseURL, url.QueryEscape(c.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gemini is not reachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return httpx.StatusError(resp, "gemini ping")
	}
	return nil
}

type geminiModelsResponse struct {
	Models []struct {
		Name                       string   `json:"name"` // "models/gemini-1.5-pro"
		DisplayName                string   `json:"displayName"`
		Description                string   `json:"description"`
		InputTokenLimit            int      `json:"inputTokenLimit"`
		OutputTokenLimit           int      `json:"outputTokenLimit"`
		SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	} `json:"models"`
}

// ListModels retrieves Gemini models capable of generateContent.
func (c *Client) ListModels(ctx context.Context) ([]domain.ModelSpec, error) {
	reqURL := fmt.Sprintf("%s/models?key=%s", c.baseURL, url.QueryEscape(c.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list models request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gemini models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, "gemini models")
	}

	var listResp geminiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode gemini models: %w", err)
	}

	specs := make([]domain.ModelSpec, 0)
	for _, m := range listResp.Models {
		isGenerateContent := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				isGenerateContent = true
				break
			}
		}
		if !isGenerateContent {
			continue
		}

		shortID := strings.TrimPrefix(m.Name, "models/")
		if !strings.HasPrefix(shortID, "gemini") {
			continue
		}

		specs = append(specs, domain.ModelSpec{
			ID:                shortID,
			Name:              m.DisplayName,
			Provider:          domain.ProviderGemini,
			ContextWindow:     m.InputTokenLimit,
			SupportsTools:     true,
			SupportsStreaming: true,
			IsLocal:           false,
			InputCostPerM:     0.35,
			OutputCostPerM:    1.05,
		})
	}
	return specs, nil
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"` // "user" or "model"
	Parts []geminiPart `json:"parts"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiCandidate struct {
	Content struct {
		Parts []geminiPart `json:"parts"`
		Role  string       `json:"role"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

func prepareGeminiPayload(req *domain.ChatRequest) geminiRequest {
	var sysParts []geminiPart
	contents := make([]geminiContent, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == domain.RoleSystem {
			sysParts = append(sysParts, geminiPart{Text: m.Content})
			continue
		}

		role := "user"
		if m.Role == domain.RoleAssistant {
			role = "model"
		}

		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	payload := geminiRequest{
		Contents: contents,
	}

	if len(sysParts) > 0 {
		payload.SystemInstruction = &geminiSystemInstruction{Parts: sysParts}
	}

	if req.Temperature > 0 || req.MaxTokens > 0 {
		payload.GenerationConfig = &geminiGenerationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	return payload
}

// Chat executes a synchronous request to Gemini API.
func (c *Client) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	modelName := req.Model
	if modelName == "" {
		modelName = "gemini-1.5-flash"
	}
	modelName = strings.TrimPrefix(modelName, "models/")

	payload := prepareGeminiPayload(req)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, url.PathEscape(modelName), url.QueryEscape(c.apiKey))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute gemini request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpx.StatusError(resp, "gemini chat")
	}

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("failed to decode gemini response: %w", err)
	}

	if len(gResp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	cand := gResp.Candidates[0]
	text := ""
	for _, p := range cand.Content.Parts {
		text += p.Text
	}

	usage := domain.TokenUsage{}
	if gResp.UsageMetadata != nil {
		usage.PromptTokens = gResp.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = gResp.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = gResp.UsageMetadata.TotalTokenCount
	}

	return &domain.ChatResponse{
		ID:    fmt.Sprintf("gemini_%d", time.Now().UnixNano()),
		Model: modelName,
		Message: domain.Message{
			Role:    domain.RoleAssistant,
			Content: text,
		},
		FinishReason: cand.FinishReason,
		Usage:        usage,
	}, nil
}

// StreamChat executes an SSE streaming request to Gemini API.
func (c *Client) StreamChat(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamChunk, error) {
	modelName := req.Model
	if modelName == "" {
		modelName = "gemini-1.5-flash"
	}
	modelName = strings.TrimPrefix(modelName, "models/")

	payload := prepareGeminiPayload(req)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini stream request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, url.PathEscape(modelName), url.QueryEscape(c.apiKey))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute gemini stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		err := httpx.StatusError(resp, "gemini stream")
		resp.Body.Close()
		return nil, err
	}

	outChan := make(chan domain.StreamChunk, 20)

	go func() {
		defer resp.Body.Close()
		defer close(outChan)
		defer stream.Recover(outChan, "gemini")

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
			var gResp geminiResponse
			if err := json.Unmarshal([]byte(dataStr), &gResp); err != nil {
				continue
			}

			if len(gResp.Candidates) > 0 {
				cand := gResp.Candidates[0]
				var delta string
				for _, p := range cand.Content.Parts {
					delta += p.Text
				}

				sc := domain.StreamChunk{
					ContentDelta: delta,
					FinishReason: cand.FinishReason,
				}

				if gResp.UsageMetadata != nil {
					sc.Usage = &domain.TokenUsage{
						PromptTokens:     gResp.UsageMetadata.PromptTokenCount,
						CompletionTokens: gResp.UsageMetadata.CandidatesTokenCount,
						TotalTokens:      gResp.UsageMetadata.TotalTokenCount,
					}
				}

				if !stream.Send(ctx, outChan, sc) {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			stream.TrySend(outChan, domain.StreamChunk{Error: fmt.Errorf("scanner error: %w", err)})
		}
	}()

	return outChan, nil
}
