package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/db"
	"tamestack/internal/dbcontainer"
	"tamestack/internal/testutil"
)

func TestShouldAutoLock(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		lastAct time.Time
		setting string
		want    bool
	}{
		{"zero last activity", time.Time{}, "5", false},
		{"empty setting", now.Add(-time.Hour), "", false},
		{"non-numeric setting", now.Add(-time.Hour), "abc", false},
		{"zero minutes", now.Add(-time.Hour), "0", false},
		{"negative minutes", now.Add(-time.Hour), "-3", false},
		{"below threshold", now.Add(-4 * time.Minute), "5", false},
		{"at threshold", now.Add(-5 * time.Minute), "5", true},
		{"above threshold", now.Add(-6 * time.Minute), "5", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldAutoLock(now, c.lastAct, c.setting); got != c.want {
				t.Fatalf("shouldAutoLock(%v, %q) = %v, want %v", c.lastAct, c.setting, got, c.want)
			}
		})
	}
}

func TestReadDBFingerprint(t *testing.T) {
	ws := testutil.Workspace(t)
	ctx := context.Background()

	working := filepath.Join(ws, "working.db")
	database, err := db.Open(working)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	base, err := database.ChangeCounter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, "CREATE TABLE fp_probe (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO fp_probe (v) VALUES ('a'), ('b')"); err != nil {
		t.Fatal(err)
	}

	a := &App{cfg: &config.AppConfig{WorkingDatabasePath: working}, database: database}
	fp := a.readDBFingerprint(ctx)
	if fp.changes != base+2 {
		t.Fatalf("expected change counter %d, got %d", base+2, fp.changes)
	}
	if fp.size <= 0 {
		t.Fatal("expected a non-zero main database file size")
	}
	if fp.modTime.IsZero() {
		t.Fatal("expected a non-zero modification time")
	}

	// With no database handle the counter is skipped and only file stats remain;
	// plain files keep the WAL size assertion deterministic.
	plain := filepath.Join(ws, "plain.db")
	if err := os.WriteFile(plain, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plain+"-wal", []byte("1234567890"), 0600); err != nil {
		t.Fatal(err)
	}
	a2 := &App{cfg: &config.AppConfig{WorkingDatabasePath: plain}}
	fp2 := a2.readDBFingerprint(ctx)
	if fp2.changes != 0 {
		t.Fatalf("nil database must yield a zero change counter, got %d", fp2.changes)
	}
	if fp2.size != 5 || fp2.walSize != 10 {
		t.Fatalf("expected size=5 walSize=10, got size=%d walSize=%d", fp2.size, fp2.walSize)
	}
}

// newSealTestApp builds an App with a real working database and, optionally, a
// valid encrypted container so shutdown exercises the success or failure branch.
func newSealTestApp(t *testing.T, containerExists bool) (*App, string) {
	t.Helper()
	ws := testutil.Workspace(t)
	runDir := filepath.Join(ws, "run")
	if err := os.MkdirAll(runDir, 0700); err != nil {
		t.Fatal(err)
	}
	working := filepath.Join(runDir, "tamestack.db")
	database, err := db.Open(working)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := database.ExecContext(ctx, "CREATE TABLE seal_probe (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO seal_probe (v) VALUES ('x')"); err != nil {
		t.Fatal(err)
	}

	containerPath := filepath.Join(ws, "tamestack.db.enc")
	if containerExists {
		src := filepath.Join(ws, "src.db")
		if err := database.SnapshotToFile(ctx, src); err != nil {
			t.Fatal(err)
		}
		if err := dbcontainer.Create(containerPath, src, "pw"); err != nil {
			t.Fatal(err)
		}
	}

	a := &App{
		cfg: &config.AppConfig{
			RunDir:                runDir,
			WorkingDatabasePath:   working,
			DatabaseContainerPath: containerPath,
		},
		database:  database,
		dbSession: &databaseSession{password: "pw", workingPath: working},
	}
	return a, working
}

// A failed seal must preserve the plaintext working copy and the session so no
// data is lost; the operator resolves the failure before the copy disappears.
func TestShutdownPreservesWorkingCopyOnSealFailure(t *testing.T) {
	a, working := newSealTestApp(t, false) // container absent -> Seal fails
	a.shutdown(context.Background())

	if a.dbSession == nil {
		t.Fatal("dbSession must be preserved when the seal fails")
	}
	if a.database != nil {
		t.Fatal("database handle should be closed and cleared after shutdown")
	}
	if _, err := os.Stat(working); err != nil {
		t.Fatalf("working copy must survive a failed seal: %v", err)
	}
}

func TestShutdownClearsSessionOnSuccessfulSeal(t *testing.T) {
	a, _ := newSealTestApp(t, true) // valid container -> Seal succeeds
	a.shutdown(context.Background())

	if a.dbSession != nil {
		t.Fatal("dbSession should be cleared after a successful seal")
	}
	if a.database != nil {
		t.Fatal("database should be nil after shutdown")
	}
}
