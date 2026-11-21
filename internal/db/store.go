package db

import (
	"context"
	"time"
)

// Per-domain repository interfaces over *DB. Modules depend on the narrow
// slice they use (or on Store via module.Base), so tests can substitute fakes
// without a live SQLite file.

type ServerStore interface {
	ListServers(ctx context.Context) ([]*ServerRecord, error)
	GetServer(ctx context.Context, id string) (*ServerRecord, error)
	SaveServer(ctx context.Context, s *ServerRecord) error
	SetServerMode(ctx context.Context, serverID string, mode string) error
	UpdateServerSudoSecret(ctx context.Context, id string, encSudoSecret string) error
	DeleteServer(ctx context.Context, id string) error
	UpdateServerStatus(ctx context.Context, id string, status string) error
	SaveDiagnostics(ctx context.Context, diag *DiagnosticsRecord) error
	GetDiagnostics(ctx context.Context, serverID string) (*DiagnosticsRecord, error)
}

type TaskStore interface {
	SaveTask(ctx context.Context, task *TaskRecord) error
	GetTask(ctx context.Context, id string) (*TaskRecord, error)
	ListTasks(ctx context.Context, limit int) ([]*TaskRecord, error)
	AppendTaskLog(ctx context.Context, taskID string, level string, message string) error
	GetTaskLogs(ctx context.Context, taskID string) ([]*TaskLogRecord, error)
}

type SiteStore interface {
	SaveSite(ctx context.Context, site *SiteRecord) error
	ListSites(ctx context.Context, serverID string) ([]*SiteRecord, error)
	GetSite(ctx context.Context, id string) (*SiteRecord, error)
	DeleteSite(ctx context.Context, id string) error
	UpdateSiteStatus(ctx context.Context, id string, status string) error
	SaveSiteConfigBackup(ctx context.Context, b *SiteConfigBackupRecord) error
	ListSiteConfigBackups(ctx context.Context, siteID string, limit int) ([]SiteConfigBackupRecord, error)
	GetSiteConfigBackup(ctx context.Context, id string) (*SiteConfigBackupRecord, error)
}

type InventoryStore interface {
	SaveInventorySnapshot(ctx context.Context, id, serverID, snapshotJSON string) error
	GetLatestInventorySnapshot(ctx context.Context, serverID string) (string, error)
}

type SSLStore interface {
	SaveSSLCertificate(ctx context.Context, cert *SSLCertificateRecord) error
	ListSSLCertificates(ctx context.Context, serverID string) ([]*SSLCertificateRecord, error)
	GetSSLCertificate(ctx context.Context, id string) (*SSLCertificateRecord, error)
	DeleteSSLCertificate(ctx context.Context, id string) error
}

type BackupStore interface {
	SaveBackup(ctx context.Context, b *BackupRecord) error
	GetBackup(ctx context.Context, id string) (*BackupRecord, error)
	MarkBackupOffsite(ctx context.Context, id string, downloadedAt time.Time) error
	MarkBackupVerified(ctx context.Context, id string, verified bool, checksum string) error
	ListBackups(ctx context.Context, serverID string) ([]*BackupRecord, error)
	DeleteBackup(ctx context.Context, id string) error
}

type AIStore interface {
	ListAIProviderConfigs(ctx context.Context) ([]*AIProviderConfigRecord, error)
	GetAIProviderConfig(ctx context.Context, id string) (*AIProviderConfigRecord, error)
	GetAIProviderConfigByType(ctx context.Context, pType string) (*AIProviderConfigRecord, error)
	SaveAIProviderConfig(ctx context.Context, c *AIProviderConfigRecord) error
	DeleteAIProviderConfig(ctx context.Context, id string) error
	GetAIGlobalSetting(ctx context.Context, key string) (string, error)
	SetAIGlobalSetting(ctx context.Context, key, value string) error
	GetAllAIGlobalSettings(ctx context.Context) (map[string]string, error)
	ListAIConversations(ctx context.Context) ([]*AIConversationRecord, error)
	GetAIConversation(ctx context.Context, id string) (*AIConversationRecord, error)
	SaveAIConversation(ctx context.Context, c *AIConversationRecord) error
	DeleteAIConversation(ctx context.Context, id string) error
	ListAIMessages(ctx context.Context, convID string) ([]*AIMessageRecord, error)
	SaveAIMessage(ctx context.Context, m *AIMessageRecord) error
	SaveAIMemoryFact(ctx context.Context, f *AIMemoryFactRecord) error
	ListAIMemoryFacts(ctx context.Context, serverID *string) ([]*AIMemoryFactRecord, error)
	SearchAIMemory(ctx context.Context, query string, serverID *string, limit int) ([]*AIMemoryFactRecord, error)
	CountAIMemoryFacts(ctx context.Context) (int, error)
	DeleteAIMemoryFacts(ctx context.Context, serverID *string) error
	SaveAIToolAudit(ctx context.Context, a *AIToolAuditRecord) error
	ListAIToolAudits(ctx context.Context, convID string) ([]*AIToolAuditRecord, error)
}

type FileSecurityStore interface {
	SaveFileSecurityExclusion(ctx context.Context, e *FileSecurityExclusionRecord) error
	ListFileSecurityExclusions(ctx context.Context, serverID string) ([]*FileSecurityExclusionRecord, error)
	DeleteFileSecurityExclusion(ctx context.Context, id string) error
	SaveFileSecuritySnapshot(ctx context.Context, s *FileSecuritySnapshotRecord) error
	ListFileSecuritySnapshots(ctx context.Context, serverID string) ([]*FileSecuritySnapshotRecord, error)
	GetFileSecuritySnapshot(ctx context.Context, id string) (*FileSecuritySnapshotRecord, error)
	DeleteFileSecuritySnapshot(ctx context.Context, id string) error
	SaveFileSecuritySignature(ctx context.Context, s *FileSecuritySignatureRecord) error
	ListFileSecuritySignatures(ctx context.Context, serverID string) ([]*FileSecuritySignatureRecord, error)
	GetFileSecuritySignature(ctx context.Context, id string) (*FileSecuritySignatureRecord, error)
	DeleteFileSecuritySignature(ctx context.Context, id string) error
	ToggleFileSecuritySignature(ctx context.Context, id string, enabled bool) error
}

type ProvisioningStore interface {
	SaveServerBlueprint(ctx context.Context, b *ServerBlueprintRecord) error
	GetServerBlueprint(ctx context.Context, id string) (*ServerBlueprintRecord, error)
	ListServerBlueprints(ctx context.Context) ([]*ServerBlueprintRecord, error)
	DeleteServerBlueprint(ctx context.Context, id string) error
	SaveProvisioningRun(ctx context.Context, r *ProvisioningRunRecord) error
	GetProvisioningRun(ctx context.Context, id string) (*ProvisioningRunRecord, error)
	ListProvisioningRuns(ctx context.Context, serverID string, limit int) ([]*ProvisioningRunRecord, error)
}

type VaultSettingsStore interface {
	GetVaultSetting(key string) (string, error)
	SetVaultSetting(key, val string) error
}

// Store is the full repository surface of *DB, consumed by module.Base.
// Lifecycle methods (Path, RunMigrations, SnapshotToFile, ...) stay on the
// concrete *DB: they belong to the composition root, not to modules.
type Store interface {
	ServerStore
	TaskStore
	SiteStore
	InventoryStore
	SSLStore
	BackupStore
	AIStore
	FileSecurityStore
	ProvisioningStore
	VaultSettingsStore
}

var _ Store = (*DB)(nil)
