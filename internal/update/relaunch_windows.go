//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	detachedProcess       = 0x00000008 // DETACHED_PROCESS
	createNewProcessGroup = 0x00000200 // CREATE_NEW_PROCESS_GROUP
)

// SpawnRelaunch starts the freshly installed executable as a detached
// independent process. It must be called from the shutdown path, after the
// DB container has been sealed and closed, so the new instance never races
// the old one for the container.
func SpawnRelaunch(exePath string) error {
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("обновлённый файл программы недоступен: %w", err)
	}
	cmd := exec.Command(exePath)
	cmd.Dir = filepath.Dir(exePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: detachedProcess | createNewProcessGroup,
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
