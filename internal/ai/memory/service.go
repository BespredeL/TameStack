package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	aiContext "tamestack/internal/ai/context"
	"tamestack/internal/db"
)

// Service provides high-level persistent knowledge management about infrastructure.
type Service struct {
	database  *db.DB
	sanitizer *aiContext.Sanitizer
}

// NewService creates a new infrastructure memory service.
func NewService(database *db.DB, sanitizer *aiContext.Sanitizer) *Service {
	if sanitizer == nil {
		sanitizer = aiContext.NewSanitizer()
	}
	return &Service{
		database:  database,
		sanitizer: sanitizer,
	}
}

// SaveFact creates or updates a piece of knowledge about a server or global preference.
func (s *Service) SaveFact(ctx context.Context, serverID *string, category, key, value, source string) (*db.AIMemoryFactRecord, error) {
	if key == "" || value == "" {
		return nil, fmt.Errorf("fact key and value are required")
	}
	if category == "" {
		category = "general"
	}
	if source == "" {
		source = "manual"
	}

	sanitizedValue := s.sanitizer.Sanitize(value)

	fact := &db.AIMemoryFactRecord{
		ID:        "fact_" + uuid.New().String(),
		ServerID:  serverID,
		Category:  category,
		FactKey:   key,
		FactValue: sanitizedValue,
		Source:    source,
	}

	if err := s.database.SaveAIMemoryFact(ctx, fact); err != nil {
		return nil, fmt.Errorf("failed to save memory fact: %w", err)
	}
	return fact, nil
}

// ListFacts retrieves all stored facts for a server (or global facts if serverID is nil).
func (s *Service) ListFacts(ctx context.Context, serverID *string) ([]*db.AIMemoryFactRecord, error) {
	return s.database.ListAIMemoryFacts(ctx, serverID)
}

// SearchFacts performs BM25-ranked full-text search across infrastructure memory.
func (s *Service) SearchFacts(ctx context.Context, query string, serverID *string, limit int) ([]*db.AIMemoryFactRecord, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return s.ListFacts(ctx, serverID)
	}
	return s.database.SearchAIMemory(ctx, cleanQuery, serverID, limit)
}

// ExtractServerFacts extracts key infrastructure facts from a server diagnostic snapshot.
func (s *Service) ExtractServerFacts(ctx context.Context, snapshot *aiContext.ServerSnapshot) error {
	if snapshot == nil || snapshot.Server == nil {
		return nil
	}
	serverID := snapshot.Server.ID

	if snapshot.Diagnostics != nil {
		d := snapshot.Diagnostics
		osFact := fmt.Sprintf("%s %s (%s, kernel %s)", d.OSName, d.OSVersion, d.Arch, d.KernelVersion)
		_, _ = s.SaveFact(ctx, &serverID, "os", "os_distribution", osFact, "auto")

		if len(d.InstalledWebServers) > 0 {
			_, _ = s.SaveFact(ctx, &serverID, "web", "installed_webservers", strings.Join(d.InstalledWebServers, ", "), "auto")
		}
		if len(d.InstalledPHPVersions) > 0 {
			_, _ = s.SaveFact(ctx, &serverID, "php", "installed_php_versions", strings.Join(d.InstalledPHPVersions, ", "), "auto")
		}
		if len(d.InstalledDBVersions) > 0 {
			_, _ = s.SaveFact(ctx, &serverID, "db", "installed_db_versions", strings.Join(d.InstalledDBVersions, ", "), "auto")
		}
	}

	if len(snapshot.Sites) > 0 {
		var domains []string
		for _, st := range snapshot.Sites {
			domains = append(domains, fmt.Sprintf("%s (webserver: %s, php: %s)", st.Domain, st.WebServer, st.PHPVersion))
		}
		_, _ = s.SaveFact(ctx, &serverID, "sites", "hosted_sites_summary", strings.Join(domains, "; "), "auto")
	}

	return nil
}
