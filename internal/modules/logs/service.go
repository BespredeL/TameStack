package logs

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
)

type Service struct {
	module.Base
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool) *Service {
	return &Service{
		Base: module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
	}
}

// ValidateLogPath ensures the requested path is safe to read as a log file.
func ValidateLogPath(filePath string) (string, error) {
	p := strings.TrimSpace(filePath)
	if p == "" {
		return "", fmt.Errorf("путь к лог-файлу не может быть пустым")
	}
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("путь должен быть абсолютным: %s", p)
	}
	clean := path.Clean(p)

	// Block dangerous files.
	lower := strings.ToLower(clean)
	blockedSubstrings := []string{
		"/etc/shadow",
		"/etc/gshadow",
		"id_rsa",
		"id_ed25519",
		"id_ecdsa",
		"id_dsa",
		".env",
		".git",
		"wp-config.php",
		"configuration.php",
		"/proc/kcore",
		"/dev/mem",
	}
	for _, b := range blockedSubstrings {
		if strings.Contains(lower, b) {
			return "", fmt.Errorf("доступ к файлу %s заблокирован политикой безопасности", clean)
		}
	}

	return clean, nil
}

// GetTailLog fetches the last N lines of a log file.
func (s *Service) GetTailLog(ctx context.Context, serverID string, filePath string, lines int) (*LogTailResult, error) {
	cleanPath, err := ValidateLogPath(filePath)
	if err != nil {
		return nil, err
	}

	if lines <= 0 {
		lines = 100
	}
	if lines > 2000 {
		lines = 2000
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	sizeCmd := fmt.Sprintf("stat -c %%s %s 2>/dev/null || wc -c < %s 2>/dev/null || echo 0", cleanPath, cleanPath)
	stdout, _, _, _ := client.NewCommand(sizeCmd).WithSudo().Exec(ctx)
	sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(string(stdout)), 10, 64)

	tailCmd := fmt.Sprintf("tail -n %d %s", lines, cleanPath)
	outBytes, errBytes, code, err := client.NewCommand(tailCmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("ошибка чтения лог-файла: %s", strings.TrimSpace(string(errBytes)))
	}

	raw := string(outBytes)
	splitLines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var resultLines []string
	for _, l := range splitLines {
		if l != "" {
			resultLines = append(resultLines, l)
		}
	}

	return &LogTailResult{
		FilePath:  cleanPath,
		Lines:     resultLines,
		LineCount: len(resultLines),
		Truncated: len(resultLines) >= lines,
		SizeBytes: sizeBytes,
	}, nil
}
