package context

import (
	"context"
	"path/filepath"
	"tamestack/internal/testutil"
	"strings"
	"testing"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
)

func TestSanitizer(t *testing.T) {
	s := NewSanitizer()

	// 1. Private Key Redaction.
	rawKey := `Host key loaded:
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0Yq3...SECRET_KEY_DATA...
-----END RSA PRIVATE KEY-----
Ready.`
	sanitized := s.Sanitize(rawKey)
	if strings.Contains(sanitized, "SECRET_KEY_DATA") {
		t.Fatalf("private key was not redacted: %s", sanitized)
	}
	if !strings.Contains(sanitized, "[REDACTED_PRIVATE_KEY]") {
		t.Fatalf("missing redaction placeholder: %s", sanitized)
	}

	// 2. Key-Value Passwords.
	configSnippet := `
[mysqld]
user = mysql
password = SuperSecretDatabasePassword123!
bind-address = 127.0.0.1
`
	sanitizedConf := s.Sanitize(configSnippet)
	if strings.Contains(sanitizedConf, "SuperSecretDatabasePassword123!") {
		t.Fatalf("database password was not sanitized: %s", sanitizedConf)
	}
	if !strings.Contains(sanitizedConf, `"[REDACTED_SECRET]"`) {
		t.Fatalf("missing redacted secret placeholder: %s", sanitizedConf)
	}

	// 3. JWT and Bearer Token.
	apiHeader := `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c`
	sanitizedAuth := s.Sanitize(apiHeader)
	if strings.Contains(sanitizedAuth, "SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c") {
		t.Fatalf("jwt signature was not sanitized: %s", sanitizedAuth)
	}

	// 4. Connection string.
	connStr := `DATABASE_URL=mysql://dbadmin:UltraP@ssw0rd!@192.168.1.50:3306/production_db`
	sanitizedConn := s.Sanitize(connStr)
	if strings.Contains(sanitizedConn, "UltraP@ssw0rd!") {
		t.Fatalf("connection string password was not sanitized: %s", sanitizedConn)
	}
}

func TestContextBuilder(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_ctx.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Seed server
	srv := &db.ServerRecord{
		ID:        "srv-test",
		Name:      "Web Alpha",
		Host:      "10.0.0.1",
		Port:      22,
		Username:  "ubuntu",
		Status:    "online",
		GroupName: "Production",
	}
	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	// Seed diagnostics
	diag := &db.DiagnosticsRecord{
		ServerID:            "srv-test",
		OSName:              "Ubuntu",
		OSVersion:           "24.04 LTS",
		OSFamily:            "debian",
		KernelVersion:       "6.8.0-generic",
		Arch:                "x86_64",
		CPUCores:            4,
		CPUModel:            "Intel Xeon",
		RAMTotalBytes:       8 * 1024 * 1024 * 1024,
		DiskTotalBytes:      50 * 1024 * 1024 * 1024,
		DiskUsedBytes:       10 * 1024 * 1024 * 1024,
		InstalledWebServers: []string{"nginx", "apache"},
	}
	if err := database.SaveDiagnostics(ctx, diag); err != nil {
		t.Fatalf("failed to save diagnostics: %v", err)
	}

	// Build context
	builder := NewContextBuilder(database, nil)
	srvID := "srv-test"
	prompt, err := builder.BuildSystemPrompt(ctx, BuilderOptions{
		ServerID:      &srvID,
		IncludeMemory: true,
	})
	if err != nil {
		t.Fatalf("failed to build system prompt: %v", err)
	}

	if !strings.Contains(prompt, "Web Alpha") || !strings.Contains(prompt, "Ubuntu 24.04 LTS") {
		t.Fatalf("prompt missing server metadata: %s", prompt)
	}
	if !strings.Contains(prompt, "nginx, apache") {
		t.Fatalf("prompt missing webservers: %s", prompt)
	}
	if !strings.Contains(prompt, "Ты — TameStack AI Assistant") {
		t.Fatalf("expected Russian default persona, got: %s", prompt)
	}

	// Build English context
	promptEN, err := builder.BuildSystemPrompt(ctx, BuilderOptions{
		ServerID:       &srvID,
		IncludeMemory:  true,
		LanguageLocale: "en",
	})
	if err != nil {
		t.Fatalf("failed to build English system prompt: %v", err)
	}
	if !strings.Contains(promptEN, "You are TameStack AI Assistant") {
		t.Fatalf("expected English persona, got: %s", promptEN)
	}
	if !strings.Contains(promptEN, "### Infrastructure Context:") {
		t.Fatalf("expected English infrastructure context header, got: %s", promptEN)
	}
	if !strings.Contains(promptEN, "### Active Server: Web Alpha") {
		t.Fatalf("expected English active server header, got: %s", promptEN)
	}

	// Verify dynamic tools rendering
	promptWithTools, err := builder.BuildSystemPrompt(ctx, BuilderOptions{
		ServerID:       &srvID,
		LanguageLocale: "en",
		AvailableTools: []domain.ToolDefinition{
			{Name: "inspect_mail_server", Description: "Inspects mail stack", IsMutating: false},
			{Name: "prepare_config_diff", Description: "Prepares config diff", IsMutating: true},
		},
	})
	if err != nil {
		t.Fatalf("failed to build prompt with tools: %v", err)
	}
	if !strings.Contains(promptWithTools, "### AVAILABLE TOOLS:") ||
		!strings.Contains(promptWithTools, "`inspect_mail_server` [read-only]") ||
		!strings.Contains(promptWithTools, "`prepare_config_diff` [requires approval]") {
		t.Fatalf("expected dynamic tools section in prompt, got: %s", promptWithTools)
	}
}
