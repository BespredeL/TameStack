package apps

import (
	"context"
	"testing"

	"tamestack/internal/db"
)

func TestInstallQueueWorkerValidation(t *testing.T) {
	site := &db.SiteRecord{ID: "12345678-abcd", Domain: "example.com", DocumentRoot: "/var/www/example.com/public"}

	if err := InstallQueueWorker(context.Background(), nil, site, QueueWorkerConfig{Connection: "redis\nExecStart=/bin/sh"}); err == nil {
		t.Errorf("expected error for newline in Connection, got nil")
	}
	if err := InstallQueueWorker(context.Background(), nil, site, QueueWorkerConfig{Queue: "default;id"}); err == nil {
		t.Errorf("expected error for shell metacharacter in Queue, got nil")
	}
}
