package logger

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"tamestack/internal/secretredact"
)

var (
	globalLogger   *slog.Logger
	logLevelVar    *slog.LevelVar
	rotatingWriter *lumberjack.Logger
	currentLogPath string
	mu             sync.RWMutex
)

// RedactingWriter scrubs sensitive data from byte stream before writing.
type RedactingWriter struct {
	target io.Writer
}

func (rw *RedactingWriter) Write(p []byte) (n int, err error) {
	origLen := len(p)
	clean := ScrubLogText(string(p))
	_, err = rw.target.Write([]byte(clean))
	if err != nil {
		return 0, err
	}
	return origLen, nil
}

// ScrubLogText masks passwords, private keys, and tokens.
func ScrubLogText(input string) string {
	return secretredact.ScrubLogText(input)
}

// Init initializes the global application logger and wires it as slog.Default.
// Infrastructure code logs through the package-level slog functions (slog.Info,
// slog.Error, ...), which route to this redacting handler; do not construct
// separate handlers that would bypass secret redaction.
func Init(logFilePath string, initialLevel string) (*slog.Logger, error) {
	mu.Lock()
	defer mu.Unlock()

	dir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	currentLogPath = logFilePath
	rotatingWriter = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // megabytes
		MaxBackups: 5,
		MaxAge:     30, // days
		Compress:   true,
	}

	logLevelVar = &slog.LevelVar{}
	setLogLevelLocked(initialLevel)

	fileRedacted := &RedactingWriter{target: rotatingWriter}
	stdoutRedacted := &RedactingWriter{target: os.Stdout}

	multiWriter := io.MultiWriter(stdoutRedacted, fileRedacted)

	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level: logLevelVar,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format(time.RFC3339))
			}
			if a.Value.Kind() == slog.KindString {
				return slog.String(a.Key, ScrubLogText(a.Value.String()))
			}
			return a
		},
	})

	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)

	globalLogger.Info("TameStack Logger initialized",
		slog.String("path", logFilePath),
		slog.String("level", logLevelVar.Level().String()),
	)

	return globalLogger, nil
}

// SetLogLevel updates the logging level dynamically.
func SetLogLevel(levelStr string) {
	mu.Lock()
	defer mu.Unlock()
	setLogLevelLocked(levelStr)
}

func setLogLevelLocked(levelStr string) {
	if logLevelVar == nil {
		return
	}
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "DEBUG":
		logLevelVar.Set(slog.LevelDebug)
	case "WARN", "WARNING":
		logLevelVar.Set(slog.LevelWarn)
	case "ERROR":
		logLevelVar.Set(slog.LevelError)
	default:
		logLevelVar.Set(slog.LevelInfo)
	}
}

// GetLogPath returns the path to the current active log file.
func GetLogPath() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLogPath
}

// GenerateSupportBundle creates a zip archive containing sanitized logs and system diagnostics.
func GenerateSupportBundle(destZipPath string, diagnostics map[string]interface{}) error {
	mu.RLock()
	logPath := currentLogPath
	mu.RUnlock()

	zipFile, err := os.Create(destZipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	diagJSON, err := json.MarshalIndent(diagnostics, "", "  ")
	if err == nil {
		w, err := zipWriter.Create("tamestack_diagnostics.json")
		if err == nil {
			_, _ = w.Write(diagJSON)
		}
	}

	if logPath != "" {
		if logBytes, err := os.ReadFile(logPath); err == nil {
			sanitized := ScrubLogText(string(logBytes))
			w, err := zipWriter.Create("tamestack.log")
			if err == nil {
				_, _ = w.Write([]byte(sanitized))
			}
		}
	}

	return nil
}
