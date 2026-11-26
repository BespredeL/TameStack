package main

import (
	"fmt"
	"strings"

	"tamestack/internal/ai/agents"
	"tamestack/internal/ai/chat"
	aiContext "tamestack/internal/ai/context"
	"tamestack/internal/ai/domain"
	"tamestack/internal/ai/router"
	"tamestack/internal/ai/workflows"
	"tamestack/internal/db"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// --- AI Subsystem RPC ---

func (a *App) AIListProviderConfigs() ([]chat.ProviderConfigDTO, error) {
	return withLock(a, func() ([]chat.ProviderConfigDTO, error) {
		return a.aiChatService.ListProviderConfigs(a.ctx)
	})
}

func (a *App) AISaveProviderConfig(input chat.ProviderConfigInput) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.aiChatService.SaveProviderConfig(a.ctx, input)
}

func (a *App) AIDeleteProviderConfig(id string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.aiChatService.DeleteProviderConfig(a.ctx, id)
}

func (a *App) AITestProvider(pType string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.aiChatService.TestProvider(a.ctx, domain.ProviderType(pType))
}

func (a *App) AIListModels(pType string) ([]domain.ModelSpec, error) {
	return withLock(a, func() ([]domain.ModelSpec, error) {
		return a.aiChatService.ListModels(a.ctx, domain.ProviderType(pType))
	})
}

// AIGetGlobalSettings retrieves AI global defaults such as default_provider, default_model, and system_prompt.
func (a *App) AIGetGlobalSettings() (map[string]string, error) {
	return withLock(a, func() (map[string]string, error) {
		settings, err := a.database.GetAllAIGlobalSettings(a.ctx)
		if err != nil {
			return nil, err
		}
		if settings == nil {
			settings = make(map[string]string)
		}
		if _, ok := settings["system_prompt"]; !ok || strings.TrimSpace(settings["system_prompt"]) == "" {
			settings["system_prompt"] = aiContext.DefaultSystemPromptText
		}
		return settings, nil
	})
}

