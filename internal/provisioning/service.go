package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/provisioning/executor"
	"tamestack/internal/provisioning/inventory"
	"tamestack/internal/provisioning/models"
	"tamestack/internal/provisioning/planner"
	"tamestack/internal/provisioning/profiles"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"

	"github.com/google/uuid"
)

// Service coordinates automated server provisioning.
type Service struct {
	module.Base
	engine   *tasks.Engine
	executor *executor.Executor
}

// NewService instantiates Provisioning Service and registers task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	exec := executor.NewExecutor(cfg, database, vault, pool)
	s := &Service{
		Base:     module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine:   engine,
		executor: exec,
	}

	engine.RegisterHandler("server_provisioning", exec.HandleProvisioningTask)
	return s
}

// DiscoverServer inspects OS, hardware, and pre-existing software on a server.
func (s *Service) DiscoverServer(ctx context.Context, serverID string) (*inventory.ServerInventory, error) {
	client, record, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return inventory.CollectServerInventory(ctx, client, record.Port)
}

// ListProfiles returns all built-in server setup profiles.
func (s *Service) ListProfiles() []profiles.ProfileInfo {
	return profiles.ListProfiles()
}

// GetProfileBlueprint returns the default blueprint template for a profile.
func (s *Service) GetProfileBlueprint(ctx context.Context, serverID, profileID string) (*models.Blueprint, error) {
	record, err := s.Database.GetServer(ctx, serverID)
	port := 22
	if err == nil && record != nil && record.Port > 0 {
		port = record.Port
	}
	bp := profiles.GetBlueprintForProfile(profileID, port)
	return &bp, nil
}

// GeneratePlan constructs the pre-flight plan for a server given a blueprint.
func (s *Service) GeneratePlan(ctx context.Context, serverID string, bp models.Blueprint) (*models.ProvisioningPlan, error) {
	inv, err := s.DiscoverServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return planner.GeneratePlan(serverID, inv, bp)
}

// ExecutePlan queues an asynchronous task to apply the provisioning blueprint.
func (s *Service) ExecutePlan(ctx context.Context, serverID string, bp models.Blueprint) (*db.TaskRecord, error) {
	record, err := s.Database.GetServer(ctx, serverID)
	if err != nil || record == nil {
		return nil, fmt.Errorf("server %s not found: %w", serverID, err)
	}

	payload := executor.ProvisioningPayload{
		ServerID:  serverID,
		Blueprint: bp,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("Настройка сервера %s (%s)", record.Name, bp.Profile)
	return s.engine.Submit(serverID, "server_provisioning", title, string(payloadBytes))
}

// ListBlueprints retrieves saved server blueprints.
func (s *Service) ListBlueprints(ctx context.Context) ([]*db.ServerBlueprintRecord, error) {
	return s.Database.ListServerBlueprints(ctx)
}

// SaveBlueprint persists a server blueprint in the database.
func (s *Service) SaveBlueprint(ctx context.Context, name, description string, bp models.Blueprint) (*db.ServerBlueprintRecord, error) {
	specJSON, err := json.Marshal(bp)
	if err != nil {
		return nil, err
	}

	rec := &db.ServerBlueprintRecord{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Profile:     bp.Profile,
		SpecJSON:    string(specJSON),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.Database.SaveServerBlueprint(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// DeleteBlueprint removes a saved blueprint.
func (s *Service) DeleteBlueprint(ctx context.Context, id string) error {
	return s.Database.DeleteServerBlueprint(ctx, id)
}

// ListProvisioningRuns returns the history of provisioning runs for a server.
func (s *Service) ListProvisioningRuns(ctx context.Context, serverID string, limit int) ([]*db.ProvisioningRunRecord, error) {
	return s.Database.ListProvisioningRuns(ctx, serverID, limit)
}
