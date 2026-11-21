package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/testutil"
)

func TestDBChangeCounterTracksWrites(t *testing.T) {
	database, err := db.Open(filepath.Join(testutil.Workspace(t), "counter.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	base, err := database.ChangeCounter(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.ExecContext(ctx, "CREATE TABLE counter_probe (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatal(err)
	}
	afterDDL, err := database.ChangeCounter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if afterDDL != base {
		t.Fatalf("DDL must not bump total_changes: %d -> %d", base, afterDDL)
	}

	if _, err := database.ExecContext(ctx, "INSERT INTO counter_probe (v) VALUES ('a'), ('b')"); err != nil {
		t.Fatal(err)
	}
	afterInsert, err := database.ChangeCounter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if afterInsert != afterDDL+2 {
		t.Fatalf("expected +2 after inserting two rows, got %d -> %d", afterDDL, afterInsert)
	}
}

func TestDBChangedSinceDetectsCheckpointedWrites(t *testing.T) {
	base := dbFingerprint{changes: 10, size: 4096, modTime: time.Now(), walSize: 800}

	// The original bug: WAL auto-checkpoint empties the WAL while the main
	// file changed; a WAL-only gate would skip the re-seal.
	checkpointed := base
	checkpointed.walSize = 0
	checkpointed.size = 8192
	if !dbChangedSince(checkpointed, base) {
		t.Fatal("checkpointed write with emptied WAL must be detected via main file stats")
	}

	sameSecond := base
	sameSecond.modTime = base.modTime.Add(500 * time.Millisecond)
	if !dbChangedSince(sameSecond, base) {
		t.Fatal("mtime change must be detected even when size is unchanged")
	}

	dml := base
	dml.changes = 11
	if !dbChangedSince(dml, base) {
		t.Fatal("DML counter change must be detected")
	}

	if dbChangedSince(base, base) {
		t.Fatal("identical fingerprints must not trigger a re-seal")
	}
}
