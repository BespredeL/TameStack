//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
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

// SpawnRelaunchDeElevated starts exePath via explorer.exe so an elevated
// updater process does not pass administrator privileges to the relaunched UI.
func SpawnRelaunchDeElevated(exePath string) error {
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("обновлённый файл программы недоступен: %w", err)
	}
	cmd := exec.Command("explorer.exe", exePath)
	if err := cmd.Start(); err == nil {
		return nil
	}
	return SpawnRelaunch(exePath)
}

// SpawnElevatedApply launches sourceExe with administrator privileges (UAC)
// to replace targetExe after waitPID exits.
func SpawnElevatedApply(sourceExe, targetExe string, waitPID int) error {
	if _, err := os.Stat(sourceExe); err != nil {
		return fmt.Errorf("файл обновления недоступен: %w", err)
	}

	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	filePtr, err := syscall.UTF16PtrFromString(sourceExe)
	if err != nil {
		return err
	}
	params := fmt.Sprintf(`--apply-update --source "%s" --target "%s" --wait-pid %d`, sourceExe, targetExe, waitPID)
	paramsPtr, err := syscall.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	dirPtr, err := syscall.UTF16PtrFromString(filepath.Dir(sourceExe))
	if err != nil {
		return err
	}

	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")
	ret, _, callErr := shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(paramsPtr)),
		uintptr(unsafe.Pointer(dirPtr)),
		0, // SW_HIDE
	)
	if ret <= 32 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("запуск с правами администратора отменён или не удался (код %d): %w", ret, callErr)
		}
		return fmt.Errorf("запуск с правами администратора отменён или не удался (код %d)", ret)
	}
	return nil
}

// WaitForProcessExit waits until the process with pid terminates or timeout elapses.
func WaitForProcessExit(pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// Process already exited or cannot be opened; give OS a short grace moment.
		time.Sleep(300 * time.Millisecond)
		return
	}
	defer windows.CloseHandle(handle)

	ms := uint32(timeout.Milliseconds())
	_, _ = windows.WaitForSingleObject(handle, ms)
}
