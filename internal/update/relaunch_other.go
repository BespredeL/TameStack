//go:build !windows

package update

import (
	"fmt"
	"time"
)

// SpawnRelaunch is a no-op stub for non-Windows builds; the self-update flow
// itself refuses to install there.
func SpawnRelaunch(exePath string) error {
	return fmt.Errorf("перезапуск после обновления поддерживается только в Windows")
}

// SpawnRelaunchDeElevated is a no-op stub for non-Windows builds.
func SpawnRelaunchDeElevated(exePath string) error {
	return SpawnRelaunch(exePath)
}

// SpawnElevatedApply is a no-op stub for non-Windows builds.
func SpawnElevatedApply(sourceExe, targetExe string, waitPID int) error {
	return fmt.Errorf("обновление с правами администратора поддерживается только в Windows")
}

// WaitForProcessExit is a no-op stub for non-Windows builds.
func WaitForProcessExit(pid int, timeout time.Duration) {}
