package apps

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/ssh"
)

// CheckLaravelScheduler checks whether the cron entry for artisan schedule:run exists for the user.
func CheckLaravelScheduler(ctx context.Context, client *ssh.SSHClient, projectRoot, user string) (bool, error) {
	if user == "" {
		user = "www-data"
	}
	if err := ssh.ValidateIdent(user, "cron user"); err != nil {
		return false, err
	}

	readCmd := fmt.Sprintf("crontab -u %s -l 2>/dev/null || true", ssh.ShellEscape(user))
	stdout, _, _, err := client.NewCommand(readCmd).WithSudo().Exec(ctx)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "artisan schedule:run") && strings.Contains(trimmed, projectRoot) {
			return true, nil
		}
	}
	return false, nil
}

// ToggleLaravelScheduler enables or disables the artisan schedule:run cron job for the given project.
func ToggleLaravelScheduler(ctx context.Context, client *ssh.SSHClient, projectRoot, user string, enable bool) error {
	if user == "" {
		user = "www-data"
	}
	if err := ssh.ValidateIdent(user, "cron user"); err != nil {
		return err
	}

	readCmd := fmt.Sprintf("crontab -u %s -l 2>/dev/null || true", ssh.ShellEscape(user))
	stdout, _, _, err := client.NewCommand(readCmd).WithSudo().Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to read crontab: %w", err)
	}

	rawLines := strings.Split(string(stdout), "\n")
	var newLines []string

	// Strip out any existing line targeting this project's schedule:run.
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "artisan schedule:run") && strings.Contains(trimmed, projectRoot) {
			continue // skip existing
		}
		newLines = append(newLines, line)
	}

	if enable {
		cronLine := fmt.Sprintf("* * * * * cd %s && php artisan schedule:run >> /dev/null 2>&1 # TameStack: Laravel Scheduler", projectRoot)
		newLines = append(newLines, cronLine)
	}

	body := strings.Join(newLines, "\n")
	if len(body) > 0 && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	marker := "TameStack_APPS_CRON"
	for strings.Contains(body, marker) {
		marker += "X"
	}

	applyCmd := fmt.Sprintf("crontab -u %s - <<'%s'\n%s%s\n", ssh.ShellEscape(user), marker, body, marker)
	_, stderr, code, err := client.NewCommand(applyCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to apply crontab for user %s: %s (%v)", user, string(stderr), err)
	}

	return nil
}
