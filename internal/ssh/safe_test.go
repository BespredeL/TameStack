package ssh

import (
	"strings"
	"testing"
)

func TestValidatorsRejectShellMetacharacters(t *testing.T) {
	if err := ValidatePHPVersion("8.2; id"); err == nil {
		t.Fatal("php version accepted metacharacters")
	}
	if err := ValidatePHPVersion("8.2"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChmodMode("777; id"); err == nil {
		t.Fatal("chmod mode accepted metacharacters")
	}
	if err := ValidateIdent("app;drop", "database name"); err == nil {
		t.Fatal("database name accepted metacharacters")
	}
	if err := ValidateRemotePath("/", false); err == nil {
		t.Fatal("root path accepted for destructive use")
	}
	if err := ValidateRemotePath("/var/backups/tamestack", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSystemctlAction("restart;id"); err == nil {
		t.Fatal("systemctl action accepted metacharacters")
	}
}

func TestHostKeyLineRejectsNewlines(t *testing.T) {
	if err := ValidateHostKeyLine("example.com ssh-ed25519 AAAA\n* ssh-ed25519 BBBB"); err == nil {
		t.Fatal("newline host key line accepted")
	}
}

func TestShellEscapeNeutralizesPayloads(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"$(id)", "'$(id)'"},
		{"`id`", "'`id`'"},
		{"a'; DROP TABLE t;--", "'a'\\''; DROP TABLE t;--'"},
		{"a\nb", "'a\nb'"},
		{"a b", "'a b'"},
		{"/etc/nginx/$(id).conf", "'/etc/nginx/$(id).conf'"},
	}

	for _, tt := range tests {
		got := ShellEscape(tt.input)
		if got != tt.expected {
			t.Errorf("ShellEscape(%q) = %q, want %q", tt.input, got, tt.expected)
		}
		if strings.Contains(got, "\"") {
			t.Errorf("ShellEscape(%q) emitted a double quote, remote expansion possible: %q", tt.input, got)
		}
	}
}

func TestValidateServiceName(t *testing.T) {
	accepted := []string{"nginx", "apache2", "php8.1-fpm", "ssh", "app@worker.service", "unit:instance"}
	for _, name := range accepted {
		if err := ValidateServiceName(name); err != nil {
			t.Errorf("ValidateServiceName(%q) rejected: %v", name, err)
		}
	}

	rejected := []string{"", " nginx", "nginx; id", "nginx$(id)", "nginx|tee", "nginx\ngrep", "-nginx", "nginx && reboot", "nginx'"}
	for _, name := range rejected {
		if err := ValidateServiceName(name); err == nil {
			t.Errorf("ValidateServiceName(%q) accepted", name)
		}
	}
}

func TestValidateLogTypeIsWhitelist(t *testing.T) {
	for _, name := range []string{"error", "access"} {
		if err := ValidateLogType(name); err != nil {
			t.Errorf("ValidateLogType(%q) rejected: %v", name, err)
		}
	}
	for _, name := range []string{"", "Error", "error2", "$(id)", "access; rm -rf /", "error\naccess"} {
		if err := ValidateLogType(name); err == nil {
			t.Errorf("ValidateLogType(%q) accepted", name)
		}
	}
}

func TestValidatePolicyPath(t *testing.T) {
	accepted := []string{"/etc/nginx/nginx.conf", "/var/log/nginx/error.log", "/run/php/php8.2-fpm.sock", "/etc/php/8.2/fpm/pool.d/www.conf"}
	for _, p := range accepted {
		if err := ValidatePolicyPath(p); err != nil {
			t.Errorf("ValidatePolicyPath(%q) rejected: %v", p, err)
		}
	}

	rejected := []string{
		"",
		"~/.ssh/id_rsa",
		"/etc/nginx/$(id).conf",
		"/etc/nginx/`id`.conf",
		"/etc/nginx/x.conf; rm -rf /",
		"/etc/nginx/conf.d/*.conf",
		"/etc/nginx/nginx.conf\n/etc/shadow",
	}
	for _, p := range rejected {
		if err := ValidatePolicyPath(p); err == nil {
			t.Errorf("ValidatePolicyPath(%q) accepted", p)
		}
	}
}
