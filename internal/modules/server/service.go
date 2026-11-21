package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/osadapter"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"

	"github.com/google/uuid"
)

// ServerInput is used when creating or updating a server.
type ServerInput struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	Username     string   `json:"username"`
	AuthType     string   `json:"auth_type"` // "password", "key", "key_passphrase"
	Secret       string   `json:"secret"`    // Plaintext secret from UI
	Passphrase   string   `json:"passphrase"`
	SudoPassword string   `json:"sudo_password"`
	SSHHostKey   string   `json:"ssh_host_key"`
	GroupName    string   `json:"group_name"`
	Tags         []string `json:"tags"`
	IsFavorite   bool     `json:"is_favorite"`
}

// Service provides high-level business operations for managing servers.
type Service struct {
	cfg      *config.AppConfig
	database *db.DB
	vault    *crypto.Vault
	pool     *ssh.Pool
	engine   *tasks.Engine
}

// NewService instantiates ServerService and registers its task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		cfg:      cfg,
		database: database,
		vault:    vault,
		pool:     pool,
		engine:   engine,
	}

	// Register the system diagnostics task handler.
	engine.RegisterHandler("system_diag", s.handleDiagnosticsTask)

	return s
}

// ListServers returns all configured servers.
func (s *Service) ListServers(ctx context.Context) ([]*db.ServerRecord, error) {
	return s.database.ListServers(ctx)
}

// GetServer returns a single server by ID.
func (s *Service) GetServer(ctx context.Context, id string) (*db.ServerRecord, error) {
	return s.database.GetServer(ctx, id)
}

// InspectHostKey probes the remote SSH port and checks it against known_hosts.
func (s *Service) InspectHostKey(host string, port int) (*ssh.HostKeyInspection, error) {
	return ssh.InspectHostKey(s.cfg.KnownHostsPath, host, port, 10*time.Second)
}

// AcceptHostKey saves the approved host key into the user's known_hosts file.
func (s *Service) AcceptHostKey(rawEntry string) error {
	return ssh.TrustAndSaveHostKey(s.cfg.KnownHostsPath, rawEntry)
}

