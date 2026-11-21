package agents

import (
	"context"
	"testing"
	"time"

	"tamestack/internal/ai/workflows"
)

type mockAgent struct {
	id       string
	name     string
	interval time.Duration
	runCount int
	findings []AgentFinding
}

func (m *mockAgent) ID() string                     { return m.id }
func (m *mockAgent) Name() string                   { return m.name }
func (m *mockAgent) Description() string            { return "Mock test agent" }
func (m *mockAgent) DefaultInterval() time.Duration { return m.interval }
func (m *mockAgent) Run(ctx context.Context) ([]AgentFinding, error) {
	m.runCount++
	return m.findings, nil
}

func TestSupervisorLifecycleAndTrigger(t *testing.T) {
	sup := NewSupervisor(nil)

	agent1 := &mockAgent{
		id:       "agent_test_1",
		name:     "Test Agent 1",
		interval: 1 * time.Hour,
		findings: []AgentFinding{
			{
				ID:        "f-1",
				AgentID:   "agent_test_1",
				ServerID:  "srv-1",
				Severity:  workflows.SeverityCritical,
				Title:     "Test Critical Anomaly",
				Details:   "Detailed test anomaly description",
				CreatedAt: time.Now().UTC(),
			},
		},
	}

	sup.Register(agent1)

	statuses := sup.GetStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 agent status, got %d", len(statuses))
	}
	if statuses[0].ID != "agent_test_1" {
		t.Fatalf("expected agent_test_1, got %s", statuses[0].ID)
	}
	if !statuses[0].Enabled {
		t.Fatalf("expected agent to be enabled by default")
	}

	// Manual trigger
	ctx := context.Background()
	findings, err := sup.TriggerAgent(ctx, "agent_test_1")
	if err != nil {
		t.Fatalf("TriggerAgent returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if agent1.runCount != 1 {
		t.Fatalf("expected runCount=1, got %d", agent1.runCount)
	}

	allFindings := sup.GetFindings(10)
	if len(allFindings) != 1 {
		t.Fatalf("expected 1 global finding, got %d", len(allFindings))
	}

	// Toggle agent
	err = sup.ToggleAgent("agent_test_1", false)
	if err != nil {
		t.Fatalf("failed to toggle agent: %v", err)
	}

	statuses = sup.GetStatuses()
	if statuses[0].Enabled {
		t.Fatalf("expected agent to be disabled")
	}

	// Start / Stop supervisor.
	sup.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	sup.Stop()
}
