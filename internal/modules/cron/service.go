package cron

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

type Service struct {
	module.Base
	engine *tasks.Engine
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	return &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}
}

func makeJobID(expr, cmd string) string {
	h := sha1.New()
	h.Write([]byte(expr + "::" + cmd))
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// ListCronJobs queries crontab entries for the user, plus system-wide crontab and /etc/cron.d files.
func (s *Service) ListCronJobs(ctx context.Context, serverID string, user string) ([]CronJob, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if user == "" {
		user = "root"
	}
	if user != "all" {
		if err := ssh.ValidateIdent(user, "cron user"); err != nil {
			return nil, err
		}
	}

	// Comprehensive shell script to retrieve user crontab, system /etc/crontab, and /etc/cron.d drop-ins
	script := fmt.Sprintf(`
export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
TARGET_USER=%s

echo "===USER_CRONTAB==="
if [ "$TARGET_USER" = "all" ]; then
    for f in /var/spool/cron/crontabs/* /var/spool/cron/*; do
        if [ -f "$f" ]; then
            u=$(basename "$f")
            echo "---USER:$u---"
            cat "$f" 2>/dev/null || true
        fi
    done
else
    crontab -u "$TARGET_USER" -l 2>/dev/null || ( [ "$TARGET_USER" = "root" ] && crontab -l 2>/dev/null ) || cat "/var/spool/cron/crontabs/$TARGET_USER" 2>/dev/null || cat "/var/spool/cron/$TARGET_USER" 2>/dev/null || true
fi

echo "===ETC_CRONTAB==="
cat /etc/crontab 2>/dev/null || true

echo "===CRON_D_START==="
for f in /etc/cron.d/*; do
    if [ -f "$f" ]; then
        echo "---FILE:$f---"
        cat "$f" 2>/dev/null || true
    fi
done
echo "===CRON_D_END==="
`, ssh.ShellEscape(user))

	stdout, _, _, _ := client.NewCommand(script).WithSudo().Exec(ctx)
	raw := string(stdout)

	var allJobs []CronJob
	seenIDs := make(map[string]bool)

	// 1. Parse user crontab
	if idxStart := strings.Index(raw, "===USER_CRONTAB==="); idxStart != -1 {
		idxEnd := strings.Index(raw, "===ETC_CRONTAB===")
		if idxEnd == -1 {
			idxEnd = len(raw)
		}
		userCrontabPart := raw[idxStart+len("===USER_CRONTAB===") : idxEnd]

		if user == "all" {
			scanner := bufio.NewScanner(strings.NewReader(userCrontabPart))
			var curUser string
			var curContent strings.Builder
			flushCur := func() {
				if curUser != "" && curContent.Len() > 0 {
					uJobs, _ := splitCrontab(curContent.String(), curUser)
					for _, j := range uJobs {
						j.Source = "crontab"
						if !seenIDs[j.ID] {
							seenIDs[j.ID] = true
							allJobs = append(allJobs, j)
						}
					}
				}
				curContent.Reset()
			}
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "---USER:") && strings.HasSuffix(line, "---") {
					flushCur()
					curUser = strings.TrimSuffix(strings.TrimPrefix(line, "---USER:"), "---")
					continue
				}
				curContent.WriteString(line + "\n")
			}
			flushCur()
		} else {
			uJobs, _ := splitCrontab(userCrontabPart, user)
			for _, j := range uJobs {
				j.Source = "crontab"
				if !seenIDs[j.ID] {
					seenIDs[j.ID] = true
					allJobs = append(allJobs, j)
				}
			}
		}
	}

	// 2. Parse /etc/crontab (system-wide cron)
	if idxStart := strings.Index(raw, "===ETC_CRONTAB==="); idxStart != -1 {
		idxEnd := strings.Index(raw, "===CRON_D_START===")
		if idxEnd == -1 {
			idxEnd = len(raw)
		}
		etcPart := raw[idxStart+len("===ETC_CRONTAB===") : idxEnd]
		etcJobs := parseSystemCronFile(etcPart, user, "/etc/crontab")
		for _, j := range etcJobs {
			if !seenIDs[j.ID] {
				seenIDs[j.ID] = true
				allJobs = append(allJobs, j)
			}
		}
	}

	// 3. Parse /etc/cron.d/* files
	if idxStart := strings.Index(raw, "===CRON_D_START==="); idxStart != -1 {
		if idxEnd := strings.Index(raw, "===CRON_D_END==="); idxEnd > idxStart {
			cronDPart := raw[idxStart+len("===CRON_D_START===") : idxEnd]
			scanner := bufio.NewScanner(strings.NewReader(cronDPart))
			var currentFile string
			var currentContent strings.Builder

			flushFile := func() {
				if currentFile != "" && currentContent.Len() > 0 {
					dJobs := parseSystemCronFile(currentContent.String(), user, currentFile)
					for _, j := range dJobs {
						if !seenIDs[j.ID] {
							seenIDs[j.ID] = true
							allJobs = append(allJobs, j)
						}
					}
				}
				currentContent.Reset()
			}

			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "---FILE:") && strings.HasSuffix(line, "---") {
					flushFile()
					currentFile = strings.TrimSuffix(strings.TrimPrefix(line, "---FILE:"), "---")
					continue
				}
				currentContent.WriteString(line + "\n")
			}
			flushFile()
		}
	}

	return allJobs, nil
}

