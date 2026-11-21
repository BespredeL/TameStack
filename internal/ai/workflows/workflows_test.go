package workflows

import (
	"context"
	"tamestack/internal/testutil"
	"testing"

	"tamestack/internal/db"
)

func TestWorkflowTypesAndReport(t *testing.T) {
	report := DiagnosticReport{
		WorkflowType: "site_outage",
		ServerID:     "srv-1",
		ServerName:   "Prod-VPS",
		Target:       "example.com",
		Status:       "critical",
		Summary:      "Обнаружена критическая ошибка",
		Issues: []Issue{
			{
				ID:             "http-502",
				Severity:       SeverityCritical,
				Component:      "webserver",
				Title:          "502 Bad Gateway",
				Description:    "PHP-FPM socket not responding",
				Recommendation: "Restart php-fpm",
				ProposedAction: "restart_service",
				ActionParams:   map[string]any{"service_name": "php8.2-fpm"},
			},
			{
				ID:             "opt-nginx",
				Severity:       SeverityInfo,
				Component:      "webserver",
				Title:          "Enable gzip",
				Description:    "Gzip is off",
				Recommendation: "Enable gzip on in nginx.conf",
			},
		},
		Metrics: map[string]any{
			"status_code": 502,
			"php_version": "8.2",
		},
	}

	if report.WorkflowType != "site_outage" {
		t.Fatalf("expected site_outage, got %s", report.WorkflowType)
	}
	if len(report.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(report.Issues))
	}
	if report.Issues[0].Severity != SeverityCritical {
		t.Fatalf("expected critical severity, got %s", report.Issues[0].Severity)
	}
	if report.Issues[0].ProposedAction != "restart_service" {
		t.Fatalf("expected restart_service action, got %s", report.Issues[0].ProposedAction)
	}
}

func TestWorkflowMockServerMissing(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(tmpDir + "/test_wf.db")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer database.Close()

	wf := NewSiteDiagnosticsWorkflow(database, nil, nil, nil, nil, "")
	_, err = wf.Diagnose(context.Background(), "non-existent-server", "example.com")
	if err == nil {
		t.Fatalf("expected error for non-existent server, got nil")
	}

	secWf := NewSecurityAuditWorkflow(database, nil, nil, nil, "")
	_, err = secWf.Audit(context.Background(), "non-existent-server")
	if err == nil {
		t.Fatalf("expected error for non-existent server in sec audit, got nil")
	}

	optWf := NewConfigOptimizerWorkflow(database, nil, nil, nil, "")
	_, err = optWf.Optimize(context.Background(), "non-existent-server")
	if err == nil {
		t.Fatalf("expected error for non-existent server in config opt, got nil")
	}
}
