package security

import (
	"testing"

	"tamestack/internal/db"
)

func TestValidateSafeFilePath(t *testing.T) {
	// Valid paths
	if err := validateSafeFilePath("/var/www/site/index.php"); err != nil {
		t.Fatalf("expected valid path, got err: %v", err)
	}
	if err := validateSafeFilePath("/home/user/public_html/app.js"); err != nil {
		t.Fatalf("expected valid path, got err: %v", err)
	}

	// Invalid dangerous paths.
	if err := validateSafeFilePath("/"); err == nil {
		t.Fatal("expected error for root '/'")
	}
	if err := validateSafeFilePath("/etc"); err == nil {
		t.Fatal("expected error for '/etc'")
	}
	if err := validateSafeFilePath("/var/www/../etc/passwd"); err == nil {
		t.Fatal("expected error for path with '..'")
	}
	if err := validateSafeFilePath("relative/path.php"); err == nil {
		t.Fatal("expected error for non-absolute path")
	}
}

func TestIsUnderUploadDir(t *testing.T) {
	if !isUnderUploadDir("/var/www/site/wp-content/uploads/2026/shell.php") {
		t.Fatal("expected true for uploads directory")
	}
	if !isUnderUploadDir("/var/www/site/public/images/avatar.jpg") {
		t.Fatal("expected true for images directory")
	}
	if isUnderUploadDir("/var/www/site/app/Http/Controllers/UserController.php") {
		t.Fatal("expected false for standard app directory")
	}
}

func TestIsPathExcluded(t *testing.T) {
	exclusions := []*db.FileSecurityExclusionRecord{
		{Pattern: "*/cache/*"},
		{Pattern: "*.log"},
		{Pattern: "/var/www/site/vendor/*"},
		{Pattern: "/var/www/site/storage/app/*"},
		{Pattern: "/var/www/site/public/robots.txt"},
	}

	if !isPathExcluded("/var/www/site/storage/cache/data.tmp", exclusions) {
		t.Fatal("expected true for cache exclusion")
	}
	if !isPathExcluded("/var/www/site/debug.log", exclusions) {
		t.Fatal("expected true for *.log exclusion")
	}
	if !isPathExcluded("/var/www/site/storage/app/subfolder/file.json", exclusions) {
		t.Fatal("expected true for nested path inside /* directory wildcard")
	}
	if !isPathExcluded("/var/www/site/public/robots.txt", exclusions) {
		t.Fatal("expected true for exact file match")
	}
	if isPathExcluded("/var/www/site/public/index.php", exclusions) {
		t.Fatal("expected false for index.php")
	}
}

func TestFormatByteSize(t *testing.T) {
	if s := formatByteSize(500); s != "500 B" {
		t.Fatalf("expected '500 B', got '%s'", s)
	}
	if s := formatByteSize(2048); s != "2.0 KB" {
		t.Fatalf("expected '2.0 KB', got '%s'", s)
	}
	if s := formatByteSize(10485760); s != "10.0 MB" {
		t.Fatalf("expected '10.0 MB', got '%s'", s)
	}
}

func TestCustomSignatureValidation(t *testing.T) {
	svc := &Service{}

	// Missing name
	_, err := svc.SaveFileSignature(nil, FileSecuritySignature{Pattern: "test"})
	if err == nil {
		t.Fatal("expected error on empty name")
	}

	// Missing pattern
	_, err = svc.SaveFileSignature(nil, FileSecuritySignature{Name: "Rule 1", Pattern: ""})
	if err == nil {
		t.Fatal("expected error on empty pattern")
	}

	// Invalid regex
	_, err = svc.SaveFileSignature(nil, FileSecuritySignature{Name: "Rule 1", Pattern: "[a-z"})
	if err == nil {
		t.Fatal("expected error on unclosed regex bracket")
	}
}
