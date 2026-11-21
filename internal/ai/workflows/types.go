package workflows

import "time"

// Severity defines the criticality level of an identified issue.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Issue describes a detected infrastructure or configuration problem.
type Issue struct {
	ID             string         `json:"id"`
	Severity       Severity       `json:"severity"`
	Component      string         `json:"component"` // "webserver", "php", "dns", "ssl", "database", "security", "resources"
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Recommendation string         `json:"recommendation"`
	ProposedAction string         `json:"proposed_action,omitempty"` // "restart_service", "apply_diff", "enable_firewall", etc.
	ActionParams   map[string]any `json:"action_params,omitempty"`
}

// DiagnosticReport is the structured output of an automated workflow analysis.
type DiagnosticReport struct {
	WorkflowType string         `json:"workflow_type"` // "site_outage", "security_audit", "config_optimizer"
	ServerID     string         `json:"server_id"`
	ServerName   string         `json:"server_name,omitempty"`
	Target       string         `json:"target,omitempty"` // site domain, server host, etc.
	Status       string         `json:"status"`           // "healthy", "warning", "critical", "error"
	Summary      string         `json:"summary"`
	Timestamp    time.Time      `json:"timestamp"`
	Issues       []Issue        `json:"issues"`
	Metrics      map[string]any `json:"metrics,omitempty"`
}
