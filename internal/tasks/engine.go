package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"tamestack/internal/crypto"
	"tamestack/internal/db"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type TaskStatus string

const (
	StatusPending          TaskStatus = "pending"
	StatusRunning          TaskStatus = "running"
	StatusCompleted        TaskStatus = "completed"
	StatusFailed           TaskStatus = "failed"
	StatusCancelled        TaskStatus = "cancelled"
	StatusRequiresApproval TaskStatus = "requires_approval"
)

// TaskLogger handles structured streaming logs for an active task.
type TaskLogger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	SetStep(step string, progress int)
}

// TaskHandler represents the execution logic for a specific operation.
type TaskHandler func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (result string, err error)

// EngineStore is the database slice the engine operates on.
type EngineStore interface {
	db.TaskStore
	GetServer(ctx context.Context, id string) (*db.ServerRecord, error)
}

// Engine is the central asynchronous job orchestrator.
type Engine struct {
	mu          sync.Mutex
	database    EngineStore
	appCtx      context.Context
	handlers    map[string]TaskHandler
	cancelFuncs map[string]context.CancelFunc
	taskQueue   chan *db.TaskRecord
	serverModes map[string]string
	workerCount int
	isStarted   bool
	isStopped   bool
	wg          sync.WaitGroup
}

// NewEngine initializes the task engine.
func NewEngine(database EngineStore, workerCount int) *Engine {
	if workerCount <= 0 {
		workerCount = 4
	}
	return &Engine{
		database:    database,
		handlers:    make(map[string]TaskHandler),
		cancelFuncs: make(map[string]context.CancelFunc),
		taskQueue:   make(chan *db.TaskRecord, 200),
		serverModes: make(map[string]string),
		workerCount: workerCount,
	}
}

// SetWailsContext attaches the Wails runtime context for event broadcasting.
func (e *Engine) SetWailsContext(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.appCtx = ctx
}

// RegisterHandler binds an operation type to its handler function.
func (e *Engine) RegisterHandler(operationType string, handler TaskHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[operationType] = handler
}

// Start spawns the background worker pool.
func (e *Engine) Start() {
	e.mu.Lock()
	if e.isStarted {
		e.mu.Unlock()
		return
	}
	e.isStarted = true
	e.isStopped = false
	e.mu.Unlock()

	for i := 0; i < e.workerCount; i++ {
		e.wg.Add(1)
		go e.workerLoop(i)
	}
}

// Stop cancels running tasks, closes the queue and waits for the workers to
// finish. Without it, a worker keeps writing to a database that shutdown has
// already closed and to an encrypted container that is being sealed.
func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	if !e.isStarted {
		e.mu.Unlock()
		return nil
	}
	e.isStarted = false
	e.isStopped = true
	cancels := make([]context.CancelFunc, 0, len(e.cancelFuncs))
	for _, cancel := range e.cancelFuncs {
		cancels = append(cancels, cancel)
	}
	e.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	close(e.taskQueue)

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("task workers did not finish before shutdown ended: %w", ctx.Err())
	}
}

// SetServerMode updates the server session operating mode ("audit" or "management").
func (e *Engine) SetServerMode(serverID, mode string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if mode != "management" {
		mode = "audit"
	}
	e.serverModes[serverID] = mode
}

// GetServerMode returns the active operating mode for the server ("audit" or "management").
func (e *Engine) GetServerMode(serverID string) string {
	e.mu.Lock()
	mode, ok := e.serverModes[serverID]
	e.mu.Unlock()
	if ok {
		return mode
	}
	if e.database != nil && serverID != "" {
		srv, err := e.database.GetServer(context.Background(), serverID)
		if err == nil && srv != nil && srv.Mode != "" {
			e.mu.Lock()
			e.serverModes[serverID] = srv.Mode
			e.mu.Unlock()
			return srv.Mode
		}
	}
	return "audit" // default mode is audit
}

// IsReadOnlyOperation reports whether the given operation only inspects and does not modify the server.
func IsReadOnlyOperation(operationType string) bool {
	switch operationType {
	case "system_diag", "server_diagnostics", "inventory_snapshot", "site_audit", "site_file_capture":
		return true
	default:
		return false
	}
}

// Submit queues a new task for asynchronous execution.
func (e *Engine) Submit(serverID string, operationType string, title string, payload string) (*db.TaskRecord, error) {
	if err := e.refuseStopped(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	_, exists := e.handlers[operationType]
	e.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for operation: %s", operationType)
	}

	// Gateway check: Audit Mode prohibits mutations without prior switch to Management (ADR-S5).
	if !IsReadOnlyOperation(operationType) && serverID != "" {
		mode := e.GetServerMode(serverID)
		if mode == "audit" {
			return nil, fmt.Errorf("сервер находится в режиме аудита (Audit Mode): мутирующие операции запрещены. Переключите режим на 'Управление'")
		}
	}

	// Payload scrub: passwords and private keys are redacted from persisted task payload.
	cleanPayload := crypto.ScrubSecrets(payload)

	task := &db.TaskRecord{
		ID:            uuid.New().String(),
		ServerID:      serverID,
		OperationType: operationType,
		Title:         title,
		Status:        string(StatusPending),
		Progress:      0,
		Payload:       cleanPayload,
		CreatedAt:     time.Now(),
	}

	if err := e.database.SaveTask(context.Background(), task); err != nil {
		return nil, fmt.Errorf("failed to persist new task: %w", err)
	}

	e.emitEvent("task:created", task)
	if !e.enqueue(task) {
		e.discardUnstartedTask(task, "TameStack is shutting down; the task was not started")
		return nil, fmt.Errorf("task engine is stopping: the task was not queued")
	}
	return task, nil
}

