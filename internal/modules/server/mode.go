package server

import (
	"context"
	"fmt"
)

// SetServerMode validates and persists the operating mode, then syncs the task engine gate.
func (s *Service) SetServerMode(ctx context.Context, serverID string, mode string) error {
	if mode != "audit" && mode != "management" {
		return fmt.Errorf("недопустимый режим: %s (допустимы: audit, management)", mode)
	}
	if err := s.database.SetServerMode(ctx, serverID, mode); err != nil {
		return err
	}
	s.engine.SetServerMode(serverID, mode)
	return nil
}

// GetServerMode returns the operating mode of a server, defaulting to "audit".
func (s *Service) GetServerMode(ctx context.Context, serverID string) (string, error) {
	srv, err := s.database.GetServer(ctx, serverID)
	if err != nil {
		return "audit", err
	}
	if srv.Mode == "" {
		return "audit", nil
	}
	return srv.Mode, nil
}
