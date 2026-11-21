// Package testutil provides shared helpers for tests. It is only ever linked
// into test binaries.
package testutil

import (
	"os"
	"testing"
	"time"
)

// Workspace returns a fresh temp directory whose cleanup tolerates the
// Windows race where SQLite WAL/shm handles are released slightly after
// db.Close(), which makes the stock t.TempDir RemoveAll fail with
// "directory is not empty" under parallel package load.
func Workspace(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for attempt := 0; attempt < 10; attempt++ {
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Errorf("failed to remove temp workspace %s", dir)
	})
	return dir
}
