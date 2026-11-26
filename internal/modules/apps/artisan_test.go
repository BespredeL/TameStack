package apps

import (
	"context"
	"testing"

	"tamestack/internal/db"
)

func TestRunArtisanCommandValidation(t *testing.T) {
	site := &db.SiteRecord{ID: "site-1", DocumentRoot: "/var/www/app/public"}

	if _, err := RunArtisanCommand(context.Background(), nil, site, "migrate; id", true); err == nil {
		t.Errorf("expected error for shell metacharacters in artisan command")
	}
	if _, err := RunArtisanCommand(context.Background(), nil, site, "tinker", true); err == nil {
		t.Errorf("expected error for non-whitelisted artisan command")
	}
	if _, err := RunArtisanCommand(context.Background(), nil, site, "migrate", false); err == nil {
		t.Errorf("expected error for dangerous artisan command without confirmation")
	}
}