func isCronSchedule(fields []string) bool {
	if len(fields) != 5 {
		return false
	}
	cronDigits := "*0123456789/,-"
	cronNameChars := "*0123456789/,-?#abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// Minute and hour: numbers, *, /, -, ,
	for _, r := range fields[0] {
		if !strings.ContainsRune(cronDigits, r) {
			return false
		}
	}
	for _, r := range fields[1] {
		if !strings.ContainsRune(cronDigits, r) {
			return false
		}
	}

	// Day of month: numbers, *, /, -, ,, L, W, ?
	domChars := "*0123456789/,-?LWlw"
	for _, r := range fields[2] {
		if !strings.ContainsRune(domChars, r) {
			return false
		}
	}

	// Month: numbers, *, /, -, ,, or names (jan, feb, ...)
	for _, r := range fields[3] {
		if !strings.ContainsRune(cronNameChars, r) {
			return false
		}
	}

	// Day of week: numbers, *, /, -, ,, or names (sun, mon, ...), L, #
	for _, r := range fields[4] {
		if !strings.ContainsRune(cronNameChars, r) {
			return false
		}
	}

	return true
}

func (s *Service) writeSystemFile(ctx context.Context, client *ssh.SSHClient, filePath, content string) error {
	marker := "TameStack_CRON_FILE"
	for strings.Contains(content, marker) {
		marker += "X"
	}
	cmd := fmt.Sprintf("cat <<'%s' > %s\n%s%s\n", marker, ssh.ShellEscape(filePath), content, marker)
	_, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to write %s: %s %v", filePath, string(stderr), err)
	}
	return nil
}

func (s *Service) toggleSystemCronJob(ctx context.Context, client *ssh.SSHClient, filePath, expr, cmd, user string, enabled bool) error {
	catCmd := fmt.Sprintf("cat %s 2>/dev/null", ssh.ShellEscape(filePath))
	stdout, _, code, err := client.NewCommand(catCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to read system cron file %s: %v", filePath, err)
	}

	lines := strings.Split(string(stdout), "\n")
	modified := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		clean := trimmed
		wasDisabled := false
		if strings.HasPrefix(clean, "#") {
			clean = strings.TrimSpace(strings.TrimPrefix(clean, "#"))
			if strings.HasPrefix(clean, "DISABLED:") {
				clean = strings.TrimSpace(strings.TrimPrefix(clean, "DISABLED:"))
				wasDisabled = true
			}
		}

		if strings.Contains(clean, cmd) && (strings.HasPrefix(clean, expr) || strings.Contains(clean, expr)) {
			if enabled && wasDisabled {
				lines[i] = clean
				modified = true
				break
			} else if !enabled && !wasDisabled {
				lines[i] = "#DISABLED: " + clean
				modified = true
				break
			}
		}
	}

	if !modified {
		return nil
	}
	return s.writeSystemFile(ctx, client, filePath, strings.Join(lines, "\n"))
}

