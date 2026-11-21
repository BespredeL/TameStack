package agents

import (
	"context"
	"time"

	"tamestack/internal/ai/workflows"
)

// Agent defines the uniform contract for scheduled autonomous inspection workers.
type Agent interface {
	ID() string
	Name() string
	Description() string
	DefaultInterval() time.Duration
	Run(ctx context.Context) ([]AgentFinding, error)
}

// AgentFinding represents a detected anomaly or infrastructure warning.
type AgentFinding struct {
	ID        string             `json:"id"`
	AgentID   string             `json:"agent_id"`
	ServerID  string             `json:"server_id"`
	Severity  workflows.Severity `json:"severity"` // "critical", "warning", "info"
	Title     string             `json:"title"`
	Details   string             `json:"details"`
	CreatedAt time.Time          `json:"created_at"`
}

// AgentStatus exposes runtime observability of an autonomous agent.
type AgentStatus struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Enabled         bool           `json:"enabled"`
	IntervalSeconds int            `json:"interval_seconds"`
	LastRunAt       *time.Time     `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time     `json:"next_run_at,omitempty"`
	LastStatus      string         `json:"last_status"` // "idle", "running", "healthy", "warning", "error"
	LastError       string         `json:"last_error,omitempty"`
	FindingsCount   int            `json:"findings_count"`
	RecentFindings  []AgentFinding `json:"recent_findings"`
}
