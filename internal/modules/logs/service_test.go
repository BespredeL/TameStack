package logs

import "testing"

func TestValidateLogPath(t *testing.T) {
	valid := []string{
		"/var/log/nginx/error.log",
		"/var/log/syslog",
	}
	for _, p := range valid {
		if _, err := ValidateLogPath(p); err != nil {
			t.Errorf("ValidateLogPath(%q) unexpected error: %v", p, err)
		}
	}

	invalid := []string{
		"",
		"relative/path.log",
		"/var/log/nginx/error.log; id",
		"/var/log/$(whoami).log",
		"/var/log/a`id`.log",
		"/var/log/a\nid.log",
		"/etc/shadow",
		"/var/www/site/.env",
	}
	for _, p := range invalid {
		if _, err := ValidateLogPath(p); err == nil {
			t.Errorf("ValidateLogPath(%q) expected error, got nil", p)
		}
	}
}
