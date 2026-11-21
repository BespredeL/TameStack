package logger_test

import (
	"archive/zip"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tamestack/internal/logger"
)

func TestLoggerScrubbing(t *testing.T) {
	raw := `Server connection: password="SuperSecret123", token: "my-api-key-999"`
	clean := logger.ScrubLogText(raw)

	if strings.Contains(clean, "SuperSecret123") {
		t.Errorf("Password was not scrubbed: %s", clean)
	}
	if strings.Contains(clean, "my-api-key-999") {
		t.Errorf("Token was not scrubbed: %s", clean)
	}
	if !strings.Contains(clean, "[REDACTED_SECRET]") {
		t.Errorf("Expected [REDACTED_SECRET], got: %s", clean)
	}
}

func TestLoggerLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tamestack_logger_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "test.log")
	log, err := logger.Init(logFile, "INFO")
	if err != nil {
		t.Fatalf("Failed to init logger: %v", err)
	}

	log.Info("Hello TameStack Production", slog.String("password", "SecretValue123"))
	logger.SetLogLevel("DEBUG")
	log.Debug("Debug trace info")

	// Read written log file.
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "Hello TameStack Production") {
		t.Errorf("Expected log message not found in file")
	}
	if strings.Contains(content, "SecretValue123") {
		t.Errorf("Secret value leaked into log file: %s", content)
	}

	// Test Support Bundle creation.
	bundlePath := filepath.Join(tempDir, "support_bundle.zip")
	diag := map[string]interface{}{
		"os":      "windows",
		"version": "1.1.5",
	}
	if err := logger.GenerateSupportBundle(bundlePath, diag); err != nil {
		t.Fatalf("Failed to generate support bundle: %v", err)
	}

	// Verify zip contents.
	r, err := zip.OpenReader(bundlePath)
	if err != nil {
		t.Fatalf("Failed to open support bundle zip: %v", err)
	}
	defer r.Close()

	var foundDiag, foundLog bool
	for _, f := range r.File {
		if f.Name == "tamestack_diagnostics.json" {
			foundDiag = true
		}
		if f.Name == "tamestack.log" {
			foundLog = true
			rc, _ := f.Open()
			buf, _ := io.ReadAll(rc)
			rc.Close()
			if strings.Contains(string(buf), "SecretValue123") {
				t.Errorf("Leaked secret found inside support bundle log!")
			}
		}
	}

	if !foundDiag || !foundLog {
		t.Errorf("Support bundle missing files: diag=%v, log=%v", foundDiag, foundLog)
	}
}
