package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

func setupTestSecurityService(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "test_sec.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	cfg := &config.AppConfig{KnownHostsPath: filepath.Join(tmpDir, "known_hosts")}
	saltPath := filepath.Join(tmpDir, "salt.bin")
	_ = os.WriteFile(saltPath, []byte("12345678901234567890123456789012"), 0600)

	vault, err := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}
	pool := ssh.NewPool()
	engine := tasks.NewEngine(database, 2)

	svc := NewService(cfg, database, vault, pool, engine)
	return svc, database
}

func TestSecurity_AddFirewallRule_SSHProtection(t *testing.T) {
	svc, database := setupTestSecurityService(t)
	defer database.Close()

	ctx := context.Background()

	// Create a test server with SSH port 22.
	err := database.SaveServer(ctx, &db.ServerRecord{
		ID:              "srv-ssh-protect",
		Name:            "Protect Test Server",
		Host:            "127.0.0.1",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "dummy",
	})
	if err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	// Blocking port 22 for Anywhere must be rejected.
	_, err = svc.AddFirewallRule(ctx, AddFirewallRuleRequest{
		ServerID: "srv-ssh-protect",
		Port:     "22",
		Protocol: "tcp",
		Action:   "DENY",
		FromIP:   "any",
	})
	if err == nil {
		t.Fatalf("expected error when attempting to DENY SSH port 22, got nil")
	}

	t.Logf("Successfully blocked dangerous SSH deny rule: %v", err)
}

func TestSecurity_UpdateFirewallRule_SSHProtection(t *testing.T) {
	svc, database := setupTestSecurityService(t)
	defer database.Close()

	ctx := context.Background()

	err := database.SaveServer(ctx, &db.ServerRecord{
		ID:              "srv-ssh-protect-update",
		Name:            "Protect Test Server",
		Host:            "127.0.0.1",
		Port:            2222,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "dummy",
		Mode:            "manage",
	})
	if err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	// Updating a rule to block port 2222 for Anywhere must be rejected.
	_, err = svc.UpdateFirewallRule(ctx, UpdateFirewallRuleRequest{
		ServerID: "srv-ssh-protect-update",
		RuleNum:  2,
		OldPort:  "80",
		OldProto: "tcp",
		Port:     "2222",
		Protocol: "tcp",
		Action:   "DENY",
		FromIP:   "any",
	})
	if err == nil {
		t.Fatalf("expected error when attempting to update rule to DENY SSH port 2222, got nil")
	}

	// Updating with safe params should succeed.
	task, err := svc.UpdateFirewallRule(ctx, UpdateFirewallRuleRequest{
		ServerID: "srv-ssh-protect-update",
		RuleNum:  2,
		OldPort:  "80",
		OldProto: "tcp",
		Port:     "8080",
		Protocol: "tcp",
		Action:   "ALLOW",
		FromIP:   "any",
		Comment:  "Web dev",
	})
	if err != nil {
		t.Fatalf("expected successful task submission, got: %v", err)
	}
	if task == nil || task.OperationType != "security_firewall_update_rule" {
		t.Fatalf("expected security_firewall_update_rule task, got: %v", task)
	}
}

func TestSecurity_ParseListeningPorts(t *testing.T) {
	svc, database := setupTestSecurityService(t)
	defer database.Close()

	ssOutput := `Netid  State  Recv-Q Send-Q Local Address:Port  Peer Address:PortProcess
tcp    LISTEN 0      128          0.0.0.0:22         0.0.0.0:*    users:(("sshd",pid=1024,fd=3))
tcp    LISTEN 0      511        127.0.0.1:3306       0.0.0.0:*    users:(("mysqld",pid=2048,fd=15))
tcp    LISTEN 0      128          0.0.0.0:80         0.0.0.0:*    users:(("nginx",pid=3096,fd=6))
tcp    LISTEN 0      128             [::]:443           [::]:*    users:(("nginx",pid=3096,fd=7))
`

	ports := svc.parseListeningPorts(ssOutput)
	if len(ports) < 3 {
		t.Fatalf("expected at least 3 listening ports, got %d", len(ports))
	}

	foundSSH := false
	foundMySQL := false
	for _, p := range ports {
		if p.Port == 22 && p.Process == "sshd" && p.IsPublic {
			foundSSH = true
		}
		if p.Port == 3306 && p.Process == "mysqld" && !p.IsPublic {
			foundMySQL = true
		}
	}

	if !foundSSH {
		t.Errorf("expected to find public SSH port 22")
	}
	if !foundMySQL {
		t.Errorf("expected to find private MySQL port 3306")
	}
}