// SubmitAndWait queues a task, waits for its completion, and returns an error if it failed.
func (e *Engine) SubmitAndWait(ctx context.Context, serverID, operationType, title, payload string) error {
	task, err := e.Submit(serverID, operationType, title, payload)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = e.Cancel(task.ID)
			return ctx.Err()
		case <-ticker.C:
			t, err := e.database.GetTask(ctx, task.ID)
			if err != nil {
				return err
			}
			if t == nil {
				return fmt.Errorf("task %s disappeared", task.ID)
			}
			switch TaskStatus(t.Status) {
			case StatusCompleted:
				return nil
			case StatusFailed:
				if t.ErrorMessage != "" {
					return fmt.Errorf("%s", t.ErrorMessage)
				}
				return fmt.Errorf("task failed")
			case StatusCancelled:
				return fmt.Errorf("task was cancelled")
			}
		}
	}
}

// SubmitRequiresApproval creates a task in requires_approval state without enqueuing it.
func (e *Engine) SubmitRequiresApproval(serverID string, operationType string, title string, payload string) (*db.TaskRecord, error) {
	if err := e.refuseStopped(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	_, exists := e.handlers[operationType]
	e.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for operation: %s", operationType)
	}

	// Gateway check: Audit Mode prohibits mutations without prior switch to Management (ADR-S5).
	if !IsReadOnlyOperation(operationType) && serverID != "" {
		mode := e.GetServerMode(serverID)
		if mode == "audit" {
			return nil, fmt.Errorf("сервер находится в режиме аудита (Audit Mode): мутирующие операции запрещены. Переключите режим на 'Управление'")
		}
	}

	// Payload scrub: passwords and private keys are redacted from persisted task payload.
	cleanPayload := crypto.ScrubSecrets(payload)

	task := &db.TaskRecord{
		ID:            uuid.New().String(),
		ServerID:      serverID,
		OperationType: operationType,
		Title:         title,
		Status:        string(StatusRequiresApproval),
		Progress:      0,
		Payload:       cleanPayload,
		CreatedAt:     time.Now(),
	}

	if err := e.database.SaveTask(context.Background(), task); err != nil {
		return nil, fmt.Errorf("failed to persist approval task: %w", err)
	}

	e.emitEvent("task:created", task)
	return task, nil
}

