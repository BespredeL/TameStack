package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	aiContext "tamestack/internal/ai/context"
	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/router"
	"tamestack/internal/ai/tools"
	"tamestack/internal/db"
)

// SecretVault abstracts encryption of sensitive API keys.
type SecretVault interface {
	Encrypt(plainText string) (string, error)
	Decrypt(cipherTextB64 string) (string, error)
}

// Service coordinates AI providers, chat sessions, message persistence, and secrets.
type Service struct {
	database  *db.DB
	vault     SecretVault
	builder   *aiContext.ContextBuilder
	sanitizer *aiContext.Sanitizer
	tools     *tools.Registry
	mu        sync.RWMutex
}

// NewService creates a new AI Chat Service.
func NewService(database *db.DB, vault SecretVault, registry ...*tools.Registry) *Service {
	sanitizer := aiContext.NewSanitizer()
	builder := aiContext.NewContextBuilder(database, sanitizer)
	var reg *tools.Registry
	if len(registry) > 0 {
		reg = registry[0]
	}
	s := &Service{
		database:  database,
		vault:     vault,
		builder:   builder,
		sanitizer: sanitizer,
		tools:     reg,
	}
	if database != nil && reg != nil {
		if raw, err := database.GetAIGlobalSetting(context.Background(), "disabled_tools"); err == nil && raw != "" {
			var list []string
			if err := json.Unmarshal([]byte(raw), &list); err == nil {
				reg.SetDisabledTools(list)
			}
		}
	}
	return s
}

// GetDisabledTools returns the names of all currently disabled AI tools.
func (s *Service) GetDisabledTools(ctx context.Context) []string {
	if s.tools == nil {
		return []string{}
	}
	if s.database != nil {
		raw, err := s.database.GetAIGlobalSetting(ctx, "disabled_tools")
		if err == nil && raw != "" {
			var list []string
			if err := json.Unmarshal([]byte(raw), &list); err == nil {
				s.tools.SetDisabledTools(list)
				return list
			}
		}
	}
	return s.tools.GetDisabledTools()
}

// SetDisabledTools updates and persists the list of disabled AI tools.
func (s *Service) SetDisabledTools(ctx context.Context, disabled []string) error {
	if s.tools != nil {
		s.tools.SetDisabledTools(disabled)
	}
	if s.database != nil {
		bytes, err := json.Marshal(disabled)
		if err != nil {
			return err
		}
		return s.database.SetAIGlobalSetting(ctx, "disabled_tools", string(bytes))
	}
	return nil
}

// --- Conversations ---

// CreateConversation initializes a new chat session.
func (s *Service) CreateConversation(ctx context.Context, title string, serverID *string, systemPrompt, provider, model string) (*db.AIConversationRecord, error) {
	if title == "" {
		title = "Новый диалог " + time.Now().Format("02.01 15:04")
	}

	if provider == "" && s.database != nil {
		defP, _ := s.database.GetAIGlobalSetting(ctx, "default_provider")
		if defP != "" {
			provider = defP
		}
	}
	if provider == "" {
		provider = string(domain.ProviderOllama)
	}

	if model == "" && s.database != nil {
		// 1. Check global default model.
		defM, _ := s.database.GetAIGlobalSetting(ctx, "default_model")
		if defM != "" {
			model = defM
		} else {
			// 2. Check provider-specific default model.
			pCfg, _ := s.database.GetAIProviderConfigByType(ctx, provider)
			if pCfg != nil && pCfg.DefaultModel != "" {
				model = pCfg.DefaultModel
			}
		}
	}
	if model == "" {
		model = "qwen2.5-coder:7b"
	}

	if systemPrompt == "" && s.database != nil {
		appLang, _ := s.database.GetAIGlobalSetting(ctx, "app_language")
		if appLang != "en" {
			appLang = "ru"
		}
		defPrompt, _ := s.database.GetAIGlobalSetting(ctx, "system_prompt_"+appLang)
		if defPrompt == "" {
			defPrompt, _ = s.database.GetAIGlobalSetting(ctx, "system_prompt")
		}
		if defPrompt != "" {
			systemPrompt = defPrompt
		}
	}

	conv := &db.AIConversationRecord{
		ID:               "conv_" + uuid.New().String(),
		Title:            title,
		ServerID:         serverID,
		SystemPrompt:     systemPrompt,
		SelectedProvider: provider,
		SelectedModel:    model,
	}

	if err := s.database.SaveAIConversation(ctx, conv); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	return conv, nil
}

// ListTools returns definitions of all registered AI tools with descriptions in the current language.
func (s *Service) ListTools(ctx context.Context) []domain.ToolDefinition {
	if s.tools == nil {
		return []domain.ToolDefinition{}
	}
	lang := "ru"
	if s.database != nil && ctx != nil {
		if l, _ := s.database.GetAIGlobalSetting(ctx, "app_language"); l != "" {
			lang = l
		}
	}
	return s.tools.DefinitionsLang(lang)
}

