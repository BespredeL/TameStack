package mail

import (
	"context"
	"testing"
)

func TestMailValidation(t *testing.T) {
	if err := ValidateEmail("admin@example.com"); err != nil {
		t.Errorf("unexpected error for valid email: %v", err)
	}
	for _, bad := range []string{"", "not-an-email", "a@b;id.com", "a@b\n.com"} {
		if err := ValidateEmail(bad); err == nil {
			t.Errorf("ValidateEmail(%q) expected error, got nil", bad)
		}
	}

	svc := &Service{}
	if _, err := svc.CreateMailbox(context.Background(), "srv-1", "info@example.com", "short", 0); err == nil {
		t.Errorf("expected error for short mailbox password")
	}
}
