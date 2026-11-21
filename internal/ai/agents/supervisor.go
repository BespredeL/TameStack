package agents

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"tamestack/internal/ai/memory"
	"tamestack/internal/ai/workflows"
)

type agentEntry struct {
	agent      Agent
	enabled    bool
	interval   time.Duration
	lastRunAt  *time.Time
	nextRunAt  *time.Time
	lastStatus string
	lastError  string
	findings   []AgentFinding
	stopChan   chan struct{}
}

// Supervisor manages life cycle, scheduling, and findings aggregation for autonomous agents.
type Supervisor struct {
	mu            sync.RWMutex
	agents        map[string]*agentEntry
	memoryService *memory.Service
	allFindings   []AgentFinding
	maxFindings   int
	isRunning     bool
	rootCtx       context.Context
	rootCancel    context.CancelFunc
}

// NewSupervisor instantiates the agent supervisor.
func NewSupervisor(memService *memory.Service) *Supervisor {
	return &Supervisor{
		agents:        make(map[string]*agentEntry),
		memoryService: memService,
		allFindings:   make([]AgentFinding, 0),
		maxFindings:   100,
	}
}

// Register adds an autonomous agent to the supervisor pool.
func (s *Supervisor) Register(agent Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents[agent.ID()] = &agentEntry{
		agent:      agent,
		enabled:    true,
		interval:   agent.DefaultInterval(),
		lastStatus: "idle",
		findings:   make([]AgentFinding, 0),
	}
}

// Start launches scheduling loops for all enabled agents.
func (s *Supervisor) Start(ctx context.Context) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.rootCtx, s.rootCancel = context.WithCancel(ctx)
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, entry := range s.agents {
		if entry.enabled {
			s.startAgentLoop(id, entry)
		}
	}
}

// Stop gracefully stops all running agent timers.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}
	s.isRunning = false
	if s.rootCancel != nil {
		s.rootCancel()
	}

	for _, entry := range s.agents {
		if entry.stopChan != nil {
			close(entry.stopChan)
			entry.stopChan = nil
		}
	}
}

func (s *Supervisor) startAgentLoop(id string, entry *agentEntry) {
	if entry.stopChan != nil {
		close(entry.stopChan)
	}
	entry.stopChan = make(chan struct{})
	stopCh := entry.stopChan
	interval := entry.interval
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}

	next := time.Now().Add(interval)
	entry.nextRunAt = &next

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.rootCtx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				s.runScheduledAgent(id)
				s.mu.Lock()
				nxt := time.Now().Add(interval)
				entry.nextRunAt = &nxt
				s.mu.Unlock()
			}
		}
	}()
}

// runScheduledAgent keeps a panicking agent from taking the process — and with
// it every SSH session — down, and from silently losing its own schedule.
func (s *Supervisor) runScheduledAgent(agentID string) {
	defer func() {
		if p := recover(); p != nil {
			slog.Error("scheduled agent panicked",
				slog.String("agent_id", agentID),
				slog.Any("panic", p),
				slog.String("stack", string(debug.Stack())))
		}
	}()
	_, _ = s.TriggerAgent(s.rootCtx, agentID)
}

// TriggerAgent executes a single agent run immediately on demand.
func (s *Supervisor) TriggerAgent(ctx context.Context, agentID string) ([]AgentFinding, error) {
	s.mu.RLock()
	entry, exists := s.agents[agentID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("агент %s не найден", agentID)
	}

	now := time.Now().UTC()
	s.mu.Lock()
	entry.lastRunAt = &now
	entry.lastStatus = "running"
	s.mu.Unlock()

	findings, err := entry.agent.Run(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		entry.lastStatus = "error"
		entry.lastError = err.Error()
		return nil, err
	}

	entry.lastError = ""
	hasWarning := false
	for _, f := range findings {
		if f.Severity == workflows.SeverityCritical || f.Severity == workflows.SeverityWarning {
			hasWarning = true
			break
		}
	}

	if hasWarning {
		entry.lastStatus = "warning"
	} else {
		entry.lastStatus = "healthy"
	}

	entry.findings = findings
	for _, f := range findings {
		s.allFindings = append(s.allFindings, f)
		if len(s.allFindings) > s.maxFindings {
			s.allFindings = s.allFindings[len(s.allFindings)-s.maxFindings:]
		}

		// Auto-persist critical anomalies to infrastructure memory.
		if s.memoryService != nil && f.Severity == workflows.SeverityCritical {
			srvID := f.ServerID
			_, _ = s.memoryService.SaveFact(ctx, &srvID, "anomaly", f.Title, f.Details, "agent:"+f.AgentID)
		}
	}

	return findings, nil
}

// ToggleAgent enables or disables automatic runs for an agent.
func (s *Supervisor) ToggleAgent(agentID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.agents[agentID]
	if !exists {
		return fmt.Errorf("агент %s не найден", agentID)
	}

	if entry.enabled == enabled {
		return nil
	}

	entry.enabled = enabled
	if enabled && s.isRunning {
		s.startAgentLoop(agentID, entry)
	} else if !enabled && entry.stopChan != nil {
		close(entry.stopChan)
		entry.stopChan = nil
		entry.nextRunAt = nil
		entry.lastStatus = "idle"
	}

	return nil
}

// GetStatuses returns current observability state for all registered agents.
func (s *Supervisor) GetStatuses() []AgentStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]AgentStatus, 0, len(s.agents))
	for _, entry := range s.agents {
		status := AgentStatus{
			ID:              entry.agent.ID(),
			Name:            entry.agent.Name(),
			Description:     entry.agent.Description(),
			Enabled:         entry.enabled,
			IntervalSeconds: int(entry.interval.Seconds()),
			LastRunAt:       entry.lastRunAt,
			NextRunAt:       entry.nextRunAt,
			LastStatus:      entry.lastStatus,
			LastError:       entry.lastError,
			FindingsCount:   len(entry.findings),
			RecentFindings:  entry.findings,
		}
		statuses = append(statuses, status)
	}

	return statuses
}

// GetFindings returns global chronological list of findings.
func (s *Supervisor) GetFindings(limit int) []AgentFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.allFindings) {
		limit = len(s.allFindings)
	}

	result := make([]AgentFinding, limit)
	copy(result, s.allFindings[len(s.allFindings)-limit:])
	return result
}
