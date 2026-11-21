package db

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Path returns the filesystem location of the database file.
func (d *DB) Path() string {
	return d.path
}

// ChangeCounter returns SQLite's total_changes: rows inserted, updated or deleted
// since the connection opened. The pool is limited to one connection, so the value
// is stable across calls and works as a write-dirty signal. DDL and VACUUM do not
// bump it; callers that must also detect those should compare file stats as well.
func (d *DB) ChangeCounter(ctx context.Context) (int64, error) {
	var n int64
	if err := d.QueryRowContext(ctx, "SELECT total_changes()").Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to read change counter: %w", err)
	}
	return n, nil
}

// SnapshotToFile writes a consistent copy of the database to destPath.
// The connection pool is limited to one writer, so holding an immediate
// transaction blocks concurrent writes while the file is copied.
func (d *DB) SnapshotToFile(ctx context.Context, destPath string) error {
	conn, err := d.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire database connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return fmt.Errorf("failed to checkpoint database: %w", err)
	}
	// Checkpointing is not allowed inside a transaction, so the write lock is
	// taken afterwards: with a single pooled connection it blocks other writers
	// while the now-static database file is copied.
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE;"); err != nil {
		return fmt.Errorf("failed to start snapshot transaction: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			_, _ = conn.ExecContext(ctx, "ROLLBACK;")
		}
	}()

	src, err := os.Open(d.path)
	if err != nil {
		return fmt.Errorf("failed to open database file for snapshot: %w", err)
	}
	defer src.Close()

	tmp := destPath + ".writing"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	copyErr := func() error {
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("failed to copy database snapshot: %w", err)
		}
		return dst.Sync()
	}()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if _, err := conn.ExecContext(ctx, "COMMIT;"); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to commit snapshot transaction: %w", err)
	}
	rollback = false
	return os.Rename(tmp, destPath)
}
