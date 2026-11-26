package context

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"tamestack/internal/db"
	"tamestack/internal/testutil"
)

func TestAliasMasker_ReversibleCloudMasking(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	database, err := db.Open(filepath.Join(tmpDir, "alias_test.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	_ = database.SaveServer(ctx, &db.ServerRecord{
		ID:       "srv-1",
		Name:     "Prod-Main",
		Host:     "203.0.113.42",
		Port:     22,
		Username: "deployadmin",
		AuthType: "key",
	})
	_ = database.SaveSite(ctx, &db.SiteRecord{
		ID:           "site-1",
		ServerID:     "srv-1",
		Domain:       "shop.example.org",
		DocumentRoot: "/var/www/shop.example.org",
		WebServer:    "nginx",
	})

	// Local provider must NOT mask.
	localMasker := NewAliasMasker(ctx, database, "ollama", "http://127.0.0.1:11434", true)
	if localMasker.Active() {
		t.Fatal("expected local provider masker to be inactive")
	}
	localCompat := NewAliasMasker(ctx, database, "openai_compatible", "http://127.0.0.1:8000/v1", true)
	if localCompat.Active() {
		t.Fatal("expected local openai_compatible (e.g. Unsloth/vLLM) masker to be inactive")
	}
	raw := "Check Prod-Main (203.0.113.42) for shop.example.org as deployadmin"
	if got := localMasker.Mask(raw); got != raw {
		t.Errorf("local masker modified text: %q", got)
	}

	// Cloud provider MUST mask and restore cleanly.
	cloudMasker := NewAliasMasker(ctx, database, "openai", "", true)
	if !cloudMasker.Active() {
		t.Fatal("expected cloud provider masker to be active")
	}

	masked := cloudMasker.Mask(raw + " and backend 10.20.30.40")
	for _, secret := range []string{"Prod-Main", "203.0.113.42", "shop.example.org", "deployadmin", "10.20.30.40"} {
		if strings.Contains(masked, secret) {
			t.Errorf("masked text still contains %q: %s", secret, masked)
		}
	}

	// Streaming unmasker split across token boundaries.
	su := cloudMasker.NewStreamUnmasker()
	var restored strings.Builder
	for i := 0; i < len(masked); i += 3 {
		end := min(i+3, len(masked))
		restored.WriteString(su.Feed(masked[i:end]))
	}
	restored.WriteString(su.Flush())

	want := raw + " and backend 10.20.30.40"
	if restored.String() != want {
		t.Errorf("restored = %q, want %q", restored.String(), want)
	}
}
