package services

import (
	"context"
	"testing"
)

func TestCheckDNSValidation(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	badDomains := []string{
		"",
		"example.com; id",
		"example.com | cat /etc/passwd",
		"$(whoami).example.com",
		"example.com`id`",
		"example.com\nrm -rf /",
	}
	for _, dom := range badDomains {
		if _, err := svc.CheckDNS(ctx, "srv-1", dom, "A"); err == nil {
			t.Errorf("CheckDNS(%q, A) expected error, got nil", dom)
		}
	}

	if _, err := svc.CheckDNS(ctx, "srv-1", "example.com", "A;id"); err == nil {
		t.Errorf("CheckDNS with invalid record type expected error, got nil")
	}
}

func TestCheckPortValidation(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	badHosts := []string{
		"",
		"127.0.0.1; id",
		"127.0.0.1 && whoami",
		"$(id)",
		"host/path",
	}
	for _, h := range badHosts {
		if _, err := svc.CheckPort(ctx, "srv-1", h, 80); err == nil {
			t.Errorf("CheckPort(%q, 80) expected error, got nil", h)
		}
	}

	if _, err := svc.CheckPort(ctx, "srv-1", "127.0.0.1", 0); err == nil {
		t.Errorf("CheckPort with port 0 expected error, got nil")
	}
	if _, err := svc.CheckPort(ctx, "srv-1", "127.0.0.1", 70000); err == nil {
		t.Errorf("CheckPort with port 70000 expected error, got nil")
	}
}