func (s *Service) deleteSystemCronJob(ctx context.Context, client *ssh.SSHClient, filePath, expr, cmd, user string) error {
	catCmd := fmt.Sprintf("cat %s 2>/dev/null", ssh.ShellEscape(filePath))
	stdout, _, code, err := client.NewCommand(catCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to read system cron file %s: %v", filePath, err)
	}

	lines := strings.Split(string(stdout), "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		clean := trimmed
		if strings.HasPrefix(clean, "#") {
			clean = strings.TrimSpace(strings.TrimPrefix(clean, "#"))
			if strings.HasPrefix(clean, "DISABLED:") {
				clean = strings.TrimSpace(strings.TrimPrefix(clean, "DISABLED:"))
			}
		}
		if strings.Contains(clean, cmd) && (strings.HasPrefix(clean, expr) || strings.Contains(clean, expr)) {
			// Skip this line to delete
			continue
		}
		newLines = append(newLines, line)
	}

	return s.writeSystemFile(ctx, client, filePath, strings.Join(newLines, "\n"))
}

func (s *Service) updateSystemCronJob(ctx context.Context, client *ssh.SSHClient, job CronJob) error {
	catCmd := fmt.Sprintf("cat %s 2>/dev/null", ssh.ShellEscape(job.Source))
	stdout, _, code, err := client.NewCommand(catCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to read system cron file %s: %v", job.Source, err)
	}

	lines := strings.Split(string(stdout), "\n")
	modified := false
	targetID := job.ID

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		clean := trimmed
		if strings.HasPrefix(clean, "#") {
			clean = strings.TrimSpace(strings.TrimPrefix(clean, "#"))
			if strings.HasPrefix(clean, "DISABLED:") {
				clean = strings.TrimSpace(strings.TrimPrefix(clean, "DISABLED:"))
			}
		}
		fields := strings.Fields(clean)
		if len(fields) >= 3 && cronShortcuts[strings.ToLower(fields[0])] {
			expr := fields[0]
			cmd := strings.Join(fields[2:], " ")
			if makeJobID(expr, cmd) == targetID {
				if job.Enabled {
					lines[i] = fmt.Sprintf("%s %s %s", job.Expression, job.User, job.Command)
				} else {
					lines[i] = fmt.Sprintf("#DISABLED: %s %s %s", job.Expression, job.User, job.Command)
				}
				modified = true
				break
			}
		} else if len(fields) >= 7 && isCronSchedule(fields[:5]) {
			expr := strings.Join(fields[:5], " ")
			cmd := strings.Join(fields[6:], " ")
			if makeJobID(expr, cmd) == targetID {
				if job.Enabled {
					lines[i] = fmt.Sprintf("%s %s %s", job.Expression, job.User, job.Command)
				} else {
					lines[i] = fmt.Sprintf("#DISABLED: %s %s %s", job.Expression, job.User, job.Command)
				}
				modified = true
				break
			}
		}
	}

	if !modified {
		return nil
	}
	return s.writeSystemFile(ctx, client, job.Source, strings.Join(lines, "\n"))
}

// SaveCronJob creates or updates a crontab entry.
func (s *Service) SaveCronJob(ctx context.Context, req SaveCronJobRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	user := req.Job.User
	if user == "" {
		user = "root"
	}

	// If editing an existing system file cron job
	if req.Job.Source != "" && req.Job.Source != "crontab" {
		return s.updateSystemCronJob(ctx, client, req.Job)
	}

	jobs, err := s.ListCronJobs(ctx, req.ServerID, user)
	if err != nil {
		return err
	}

	found := false
	var crontabJobs []CronJob
	for _, j := range jobs {
		if j.Source == "" || j.Source == "crontab" {
			if j.ID == req.Job.ID {
				crontabJobs = append(crontabJobs, req.Job)
				found = true
			} else {
				crontabJobs = append(crontabJobs, j)
			}
		}
	}
	if !found {
		if req.Job.ID == "" {
			req.Job.ID = makeJobID(req.Job.Expression, req.Job.Command)
		}
		crontabJobs = append(crontabJobs, req.Job)
	}

	return s.writeCrontab(ctx, client, user, crontabJobs)
}