func TestSecurity_ParseMemInfo(t *testing.T) {
	svc, database := setupTestSecurityService(t)
	defer database.Close()

	memInfoSample := `MemTotal:       16384000 kB
MemFree:         4096000 kB
MemAvailable:    8192000 kB
SwapTotal:       2048000 kB
SwapFree:        1024000 kB
`

	metrics := &RealtimeSystemMetrics{}
	svc.parseMemInfo(memInfoSample, metrics)

	if metrics.MemoryTotalMB != 16000 {
		t.Errorf("expected MemoryTotalMB=16000, got %d", metrics.MemoryTotalMB)
	}
	if metrics.MemoryUsedMB != 8000 {
		t.Errorf("expected MemoryUsedMB=8000, got %d", metrics.MemoryUsedMB)
	}
	if metrics.SwapTotalMB != 2000 {
		t.Errorf("expected SwapTotalMB=2000, got %d", metrics.SwapTotalMB)
	}
	if metrics.SwapUsedMB != 1000 {
		t.Errorf("expected SwapUsedMB=1000, got %d", metrics.SwapUsedMB)
	}
}

func TestSecurity_ParseDiskInfo(t *testing.T) {
	svc, database := setupTestSecurityService(t)
	defer database.Close()

	// df -k -P / sample.
	diskSample := `Filesystem     1024-blocks      Used Available Capacity Mounted on
/dev/sda1         41208984  15204352  24004632      39% /
`
	metrics := &RealtimeSystemMetrics{}
	svc.parseDiskInfo(diskSample, metrics)

	if metrics.DiskTotalGB != 39.3 {
		t.Errorf("expected DiskTotalGB=39.3, got %f", metrics.DiskTotalGB)
	}
	if metrics.DiskUsedGB != 14.5 {
		t.Errorf("expected DiskUsedGB=14.5, got %f", metrics.DiskUsedGB)
	}
	if metrics.DiskFreeGB != 22.9 {
		t.Errorf("expected DiskFreeGB=22.9, got %f", metrics.DiskFreeGB)
	}
	if metrics.DiskUsedPercent != 36.9 {
		t.Errorf("expected DiskUsedPercent=36.9, got %f", metrics.DiskUsedPercent)
	}
}

func TestSecurity_CheckEndpoint(t *testing.T) {
	svc, database := setupTestSecurityService(t)
	defer database.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	result, err := svc.CheckEndpoint(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error checking endpoint: %v", err)
	}

	if !result.IsUp {
		t.Errorf("expected endpoint to be UP, got DOWN")
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", result.StatusCode)
	}
	if result.ResponseTimeMs < 0 {
		t.Errorf("expected positive response time, got %d", result.ResponseTimeMs)
	}
}