// AddServer creates and persists a new server.
func (s *Service) AddServer(ctx context.Context, in ServerInput) (*db.ServerRecord, error) {
	if in.Host == "" {
		return nil, errors.New("host is required")
	}
	if in.Name == "" {
		in.Name = in.Host
	}
	if in.Port <= 0 {
		in.Port = 22
	}
	if in.Username == "" {
		in.Username = "root"
	}
	if in.GroupName == "" {
		in.GroupName = "Default"
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}

	encryptedSecret, err := s.vault.Encrypt(in.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	encryptedSudoSecret := ""
	if in.SudoPassword != "" {
		enc, err := s.vault.Encrypt(in.SudoPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt sudo password: %w", err)
		}
		encryptedSudoSecret = enc
	} else if in.AuthType == "password" {
		encryptedSudoSecret = encryptedSecret
	}

	encryptedPassphrase := ""
	if in.AuthType == "key_passphrase" {
		if in.Passphrase == "" {
			return nil, fmt.Errorf("passphrase is required for key_passphrase authentication")
		}
		enc, err := s.vault.Encrypt(in.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key passphrase: %w", err)
		}
		encryptedPassphrase = enc
	}

	record := &db.ServerRecord{
		ID:                     uuid.New().String(),
		Name:                   in.Name,
		Host:                   in.Host,
		Port:                   in.Port,
		Username:               in.Username,
		AuthType:               in.AuthType,
		EncryptedSecret:        encryptedSecret,
		EncryptedSudoSecret:    encryptedSudoSecret,
		EncryptedKeyPassphrase: encryptedPassphrase,
		SSHHostKey:             in.SSHHostKey,
		GroupName:              in.GroupName,
		Tags:                   in.Tags,
		IsFavorite:             in.IsFavorite,
		Status:                 "unknown",
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	if err := s.database.SaveServer(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to save server: %w", err)
	}

	return record, nil
}

// UpdateServer modifies an existing server.
func (s *Service) UpdateServer(ctx context.Context, in ServerInput) (*db.ServerRecord, error) {
	existing, err := s.database.GetServer(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	existing.Name = in.Name
	existing.Host = in.Host
	existing.Port = in.Port
	existing.Username = in.Username
	existing.AuthType = in.AuthType
	existing.GroupName = in.GroupName
	existing.Tags = in.Tags
	existing.IsFavorite = in.IsFavorite
	existing.SSHHostKey = in.SSHHostKey

	// Always evict previous connection from pool so updated IP/port/credentials take effect immediately.
	s.pool.Invalidate(in.ID)

	// If secret was changed, encrypt new secret.
	if in.Secret != "" {
		enc, err := s.vault.Encrypt(in.Secret)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt updated secret: %w", err)
		}
		existing.EncryptedSecret = enc
		if in.AuthType == "password" && in.SudoPassword == "" {
			existing.EncryptedSudoSecret = enc
		}
	}

	if in.SudoPassword != "" {
		enc, err := s.vault.Encrypt(in.SudoPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt sudo password: %w", err)
		}
		existing.EncryptedSudoSecret = enc
	}

	if in.Passphrase != "" {
		enc, err := s.vault.Encrypt(in.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key passphrase: %w", err)
		}
		existing.EncryptedKeyPassphrase = enc
	} else if in.AuthType != "key_passphrase" {
		existing.EncryptedKeyPassphrase = ""
	}

	existing.UpdatedAt = time.Now()

	if err := s.database.SaveServer(ctx, existing); err != nil {
		return nil, fmt.Errorf("failed to update server: %w", err)
	}

	return existing, nil
}

// SetSudoPassword updates the encrypted sudo password for a server.
func (s *Service) SetSudoPassword(ctx context.Context, serverID string, sudoPassword string) error {
	enc, err := s.vault.Encrypt(sudoPassword)
	if err != nil {
		return fmt.Errorf("failed to encrypt sudo password: %w", err)
	}
	s.pool.Invalidate(serverID)
	return s.database.UpdateServerSudoSecret(ctx, serverID, enc)
}

// DeleteServer removes a server and evicts its connection from the pool.
func (s *Service) DeleteServer(ctx context.Context, id string) error {
	s.pool.Invalidate(id)
	return s.database.DeleteServer(ctx, id)
}

// TestConnection attempts to authenticate and establish an SSH connection.
func (s *Service) TestConnection(ctx context.Context, serverID string) (bool, string, error) {
	record, err := s.database.GetServer(ctx, serverID)
	if err != nil {
		return false, "", fmt.Errorf("server %s not found: %w", serverID, err)
	}

	opts, err := ssh.OptionsForServer(s.vault, record, s.cfg.KnownHostsPath, 10*time.Second)
	if err != nil {
		return false, "", err
	}

	_, err = s.pool.Get(ctx, record.ID, opts)
	if err != nil {
		_ = s.database.UpdateServerStatus(ctx, record.ID, "offline")
		return false, err.Error(), nil
	}

	_ = s.database.UpdateServerStatus(ctx, record.ID, "online")
	return true, fmt.Sprintf("Successfully connected to %s@%s:%d", record.Username, record.Host, record.Port), nil
}

// TriggerDiagnostics queues a full diagnostic run for the server.
func (s *Service) TriggerDiagnostics(serverID string) (*db.TaskRecord, error) {
	record, err := s.database.GetServer(context.Background(), serverID)
	if err != nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	payload, _ := json.Marshal(map[string]string{"server_id": record.ID})
	return s.engine.Submit(record.ID, "system_diag", fmt.Sprintf("Diagnostics for %s (%s)", record.Name, record.Host), string(payload))
}

// GetDiagnostics returns cached telemetry for the server.
func (s *Service) GetDiagnostics(ctx context.Context, serverID string) (*db.DiagnosticsRecord, error) {
	return s.database.GetDiagnostics(ctx, serverID)
}

// handleDiagnosticsTask is the worker task that connects and scans the remote server.
func (s *Service) handleDiagnosticsTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	logger.SetStep("Connecting to server via SSH", 10)
	record, err := s.database.GetServer(ctx, task.ServerID)
	if err != nil {
		return "", fmt.Errorf("server %s not found: %w", task.ServerID, err)
	}

	opts, err := ssh.OptionsForServer(s.vault, record, s.cfg.KnownHostsPath, 15*time.Second)
	if err != nil {
		return "", err
	}

	client, err := s.pool.Get(ctx, record.ID, opts)
	if err != nil {
		_ = s.database.UpdateServerStatus(ctx, record.ID, "offline")
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	_ = s.database.UpdateServerStatus(ctx, record.ID, "online")

	logger.SetStep("Detecting Operating System distribution", 30)
	adapter, osInfo, err := osadapter.DetectAndCreateAdapter(ctx, client)
	if err != nil {
		return "", fmt.Errorf("OS detection failed: %w", err)
	}
	logger.Info(fmt.Sprintf("Detected OS: %s (Family: %s, Kernel: %s)", osInfo.PrettyName, osInfo.Family, osInfo.Kernel))

	logger.SetStep("Collecting hardware metrics", 60)
	hw, err := adapter.CollectHardware(ctx, client)
	if err != nil {
		return "", fmt.Errorf("hardware collection failed: %w", err)
	}
	logger.Info(fmt.Sprintf("Hardware: %d cores, %s, RAM total: %d MB", hw.CPUCores, hw.CPUModel, hw.RAMTotalBytes/(1024*1024)))

	logger.SetStep("Scanning installed software & services", 85)
	sw, err := adapter.DetectSoftware(ctx, client)
	if err != nil {
		logger.Warn(fmt.Sprintf("Software detection warning: %v", err))
		sw = &osadapter.InstalledSoftware{}
	}

	diag := &db.DiagnosticsRecord{
		ServerID:             record.ID,
		OSName:               osInfo.PrettyName,
		OSVersion:            osInfo.Version,
		OSFamily:             osInfo.Family,
		KernelVersion:        osInfo.Kernel,
		Arch:                 osInfo.Arch,
		CPUCores:             hw.CPUCores,
		CPUModel:             hw.CPUModel,
		RAMTotalBytes:        hw.RAMTotalBytes,
		DiskTotalBytes:       hw.DiskTotalBytes,
		DiskUsedBytes:        hw.DiskUsedBytes,
		UptimeSeconds:        hw.UptimeSeconds,
		SSHVersion:           hw.SSHVersion,
		InstalledWebServers:  sw.WebServers,
		InstalledPHPVersions: sw.PHPVersions,
		InstalledDBVersions:  sw.DBVersions,
		DockerVersion:        sw.DockerVersion,
		DiagnosedAt:          time.Now(),
	}

	if err := s.database.SaveDiagnostics(ctx, diag); err != nil {
		return "", fmt.Errorf("failed to save diagnostics: %w", err)
	}

	logger.SetStep("Diagnostics completed", 100)
	return fmt.Sprintf("OS: %s | CPU: %d cores | RAM: %d MB", osInfo.PrettyName, hw.CPUCores, hw.RAMTotalBytes/(1024*1024)), nil
}
