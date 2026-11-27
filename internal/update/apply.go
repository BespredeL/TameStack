package update

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HandleApplyUpdateArgs checks whether the process was launched in elevated
// self-update mode (--apply-update). If so, it waits for the parent process to
// exit, swaps the verified binary into target, cleans up temporary files,
// relaunches the application without elevation, and returns true.
func HandleApplyUpdateArgs(args []string) bool {
	hasFlag := false
	for _, a := range args {
		if a == "--apply-update" {
			hasFlag = true
			break
		}
	}
	if !hasFlag {
		return false
	}

	fs := flag.NewFlagSet("apply-update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var apply bool
	var source string
	var target string
	var waitPID int
	fs.BoolVar(&apply, "apply-update", false, "")
	fs.StringVar(&source, "source", "", "")
	fs.StringVar(&target, "target", "", "")
	fs.IntVar(&waitPID, "wait-pid", 0, "")
	if err := fs.Parse(args); err != nil || !apply {
		return true
	}

	source = filepath.Clean(strings.TrimSpace(source))
	target = filepath.Clean(strings.TrimSpace(target))
	if source == "" || target == "" ||
		!strings.EqualFold(filepath.Ext(source), ".exe") ||
		!strings.EqualFold(filepath.Ext(target), ".exe") {
		return true
	}

	if waitPID > 0 {
		WaitForProcessExit(waitPID, 15*time.Second)
	}

	// Retry swap briefly in case Windows releases the executable file lock asynchronously.
	var swapErr error
	for attempt := 0; attempt < 6; attempt++ {
		swapErr = swapBinary(source, target)
		if swapErr == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if swapErr == nil {
		// Best-effort cleanup of the .old backup and staging folder.
		removeWithRetry(target + ".old")
		stagingDir := filepath.Dir(source)
		if strings.HasPrefix(filepath.Base(stagingDir), "tamestack-update-") {
			_ = os.RemoveAll(stagingDir)
		}
		_ = SpawnRelaunchDeElevated(target)
	}

	return true
}
