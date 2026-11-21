package policies

import (
	"strings"
	"testing"

	"tamestack/internal/ai/domain"
)

func TestSecurityGate(t *testing.T) {
	gate := NewSecurityGate()

	// 1. Forbidden Paths.
	forbidden := []string{
		"/boot/vmlinuz",
		"/etc/shadow",
		"/etc/sudoers",
		"/bin/bash",
		"/root/.ssh/id_rsa",
		"/usr/bin/python",
	}

	for _, path := range forbidden {
		if err := gate.ValidatePath(path); err == nil {
			t.Errorf("expected error for forbidden path '%s', got nil", path)
		}
	}

	// 2. Allowed Paths.
	allowed := []string{
		"/etc/nginx/nginx.conf",
		"/etc/nginx/sites-available/my-site.conf",
		"/etc/apache2/sites-available/000-default.conf",
		"/etc/mysql/my.cnf",
		"/etc/php/8.2/fpm/pool.d/www.conf",
		"/var/www/html/index.php",
	}

	rejectedPrefix := []string{"/etc/php.ini", "/etc/phpmyadmin", "/etc/nginx.conf", "/etc/apache2.conf"}
	for _, path := range rejectedPrefix {
		if err := gate.ValidatePath(path); err == nil {
			t.Errorf("expected path %q to be rejected", path)
		}
	}

	for _, path := range allowed {
		if err := gate.ValidatePath(path); err != nil {
			t.Errorf("expected path '%s' to be allowed, got error: %v", path, err)
		}
	}

	// 3. Human Approval Requirement.
	safeTool := domain.ToolDefinition{Name: "get_status", RiskLevel: domain.RiskSafe, IsMutating: false}
	if gate.RequiresHumanApproval(safeTool) {
		t.Errorf("safe read-only tool should not require human approval")
	}

	mutatingTool := domain.ToolDefinition{Name: "restart_nginx", RiskLevel: domain.RiskModerate, IsMutating: true}
	if !gate.RequiresHumanApproval(mutatingTool) {
		t.Errorf("mutating tool must require human approval")
	}

	highRiskTool := domain.ToolDefinition{Name: "apply_diff", RiskLevel: domain.RiskHigh, IsMutating: false}
	if !gate.RequiresHumanApproval(highRiskTool) {
		t.Errorf("high risk tool must require human approval")
	}

	// Read-only labels alone are not enough: only an explicit safe classification runs unattended.
	unlabelled := domain.ToolDefinition{Name: "mystery", IsMutating: false}
	if !gate.RequiresHumanApproval(unlabelled) {
		t.Errorf("tool without a risk classification must require human approval")
	}
	moderateReadOnly := domain.ToolDefinition{Name: "query", RiskLevel: domain.RiskModerate, IsMutating: false}
	if !gate.RequiresHumanApproval(moderateReadOnly) {
		t.Errorf("moderate risk tool must require human approval")
	}

	// Proposal tools only create a requires-approval task; the mutation itself is
	// gated by the user's task approval, so they may auto-run in the chat loop.
	proposal := domain.ToolDefinition{Name: "prepare_config_diff", RiskLevel: domain.RiskHigh, IsMutating: true, ApprovalViaTask: true}
	if gate.RequiresHumanApproval(proposal) {
		t.Errorf("proposal tool gated by task approval must not require chat-level approval")
	}
}

func TestSecurityGateRejectsShellReinterpretation(t *testing.T) {
	gate := NewSecurityGate()

	rejected := []string{
		"",
		"/etc/nginx/$(id).conf",
		"/etc/nginx/`id`.conf",
		"/etc/nginx/nginx.conf; rm -rf /",
		"/etc/nginx/nginx.conf | tee /etc/shadow",
		"/etc/nginx/conf.d/*.conf",
		"/etc/nginx/nginx.conf\n/etc/shadow",
		"~/.ssh/config",
		"/etc/nginx/../nginx.conf",
		"/etc/nginx//nginx.conf",
		"/etc/nginx/./nginx.conf",
		"nginx.conf",
	}

	for _, path := range rejected {
		if err := gate.ValidatePath(path); err == nil {
			t.Errorf("ValidatePath(%q) accepted", path)
		}
	}

	// Trailing slashes are tolerated, everything else must stay canonical.
	if err := gate.ValidatePath("/etc/nginx/sites-available/"); err != nil {
		t.Errorf("ValidatePath rejected a canonical directory: %v", err)
	}
}

func TestUnifiedDiffAndPlan(t *testing.T) {
	oldContent := `server {
    listen 80;
    server_name example.com;
}`

	newContent := `server {
    listen 80;
    listen 443 ssl;
    server_name example.com;
}`

	diff := ComputeUnifiedDiff("example.conf", oldContent, newContent)
	if !strings.Contains(diff, "--- a/example.conf") || !strings.Contains(diff, "+++ b/example.conf") {
		t.Fatalf("diff missing standard header: %s", diff)
	}
	if !strings.Contains(diff, "+    listen 443 ssl;") {
		t.Fatalf("diff missing added line: %s", diff)
	}

	plan, err := GenerateConfigChangePlan("srv-1", "/etc/nginx/sites-available/example.conf", oldContent, newContent, "nginx")
	if err != nil {
		t.Fatalf("plan generation failed: %v", err)
	}
	if plan.RiskLevel != domain.RiskHigh {
		t.Fatalf("expected RiskHigh, got %s", plan.RiskLevel)
	}
	if len(plan.Steps) < 3 {
		t.Fatalf("expected at least 3 execution steps, got %d", len(plan.Steps))
	}
	if !strings.Contains(plan.BackupPath, ".bak_") {
		t.Fatalf("backup path missing timestamp format: %s", plan.BackupPath)
	}
}