// Approve transitions a requires_approval task to pending and enqueues it.
func (e *Engine) Approve(taskID string) error {
	if err := e.refuseStopped(); err != nil {
		return err
	}

	task, err := e.database.GetTask(context.Background(), taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != string(StatusRequiresApproval) {
		return fmt.Errorf("task %s is not in requires_approval status (status: %s)", taskID, task.Status)
	}

	task.Status = string(StatusPending)
	if err := e.database.SaveTask(context.Background(), task); err != nil {
		return fmt.Errorf("failed to update approved task: %w", err)
	}

	e.emitEvent("task:updated", task)
	if !e.enqueue(task) {
		e.discardUnstartedTask(task, "TameStack is shutting down; the task was not started")
		return fmt.Errorf("task engine is stopping: the task was not queued")
	}
	return nil
}

// Reject cancels a task awaiting approval.
func (e *Engine) Reject(taskID string) error {
	task, err := e.database.GetTask(context.Background(), taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != string(StatusRequiresApproval) {
		return fmt.Errorf("task %s is not in requires_approval status", taskID)
	}

	now := time.Now()
	task.Status = string(StatusCancelled)
	task.ErrorMessage = "Отклонено пользователем"
	task.CompletedAt = &now

	if err := e.database.SaveTask(context.Background(), task); err != nil {
		return fmt.Errorf("failed to update rejected task: %w", err)
	}

	e.emitEvent("task:finished", task)
	return nil
}

// Cancel terminates a running task using its cancellation context.
func (e *Engine) Cancel(taskID string) error {
	e.mu.Lock()
	cancel, exists := e.cancelFuncs[taskID]
	e.mu.Unlock()

	if exists && cancel != nil {
		cancel()
		return nil
	}
	return fmt.Errorf("task %s is not actively running", taskID)
}

// refuseStopped reports an error once shutdown has drained the workers, so no
// task row is written into a database that is about to close.
func (e *Engine) refuseStopped() error {
	e.mu.Lock()
	stopped := e.isStopped
	e.mu.Unlock()
	if stopped {
		return fmt.Errorf("task engine is stopped; no new tasks are accepted")
	}
	return nil
}

// enqueue hands a persisted task to a worker. It reports false once the engine is
// stopping, because the queue is closed and sending to it would panic.
func (e *Engine) enqueue(task *db.TaskRecord) bool {
	e.mu.Lock()
	stopping := e.isStopped
	e.mu.Unlock()
	if stopping {
		return false
	}

	select {
	case e.taskQueue <- task:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

// discardUnstartedTask records why a saved task never reached a worker, so the
// UI does not show it as pending forever.
func (e *Engine) discardUnstartedTask(task *db.TaskRecord, reason string) {
	now := time.Now()
	task.Status = string(StatusCancelled)
	task.ErrorMessage = reason
	task.CompletedAt = &now
	_ = e.database.SaveTask(context.Background(), task)
	e.emitEvent("task:finished", task)
}

func (e *Engine) workerLoop(workerID int) {
	defer e.wg.Done()
	for task := range e.taskQueue {
		e.mu.Lock()
		stopping := e.isStopped
		e.mu.Unlock()
		if stopping {
			e.discardUnstartedTask(task, "TameStack is shutting down; the task was not started")
			continue
		}
		e.runTask(task)
	}
}

// runTask isolates a handler panic: the task is recorded as failed and the worker
// keeps draining the queue instead of taking the whole process down.
func (e *Engine) runTask(task *db.TaskRecord) {
	defer func() {
		if p := recover(); p != nil {
			slog.Error("task handler panicked",
				slog.String("task_id", task.ID),
				slog.String("operation", task.OperationType),
				slog.Any("panic", p),
				slog.String("stack", string(debug.Stack())))
			now := time.Now()
			task.Status = string(StatusFailed)
			task.CompletedAt = &now
			task.ErrorMessage = fmt.Sprintf("internal error: %v", p)
			_ = e.database.SaveTask(context.Background(), task)
			e.emitEvent("task:finished", task)
		}
	}()
	e.executeTask(task)
}

func (e *Engine) executeTask(task *db.TaskRecord) {
	e.mu.Lock()
	parent := e.appCtx
	e.mu.Unlock()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	e.mu.Lock()
	e.cancelFuncs[task.ID] = cancel
	handler := e.handlers[task.OperationType]
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.cancelFuncs, task.ID)
		e.mu.Unlock()
	}()

	now := time.Now()
	task.Status = string(StatusRunning)
	task.StartedAt = &now
	_ = e.database.SaveTask(ctx, task)
	e.emitEvent("task:started", task)

	logger := &taskLoggerImpl{
		engine: e,
		task:   task,
	}

	logger.Info(fmt.Sprintf("Task [%s] started: %s", task.OperationType, task.Title))

	result, err := handler(ctx, task, logger)
	completedTime := time.Now()
	task.CompletedAt = &completedTime

	if err != nil {
		if ctx.Err() == context.Canceled {
			task.Status = string(StatusCancelled)
			task.ErrorMessage = "Operation cancelled by user."
			logger.Warn("Task cancelled by user.")
		} else {
			task.Status = string(StatusFailed)
			task.ErrorMessage = err.Error()
			logger.Error(fmt.Sprintf("Task failed: %v", err))
		}
	} else {
		task.Status = string(StatusCompleted)
		task.Progress = 100
		task.Result = result
		logger.Info("Task completed successfully.")
	}

	_ = e.database.SaveTask(context.Background(), task)
	e.emitEvent("task:finished", task)
}

func (e *Engine) emitEvent(eventName string, data interface{}) {
	e.mu.Lock()
	ctx := e.appCtx
	e.mu.Unlock()

	if ctx != nil {
		wailsRuntime.EventsEmit(ctx, eventName, data)
	}
}

type taskLoggerImpl struct {
	engine *Engine
	task   *db.TaskRecord
}

func (l *taskLoggerImpl) log(level string, message string) {
	cleanMsg := crypto.ScrubSecrets(message)
	_ = l.engine.database.AppendTaskLog(context.Background(), l.task.ID, level, cleanMsg)
	l.engine.emitEvent("task:log", map[string]interface{}{
		"task_id": l.task.ID,
		"level":   level,
		"message": cleanMsg,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (l *taskLoggerImpl) Debug(msg string) { l.log("debug", msg) }
func (l *taskLoggerImpl) Info(msg string)  { l.log("info", msg) }
func (l *taskLoggerImpl) Warn(msg string)  { l.log("warn", msg) }
func (l *taskLoggerImpl) Error(msg string) { l.log("error", msg) }

func (l *taskLoggerImpl) SetStep(step string, progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	l.task.CurrentStep = step
	l.task.Progress = progress
	_ = l.engine.database.SaveTask(context.Background(), l.task)
	l.engine.emitEvent("task:progress", map[string]interface{}{
		"task_id":  l.task.ID,
		"step":     step,
		"progress": progress,
	})
}
