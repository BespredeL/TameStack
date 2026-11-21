package tasks

import (
	"context"
	"path/filepath"
	"tamestack/internal/testutil"
	"strings"
	"testing"
	"time"

	"tamestack/internal/db"
)

func TestTaskEngineExecution(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_tasks.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	engine := NewEngine(database, 2)
	engine.RegisterHandler("mock_op", func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (string, error) {
		logger.Info("Mock running step 1")
		logger.SetStep("Step 1", 50)
		logger.Info("Mock running step 2")
		logger.SetStep("Done", 100)
		return "Mock Success", nil
	})
	engine.Start()

	task, err := engine.Submit("", "mock_op", "Mock Task Title", "{}")
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Wait for task to finish.
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		tasksList, _ := database.ListTasks(context.Background(), 1)
		if len(tasksList) > 0 && tasksList[0].ID == task.ID && tasksList[0].Status == string(StatusCompleted) {
			logs, _ := database.GetTaskLogs(context.Background(), task.ID)
			if len(logs) < 2 {
				t.Fatalf("expected at least 2 logs, got %d", len(logs))
			}
			return
		}
	}

	t.Fatalf("task did not complete in time")
}

func TestAuditModeGateway(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_audit.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	engine := NewEngine(database, 2)
	engine.RegisterHandler("site_create", func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (string, error) {
		return "created", nil
	})
	engine.RegisterHandler("server_diagnostics", func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (string, error) {
		return "diag ok", nil
	})
	engine.RegisterHandler("system_diag", func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (string, error) {
		return "diag ok", nil
	})

	serverID := "srv-audit-1"
	_ = database.SaveServer(context.Background(), &db.ServerRecord{
		ID:       serverID,
		Name:     "Test Audit Srv",
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
		AuthType: "password",
		Mode:     "audit",
	})
	engine.SetServerMode(serverID, "audit")

	// 1. Mutating task in audit mode MUST be rejected.
	_, err = engine.Submit(serverID, "site_create", "Create Site", `{"domain":"example.com"}`)
	if err == nil {
		t.Fatal("expected mutating task to be rejected in audit mode")
	}

	// 2. Read-only task in audit mode MUST be allowed.
	_, err = engine.Submit(serverID, "server_diagnostics", "Diagnostics", `{}`)
	if err != nil {
		t.Fatalf("expected read-only task to be accepted in audit mode: %v", err)
	}

	// 2b. The server module submits diagnostics as "system_diag"; it must pass
	// the audit gate too (regression: the whitelist once knew only the alias).
	_, err = engine.Submit(serverID, "system_diag", "Diagnostics", `{}`)
	if err != nil {
		t.Fatalf("expected system_diag to be accepted in audit mode: %v", err)
	}

	// 3. Switch to management mode allows mutating task.
	engine.SetServerMode(serverID, "management")
	task, err := engine.Submit(serverID, "site_create", "Create Site", `{"domain":"example.com", "password":"secret_pass"}`)
	if err != nil {
		t.Fatalf("expected mutating task to be accepted in management mode: %v", err)
	}

	// 4. Secret scrubbing check.
	if task.Payload == `{"domain":"example.com", "password":"secret_pass"}` {
		t.Fatal("task payload was not scrubbed of password")
	}
}

// A handler panic must fail its own task and leave the worker alive for the next
// one; without recovery the panic would kill the process and every SSH session.
func TestTaskEngineSurvivesHandlerPanic(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "test_panic.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	engine := NewEngine(database, 1)
	engine.RegisterHandler("boom_op", func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (string, error) {
		panic("handler exploded")
	})
	engine.RegisterHandler("after_op", func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (string, error) {
		return "reached after the panic", nil
	})
	engine.Start()

	panicking, err := engine.Submit("", "boom_op", "Panicking task", "{}")
	if err != nil {
		t.Fatalf("failed to submit: %v", err)
	}
	followUp, err := engine.Submit("", "after_op", "Next task", "{}")
	if err != nil {
		t.Fatalf("failed to submit follow-up: %v", err)
	}

	status := map[string]string{}
	for i := 0; i < 40; i++ {
		time.Sleep(50 * time.Millisecond)
		tasksList, _ := database.ListTasks(context.Background(), 10)
		for _, rec := range tasksList {
			status[rec.ID] = rec.Status
		}
		if status[panicking.ID] == string(StatusFailed) && status[followUp.ID] == string(StatusCompleted) {
			rec, err := database.GetTask(context.Background(), panicking.ID)
			if err != nil || rec == nil {
				t.Fatalf("failed task not readable: %v", err)
			}
			if !strings.Contains(rec.ErrorMessage, "handler exploded") {
				t.Fatalf("panic is not recorded in the task error: %q", rec.ErrorMessage)
			}
			return
		}
	}

	t.Fatalf("worker did not recover: %+v", status)
}

// Shutdown has to cancel the running task, drain the queued one and refuse new
// work, so no worker writes into a database that is about to close.
func TestTaskEngineStopCancelsTasksAndRefusesNewWork(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "test_stop.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	engine := NewEngine(database, 1)
	started := make(chan struct{})
	engine.RegisterHandler("slow_op", func(ctx context.Context, task *db.TaskRecord, logger TaskLogger) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	engine.Start()

	running, err := engine.Submit("", "slow_op", "Running task", "{}")
	if err != nil {
		t.Fatalf("failed to submit: %v", err)
	}
	<-started

	queued, err := engine.Submit("", "slow_op", "Queued task", "{}")
	if err != nil {
		t.Fatalf("failed to submit the queued task: %v", err)
	}

	if err := engine.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned %v", err)
	}

	for _, id := range []string{running.ID, queued.ID} {
		rec, err := database.GetTask(context.Background(), id)
		if err != nil || rec == nil {
			t.Fatalf("task %s not readable: %v", id, err)
		}
		if rec.Status != string(StatusCancelled) {
			t.Errorf("task %s ended as %s, want cancelled", id, rec.Status)
		}
	}

	if _, err := engine.Submit("", "slow_op", "After stop", "{}"); err == nil {
		t.Error("Submit accepted a task after Stop")
	}
	if err := engine.Approve(running.ID); err == nil {
		t.Error("Approve accepted a task after Stop")
	}
	if err := engine.Stop(context.Background()); err != nil {
		t.Errorf("second Stop must be a no-op, got %v", err)
	}
}
