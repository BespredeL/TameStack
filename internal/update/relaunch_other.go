//go:build !windows

package update

import "fmt"

// SpawnRelaunch is a no-op stub for non-Windows builds; the self-update flow
// itself refuses to install there.
func SpawnRelaunch(exePath string) error {
	return fmt.Errorf("перезапуск после обновления поддерживается только в Windows")
}
