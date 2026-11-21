package db

import (
	"context"
	"os"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"
)

func TestSnapshotToFileProducesOpenableCopy(t *testing.T) {
	ctx := context.Background()
	dir := testutil.Workspace(t)

	database, err := Open(filepath.Join(dir, "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveServer(ctx, &ServerRecord{
		ID: "srv-1", Name: "box", Host: "10.0.0.1", Port: 22,
		Username: "root", AuthType: "password", EncryptedSecret: "x", Status: "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	snapPath := filepath.Join(dir, "snapshot.db")
	if err := database.SnapshotToFile(ctx, snapPath); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := os.Stat(snapPath + ".writing"); !os.IsNotExist(err) {
		t.Fatal("temp snapshot file left behind")
	}
	database.Close()

	clone, err := Open(snapPath)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer clone.Close()
	rec, err := clone.GetServer(ctx, "srv-1")
	if err != nil || rec.Name != "box" {
		t.Fatalf("snapshot content: %+v err=%v", rec, err)
	}
}
