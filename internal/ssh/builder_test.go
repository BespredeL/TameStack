package ssh

import (
	"strings"
	"testing"
)

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"it's fine", "'it'\\''s fine'"},
		{"$VAR & `cmd`", "'$VAR & `cmd`'"},
		{"", "''"},
	}

	for _, tt := range tests {
		actual := ShellEscape(tt.input)
		if actual != tt.expected {
			t.Errorf("ShellEscape(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestCommandBuilder_Build(t *testing.T) {
	client := &SSHClient{
		user:         "developer",
		sudoPassword: "secret'password",
	}

	// 1. Basic command.
	b1 := client.NewCommand("echo hello")
	script1 := b1.Build()
	if !strings.Contains(script1, "echo hello") {
		t.Fatalf("expected script to contain echo hello, got:\n%s", script1)
	}
	if strings.Contains(script1, "run_sudo") {
		t.Fatalf("expected script NOT to contain run_sudo when sudo not requested, got:\n%s", script1)
	}

	// 2. Command with Args.
	b2 := client.NewCommandWithArgs("touch", "my file's name.txt", "arg with space")
	script2 := b2.Build()
	if !strings.Contains(script2, "touch 'my file'\\''s name.txt' 'arg with space'") {
		t.Fatalf("unexpected args escaping, got:\n%s", script2)
	}

	// 3. Command with Sudo.
	b3 := client.NewCommand("apt-get update").WithSudo()
	script3 := b3.Build()
	if !strings.Contains(script3, "run_sudo apt-get update") {
		t.Fatalf("expected script to prefix with run_sudo, got:\n%s", script3)
	}
	if strings.Contains(script3, "secret") {
		t.Fatalf("sudo password must not appear in the script, got:\n%s", script3)
	}
	if !strings.Contains(script3, "sudo -A") {
		t.Fatalf("expected askpass sudo, got:\n%s", script3)
	}

	// 4. Command with Environment and WorkDir.
	b4 := client.NewCommand("npm run build").
		WithEnv("NODE_ENV", "production").
		WithEnv("API_URL", "https://example.com/api").
		WithWorkDir("/var/www/site")
	script4 := b4.Build()
	if !strings.Contains(script4, "export API_URL='https://example.com/api'") ||
		!strings.Contains(script4, "export NODE_ENV='production'") {
		t.Fatalf("expected exported env vars, got:\n%s", script4)
	}
	if !strings.Contains(script4, "cd '/var/www/site' || exit 1") {
		t.Fatalf("expected cd to workDir, got:\n%s", script4)
	}

	// 5. Command with Lock.
	b5 := client.NewCommand("dpkg -i pkg.deb").
		WithSudo().
		WithLock("dpkg_install", 120)
	script5 := b5.Build()
	if !strings.Contains(script5, "flock -w 120 200") {
		t.Fatalf("expected flock wrapper, got:\n%s", script5)
	}
	if !strings.Contains(script5, "/tmp/tamestack_dpkg_install.lock") {
		t.Fatalf("expected lock file name, got:\n%s", script5)
	}
	if !strings.Contains(script5, "run_sudo bash -s") {
		t.Fatalf("expected the lock to run inside sudo, got:\n%s", script5)
	}
	lockAt := strings.Index(script5, "LOCKFILE=")
	invokeAt := strings.LastIndex(script5, "run_sudo bash -s")
	if lockAt < invokeAt {
		t.Fatalf("lock must be inside the elevated script, got:\n%s", script5)
	}

	b6 := client.NewCommand("apt-get update -y && apt-get install -y nginx").WithSudo()
	script6 := b6.Build()
	if strings.Contains(script6, "run_sudo apt-get update") {
		t.Fatalf("compound command must not elevate only the first segment:\n%s", script6)
	}
	if !strings.Contains(script6, "apt-get update -y && apt-get install -y nginx") {
		t.Fatalf("expected the full chain inside sudo, got:\n%s", script6)
	}
	if strings.Contains(script6, "secret") {
		t.Fatalf("password leaked:\n%s", script6)
	}

	noPass := &SSHClient{user: "developer"}
	script7 := noPass.NewCommand("id").WithSudo().Build()
	if !strings.Contains(script7, "passwordless sudo is unavailable") {
		t.Fatalf("expected a hard sudo failure without a password, got:\n%s", script7)
	}
}