// ListConversations returns all conversations ordered by recent activity.
func (s *Service) ListConversations(ctx context.Context) ([]*db.AIConversationRecord, error) {
	return s.database.ListAIConversations(ctx)
}

// GetConversation fetches a conversation by its ID.
func (s *Service) GetConversation(ctx context.Context, id string) (*db.AIConversationRecord, error) {
	return s.database.GetAIConversation(ctx, id)
}

// DeleteConversation deletes a conversation and its messages.
func (s *Service) DeleteConversation(ctx context.Context, id string) error {
	return s.database.DeleteAIConversation(ctx, id)
}

// ListMessages returns all messages for a given conversation.
func (s *Service) ListMessages(ctx context.Context, convID string) ([]*db.AIMessageRecord, error) {
	return s.database.ListAIMessages(ctx, convID)
}

// --- Messaging & Streaming ---

// StreamMessage sends a user message and streams assistant tokens via callback.
func (s *Service) StreamMessage(
	ctx context.Context,
	convID string,
	userPrompt string,
	onChunk func(chunk domain.StreamChunk),
) (*db.AIMessageRecord, error) {
	return s.StreamMessageWithContext(ctx, convID, userPrompt, nil, onChunk)
}

// StreamMessageWithContext sends a user message with optional UI screen context and streams assistant tokens.
func (s *Service) StreamMessageWithContext(
	ctx context.Context,
	convID string,
	userPrompt string,
	uiCtx *aiContext.UIContext,
	onChunk func(chunk domain.StreamChunk),
) (*db.AIMessageRecord, error) {
	conv, err := s.database.GetAIConversation(ctx, convID)
	if err != nil || conv == nil {
		return nil, fmt.Errorf("conversation not found: %s", convID)
	}

	sanitizedUserPrompt := s.sanitizer.Sanitize(userPrompt)
	userMsg := &db.AIMessageRecord{
		ID:             "msg_" + uuid.New().String(),
		ConversationID: convID,
		Role:           string(domain.RoleUser),
		Content:        sanitizedUserPrompt,
		CreatedAt:      time.Now(),
	}
	if err := s.database.SaveAIMessage(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	history, err := s.database.ListAIMessages(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation history: %w", err)
	}

	// Dynamic System Prompt construction.
	appLang := "ru"
	if s.database != nil {
		if l, _ := s.database.GetAIGlobalSetting(ctx, "app_language"); l != "" {
			appLang = l
		}
	}
	sysPrompt, _ := s.builder.BuildSystemPrompt(ctx, aiContext.BuilderOptions{
		ServerID:       conv.ServerID,
		CustomPrompt:   conv.SystemPrompt,
		IncludeMemory:  true,
		LanguageLocale: appLang,
		UIContext:      uiCtx,
	})

	domainMsgs := make([]domain.Message, 0, len(history)+1)
	if sysPrompt != "" {
		domainMsgs = append(domainMsgs, domain.Message{
			Role:    domain.RoleSystem,
			Content: sysPrompt,
		})
	}

	for _, m := range history {
		var tcs []domain.ToolCall
		if m.ToolCallsJSON != "" {
			_ = json.Unmarshal([]byte(m.ToolCallsJSON), &tcs)
		}
		domainMsgs = append(domainMsgs, domain.Message{
			ID:         m.ID,
			Role:       domain.Role(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			ToolCalls:  tcs,
		})
	}

	prov, err := s.GetProvider(ctx, domain.ProviderType(conv.SelectedProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider %s: %w", conv.SelectedProvider, err)
	}

	var toolDefs []domain.ToolDefinition
	if s.tools != nil {
		toolDefs = s.tools.ActiveDefinitionsLang(appLang)
	}

	chatReq := &domain.ChatRequest{
		Model:    conv.SelectedModel,
		Messages: domainMsgs,
		Tools:    toolDefs,
		Stream:   true,
	}

	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()

	streamChan, err := prov.StreamChat(streamCtx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start stream chat: %w", err)
	}

	res, errChunk := accumulateStream(streamChan, onChunk)
	if errChunk != nil {
		if onChunk != nil {
			onChunk(*errChunk)
		}
		return nil, errChunk.Error
	}
	accumulatedContent := res.content
	promptTokens, completionTokens := res.promptTokens, res.completionTokens
	detectedToolCalls := normalizeToolCalls(res.toolCalls)
	toolCallsJSON := marshalToolCalls(detectedToolCalls)

	assistantMsg := &db.AIMessageRecord{
		ID:               "msg_" + uuid.New().String(),
		ConversationID:   convID,
		Role:             string(domain.RoleAssistant),
		Content:          accumulatedContent,
		ToolCallsJSON:    toolCallsJSON,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		CreatedAt:        time.Now(),
	}

	if err := s.database.SaveAIMessage(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}

	maxSteps := 5
	currentAssistantMsg := assistantMsg

	// Determine fallback serverID if conversation or workspace only has one.
	targetServerID := ""
	if conv.ServerID != nil && *conv.ServerID != "" {
		targetServerID = *conv.ServerID
	} else if s.database != nil {
		servers, _ := s.database.ListServers(ctx)
		if len(servers) == 1 {
			targetServerID = servers[0].ID
		}
	}

	for step := 0; step < maxSteps; step++ {
		if len(detectedToolCalls) == 0 || s.tools == nil {
			break
		}

		// Registry.Execute enforces the security gate: tools that require human
		// approval are refused here, proposal tools run and create their own
		// requires-approval task in the Tasks engine.
		domainMsgs = append(domainMsgs, domain.Message{
			Role:      domain.RoleAssistant,
			Content:   accumulatedContent,
			ToolCalls: detectedToolCalls,
		})

		for _, tc := range detectedToolCalls {
			if onChunk != nil {
				onChunk(domain.StreamChunk{
					ContentDelta: fmt.Sprintf("\n> 🔍 *[Инструмент чтения]* Выполняется `%s`...\n\n", tc.Function.Name),
				})
			}

			argsJSON := tc.Function.Arguments
			var parsedArgs map[string]any
			if err := json.Unmarshal([]byte(argsJSON), &parsedArgs); err == nil {
				sid, _ := parsedArgs["server_id"].(string)
				if (sid == "" || sid == "null") && targetServerID != "" {
					parsedArgs["server_id"] = targetServerID
				} else if sid != "" && s.database != nil {
					if srv, err := tools.ResolveServer(ctx, s.database, sid); err == nil && srv != nil {
						parsedArgs["server_id"] = srv.ID
					}
				}
				if b, err := json.Marshal(parsedArgs); err == nil {
					argsJSON = string(b)
				}
			}

			res, execErr := s.tools.Execute(ctx, convID, currentAssistantMsg.ID, tc.Function.Name, argsJSON)
			outContent := ""
			if execErr != nil {
				outContent = fmt.Sprintf(`{"error": "%s"}`, execErr.Error())
			} else {
				outContent = res.RawOutput
			}

			toolMsg := &db.AIMessageRecord{
				ID:             "msg_" + uuid.New().String(),
				ConversationID: convID,
				Role:           string(domain.RoleTool),
				Content:        outContent,
				ToolCallID:     tc.ID,
				CreatedAt:      time.Now(),
			}
			_ = s.database.SaveAIMessage(ctx, toolMsg)

			domainMsgs = append(domainMsgs, domain.Message{
				Role:       domain.RoleTool,
				Content:    outContent,
				ToolCallID: tc.ID,
			})
		}

		// Follow-up stream generation for synthesis with tools available.
		followupReq := &domain.ChatRequest{
			Model:    conv.SelectedModel,
			Messages: domainMsgs,
			Tools:    toolDefs,
			Stream:   true,
		}

		followupCtx, cancelFollowup := context.WithCancel(ctx)
		followupChan, fErr := prov.StreamChat(followupCtx, followupReq)
		if fErr != nil {
			cancelFollowup()
			break
		}

		fRes, errChunk := accumulateStream(followupChan, onChunk)
		cancelFollowup()
		if errChunk != nil {
			if onChunk != nil {
				onChunk(*errChunk)
			}
			break
		}

		followupContent := fRes.content
		followupPromptTokens, followupCompletionTokens := fRes.promptTokens, fRes.completionTokens
		validNextCalls := normalizeToolCalls(fRes.toolCalls)
		nextToolCallsJSON := marshalToolCalls(validNextCalls)

		stepAssistantMsg := &db.AIMessageRecord{
			ID:               "msg_" + uuid.New().String(),
			ConversationID:   convID,
			Role:             string(domain.RoleAssistant),
			Content:          followupContent,
			ToolCallsJSON:    nextToolCallsJSON,
			PromptTokens:     followupPromptTokens,
			CompletionTokens: followupCompletionTokens,
			CreatedAt:        time.Now(),
		}
		_ = s.database.SaveAIMessage(ctx, stepAssistantMsg)
		currentAssistantMsg = stepAssistantMsg

		accumulatedContent = followupContent
		detectedToolCalls = validNextCalls
	}

	return currentAssistantMsg, nil
}

// RouteModel analyzes the prompt and recommends the optimal provider and model based on policy.
func (s *Service) RouteModel(ctx context.Context, prompt string, policy string) (*router.RouteDecision, error) {
	providers := make(map[domain.ProviderType]domain.Provider)
	for _, pt := range []domain.ProviderType{
		domain.ProviderOllama,
		domain.ProviderOpenAICompatible,
		domain.ProviderLlamaCpp,
		domain.ProviderOpenAI,
		domain.ProviderGemini,
		domain.ProviderAnthropic,
	} {
		p, err := s.GetProvider(ctx, pt)
		if err == nil && p != nil {
			providers[pt] = p
		}
	}

	pPolicy := router.RoutingPolicy(policy)
	if pPolicy == "" {
		pPolicy = router.PolicyAuto
	}

	r := router.NewRouter(router.RouterConfig{Policy: pPolicy}, providers)
	return r.SelectModel(ctx, prompt, nil)
}
