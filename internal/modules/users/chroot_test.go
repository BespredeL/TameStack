package users

import (
	"context"
	"testing"
)

func TestUsersChrootAndPasswordValidation(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	if _, err := svc.EnableSFTPChroot(ctx, "srv-1", "root", "/var/www/root", ""); err == nil {
		t.Errorf("expected error when isolating root")
	}
	if _, err := svc.EnableSFTPChroot(ctx, "srv-1", "site1", "../etc", ""); err == nil {
		t.Errorf("expected error for relative/traversal chroot path")
	}
	if _, err := svc.EnableSFTPChroot(ctx, "srv-1", "site1", "/var/www/site1", "sub;id"); err == nil {
		t.Errorf("expected error for invalid writable subdir")
	}
	if _, err := svc.SetUserPassword(ctx, "srv-1", "root", "strongpassword"); err == nil {
		t.Errorf("expected error when setting root password")
	}
	if _, err := svc.SetUserPassword(ctx, "srv-1", "site1", "short"); err == nil {
		t.Errorf("expected error for short password")
	}
}
