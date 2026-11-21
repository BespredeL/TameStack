package apps

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

var secretKeyPattern = regexp.MustCompile(`(?i)(KEY|SECRET|PASSWORD|PASS|TOKEN|AUTH|CREDENTIALS|PRIVATE|SIGNATURE)`)

// ScrubEnvForLogging hides sensitive values from task logs.
func ScrubEnvForLogging(content string) string {
	lines := strings.Split(content, "\n")
	var scrubbed []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			scrubbed = append(scrubbed, line)
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			if secretKeyPattern.MatchString(k) {
				scrubbed = append(scrubbed, fmt.Sprintf("%s=********", k))
				continue
			}
		}
		scrubbed = append(scrubbed, line)
	}
	return strings.Join(scrubbed, "\n")
}

// GetEnvFile reads the site .env file.
func GetEnvFile(ctx context.Context, client *ssh.SSHClient, site *db.SiteRecord) (*EnvFileResult, error) {
	projectRoot := ResolveProjectRoot(site.DocumentRoot)
	envPath := fmt.Sprintf("%s/.env", projectRoot)

	if !client.FileExists(ctx, envPath) {
		// fallback to .env.example if exists.
		examplePath := fmt.Sprintf("%s/.env.example", projectRoot)
		if client.FileExists(ctx, examplePath) {
			content, err := client.ReadFile(ctx, examplePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read .env.example: %w", err)
			}
			return &EnvFileResult{
				SiteID:  site.ID,
				Path:    examplePath,
				Content: string(content),
			}, nil
		}
		return nil, fmt.Errorf("file .env does not exist at %s", envPath)
	}

	content, err := client.ReadFile(ctx, envPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}

	return &EnvFileResult{
		SiteID:  site.ID,
		Path:    envPath,
		Content: string(content),
	}, nil
}

// SaveEnvFile creates an automated backup and updates .env atomically with restricted permissions.
func SaveEnvFile(ctx context.Context, client *ssh.SSHClient, site *db.SiteRecord, content string) error {
	projectRoot := ResolveProjectRoot(site.DocumentRoot)
	envPath := fmt.Sprintf("%s/.env", projectRoot)

	if client.FileExists(ctx, envPath) {
		backupPath := fmt.Sprintf("%s/.env.bak.%d", projectRoot, time.Now().Unix())
		cpCmd := fmt.Sprintf("cp %s %s", ssh.ShellEscape(envPath), ssh.ShellEscape(backupPath))
		_, _, _, _ = client.NewCommand(cpCmd).WithSudo().Exec(ctx)
	}

	if err := client.WriteFile(ctx, envPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	chownCmd := fmt.Sprintf("chown www-data:www-data %s 2>/dev/null || chown apache:apache %s 2>/dev/null || chown nginx:nginx %s 2>/dev/null; chmod 600 %s",
		ssh.ShellEscape(envPath),
		ssh.ShellEscape(envPath),
		ssh.ShellEscape(envPath),
		ssh.ShellEscape(envPath),
	)
	_, _, _, _ = client.NewCommand(chownCmd).WithSudo().Exec(ctx)

	return nil
}
