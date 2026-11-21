package ssh

import (
	"context"
	"fmt"
	"os"
	"time"
)

type GuardianResult struct {
	Success      bool   `json:"success"`
	BackupPath   string `json:"backup_path"`
	TestOutput   string `json:"test_output"`
	ErrorMessage string `json:"error_message,omitempty"`
	RolledBack   bool   `json:"rolled_back"`
}

// BuildBackupPath constructs a versioned backup path for safe rollbacks.
func BuildBackupPath(remotePath string, now time.Time) string {
	ts := now.Format("20060102_150405")
	return fmt.Sprintf("%s.tamestack.bak_%s", remotePath, ts)
}

// SafeWriteWithTest writes a configuration file, executes a pre-flight syntax test command,
// and instantly rolls back to the backup if the syntax test or optional service reload fails.
func (c *SSHClient) SafeWriteWithTest(ctx context.Context, remotePath string, content []byte, testCmd string, reloadCmd string) (*GuardianResult, error) {
	now := time.Now()
	backupPath := BuildBackupPath(remotePath, now)
	tmpPath := fmt.Sprintf("/tmp/.sh_guard_%d_%d", now.UnixNano(), os.Getpid())

	res := &GuardianResult{
		BackupPath: backupPath,
	}

	exists := c.FileExists(ctx, remotePath)
	if exists {
		backupCmd := fmt.Sprintf("cp %s %s", ShellEscape(remotePath), ShellEscape(backupPath))
		if _, _, code, err := c.NewCommand(backupCmd).WithSudo().Exec(ctx); err != nil || code != 0 {
			res.ErrorMessage = "failed to create configuration backup before modification"
			return res, fmt.Errorf("%s", res.ErrorMessage)
		}
	}

	if err := c.WriteFile(ctx, tmpPath, content, 0644); err != nil {
		res.ErrorMessage = fmt.Sprintf("failed to write temporary file: %v", err)
		return res, err
	}

	moveCmd := fmt.Sprintf("cp %s %s", ShellEscape(tmpPath), ShellEscape(remotePath))
	if _, _, code, err := c.NewCommand(moveCmd).WithSudo().Exec(ctx); err != nil || code != 0 {
		_ = c.RemovePath(ctx, tmpPath, false)
		res.ErrorMessage = "failed to copy temporary file into target location"
		return res, fmt.Errorf("%s", res.ErrorMessage)
	}

	if testCmd != "" {
		outTest, errTest, codeTest, errExec := c.NewCommand(testCmd).WithSudo().Exec(ctx)
		res.TestOutput = string(outTest)
		if len(errTest) > 0 {
			res.TestOutput = fmt.Sprintf("%s\n%s", res.TestOutput, string(errTest))
		}

		if errExec != nil || codeTest != 0 {
			// Instant Rollback
			res.RolledBack = true
			if exists {
				rollbackCmd := fmt.Sprintf("cp %s %s", ShellEscape(backupPath), ShellEscape(remotePath))
				_, _, _, _ = c.NewCommand(rollbackCmd).WithSudo().Exec(ctx)
			} else {
				_ = c.RemovePath(ctx, remotePath, false)
			}
			_ = c.RemovePath(ctx, tmpPath, false)
			res.ErrorMessage = fmt.Sprintf("pre-flight test failed (code %d): %s", codeTest, res.TestOutput)
			return res, fmt.Errorf("%s", res.ErrorMessage)
		}
	}

	if reloadCmd != "" {
		outReload, errReload, codeReload, errExec := c.NewCommand(reloadCmd).WithSudo().Exec(ctx)
		if errExec != nil || codeReload != 0 {
			// Rollback on reload failure.
			res.RolledBack = true
			if exists {
				rollbackCmd := fmt.Sprintf("cp %s %s", ShellEscape(backupPath), ShellEscape(remotePath))
				_, _, _, _ = c.NewCommand(rollbackCmd).WithSudo().Exec(ctx)
				// Re-attempt reload to recover previous working state.
				_, _, _, _ = c.NewCommand(reloadCmd).WithSudo().Exec(ctx)
			} else {
				_ = c.RemovePath(ctx, remotePath, false)
				_, _, _, _ = c.NewCommand(reloadCmd).WithSudo().Exec(ctx)
			}
			_ = c.RemovePath(ctx, tmpPath, false)
			res.ErrorMessage = fmt.Sprintf("service reload failed after configuration update (code %d): %s %s", codeReload, string(outReload), string(errReload))
			return res, fmt.Errorf("%s", res.ErrorMessage)
		}
	}

	_ = c.RemovePath(ctx, tmpPath, false)
	res.Success = true
	return res, nil
}
