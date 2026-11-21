package ssl

import (
	"context"
	"path/filepath"
	"tamestack/internal/testutil"
	"testing"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

func TestSSLMetricsAndStorage(t *testing.T) {
	tmpDir := testutil.Workspace(t)
	dbPath := filepath.Join(tmpDir, "test_ssl.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// 1. Mock server.
	srv := &db.ServerRecord{
		ID:              "srv-ssl-test",
		Name:            "SSL Test Server",
		Host:            "10.10.10.1",
		Port:            22,
		Username:        "root",
		AuthType:        "password",
		EncryptedSecret: "secret",
	}
	if err := database.SaveServer(ctx, srv); err != nil {
		t.Fatalf("failed to save server: %v", err)
	}

	// 2. Insert test certificates.
	now := time.Now()
	expiring := now.Add(15 * 24 * time.Hour) // 15 days
	valid := now.Add(80 * 24 * time.Hour)    // 80 days

	c1 := &db.SSLCertificateRecord{
		ID:            "cert-1",
		ServerID:      "srv-ssl-test",
		PrimaryDomain: "example.com",
		SanDomains:    []string{"www.example.com"},
		CertPath:      "/etc/letsencrypt/live/example.com/fullchain.pem",
		KeyPath:       "/etc/letsencrypt/live/example.com/privkey.pem",
		Issuer:        "Let's Encrypt",
		ValidFrom:     &now,
		ValidTo:       &valid,
		DaysRemaining: 80,
		AutoRenew:     true,
		Status:        "valid",
	}
	if err := database.SaveSSLCertificate(ctx, c1); err != nil {
		t.Fatalf("failed to save cert: %v", err)
	}

	c2 := &db.SSLCertificateRecord{
		ID:            "cert-2",
		ServerID:      "srv-ssl-test",
		PrimaryDomain: "old.example.com",
		SanDomains:    []string{},
		CertPath:      "/etc/letsencrypt/live/old.example.com/fullchain.pem",
		KeyPath:       "/etc/letsencrypt/live/old.example.com/privkey.pem",
		Issuer:        "Let's Encrypt",
		ValidFrom:     &now,
		ValidTo:       &expiring,
		DaysRemaining: 15,
		AutoRenew:     true,
		Status:        "expiring_soon",
	}
	if err := database.SaveSSLCertificate(ctx, c2); err != nil {
		t.Fatalf("failed to save cert: %v", err)
	}

	saltPath := filepath.Join(tmpDir, "vault.salt")
	appCfg := &config.AppConfig{
		DataDir:        tmpDir,
		DatabasePath:   dbPath,
		KnownHostsPath: filepath.Join(tmpDir, "known_hosts"),
		SaltPath:       saltPath,
	}
	vault, _ := crypto.NewVault(saltPath, filepath.Join(tmpDir, "vault.key"))
	pool := ssh.NewPool()
	engine := tasks.NewEngine(database, 2)
	service := NewService(appCfg, database, vault, pool, engine)

	metrics, err := service.GetMetrics(ctx, "srv-ssl-test")
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}

	if metrics.TotalCertificates != 2 {
		t.Fatalf("expected 2 certificates, got %d", metrics.TotalCertificates)
	}
	if metrics.ExpiringSoonCount != 1 {
		t.Fatalf("expected 1 expiring soon, got %d", metrics.ExpiringSoonCount)
	}
	if metrics.ValidCount != 1 {
		t.Fatalf("expected 1 valid, got %d", metrics.ValidCount)
	}
}
