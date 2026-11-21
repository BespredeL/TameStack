package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FileSecurityExclusionRecord represents a path pattern excluded from security scanning.
type FileSecurityExclusionRecord struct {
	ID        string    `json:"id"`
	ServerID  string    `json:"server_id"`
	Pattern   string    `json:"pattern"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// FileSecuritySnapshotRecord represents a saved file integrity baseline.
type FileSecuritySnapshotRecord struct {
	ID             string    `json:"id"`
	ServerID       string    `json:"server_id"`
	SiteID         string    `json:"site_id"`
	Name           string    `json:"name"`
	BasePath       string    `json:"base_path"`
	FilesCount     int       `json:"files_count"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	ManifestJSON   string    `json:"manifest_json"`
	CreatedAt      time.Time `json:"created_at"`
}

// FileSecuritySignatureRecord represents a user-defined heuristic regex signature.
type FileSecuritySignatureRecord struct {
	ID          string    `json:"id"`
	ServerID    string    `json:"server_id"`
	Name        string    `json:"name"`
	Pattern     string    `json:"pattern"`
	ThreatLevel string    `json:"threat_level"` // "malicious", "suspicious"
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SaveFileSecurityExclusion inserts or updates an exclusion.
func (d *DB) SaveFileSecurityExclusion(ctx context.Context, e *FileSecurityExclusionRecord) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO file_security_exclusions (id, server_id, pattern, reason, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			pattern = excluded.pattern,
			reason = excluded.reason;
	`
	_, err := d.ExecContext(ctx, query, e.ID, e.ServerID, e.Pattern, e.Reason, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to save file security exclusion: %w", err)
	}
	return nil
}

// ListFileSecurityExclusions returns all exclusions for a server.
func (d *DB) ListFileSecurityExclusions(ctx context.Context, serverID string) ([]*FileSecurityExclusionRecord, error) {
	query := `
		SELECT id, server_id, pattern, reason, created_at
		FROM file_security_exclusions
		WHERE server_id = ?
		ORDER BY created_at DESC;
	`
	rows, err := d.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to list file security exclusions: %w", err)
	}
	defer rows.Close()

	var result []*FileSecurityExclusionRecord
	for rows.Next() {
		var e FileSecurityExclusionRecord
		if err := rows.Scan(&e.ID, &e.ServerID, &e.Pattern, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &e)
	}
	return result, rows.Err()
}

// DeleteFileSecurityExclusion deletes an exclusion by ID.
func (d *DB) DeleteFileSecurityExclusion(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM file_security_exclusions WHERE id = ?;", id)
	if err != nil {
		return fmt.Errorf("failed to delete file security exclusion: %w", err)
	}
	return nil
}

// SaveFileSecuritySnapshot saves a baseline snapshot with its manifest.
func (d *DB) SaveFileSecuritySnapshot(ctx context.Context, s *FileSecuritySnapshotRecord) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO file_security_snapshots (id, server_id, site_id, name, base_path, files_count, total_size_bytes, manifest_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			files_count = excluded.files_count,
			total_size_bytes = excluded.total_size_bytes,
			manifest_json = excluded.manifest_json;
	`
	_, err := d.ExecContext(ctx, query, s.ID, s.ServerID, s.SiteID, s.Name, s.BasePath, s.FilesCount, s.TotalSizeBytes, s.ManifestJSON, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to save file security snapshot: %w", err)
	}
	return nil
}

// ListFileSecuritySnapshots returns snapshots for a server without large manifest_json.
func (d *DB) ListFileSecuritySnapshots(ctx context.Context, serverID string) ([]*FileSecuritySnapshotRecord, error) {
	query := `
		SELECT id, server_id, site_id, name, base_path, files_count, total_size_bytes, created_at
		FROM file_security_snapshots
		WHERE server_id = ?
		ORDER BY created_at DESC;
	`
	rows, err := d.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to list file security snapshots: %w", err)
	}
	defer rows.Close()

	var result []*FileSecuritySnapshotRecord
	for rows.Next() {
		var s FileSecuritySnapshotRecord
		if err := rows.Scan(&s.ID, &s.ServerID, &s.SiteID, &s.Name, &s.BasePath, &s.FilesCount, &s.TotalSizeBytes, &s.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &s)
	}
	return result, rows.Err()
}

