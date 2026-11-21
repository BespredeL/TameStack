package php

import (
	"reflect"
	"testing"
)

func TestNormalizeExtensions(t *testing.T) {
	got, err := normalizeExtensions([]string{" curl ", "CURL", "mbstring", "", "opcache", "gd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"curl", "mbstring", "gd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeExtensions = %v, want %v", got, want)
	}
}

func TestNormalizeExtensionsEmpty(t *testing.T) {
	got, err := normalizeExtensions([]string{"", "  ", "opcache"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

func TestNormalizeExtensionsRejectsInjection(t *testing.T) {
	for _, bad := range []string{"curl; rm -rf /", "curl && reboot", "curl|sh", "cu rl", "curl\nreboot"} {
		if _, err := normalizeExtensions([]string{bad}); err == nil {
			t.Fatalf("expected error for %q, got nil", bad)
		}
	}
}

func TestExtensionPackagesDebian(t *testing.T) {
	got := extensionPackages("debian", "8.2", []string{"curl", "mbstring", "gd"})
	want := []string{"php8.2-curl", "php8.2-mbstring", "php8.2-gd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extensionPackages debian = %v, want %v", got, want)
	}
}

func TestExtensionPackagesRhel(t *testing.T) {
	got := extensionPackages("rhel", "8.3", []string{"curl", "redis"})
	want := []string{"php83-php-curl", "php83-php-redis"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extensionPackages rhel = %v, want %v", got, want)
	}
}

func TestFpmServiceName(t *testing.T) {
	if got := fpmServiceName("debian", "8.2"); got != "php8.2-fpm" {
		t.Fatalf("debian fpmServiceName = %q, want php8.2-fpm", got)
	}
	if got := fpmServiceName("rhel", "8.3"); got != "php83-php-fpm" {
		t.Fatalf("rhel fpmServiceName = %q, want php83-php-fpm", got)
	}
}