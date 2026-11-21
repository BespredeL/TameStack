package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tamestack/internal/db"
	"tamestack/internal/modules/php"
	"tamestack/internal/ssh"
)

type Service struct {
	database *db.DB
}

func NewService(database *db.DB) *Service {
	return &Service{database: database}
}

// Collect gathers the entire server inventory in read-only mode with strict per-command timeouts.
// A hanging command does not abort other factual observations (ADR-S5).
// dbFacts is inspection data gathered by the database module via the facade.
func (s *Service) Collect(
	ctx context.Context,
	client *ssh.SSHClient,
	serverID string,
	sites []*db.SiteRecord,
	pools []php.FpmPoolConfig,
	certs []*db.SSLCertificateRecord,
	dbFacts []DatabaseFacts,
) (*ServerInventorySnapshot, error) {
	snap := &ServerInventorySnapshot{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		CreatedAt: time.Now(),
	}

	execWithTimeout := func(cmd string, timeout time.Duration) string {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		out, _, _, _ := client.NewCommand(cmd).WithSudo().Exec(cmdCtx)
		return string(out)
	}

	unitsOut := execWithTimeout("systemctl list-units --type=service --type=socket --no-legend --no-pager --plain 2>/dev/null", 5*time.Second)
	snap.Units = ParseUnits(unitsOut)

	socketsOut := execWithTimeout("ss -tulpn 2>/dev/null || netstat -tulpn 2>/dev/null", 4*time.Second)
	snap.Sockets = ParseSockets(socketsOut)

	usersOut := execWithTimeout("cat /etc/passwd 2>/dev/null", 3*time.Second)
	snap.Users = ParsePasswd(usersOut)

	dfOut := execWithTimeout("df -B1 -x tmpfs -x devtmpfs -x squashfs 2>/dev/null", 4*time.Second)
	snap.Mounts = ParseDF(dfOut)

	// Cron: root crontab, system cron files, and per-pool-user crontabs.
	cronOut := execWithTimeout("crontab -l 2>/dev/null", 3*time.Second)
	snap.CronJobs = ParseCron(cronOut, "root", "crontab")

	etcCrontab := execWithTimeout("cat /etc/crontab 2>/dev/null", 3*time.Second)
	snap.CronJobs = append(snap.CronJobs, ParseCronSystem(etcCrontab, "/etc/crontab")...)

	cronD := execWithTimeout("cat /etc/cron.d/* 2>/dev/null", 3*time.Second)
	snap.CronJobs = append(snap.CronJobs, ParseCronSystem(cronD, "/etc/cron.d")...)

	seenCronUsers := map[string]bool{"root": true}
	for _, p := range pools {
		u := strings.TrimSpace(p.User)
		if u == "" || seenCronUsers[u] {
			continue
		}
		seenCronUsers[u] = true
		userOut := execWithTimeout(fmt.Sprintf("crontab -u %s -l 2>/dev/null", ssh.ShellEscape(u)), 3*time.Second)
		snap.CronJobs = append(snap.CronJobs, ParseCron(userOut, u, "crontab")...)
	}

	dockerOut := execWithTimeout("docker ps -a --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null", 4*time.Second)
	if strings.TrimSpace(dockerOut) != "" {
		snap.Containers = ParseContainers(dockerOut)
	}

	// Database engines inspected by the database module (facade-supplied).
	snap.Databases = dbFacts
	snap.DBInstances = PopulateDatabases(dbFacts, snap.Sockets)

	// OS and Privileges inspection
	osRelOut := execWithTimeout("cat /etc/os-release 2>/dev/null", 2*time.Second)
	unameOut := execWithTimeout("uname -r; uname -m", 2*time.Second)
	userOut := execWithTimeout("id -u; id -un; id -Gn; command -v apt-get dnf yum pacman apk 2>/dev/null", 2*time.Second)
	snap.OS = ParseOSInfo(osRelOut, unameOut, userOut)

	// Process inspection
	psOut := execWithTimeout("ps -eo pid,ppid,user,args 2>/dev/null", 4*time.Second)
	procs := ParseProcesses(psOut)

	// Multi-instance Web Server discovery
	webServers, errWS := DiscoverWebServers(ctx, client, snap.Sockets, procs)
	if errWS == nil {
		snap.WebServers = webServers
	}

	// PHP Runtime discovery (CLI, FPM, mod_php, orphaned failed units)
	phpRuntimes, errPHP := DiscoverPHPRuntimes(ctx, client, snap.Sockets)
	if errPHP == nil {
		snap.PHPRuntimes = phpRuntimes
	}

	// APT and repository diagnostics (mismatched codenames, missing GPG keys)
	aptDiag, errAPT := DiscoverAPTDiagnostics(ctx, client, snap.OS.Codename)
	if errAPT == nil {
		snap.APT = aptDiag
	}

	// Site topology chain resolution (Domain -> WebServer -> Backend -> PHP Runtime)
	snap.SiteChains = DiscoverTopology(ctx, client, sites, snap.WebServers, snap.PHPRuntimes, certs)

	snap.Links = CorrelateLinks(sites, pools, certs)

	// Persist snapshot to SQLite.
	snapJSON, err := json.Marshal(snap)
	if err == nil && s.database != nil {
		_ = s.database.SaveInventorySnapshot(ctx, snap.ID, serverID, string(snapJSON))
	}

	return snap, nil
}

// GetLatest retrieves the latest stored snapshot for a server.
func (s *Service) GetLatest(ctx context.Context, serverID string) (*ServerInventorySnapshot, error) {
	if s.database == nil {
		return nil, fmt.Errorf("no database configured")
	}
	snapJSON, err := s.database.GetLatestInventorySnapshot(ctx, serverID)
	if err != nil {
		return nil, err
	}
	if snapJSON == "" {
		return nil, nil
	}
	var snap ServerInventorySnapshot
	if err := json.Unmarshal([]byte(snapJSON), &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}
