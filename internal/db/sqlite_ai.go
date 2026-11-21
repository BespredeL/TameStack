package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AIProviderConfigRecord represents a configured AI provider in SQLite.
type AIProviderConfigRecord struct {
	ID              string    `json:"id"`
	ProviderType    string    `json:"provider_type"` // 'ollama', 'openai_compatible', 'llamacpp', 'openai', 'gemini', 'anthropic'
	Name            string    `json:"name"`
	BaseURL         string    `json:"base_url"`
	APIKeyEncrypted string    `json:"-"`
	IsEnabled       bool      `json:"is_enabled"`
	IsDefault       bool      `json:"is_default"`
	DefaultModel    string    `json:"default_model"`
	SystemPrompt    string    `json:"system_prompt"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AIConversationRecord represents a chat session.
type AIConversationRecord struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	ServerID         *string   `json:"server_id"`
	SystemPrompt     string    `json:"system_prompt"`
	SelectedProvider string    `json:"selected_provider"`
	SelectedModel    string    `json:"selected_model"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// AIMessageRecord represents a single message in a chat conversation.
type AIMessageRecord struct {
	ID               string    `json:"id"`
	ConversationID   string    `json:"conversation_id"`
	Role             string    `json:"role"` // 'system', 'user', 'assistant', 'tool'
	Content          string    `json:"content"`
	ToolCallsJSON    string    `json:"tool_calls_json"`
	ToolCallID       string    `json:"tool_call_id"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CreatedAt        time.Time `json:"created_at"`
}

// AIMemoryFactRecord represents a persistent piece of knowledge about infrastructure.
type AIMemoryFactRecord struct {
	ID        string    `json:"id"`
	ServerID  *string   `json:"server_id"`
	Category  string    `json:"category"` // 'os', 'web', 'db', 'security', 'preference'
	FactKey   string    `json:"fact_key"`
	FactValue string    `json:"fact_value"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AIToolAuditRecord logs any tool invocation made by or via AI.
type AIToolAuditRecord struct {
	ID                  string    `json:"id"`
	ConversationID      string    `json:"conversation_id"`
	MessageID           string    `json:"message_id"`
	ToolName            string    `json:"tool_name"`
	ArgumentsJSON       string    `json:"arguments_json"`
	RiskLevel           string    `json:"risk_level"`
	Approved            bool      `json:"approved"`
	ExecutedAt          time.Time `json:"executed_at"`
	ExecutionDurationMs int64     `json:"execution_duration_ms"`
	Success             bool      `json:"success"`
	ErrorMessage        string    `json:"error_message"`
}

func (d *DB) migrateAI() error {
	return nil
}

// --- AI Provider Configs Repository ---

func (d *DB) ListAIProviderConfigs(ctx context.Context) ([]*AIProviderConfigRecord, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, provider_type, name, base_url, api_key_encrypted, is_enabled, is_default, default_model, system_prompt, created_at, updated_at
		FROM ai_provider_configs ORDER BY is_default DESC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*AIProviderConfigRecord
	for rows.Next() {
		c := &AIProviderConfigRecord{}
		var enabledInt, defaultInt int
		if err := rows.Scan(&c.ID, &c.ProviderType, &c.Name, &c.BaseURL, &c.APIKeyEncrypted, &enabledInt, &defaultInt, &c.DefaultModel, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.IsEnabled = enabledInt == 1
		c.IsDefault = defaultInt == 1
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (d *DB) GetAIProviderConfig(ctx context.Context, id string) (*AIProviderConfigRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, provider_type, name, base_url, api_key_encrypted, is_enabled, is_default, default_model, system_prompt, created_at, updated_at
		FROM ai_provider_configs WHERE id = ?
	`, id)

	c := &AIProviderConfigRecord{}
	var enabledInt, defaultInt int
	if err := row.Scan(&c.ID, &c.ProviderType, &c.Name, &c.BaseURL, &c.APIKeyEncrypted, &enabledInt, &defaultInt, &c.DefaultModel, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	c.IsEnabled = enabledInt == 1
	c.IsDefault = defaultInt == 1
	return c, nil
}

func (d *DB) GetAIProviderConfigByType(ctx context.Context, pType string) (*AIProviderConfigRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, provider_type, name, base_url, api_key_encrypted, is_enabled, is_default, default_model, system_prompt, created_at, updated_at
		FROM ai_provider_configs WHERE provider_type = ? AND is_enabled = 1 ORDER BY is_default DESC LIMIT 1
	`, pType)

	c := &AIProviderConfigRecord{}
	var enabledInt, defaultInt int
	if err := row.Scan(&c.ID, &c.ProviderType, &c.Name, &c.BaseURL, &c.APIKeyEncrypted, &enabledInt, &defaultInt, &c.DefaultModel, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	c.IsEnabled = enabledInt == 1
	c.IsDefault = defaultInt == 1
	return c, nil
}

func (d *DB) SaveAIProviderConfig(ctx context.Context, c *AIProviderConfigRecord) error {
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now

	enabledInt := 0
	if c.IsEnabled {
		enabledInt = 1
	}
	defaultInt := 0
	if c.IsDefault {
		defaultInt = 1
		// If marking default, unmark other defaults.
		_, _ = d.ExecContext(ctx, `UPDATE ai_provider_configs SET is_default = 0 WHERE id != ?`, c.ID)
	}

	_, err := d.ExecContext(ctx, `
		INSERT INTO ai_provider_configs (id, provider_type, name, base_url, api_key_encrypted, is_enabled, is_default, default_model, system_prompt, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider_type = excluded.provider_type,
			name = excluded.name,
			base_url = excluded.base_url,
			api_key_encrypted = excluded.api_key_encrypted,
			is_enabled = excluded.is_enabled,
			is_default = excluded.is_default,
			default_model = excluded.default_model,
			system_prompt = excluded.system_prompt,
			updated_at = excluded.updated_at
	`, c.ID, c.ProviderType, c.Name, c.BaseURL, c.APIKeyEncrypted, enabledInt, defaultInt, c.DefaultModel, c.SystemPrompt, c.CreatedAt, c.UpdatedAt)
	return err
}

func (d *DB) DeleteAIProviderConfig(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM ai_provider_configs WHERE id = ?`, id)
	return err
}

// --- AI Global Settings Repository ---

func (d *DB) GetAIGlobalSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := d.QueryRowContext(ctx, `SELECT value FROM ai_global_settings WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (d *DB) SetAIGlobalSetting(ctx context.Context, key, value string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO ai_global_settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}

func (d *DB) GetAllAIGlobalSettings(ctx context.Context) (map[string]string, error) {
	rows, err := d.QueryContext(ctx, `SELECT key, value FROM ai_global_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			res[k] = v
		}
	}
	return res, rows.Err()
}

// --- AI Conversations Repository ---

func (d *DB) ListAIConversations(ctx context.Context) ([]*AIConversationRecord, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, title, server_id, system_prompt, selected_provider, selected_model, created_at, updated_at
		FROM ai_conversations ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []*AIConversationRecord
	for rows.Next() {
		c := &AIConversationRecord{}
		if err := rows.Scan(&c.ID, &c.Title, &c.ServerID, &c.SystemPrompt, &c.SelectedProvider, &c.SelectedModel, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

func (d *DB) GetAIConversation(ctx context.Context, id string) (*AIConversationRecord, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, title, server_id, system_prompt, selected_provider, selected_model, created_at, updated_at
		FROM ai_conversations WHERE id = ?
	`, id)

	c := &AIConversationRecord{}
	if err := row.Scan(&c.ID, &c.Title, &c.ServerID, &c.SystemPrompt, &c.SelectedProvider, &c.SelectedModel, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (d *DB) SaveAIConversation(ctx context.Context, c *AIConversationRecord) error {
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now

	_, err := d.ExecContext(ctx, `
		INSERT INTO ai_conversations (id, title, server_id, system_prompt, selected_provider, selected_model, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			server_id = excluded.server_id,
			system_prompt = excluded.system_prompt,
			selected_provider = excluded.selected_provider,
			selected_model = excluded.selected_model,
			updated_at = excluded.updated_at
	`, c.ID, c.Title, c.ServerID, c.SystemPrompt, c.SelectedProvider, c.SelectedModel, c.CreatedAt, c.UpdatedAt)
	return err
}

func (d *DB) DeleteAIConversation(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM ai_conversations WHERE id = ?`, id)
	return err
}

// --- AI Messages Repository ---

func (d *DB) ListAIMessages(ctx context.Context, convID string) ([]*AIMessageRecord, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, conversation_id, role, content, tool_calls_json, tool_call_id, prompt_tokens, completion_tokens, created_at
		FROM ai_messages WHERE conversation_id = ? ORDER BY created_at ASC
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*AIMessageRecord
	for rows.Next() {
		m := &AIMessageRecord{}
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolCallsJSON, &m.ToolCallID, &m.PromptTokens, &m.CompletionTokens, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (d *DB) SaveAIMessage(ctx context.Context, m *AIMessageRecord) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	if m.ToolCallsJSON == "" {
		m.ToolCallsJSON = "[]"
	}

	_, err := d.ExecContext(ctx, `
		INSERT INTO ai_messages (id, conversation_id, role, content, tool_calls_json, tool_call_id, prompt_tokens, completion_tokens, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.ConversationID, m.Role, m.Content, m.ToolCallsJSON, m.ToolCallID, m.PromptTokens, m.CompletionTokens, m.CreatedAt)
	if err != nil {
		return err
	}

	// Update conversation's updated_at timestamp.
	_, _ = d.ExecContext(ctx, `UPDATE ai_conversations SET updated_at = ? WHERE id = ?`, m.CreatedAt, m.ConversationID)
	return nil
}

// --- AI Memory Facts & FTS Repository ---

func (d *DB) SaveAIMemoryFact(ctx context.Context, f *AIMemoryFactRecord) error {
	now := time.Now()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	f.UpdatedAt = now

	_, err := d.ExecContext(ctx, `
		INSERT INTO ai_memory_facts (id, server_id, category, fact_key, fact_value, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			category = excluded.category,
			fact_key = excluded.fact_key,
			fact_value = excluded.fact_value,
			source = excluded.source,
			updated_at = excluded.updated_at
	`, f.ID, f.ServerID, f.Category, f.FactKey, f.FactValue, f.Source, f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return err
	}

	// Update FTS index.
	_, _ = d.ExecContext(ctx, `DELETE FROM ai_memory_fts WHERE fact_id = ?`, f.ID)
	factText := fmt.Sprintf("[%s] %s: %s", f.Category, f.FactKey, f.FactValue)
	_, _ = d.ExecContext(ctx, `INSERT INTO ai_memory_fts (fact_id, fact_text) VALUES (?, ?)`, f.ID, factText)

	return nil
}

func (d *DB) ListAIMemoryFacts(ctx context.Context, serverID *string) ([]*AIMemoryFactRecord, error) {
	var query string
	var args []any
	if serverID != nil && *serverID != "" {
		query = `SELECT id, server_id, category, fact_key, fact_value, source, created_at, updated_at FROM ai_memory_facts WHERE server_id = ? OR server_id IS NULL ORDER BY updated_at DESC`
		args = append(args, *serverID)
	} else {
		query = `SELECT id, server_id, category, fact_key, fact_value, source, created_at, updated_at FROM ai_memory_facts ORDER BY updated_at DESC`
	}

	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []*AIMemoryFactRecord
	for rows.Next() {
		f := &AIMemoryFactRecord{}
		if err := rows.Scan(&f.ID, &f.ServerID, &f.Category, &f.FactKey, &f.FactValue, &f.Source, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

func (d *DB) SearchAIMemory(ctx context.Context, query string, serverID *string, limit int) ([]*AIMemoryFactRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	// FTS match query.
	ftsRows, err := d.QueryContext(ctx, `
		SELECT fact_id FROM ai_memory_fts WHERE ai_memory_fts MATCH ? ORDER BY rank LIMIT ?
	`, query, limit*2)
	if err != nil {
		return nil, err
	}
	defer ftsRows.Close()

	var factIDs []string
	for ftsRows.Next() {
		var id string
		if err := ftsRows.Scan(&id); err == nil {
			factIDs = append(factIDs, id)
		}
	}

	if len(factIDs) == 0 {
		return []*AIMemoryFactRecord{}, nil
	}

	var results []*AIMemoryFactRecord
	for _, id := range factIDs {
		row := d.QueryRowContext(ctx, `
			SELECT id, server_id, category, fact_key, fact_value, source, created_at, updated_at
			FROM ai_memory_facts WHERE id = ?
		`, id)
		f := &AIMemoryFactRecord{}
		if err := row.Scan(&f.ID, &f.ServerID, &f.Category, &f.FactKey, &f.FactValue, &f.Source, &f.CreatedAt, &f.UpdatedAt); err == nil {
			if serverID == nil || *serverID == "" || f.ServerID == nil || *f.ServerID == *serverID {
				results = append(results, f)
			}
		}
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (d *DB) CountAIMemoryFacts(ctx context.Context) (int, error) {
	var count int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_memory_facts`).Scan(&count)
	return count, err
}

func (d *DB) DeleteAIMemoryFacts(ctx context.Context, serverID *string) error {
	ftsQuery := `DELETE FROM ai_memory_fts`
	factsQuery := `DELETE FROM ai_memory_facts`
	var args []any
	if serverID != nil && *serverID != "" {
		ftsQuery += ` WHERE fact_id IN (SELECT id FROM ai_memory_facts WHERE server_id = ?)`
		factsQuery += ` WHERE server_id = ?`
		args = []any{*serverID}
	}
	return d.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, ftsQuery, args...); err != nil {
			return fmt.Errorf("failed to delete memory facts search index: %w", err)
		}
		if _, err := tx.ExecContext(ctx, factsQuery, args...); err != nil {
			return fmt.Errorf("failed to delete memory facts: %w", err)
		}
		return nil
	})
}

// --- AI Tool Audit Repository ---

func (d *DB) SaveAIToolAudit(ctx context.Context, a *AIToolAuditRecord) error {
	if a.ExecutedAt.IsZero() {
		a.ExecutedAt = time.Now()
	}
	apprInt := 0
	if a.Approved {
		apprInt = 1
	}
	succInt := 0
	if a.Success {
		succInt = 1
	}

	_, err := d.ExecContext(ctx, `
		INSERT INTO ai_tool_audit (id, conversation_id, message_id, tool_name, arguments_json, risk_level, approved, executed_at, execution_duration_ms, success, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.ID, a.ConversationID, a.MessageID, a.ToolName, a.ArgumentsJSON, a.RiskLevel, apprInt, a.ExecutedAt, a.ExecutionDurationMs, succInt, a.ErrorMessage)
	return err
}

func (d *DB) ListAIToolAudits(ctx context.Context, convID string) ([]*AIToolAuditRecord, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, conversation_id, message_id, tool_name, arguments_json, risk_level, approved, executed_at, execution_duration_ms, success, error_message
		FROM ai_tool_audit WHERE conversation_id = ? ORDER BY executed_at ASC
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var audits []*AIToolAuditRecord
	for rows.Next() {
		a := &AIToolAuditRecord{}
		var apprInt, succInt int
		if err := rows.Scan(&a.ID, &a.ConversationID, &a.MessageID, &a.ToolName, &a.ArgumentsJSON, &a.RiskLevel, &apprInt, &a.ExecutedAt, &a.ExecutionDurationMs, &succInt, &a.ErrorMessage); err != nil {
			return nil, err
		}
		a.Approved = apprInt == 1
		a.Success = succInt == 1
		audits = append(audits, a)
	}
	return audits, rows.Err()
}
