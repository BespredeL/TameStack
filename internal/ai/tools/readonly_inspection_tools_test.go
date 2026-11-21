package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tamestack/internal/ai/domain"
	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
	"tamestack/internal/testutil"
)

func setupInspectionRegistry(t *testing.T) (*Registry, *db.DB) {
	t.Helper()

	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "inspection_tools.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	ctx := context.Background()
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID:              "srv-1",
		Name:            "Inspection Test Server",
		Host:            "127.0.0.1",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "dummy",
		SSHHostKey:      "SHA256:dummy",
		Mode:            "management",
	}); err != nil {
		t.Fatalf("failed to seed server: %v", err)
	}

	saltPath := filepath.Join(tmpDir, "salt.bin")
	if err := os.WriteFile(saltPath, []byte("12345678901234567890123456789012"), 0600); err != nil {
		t.Fatalf("failed to write salt: %v", err)
	}
	vault, err := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	secService := security.NewService(
		&config.AppConfig{DataDir: tmpDir, KnownHostsPath: filepath.Join(tmpDir, "known_hosts")},
		database,
		vault,
		ssh.NewPool(),
		tasks.NewEngine(database, 1),
	)

	reg := NewRegistry(database, nil)
	RegisterReadOnlyInspectionTools(reg, database, nil, nil, nil, secService)
	return reg, database
}

func TestRegisterReadOnlyInspectionTools(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "test_readonly_tools.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	reg := NewRegistry(database, nil)
	RegisterReadOnlyInspectionTools(reg, database, nil, nil, nil, nil)

	expectedTools := []string{
		"read_webserver_logs",
		"get_service_status",
		"get_service_logs",
		"check_web_topology",
		"inspect_php_environment",
		"read_config_file",
		"get_listening_ports",
		"test_webserver_syntax",
	}

	defs := reg.Definitions()
	if len(defs) != len(expectedTools) {
		t.Fatalf("expected %d tools registered, got %d", len(expectedTools), len(defs))
	}

	toolMap := make(map[string]bool)
	for _, d := range defs {
		toolMap[d.Name] = true
		if d.IsMutating {
			t.Errorf("tool %s should be non-mutating (read-only)", d.Name)
		}
		if d.RiskLevel != domain.RiskSafe {
			t.Errorf("tool %s should be classified safe, got %s", d.Name, d.RiskLevel)
		}
	}

	for _, name := range expectedTools {
		if !toolMap[name] {
			t.Errorf("expected tool %s to be registered", name)
		}
	}
}

// TestReadOnlyToolsRejectModelControlledExpansions covers the arguments a model can steer and
// expects a validation message, not the "service not initialised" fallback used by earlier tests.
func TestReadOnlyToolsRejectModelControlledExpansions(t *testing.T) {
	reg, database := setupInspectionRegistry(t)
	defer database.Close()

	ctx := context.Background()

	tests := []struct {
		tool        string
		args        string
		wantMessage string
	}{
		{"read_webserver_logs", `{"server_id":"srv-1","log_type":"$(id)"}`, "invalid log type"},
		{"read_webserver_logs", `{"server_id":"srv-1","log_type":"error2"}`, "invalid log type"},
		{"read_webserver_logs", `{"server_id":"srv-1","log_type":"access; rm -rf /"}`, "invalid log type"},
		{"get_service_status", `{"server_id":"srv-1","service_name":"nginx; rm -rf /"}`, "invalid service name"},
		{"get_service_status", `{"server_id":"srv-1","service_name":"nginx$(id)"}`, "invalid service name"},
		{"get_service_logs", `{"server_id":"srv-1","service_name":"nginx|tee /tmp/x"}`, "invalid service name"},
		{"inspect_php_environment", `{"server_id":"srv-1","version":"8.1; id"}`, "invalid PHP version"},
		{"inspect_php_environment", `{"server_id":"srv-1","version":"8.1` + "`id`" + `"}`, "invalid PHP version"},
		{"test_webserver_syntax", `{"server_id":"srv-1","webserver_type":"nginx; id"}`, "unsupported webserver type"},
	}

	for _, tt := range tests {
		res, _ := reg.Execute(ctx, "conv1", "msg1", tt.tool, tt.args)
		if res == nil {
			t.Fatalf("%s %s: no result", tt.tool, tt.args)
		}
		if res.Success {
			t.Errorf("%s accepted %s", tt.tool, tt.args)
			continue
		}
		if !strings.Contains(res.Error, tt.wantMessage) {
			t.Errorf("%s(%s): want %q in error, got %q", tt.tool, tt.args, tt.wantMessage, res.Error)
		}
	}
}

func TestReadOnlyToolsRejectConfigPathsOutsidePolicy(t *testing.T) {
	reg, database := setupInspectionRegistry(t)
	defer database.Close()

	ctx := context.Background()

	rejected := []string{
		"/etc/../root/.ssh/id_rsa",
		"/etc/ssl/private/privkey.pem",
		"/etc/shadow",
		"/home/user/my_secret.txt",
		"/proc/self/environ",
		"/etc/nginx/$(id).conf",
		"/etc/nginx/conf.d/*.conf",
		"~/.ssh/config",
	}

	for _, path := range rejected {
		raw, err := json.Marshal(map[string]any{"server_id": "srv-1", "file_path": path})
		if err != nil {
			t.Fatalf("build args for %s: %v", path, err)
		}
		res, _ := reg.Execute(ctx, "conv1", "msg1", "read_config_file", string(raw))
		if res == nil {
			t.Fatalf("read_config_file %s: no result", path)
		}
		if res.Success {
			t.Errorf("read_config_file accepted %s", path)
			continue
		}
		// The rejection must come from the read policy, not from a missing dependency or server.
		for _, unrelated := range []string{"не инициализирован", "не найден"} {
			if strings.Contains(res.Error, unrelated) {
				t.Errorf("read_config_file(%s) rejected for an unrelated reason: %s", path, res.Error)
			}
		}
	}
}

func TestIntArgBoundsModelSuppliedCounts(t *testing.T) {
	args := map[string]any{"lines": float64(100000), "max_lines": float64(0), "trick": "not a number"}
	if got := intArg(args, "lines", 50, 200); got != 200 {
		t.Errorf("intArg did not clamp an oversized line count: %d", got)
	}
	if got := intArg(args, "max_lines", 150, 300); got != 150 {
		t.Errorf("intArg ignored the default for a non-positive value: %d", got)
	}
	if got := intArg(args, "trick", 50, 200); got != 50 {
		t.Errorf("intArg accepted a non-numeric value: %d", got)
	}
	if got := intArg(map[string]any{}, "missing", 50, 200); got != 50 {
		t.Errorf("intArg missing key default broken: %d", got)
	}
}

func TestLogTargetForIsCodeOwned(t *testing.T) {
	cases := map[string]security.LogTarget{
		"Apache":       security.LogTargetApache,
		"httpd":        security.LogTargetApache,
		"php8.2-fpm":   security.LogTargetPHPFPM,
		"nginx":        security.LogTargetNginx,
		"site.ru":      security.LogTargetAuto,
		"$(id)":        security.LogTargetAuto,
		"nginx; rm -f": security.LogTargetNginx,
	}
	for in, want := range cases {
		if got := logTargetFor(in); got != want {
			t.Errorf("logTargetFor(%q) = %s, want %s", in, got, want)
		}
	}
}
