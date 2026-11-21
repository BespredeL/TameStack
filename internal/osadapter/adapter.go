package osadapter

import (
	"context"
	"fmt"

	"tamestack/internal/ssh"
)

// OSFamily constants
const (
	FamilyDebian  = "debian"
	FamilyRhel    = "rhel"
	FamilyUnknown = "unknown"
)

// OSInfo holds detected Linux distribution details.
type OSInfo struct {
	Family     string `json:"family"`  // "debian", "rhel"
	Distro     string `json:"distro"`  // "ubuntu", "debian", "almalinux", "rocky", "rhel"
	Version    string `json:"version"` // e.g. "24.04", "12", "9.4"
	PrettyName string `json:"pretty_name"`
	Kernel     string `json:"kernel"`
	Arch       string `json:"arch"`
}

// HardwareInfo holds core hardware stats.
type HardwareInfo struct {
	CPUCores       int    `json:"cpu_cores"`
	CPUModel       string `json:"cpu_model"`
	RAMTotalBytes  int64  `json:"ram_total_bytes"`
	DiskTotalBytes int64  `json:"disk_total_bytes"`
	DiskUsedBytes  int64  `json:"disk_used_bytes"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	SSHVersion     string `json:"ssh_version"`
}

// InstalledSoftware holds detected versions of common stack components.
type InstalledSoftware struct {
	WebServers    []string `json:"webservers"`
	PHPVersions   []string `json:"php_versions"`
	DBVersions    []string `json:"db_versions"`
	DockerVersion string   `json:"docker_version"`
}

// OSAdapter defines the contract for interacting with specific Linux distributions.
type OSAdapter interface {
	GetOSInfo() *OSInfo
	CollectHardware(ctx context.Context, client *ssh.SSHClient) (*HardwareInfo, error)
	DetectSoftware(ctx context.Context, client *ssh.SSHClient) (*InstalledSoftware, error)
	UpdatePackageIndex(ctx context.Context, client *ssh.SSHClient) (string, error)
	InstallPackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error)
	RemovePackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error)
	// UpgradePackages upgrades the given packages, or the whole system when packages is empty.
	UpgradePackages(ctx context.Context, client *ssh.SSHClient, packages []string) (string, error)
	SecurityUpgrade(ctx context.Context, client *ssh.SSHClient) (string, error)
	// SimulateInstall previews an install without modifying the system, returning
	// the raw output and the command that produced it for display.
	SimulateInstall(ctx context.Context, client *ssh.SSHClient, packages []string) (string, string, error)
	SimulateUpgrade(ctx context.Context, client *ssh.SSHClient) (string, string, error)
	SimulateRemove(ctx context.Context, client *ssh.SSHClient, packages []string) (string, string, error)
	ListInstalledPackages(ctx context.Context, client *ssh.SSHClient, query string) (string, error)
	ServiceStatus(ctx context.Context, client *ssh.SSHClient, serviceName string) (string, error)
	ServiceAction(ctx context.Context, client *ssh.SSHClient, serviceName string, action string) error
}

// NewAdapterForFamily builds an adapter from a known OS family without probing
// the remote host, for callers that already cached the family (e.g. diagnostics).
func NewAdapterForFamily(family string) OSAdapter {
	info := &OSInfo{Family: family}
	if family == FamilyRhel {
		return NewRhelAdapter(info)
	}
	return NewDebianAdapter(info)
}

// DetectAndCreateAdapter connects via SSH, analyzes /etc/os-release and instantiates the correct adapter.
func DetectAndCreateAdapter(ctx context.Context, client *ssh.SSHClient) (OSAdapter, *OSInfo, error) {
	out, _, _, err := client.Exec(ctx, "cat /etc/os-release; uname -r; uname -m")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read OS release information: %w", err)
	}

	osInfo := parseOSRelease(string(out))
	if osInfo.Family == FamilyRhel {
		return NewRhelAdapter(osInfo), osInfo, nil
	}
	return NewDebianAdapter(osInfo), osInfo, nil
}