// AISaveGlobalSettings saves AI global defaults.
func (a *App) AISaveGlobalSettings(settings map[string]string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	for k, v := range settings {
		if err := a.database.SetAIGlobalSetting(a.ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}

// AIGetDefaultSystemPrompt returns the baseline system prompt text for the specified language.
func (a *App) AIGetDefaultSystemPrompt(lang string) string {
	if strings.TrimSpace(lang) == "" {
		if a.database != nil {
			saved, _ := a.database.GetAIGlobalSetting(a.ctx, "app_language")
			lang = saved
		}
	}
	return aiContext.GetDefaultSystemPrompt(lang)
}

// AISetAppLanguage updates the active application language for AI prompts and contexts.
func (a *App) AISetAppLanguage(lang string) error {
	trimmed := strings.ToLower(strings.TrimSpace(lang))
	if trimmed != "en" {
		trimmed = "ru"
	}
	if a.database != nil {
		return a.database.SetAIGlobalSetting(a.ctx, "app_language", trimmed)
	}
	return nil
}

func (a *App) AICreateConversation(title string, serverID *string, systemPrompt, provider, model string) (*db.AIConversationRecord, error) {
	return withLock(a, func() (*db.AIConversationRecord, error) {
		return a.aiChatService.CreateConversation(a.ctx, title, serverID, systemPrompt, provider, model)
	})
}

func (a *App) AIListConversations() ([]*db.AIConversationRecord, error) {
	return withLock(a, func() ([]*db.AIConversationRecord, error) {
		return a.aiChatService.ListConversations(a.ctx)
	})
}

func (a *App) AIGetConversation(id string) (*db.AIConversationRecord, error) {
	return withLock(a, func() (*db.AIConversationRecord, error) {
		return a.aiChatService.GetConversation(a.ctx, id)
	})
}

func (a *App) AIDeleteConversation(id string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.aiChatService.DeleteConversation(a.ctx, id)
}

func (a *App) AIListMessages(convID string) ([]*db.AIMessageRecord, error) {
	return withLock(a, func() ([]*db.AIMessageRecord, error) {
		return a.aiChatService.ListMessages(a.ctx, convID)
	})
}

// AIStreamMessage initiates an asynchronous streaming chat generation with Wails events.
func (a *App) AIStreamMessage(convID string, userPrompt string) error {
	return a.AIStreamMessageWithContext(convID, userPrompt, nil)
}

// AIStreamMessageWithContext initiates streaming chat generation with optional UI screen context.
func (a *App) AIStreamMessageWithContext(convID string, userPrompt string, uiCtx *aiContext.UIContext) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	if _, loaded := a.aiStreamingConvs.LoadOrStore(convID, struct{}{}); loaded {
		return fmt.Errorf("генерация ответа для этого диалога уже выполняется")
	}

	go func() {
		defer a.aiStreamingConvs.Delete(convID)

		var errEmitted bool
		_, err := a.aiChatService.StreamMessageWithContext(a.ctx, convID, userPrompt, uiCtx, func(chunk domain.StreamChunk) {
			if chunk.Error != nil {
				errEmitted = true
				wailsRuntime.EventsEmit(a.ctx, "ai:stream:error", map[string]any{
					"conversation_id": convID,
					"error":           chunk.Error.Error(),
				})
				return
			}

			wailsRuntime.EventsEmit(a.ctx, "ai:stream:chunk", map[string]any{
				"conversation_id": convID,
				"content_delta":   chunk.ContentDelta,
				"tool_call_delta": chunk.ToolCallDelta,
				"finish_reason":   chunk.FinishReason,
				"usage":           chunk.Usage,
			})
		})

		if err != nil {
			if !errEmitted {
				wailsRuntime.EventsEmit(a.ctx, "ai:stream:error", map[string]any{
					"conversation_id": convID,
					"error":           err.Error(),
				})
			}
			return
		}

		wailsRuntime.EventsEmit(a.ctx, "ai:stream:complete", map[string]any{
			"conversation_id": convID,
		})
	}()

	return nil
}

func (a *App) AIRouteModel(prompt string, policy string) (*router.RouteDecision, error) {
	return withLock(a, func() (*router.RouteDecision, error) {
		return a.aiChatService.RouteModel(a.ctx, prompt, policy)
	})
}

func (a *App) AIListTools() []domain.ToolDefinition {
	if err := a.requireUnlocked(); err != nil {
		return nil
	}
	return a.aiChatService.ListTools(a.ctx)
}

func (a *App) AIGetDisabledTools() []string {
	if err := a.requireUnlocked(); err != nil {
		return nil
	}
	return a.aiChatService.GetDisabledTools(a.ctx)
}

func (a *App) AISetDisabledTools(disabled []string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.aiChatService.SetDisabledTools(a.ctx, disabled)
}

func (a *App) AISaveMemoryFact(serverID *string, category, key, value string) (*db.AIMemoryFactRecord, error) {
	return withLock(a, func() (*db.AIMemoryFactRecord, error) {
		return a.aiMemoryService.SaveFact(a.ctx, serverID, category, key, value, "user")
	})
}

func (a *App) AIListMemoryFacts(serverID *string) ([]*db.AIMemoryFactRecord, error) {
	return withLock(a, func() ([]*db.AIMemoryFactRecord, error) {
		return a.aiMemoryService.ListFacts(a.ctx, serverID)
	})
}

func (a *App) AISearchMemoryFacts(query string, serverID *string, limit int) ([]*db.AIMemoryFactRecord, error) {
	return withLock(a, func() ([]*db.AIMemoryFactRecord, error) {
		return a.aiMemoryService.SearchFacts(a.ctx, query, serverID, limit)
	})
}

func (a *App) AIApproveTask(taskID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.engine.Approve(taskID)
}

func (a *App) AIRejectTask(taskID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.engine.Reject(taskID)
}

func (a *App) AIRunSiteDiagnostics(serverID string, siteIDOrDomain string) (*workflows.DiagnosticReport, error) {
	return withLock(a, func() (*workflows.DiagnosticReport, error) {
		return a.siteDiagWf.Diagnose(a.ctx, serverID, siteIDOrDomain)
	})
}

func (a *App) AIRunSecurityAudit(serverID string) (*workflows.DiagnosticReport, error) {
	return withLock(a, func() (*workflows.DiagnosticReport, error) {
		return a.secAuditWf.Audit(a.ctx, serverID)
	})
}

func (a *App) AIRunConfigOptimizer(serverID string) (*workflows.DiagnosticReport, error) {
	return withLock(a, func() (*workflows.DiagnosticReport, error) {
		return a.configOptWf.Optimize(a.ctx, serverID)
	})
}

func (a *App) AIListAgents() []agents.AgentStatus {
	if err := a.requireUnlocked(); err != nil {
		return nil
	}
	return a.aiAgentSupervisor.GetStatuses()
}

func (a *App) AIToggleAgent(agentID string, enabled bool) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.aiAgentSupervisor.ToggleAgent(agentID, enabled)
}

func (a *App) AITriggerAgent(agentID string) ([]agents.AgentFinding, error) {
	return withLock(a, func() ([]agents.AgentFinding, error) {
		return a.aiAgentSupervisor.TriggerAgent(a.ctx, agentID)
	})
}

func (a *App) AIListAgentFindings(limit int) []agents.AgentFinding {
	if err := a.requireUnlocked(); err != nil {
		return nil
	}
	return a.aiAgentSupervisor.GetFindings(limit)
}

func (a *App) AIClearFacts(serverID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	var sID *string
	if serverID != "" {
		sID = &serverID
	}
	return a.database.DeleteAIMemoryFacts(a.ctx, sID)
}