func TestSecurity_Fail2banJailManagement(t *testing.T) {
	svc, database := setupTestSecurityService(t)
	defer database.Close()

	ctx := context.Background()

	_ = database.SaveServer(ctx, &db.ServerRecord{
		ID:              "srv-1",
		Name:            "Test Server",
		Host:            "127.0.0.1",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "dummy",
		Mode:            "manage",
	})

	// 1. Invalid jail name must be rejected
	_, err := svc.SaveFail2banJail(ctx, SaveFail2banJailRequest{
		ServerID: "srv-1",
		Name:     "bad; jail name",
		Port:     "ssh",
	})
	if err == nil {
		t.Fatalf("expected error for invalid jail name with spaces/semicolons, got nil")
	}

	// 2. Valid SaveFail2banJail should submit a task
	task, err := svc.SaveFail2banJail(ctx, SaveFail2banJailRequest{
		ServerID: "srv-1",
		Name:     "nginx-botsearch",
		Enabled:  true,
		Port:     "http,https",
		Filter:   "nginx-botsearch",
		LogPath:  "/var/log/nginx/*access.log",
		MaxRetry: 2,
		FindTime: "10m",
		BanTime:  "24h",
	})
	if err != nil {
		t.Fatalf("failed to submit save jail task: %v", err)
	}
	if task == nil || task.OperationType != "security_fail2ban_save_jail" {
		t.Fatalf("expected security_fail2ban_save_jail task, got: %v", task)
	}

	// 3. ToggleFail2banJail
	toggleTask, err := svc.ToggleFail2banJail(ctx, ToggleFail2banJailRequest{
		ServerID: "srv-1",
		Name:     "nginx-botsearch",
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("failed to submit toggle jail task: %v", err)
	}
	if toggleTask == nil || toggleTask.OperationType != "security_fail2ban_toggle_jail" {
		t.Fatalf("expected security_fail2ban_toggle_jail task, got: %v", toggleTask)
	}

	// 4. DeleteFail2banJail
	delTask, err := svc.DeleteFail2banJail(ctx, DeleteFail2banJailRequest{
		ServerID: "srv-1",
		Name:     "nginx-botsearch",
	})
	if err != nil {
		t.Fatalf("failed to submit delete jail task: %v", err)
	}
	if delTask == nil || delTask.OperationType != "security_fail2ban_delete_jail" {
		t.Fatalf("expected security_fail2ban_delete_jail task, got: %v", delTask)
	}

	// 5. Test parsing of INI configs
	sampleConfig := `---FILE:/etc/fail2ban/jail.local---
[DEFAULT]
bantime = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log

---FILE:/etc/fail2ban/jail.d/TameStack-nginx.local---
[nginx-bad-request]
enabled = false
port = http,https
filter = nginx-bad-request
logpath = /var/log/nginx/*error.log
maxretry = 3
findtime = 10m
bantime = 24h
`
	parsed := parseFail2banConfigFiles(sampleConfig)
	if len(parsed) != 2 {
		t.Fatalf("expected 2 parsed jails, got %d", len(parsed))
	}
	sshd := parsed["sshd"]
	if sshd == nil || !sshd.Enabled || sshd.Port != "ssh" || sshd.Filter != "sshd" {
		t.Fatalf("unexpected sshd config: %+v", sshd)
	}
	nginx := parsed["nginx-bad-request"]
	if nginx == nil || nginx.Enabled || nginx.MaxRetry != 3 || nginx.BanTime != "24h" {
		t.Fatalf("unexpected nginx config: %+v", nginx)
	}

	sampleJailConf := `
---FILE:/etc/fail2ban/jail.conf---
[DEFAULT]
enabled = false

[3proxy]
port = 3128
logpath = /var/log/3proxy.log

[apache-auth]
port = http,https
logpath = /var/log/apache2/error.log
`
	parsedConf := parseFail2banConfigFiles(sampleJailConf)
	if parsedConf["3proxy"] == nil || parsedConf["3proxy"].Enabled {
		t.Errorf("3proxy in jail.conf should not be enabled by default")
	}
	if parsedConf["apache-auth"] == nil || parsedConf["apache-auth"].Enabled {
		t.Errorf("apache-auth in jail.conf should not be enabled by default")
	}

	// 6. Test duration formatter
	if got := formatSecondsToDuration("3600"); got != "1h" {
		t.Errorf("expected 1h, got %s", got)
	}
	if got := formatSecondsToDuration("86400"); got != "1d" {
		t.Errorf("expected 1d, got %s", got)
	}
	if got := formatSecondsToDuration("600"); got != "10m" {
		t.Errorf("expected 10m, got %s", got)
	}
	if got := formatSecondsToDuration("10m"); got != "10m" {
		t.Errorf("expected 10m, got %s", got)
	}
}
