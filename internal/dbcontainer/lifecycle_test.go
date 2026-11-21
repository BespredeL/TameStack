package dbcontainer_test

import (
	"context"
	"path/filepath"
	"tamestack/internal/db"
	"tamestack/internal/dbcontainer"
	"tamestack/internal/testutil"
	"testing"
)

// End-to-end: real SQLite database -> snapshot -> container -> unlock -> reopen.
func TestDatabaseContainerFullLifecycle(t *testing.T) {
	ctx := context.Background()
	dir := testutil.Workspace(t)
	plainPath := filepath.Join(dir, "TameStack.db")
	container := filepath.Join(dir, "TameStack.db.enc")
	workPath := filepath.Join(dir, "run", "TameStack.db")

	database, err := db.Open(plainPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveServer(ctx, &db.ServerRecord{
		ID: "srv-1", Name: "prod-box", Host: "203.0.113.7", Port: 2222,
		Username: "root", AuthType: "password", EncryptedSecret: "blob", Status: "unknown",
	}); err != nil {
		t.Fatal(err)
	}
	snap := filepath.Join(dir, "staging.db")
	if err := database.SnapshotToFile(ctx, snap); err != nil {
		t.Fatal(err)
	}
	if err := dbcontainer.Create(container, snap, "db-master"); err != nil {
		t.Fatal(err)
	}
	database.Close()

	if err := dbcontainer.Unlock(container, workPath, "db-master"); err != nil {
		t.Fatal(err)
	}
	reopened, err := db.Open(workPath)
	if err != nil {
		t.Fatalf("reopen from container: %v", err)
	}
	rec, err := reopened.GetServer(ctx, "srv-1")
	if err != nil || rec.Host != "203.0.113.7" {
		t.Fatalf("content after container roundtrip: %+v err=%v", rec, err)
	}

	// mutate, seal with checkpoint, unlock again.
	if err := reopened.SaveServer(ctx, &db.ServerRecord{
		ID: "srv-2", Name: "stage", Host: "10.1.1.9", Port: 22,
		Username: "deploy", AuthType: "key", EncryptedSecret: "blob2", Status: "unknown",
	}); err != nil {
		t.Fatal(err)
	}
	if err := reopened.SnapshotToFile(ctx, snap); err != nil {
		t.Fatal(err)
	}
	if err := dbcontainer.Seal(container, snap, "db-master"); err != nil {
		t.Fatal(err)
	}
	reopened.Close()

	work2 := filepath.Join(dir, "run2", "TameStack.db")
	if err := dbcontainer.Unlock(container, work2, "db-master"); err != nil {
		t.Fatal(err)
	}
	again, err := db.Open(work2)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if _, err := again.GetServer(ctx, "srv-2"); err != nil {
		t.Fatalf("post-seal content missing: %v", err)
	}
}
