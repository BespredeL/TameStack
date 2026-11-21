package server

import (
	"context"

	"tamestack/internal/db"
)

// DashboardStats aggregates fleet-wide counters for the dashboard view.
type DashboardStats struct {
	TotalServers      int                     `json:"total_servers"`
	OnlineServers     int                     `json:"online_servers"`
	OfflineServers    int                     `json:"offline_servers"`
	UnknownServers    int                     `json:"unknown_servers"`
	TotalCores        int                     `json:"total_cores"`
	TotalRAMBytes     int64                   `json:"total_ram_bytes"`
	TotalDiskBytes    int64                   `json:"total_disk_bytes"`
	TotalDiskUsed     int64                   `json:"total_disk_used"`
	TotalSites        int                     `json:"total_sites"`
	ActiveSites       int                     `json:"active_sites"`
	TotalSSLCerts     int                     `json:"total_ssl_certs"`
	ExpiringSSLCerts  int                     `json:"expiring_ssl_certs"`
	TotalBackups      int                     `json:"total_backups"`
	TotalTasks        int                     `json:"total_tasks"`
	RunningTasks      int                     `json:"running_tasks"`
	FailedTasks       int                     `json:"failed_tasks"`
	RecentTasks       []*db.TaskRecord        `json:"recent_tasks"`
	ServerDiagnostics []*db.DiagnosticsRecord `json:"server_diagnostics"`
}

// GetDashboardStats aggregates server statuses, diagnostics, sites, ssl, backups and recent tasks.
func (s *Service) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	servers, err := s.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	stats := &DashboardStats{
		TotalServers:      len(servers),
		RecentTasks:       []*db.TaskRecord{},
		ServerDiagnostics: []*db.DiagnosticsRecord{},
	}

	for _, srv := range servers {
		switch srv.Status {
		case "online":
			stats.OnlineServers++
		case "offline":
			stats.OfflineServers++
		default:
			stats.UnknownServers++
		}

		diag, err := s.GetDiagnostics(ctx, srv.ID)
		if err == nil && diag != nil {
			stats.TotalCores += diag.CPUCores
			stats.TotalRAMBytes += diag.RAMTotalBytes
			stats.TotalDiskBytes += diag.DiskTotalBytes
			stats.TotalDiskUsed += diag.DiskUsedBytes
			stats.ServerDiagnostics = append(stats.ServerDiagnostics, diag)
		}
	}

	// Aggregate sites count
	sites, err := s.database.ListSites(ctx, "")
	if err == nil {
		stats.TotalSites = len(sites)
		for _, st := range sites {
			if st.Status == "active" || st.Status == "online" || st.Status == "enabled" {
				stats.ActiveSites++
			}
		}
	}

	// Aggregate SSL certificates
	certs, err := s.database.ListSSLCertificates(ctx, "")
	if err == nil {
		stats.TotalSSLCerts = len(certs)
		for _, c := range certs {
			if c.DaysRemaining <= 30 && c.Status != "revoked" {
				stats.ExpiringSSLCerts++
			}
		}
	}

	// Aggregate backups count
	backups, err := s.database.ListBackups(ctx, "")
	if err == nil {
		stats.TotalBackups = len(backups)
	}

	tasksList, err := s.database.ListTasks(ctx, 15)
	if err == nil {
		stats.TotalTasks = len(tasksList)
		stats.RecentTasks = tasksList
		for _, t := range tasksList {
			if t.Status == "running" {
				stats.RunningTasks++
			} else if t.Status == "failed" {
				stats.FailedTasks++
			}
		}
	}

	return stats, nil
}