// DeleteCronJob removes a job from crontab or system cron file.
func (s *Service) DeleteCronJob(ctx context.Context, req DeleteCronJobRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	user := req.User
	if user == "" {
		user = "root"
	}

	jobs, err := s.ListCronJobs(ctx, req.ServerID, user)
	if err != nil {
		return err
	}

	var targetJob *CronJob
	for _, j := range jobs {
		if j.ID == req.JobID {
			targetJob = &j
			break
		}
	}
	if targetJob == nil {
		return nil
	}

	if targetJob.Source != "" && targetJob.Source != "crontab" {
		return s.deleteSystemCronJob(ctx, client, targetJob.Source, targetJob.Expression, targetJob.Command, targetJob.User)
	}

	var updated []CronJob
	for _, j := range jobs {
		if (j.Source == "" || j.Source == "crontab") && j.ID != req.JobID {
			updated = append(updated, j)
		}
	}

	return s.writeCrontab(ctx, client, user, updated)
}

// ToggleCronJob enables or disables a job.
func (s *Service) ToggleCronJob(ctx context.Context, req ToggleCronJobRequest) error {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return err
	}

	user := req.User
	if user == "" {
		user = "root"
	}

	jobs, err := s.ListCronJobs(ctx, req.ServerID, user)
	if err != nil {
		return err
	}

	var targetJob *CronJob
	for _, j := range jobs {
		if j.ID == req.JobID {
			targetJob = &j
			break
		}
	}
	if targetJob == nil {
		return fmt.Errorf("job not found: %s", req.JobID)
	}

	if targetJob.Source != "" && targetJob.Source != "crontab" {
		return s.toggleSystemCronJob(ctx, client, targetJob.Source, targetJob.Expression, targetJob.Command, targetJob.User, req.Enabled)
	}

	var crontabJobs []CronJob
	for _, j := range jobs {
		if j.Source == "" || j.Source == "crontab" {
			if j.ID == req.JobID {
				j.Enabled = req.Enabled
			}
			crontabJobs = append(crontabJobs, j)
		}
	}

	return s.writeCrontab(ctx, client, user, crontabJobs)
}

func buildCrontabCommand(user, body string) (string, error) {
	if err := ssh.ValidateIdent(user, "cron user"); err != nil {
		return "", err
	}
	marker := "TameStack_CRON"
	for strings.Contains(body, marker) {
		marker += "X"
	}
	return fmt.Sprintf("crontab -u %s - <<'%s'\n%s%s\n", ssh.ShellEscape(user), marker, body, marker), nil
}

func (s *Service) writeCrontab(ctx context.Context, client *ssh.SSHClient, user string, jobs []CronJob) error {
	rawCmd := fmt.Sprintf("crontab -u %s -l 2>/dev/null || ( [ %s = 'root' ] && crontab -l 2>/dev/null ) || true", ssh.ShellEscape(user), ssh.ShellEscape(user))
	rawOut, _, _, _ := client.NewCommand(rawCmd).WithSudo().Exec(ctx)
	_, preserved := splitCrontab(string(rawOut), user)

	var sb strings.Builder
	sb.WriteString("# Managed by TameStack - Do not edit manually\n")
	sb.WriteString("SHELL=/bin/bash\n")
	sb.WriteString("PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin\n\n")

	for _, j := range jobs {
		if j.Comment != "" {
			sb.WriteString(fmt.Sprintf("# %s\n", j.Comment))
		}
		if j.Enabled {
			sb.WriteString(fmt.Sprintf("%s %s\n", j.Expression, j.Command))
		} else {
			sb.WriteString(fmt.Sprintf("#DISABLED: %s %s\n", j.Expression, j.Command))
		}
	}
	if len(preserved) > 0 {
		sb.WriteString("\n# Preserved by TameStack\n")
		for _, line := range preserved {
			sb.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				sb.WriteString("\n")
			}
		}
	}

	cmd, err := buildCrontabCommand(user, sb.String())
	if err != nil {
		return err
	}
	_, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("failed to apply crontab: %s %v", string(stderr), err)
	}

	return nil
}