// GetFileSecuritySnapshot returns a snapshot with its full manifest_json.
func (d *DB) GetFileSecuritySnapshot(ctx context.Context, id string) (*FileSecuritySnapshotRecord, error) {
	query := `
		SELECT id, server_id, site_id, name, base_path, files_count, total_size_bytes, manifest_json, created_at
		FROM file_security_snapshots
		WHERE id = ?;
	`
	var s FileSecuritySnapshotRecord
	err := d.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.ServerID, &s.SiteID, &s.Name, &s.BasePath, &s.FilesCount, &s.TotalSizeBytes, &s.ManifestJSON, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get file security snapshot: %w", err)
	}
	return &s, nil
}

// DeleteFileSecuritySnapshot deletes a snapshot.
func (d *DB) DeleteFileSecuritySnapshot(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM file_security_snapshots WHERE id = ?;", id)
	if err != nil {
		return fmt.Errorf("failed to delete file security snapshot: %w", err)
	}
	return nil
}

// SaveFileSecuritySignature inserts or updates a heuristic signature.
func (d *DB) SaveFileSecuritySignature(ctx context.Context, s *FileSecuritySignatureRecord) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	s.UpdatedAt = time.Now()
	if s.ThreatLevel == "" {
		s.ThreatLevel = "malicious"
	}

	query := `
		INSERT INTO file_security_signatures (id, server_id, name, pattern, threat_level, description, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			server_id = excluded.server_id,
			name = excluded.name,
			pattern = excluded.pattern,
			threat_level = excluded.threat_level,
			description = excluded.description,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at;
	`
	enabledInt := 0
	if s.Enabled {
		enabledInt = 1
	}
	_, err := d.ExecContext(ctx, query, s.ID, s.ServerID, s.Name, s.Pattern, s.ThreatLevel, s.Description, enabledInt, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save file security signature: %w", err)
	}
	return nil
}

// ListFileSecuritySignatures returns all signatures (both global and server-specific).
func (d *DB) ListFileSecuritySignatures(ctx context.Context, serverID string) ([]*FileSecuritySignatureRecord, error) {
	query := `
		SELECT id, server_id, name, pattern, threat_level, description, enabled, created_at, updated_at
		FROM file_security_signatures
		WHERE server_id = '' OR server_id = ?
		ORDER BY created_at ASC;
	`
	rows, err := d.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to list file security signatures: %w", err)
	}
	defer rows.Close()

	var result []*FileSecuritySignatureRecord
	for rows.Next() {
		var r FileSecuritySignatureRecord
		var enabledInt int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.Name, &r.Pattern, &r.ThreatLevel, &r.Description, &enabledInt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan file security signature: %w", err)
		}
		r.Enabled = (enabledInt == 1)
		result = append(result, &r)
	}
	return result, rows.Err()
}

// GetFileSecuritySignature returns a signature by ID.
func (d *DB) GetFileSecuritySignature(ctx context.Context, id string) (*FileSecuritySignatureRecord, error) {
	query := `
		SELECT id, server_id, name, pattern, threat_level, description, enabled, created_at, updated_at
		FROM file_security_signatures
		WHERE id = ?;
	`
	var r FileSecuritySignatureRecord
	var enabledInt int
	err := d.QueryRowContext(ctx, query, id).Scan(
		&r.ID, &r.ServerID, &r.Name, &r.Pattern, &r.ThreatLevel, &r.Description, &enabledInt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get file security signature: %w", err)
	}
	r.Enabled = (enabledInt == 1)
	return &r, nil
}

// DeleteFileSecuritySignature deletes a signature by ID.
func (d *DB) DeleteFileSecuritySignature(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM file_security_signatures WHERE id = ?;", id)
	if err != nil {
		return fmt.Errorf("failed to delete file security signature: %w", err)
	}
	return nil
}

// ToggleFileSecuritySignature enables or disables a signature.
func (d *DB) ToggleFileSecuritySignature(ctx context.Context, id string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := d.ExecContext(ctx, "UPDATE file_security_signatures SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;", enabledInt, id)
	if err != nil {
		return fmt.Errorf("failed to toggle file security signature: %w", err)
	}
	return nil
}
