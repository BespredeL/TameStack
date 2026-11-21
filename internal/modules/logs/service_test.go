package logs

import (
	"testing"
)

func TestValidateLogPath(t *testing.T) {
	valid := []string{
		"/var/log/nginx/access.log",
		"/var/log/apache2/error.log",
		"/var/log/syslog",
		"/var/log/php8.2-fpm.log",
		"/var/www/site/logs/app.log",
	}
	for _, p := range valid {
		clean, err := ValidateLogPath(p)
		if err != nil {
			t.Errorf("expected valid path %s, got error: %v", p, err)
		}
		if clean != p {
			t.Errorf("expected clean == p, got %s vs %s", clean, p)
		}
	}

	invalid := []string{
		"",
		"relative/path.log",
		"/etc/shadow",
		"/var/log/../../etc/shadow",
		"/home/user/.ssh/id_rsa",
		"/var/www/site/.env",
		"/var/www/html/wp-config.php",
	}
	for _, p := range invalid {
		if _, err := ValidateLogPath(p); err == nil {
			t.Errorf("expected error for path %s, got nil", p)
		}
	}
}
