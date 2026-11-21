package backup

import (
	"context"
	"errors"
	"testing"

	"tamestack/internal/config"
	"tamestack/internal/db"
	"tamestack/internal/module"
)

// fakeRepo satisfies db.Store (the type of module.Base.Database) by embedding the
// interface, but implements only the methods these tests exercise. The embedded
// interface is nil on purpose: any call into unstubbed territory panics, so a
// test that drifts beyond the covered behaviour fails loudly instead of silently
// hitting a live database. This is the fake-repository seam opened by the
// per-domain db.Store interfaces — no SQLite file and no SSH server required.
type fakeRepo struct {
	db.Store

	backup    *db.BackupRecord
	getErr    error
	deleteErr error
	list      []*db.BackupRecord

	serverErr error
	deleted   []string
}

var _ db.Store = (*fakeRepo)(nil)

func (f *fakeRepo) GetBackup(_ context.Context, _ string) (*db.BackupRecord, error) {
	return f.backup, f.getErr
}

func (f *fakeRepo) DeleteBackup(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return f.deleteErr
}

func (f *fakeRepo) ListBackups(_ context.Context, _ string) ([]*db.BackupRecord, error) {
	return f.list, nil
}

// GetServer fails so GetSSHClient returns before touching the nil vault/pool,
// exercising DeleteBackup's best-effort remote-cleanup branch.
func (f *fakeRepo) GetServer(_ context.Context, _ string) (*db.ServerRecord, error) {
	return nil, f.serverErr
}

func newFakeService(repo db.Store) *Service {
	return &Service{Base: module.Base{Cfg: &config.AppConfig{}, Database: repo}}
}

func TestDeleteBackupPropagatesLookupError(t *testing.T) {
	sentinel := errors.New("backup lookup failed")
	repo := &fakeRepo{getErr: sentinel}
	srv := newFakeService(repo)

	err := srv.DeleteBackup(context.Background(), "srv-1", "b1")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the lookup error to propagate, got %v", err)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("a failed lookup must not delete anything, deleted %v", repo.deleted)
	}
}

func TestDeleteBackupStillDeletesWhenSSHUnavailable(t *testing.T) {
	repo := &fakeRepo{
		backup:    &db.BackupRecord{ID: "b1", FilePath: "/var/backups/TameStack/site_x.tar.gz"},
		serverErr: errors.New("server not found"),
	}
	srv := newFakeService(repo)

	if err := srv.DeleteBackup(context.Background(), "srv-1", "b1"); err != nil {
		t.Fatalf("remote cleanup is best-effort; deletion must succeed, got %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "b1" {
		t.Fatalf("expected the database record to be deleted despite SSH failure, deleted %v", repo.deleted)
	}
}

func TestDeleteBackupHandlesMissingRecord(t *testing.T) {
	repo := &fakeRepo{backup: nil}
	srv := newFakeService(repo)

	if err := srv.DeleteBackup(context.Background(), "srv-1", "b1"); err != nil {
		t.Fatalf("a nil record should skip remote cleanup and delete, got %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "b1" {
		t.Fatalf("expected deletion of b1, deleted %v", repo.deleted)
	}
}

func TestDeleteBackupPropagatesDeleteError(t *testing.T) {
	sentinel := errors.New("delete failed")
	repo := &fakeRepo{backup: nil, deleteErr: sentinel}
	srv := newFakeService(repo)

	if err := srv.DeleteBackup(context.Background(), "srv-1", "b1"); !errors.Is(err, sentinel) {
		t.Fatalf("expected the delete error to propagate, got %v", err)
	}
}

func TestListBackupsDelegatesToRepository(t *testing.T) {
	want := []*db.BackupRecord{{ID: "b1"}, {ID: "b2"}}
	repo := &fakeRepo{list: want}
	srv := newFakeService(repo)

	got, err := srv.ListBackups(context.Background(), "srv-1")
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d backups, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backup %d: expected the repository record to be returned unchanged", i)
		}
	}
}
