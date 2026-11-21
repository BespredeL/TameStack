package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/tasks"
	"tamestack/internal/testutil"
)

func setupProposalTest(t *testing.T, mode string) (*db.DB, *tasks.Engine, *Registry, string) {
	t.Helper()
	database, err := db.Open(filepath.Join(testutil.Workspace(t), "proposal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	serverID := "srv-proposal"
	if err := database.SaveServer(context.Background(), &db.ServerRecord{
		ID: serverID, Name: "Proposal Srv", Host: "127.0.0.1", Port: 22,
		Username: "root", AuthType: "password", Mode: mode,
	}); err != nil {
		t.Fatal(err)
	}

	engine := tasks.NewEngine(database, 1)
	engine.RegisterHandler("ai_apply_config_diff", func(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
		return "applied", nil
	})
	engine.RegisterHandler("ai_restart_service", func(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
		return "restarted", nil
	})
	engine.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = engine.Stop(ctx)
	})

	registry := NewRegistry(database, nil)
	RegisterMutatingTools(registry, database, nil, engine)
	return database, engine, registry, serverID
}

func TestProposalToolsCreateApprovalTasks(t *testing.T) {
	database, engine, registry, serverID := setupProposalTest(t, "management")
	ctx := context.Background()
	convID := "conv-proposal"

	cases := []struct {
		tool string
		args string
	}{
		{"prepare_config_diff", `{"server_id":"` + serverID + `","file_path":"/etc/nginx/sites-available/test.conf","proposed_content":"server { listen 80; }","webserver_type":"nginx"}`},
		{"propose_service_restart", `{"server_id":"` + serverID + `","service_name":"nginx"}`},
	}

	for _, tc := range cases {
		res, err := registry.Execute(ctx, convID, "msg-1", tc.tool, tc.args)
		if err != nil {
			t.Fatalf("%s: execute failed: %v", tc.tool, err)
		}
		if !res.Success {
			t.Fatalf("%s: expected success, got error: %s", tc.tool, res.Error)
		}

		var out map[string]any
		if err := json.Unmarshal([]byte(res.RawOutput), &out); err != nil {
			t.Fatalf("%s: bad tool output %q: %v", tc.tool, res.RawOutput, err)
		}
		if out["status"] != "awaiting_approval" {
			t.Fatalf("%s: expected awaiting_approval, got %v", tc.tool, out["status"])
		}
		taskID, _ := out["task_id"].(string)
		if taskID == "" {
			t.Fatalf("%s: tool output must carry a task_id for the UI approval button", tc.tool)
		}

		task, err := database.GetTask(ctx, taskID)
		if err != nil || task == nil {
			t.Fatalf("%s: approval task %s not persisted (err: %v)", tc.tool, taskID, err)
		}
		if task.Status != string(tasks.StatusRequiresApproval) {
			t.Fatalf("%s: task status = %s, want requires_approval", tc.tool, task.Status)
		}

		if err := engine.Approve(taskID); err != nil {
			t.Fatalf("%s: approve failed: %v", tc.tool, err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			task, err = database.GetTask(ctx, taskID)
			if err != nil {
				t.Fatal(err)
			}
			if task.Status == string(tasks.StatusCompleted) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: approved task did not complete, status %s", tc.tool, task.Status)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestProposalToolsRejectedInAuditMode(t *testing.T) {
	_, _, registry, serverID := setupProposalTest(t, "audit")

	res, err := registry.Execute(context.Background(), "conv-audit", "msg-1", "propose_service_restart",
		`{"server_id":"`+serverID+`","service_name":"nginx"}`)
	if err != nil {
		t.Fatalf("execute should surface the refusal as a tool result, got err: %v", err)
	}
	if res.Success {
		t.Fatal("audit mode must refuse mutation proposals")
	}
}
