// Copyright (C) 2026 Aleksandr Kireev <hello@bespredel.name>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"tamestack/internal/ai/agents"
	"tamestack/internal/ai/chat"
	"tamestack/internal/ai/memory"
	"tamestack/internal/ai/tools"
	"tamestack/internal/ai/workflows"
	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/dbcontainer"
	"tamestack/internal/i18n"
	"tamestack/internal/inventory"
	"tamestack/internal/logger"
	"tamestack/internal/module"
	"tamestack/internal/modules/apps"
	"tamestack/internal/modules/backup"
	"tamestack/internal/modules/cron"
	"tamestack/internal/modules/database"
	"tamestack/internal/modules/docker"
	"tamestack/internal/modules/dns"
	"tamestack/internal/modules/files"
	"tamestack/internal/modules/logs"
	"tamestack/internal/modules/mail"
	"tamestack/internal/modules/packages"
	"tamestack/internal/modules/php"
	"tamestack/internal/modules/security"
	"tamestack/internal/modules/server"
	"tamestack/internal/modules/services"
	"tamestack/internal/modules/site"
	"tamestack/internal/modules/siteaudit"
	"tamestack/internal/modules/ssl"
	"tamestack/internal/modules/terminal"
	"tamestack/internal/modules/users"
	"tamestack/internal/modules/webserver"
	"tamestack/internal/provisioning"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
	"tamestack/internal/update"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the root controller exposed to Wails frontend.
type App struct {
	ctx               context.Context
	cfg               *config.AppConfig
	database          *db.DB
	vault             *crypto.Vault
	pool              *ssh.Pool
	engine            *tasks.Engine
	serverService     *server.Service
	inventoryService  *inventory.Service
	phpService        *php.Service
	webserverService  *webserver.Service
	provisioningService *provisioning.Service
	siteService       *site.Service
	siteAuditService  *siteaudit.Service
	sslService        *ssl.Service
	dbService         *database.Service
	dockerService     *docker.Service
	filesService      *files.Service
	cronService       *cron.Service
	backupService     *backup.Service
	packagesService   *packages.Service
	servicesService   *services.Service
	logsService       *logs.Service
	usersService      *users.Service
	securityService   *security.Service
	terminalService   *terminal.Service
	appsService       *apps.Service
	mailService       *mail.Service
	dnsService        *dns.Service
	aiChatService     *chat.Service
	aiMemoryService   *memory.Service
	siteDiagWf        *workflows.SiteDiagnosticsWorkflow
	secAuditWf        *workflows.SecurityAuditWorkflow
	configOptWf       *workflows.ConfigOptimizerWorkflow
	aiAgentSupervisor *agents.Supervisor
	aiStreamingConvs  sync.Map

	bootMu    sync.Mutex
	dbLocked  atomic.Bool
	dbSession *databaseSession
	sealMu    sync.Mutex

	updateService    *update.Service
	updateInstalling atomic.Bool
	pendingRestart   atomic.Bool
}

// databaseSession holds the runtime material needed to keep an encrypted
// database container in sync with the plaintext working copy.
type databaseSession struct {
	password    string
	workingPath string
}

// NewApp initializes backend core subsystems and dependencies.
func NewApp() (*App, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize structured logging system.
	if _, err := logger.Init(cfg.LogPath, "INFO"); err != nil {
		log.Printf("Warning: failed to initialize structured logger: %v", err)
	}

	update.CleanupOldBinaries()

	app := &App{cfg: cfg, updateService: update.NewService(update.DefaultRepo, AppVersion)}
	if dbcontainer.Exists(cfg.DatabaseContainerPath) {
		app.dbLocked.Store(true)
		log.Printf("TameStack: encrypted database container detected; services start after master password unlock")
		return app, nil
	}
	if err := app.bootstrapServices(cfg.DatabasePath); err != nil {
		return nil, err
	}
	return app, nil
}

// bootstrapServices opens the database at dbPath and wires every backend subsystem.
// It runs at startup for plaintext databases and after UnlockDatabase for containers.
func (a *App) bootstrapServices(dbPath string) error {
	cfg := a.cfg

	sqliteDB, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	vault, err := crypto.NewVault(cfg.SaltPath, cfg.VaultKeyPath)
	if err != nil {
		sqliteDB.Close()
		return fmt.Errorf("failed to initialize encryption vault: %w", err)
	}

	// Check if master password was previously configured in vault_settings.
	hasMasterPass, _ := sqliteDB.GetVaultSetting("has_master_password")
	if hasMasterPass == "true" {
		vault.SetMasterPasswordConfigured(true)
	}

	pool := ssh.NewPool()
	engine := tasks.NewEngine(sqliteDB, 4)
	serverService := server.NewService(cfg, sqliteDB, vault, pool, engine)
	phpService := php.NewService(cfg, sqliteDB, vault, pool, engine)
	provisioningService := provisioning.NewService(cfg, sqliteDB, vault, pool, engine)
	webserverService := webserver.NewService(cfg, sqliteDB, vault, pool, engine)
	siteService := site.NewService(cfg, sqliteDB, vault, pool, engine)
	siteAuditService := siteaudit.NewService(cfg, sqliteDB, vault, pool, engine)
	sslService := ssl.NewService(cfg, sqliteDB, vault, pool, engine)
	dbService := database.NewService(cfg, sqliteDB, vault, pool, engine)
	dockerService := docker.NewService(cfg, sqliteDB, vault, pool, engine)
	filesService := files.NewService(cfg, sqliteDB, vault, pool, engine)
	cronService := cron.NewService(cfg, sqliteDB, vault, pool, engine)
	backupService := backup.NewService(cfg, sqliteDB, vault, pool, engine)
	packagesService := packages.NewService(cfg, sqliteDB, vault, pool, engine)
	servicesService := services.NewService(cfg, sqliteDB, vault, pool, engine)
	logsService := logs.NewService(cfg, sqliteDB, vault, pool)
	usersService := users.NewService(cfg, sqliteDB, vault, pool, engine)
	securityService := security.NewService(cfg, sqliteDB, vault, pool, engine)
	terminalService := terminal.NewService(cfg, sqliteDB, vault, pool)
	appsService := apps.NewService(cfg, sqliteDB, vault, pool, engine)
	mailService := mail.NewService(cfg, sqliteDB, vault, pool, engine)
	dnsService := dns.NewService(cfg, sqliteDB, vault, pool, engine)

	siteDiagWf := workflows.NewSiteDiagnosticsWorkflow(sqliteDB, siteService, securityService, vault, pool, cfg.KnownHostsPath)
	secAuditWf := workflows.NewSecurityAuditWorkflow(sqliteDB, securityService, vault, pool, cfg.KnownHostsPath)
	configOptWf := workflows.NewConfigOptimizerWorkflow(sqliteDB, securityService, vault, pool, cfg.KnownHostsPath)

	aiToolsRegistry := tools.NewRegistry(sqliteDB, nil)
	tools.RegisterReadOnlyInspectionTools(aiToolsRegistry, sqliteDB, webserverService, phpService, siteService, securityService)
	tools.RegisterServerTools(aiToolsRegistry, sqliteDB, securityService)
	tools.RegisterSiteTools(aiToolsRegistry, sqliteDB, siteService, webserverService)
	tools.RegisterDatabaseTools(aiToolsRegistry, dbService)
	tools.RegisterSecurityTools(aiToolsRegistry, securityService)
	tools.RegisterMutatingTools(aiToolsRegistry, sqliteDB, webserverService, engine)
	tools.RegisterWorkflowTools(aiToolsRegistry, siteDiagWf, secAuditWf, configOptWf)
	tools.RegisterInfraTools(aiToolsRegistry, sqliteDB, dockerService, sslService, cronService, packagesService, filesService, servicesService, backupService)

	aiTaskHandlers := tools.NewTaskHandlerManager(cfg, sqliteDB, vault, pool)
	aiTaskHandlers.RegisterHandlers(engine)

	aiChatService := chat.NewService(sqliteDB, vault, aiToolsRegistry)
	aiMemoryService := memory.NewService(sqliteDB, nil)

	aiAgentSupervisor := agents.NewSupervisor(aiMemoryService)
	aiAgentSupervisor.Register(agents.NewMonitoringAgent(sqliteDB, securityService, 5*time.Minute))
	aiAgentSupervisor.Register(agents.NewSecurityAgent(sqliteDB, securityService, sslService, 10*time.Minute))

	inventoryService := inventory.NewService(sqliteDB)

	a.database = sqliteDB
	a.vault = vault
	a.pool = pool
	a.engine = engine
	a.serverService = serverService
	a.inventoryService = inventoryService
	a.phpService = phpService
	a.provisioningService = provisioningService
	a.webserverService = webserverService
	a.siteService = siteService
	a.siteAuditService = siteAuditService
	a.sslService = sslService
	a.dbService = dbService
	a.dockerService = dockerService
	a.filesService = filesService
	a.cronService = cronService
	a.backupService = backupService
	a.packagesService = packagesService
	a.servicesService = servicesService
	a.logsService = logsService
	a.usersService = usersService
	a.securityService = securityService
	a.terminalService = terminalService
	a.appsService = appsService
	a.mailService = mailService
	a.dnsService = dnsService
	a.aiChatService = aiChatService
	a.aiMemoryService = aiMemoryService
	a.siteDiagWf = siteDiagWf
	a.secAuditWf = secAuditWf
	a.configOptWf = configOptWf
	a.aiAgentSupervisor = aiAgentSupervisor
	a.registerInventoryTaskHandler()
	return nil
}

// startup is called by Wails when runtime starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.dbLocked.Load() {
		return
	}
	a.startRuntime()
}

// startRuntime launches background subsystems once services are wired.
func (a *App) startRuntime() {
	ctx := a.ctx
	if a.database != nil {
		if savedLang, err := a.database.GetAIGlobalSetting(ctx, "app_language"); err == nil && savedLang != "" {
			i18n.SetLanguage(savedLang)
		}
	}
	a.engine.SetWailsContext(ctx)
	a.engine.Start()
	a.terminalService.SetContext(ctx)
	a.aiAgentSupervisor.Start(ctx)

	// Notify frontend whenever locked vault access is attempted by any service.
	a.vault.SetOnLockedAccess(func() {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "vault:require_unlock", map[string]string{
				"reason": i18n.Tr("common.vaultUnlockReason"),
			})
		}
	})

	// Background watcher for inactivity auto-lock.
	go a.startVaultAutoLockWatcher(ctx)

	if a.dbSession != nil {
		go a.startContainerCheckpointWatcher(ctx)
	}
}

// recoverBackground stops a panic in a long-lived goroutine from taking down the
// process, which would also drop every live SSH session. The watcher itself is
// gone afterwards, so the failure is logged loudly rather than swallowed.
func recoverBackground(name string) {
	if p := recover(); p != nil {
		slog.Error("background watcher panicked",
			slog.String("name", name),
			slog.Any("panic", p),
			slog.String("stack", string(debug.Stack())))
	}
}

func (a *App) startVaultAutoLockWatcher(ctx context.Context) {
	defer recoverBackground("vault auto-lock watcher")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.vault.HasMasterPassword() && !a.vault.IsLocked() {
				autoLockStr, err := a.database.GetVaultSetting("auto_lock_minutes")
				if err == nil && shouldAutoLock(time.Now(), a.vault.LastActivity(), autoLockStr) {
					a.vault.Lock()
					if a.ctx != nil {
						wailsRuntime.EventsEmit(a.ctx, "vault:locked")
					}
				}
			}
		}
	}
}

// shouldAutoLock reports whether inactivity since lastAct reaches the configured
// auto-lock threshold. now is injected so the decision can be tested without
// waiting for real time to elapse.
func shouldAutoLock(now, lastAct time.Time, autoLockMinutes string) bool {
	if lastAct.IsZero() || autoLockMinutes == "" {
		return false
	}
	mins, err := strconv.Atoi(autoLockMinutes)
	if err != nil || mins <= 0 {
		return false
	}
	return now.Sub(lastAct) >= time.Duration(mins)*time.Minute
}

// shutdown is called when the application closes.
func (a *App) shutdown(ctx context.Context) {
	if a.aiAgentSupervisor != nil {
		a.aiAgentSupervisor.Stop()
	}
	// Workers must finish before the pool, database and container are closed;
	// otherwise a task writes into a database that is already gone.
	if a.engine != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := a.engine.Stop(stopCtx); err != nil {
			slog.Warn("task engine did not stop cleanly", slog.String("error", err.Error()))
		}
	}
	if a.pool != nil {
		a.pool.CloseAll()
	}

	a.sealMu.Lock()
	sealErr := error(nil)
	if a.dbSession != nil && a.database != nil {
		sealErr = a.sealDatabaseLocked()
	}
	if a.database != nil {
		_ = a.database.Close()
		a.database = nil
	}
	session := a.dbSession
	if session != nil && sealErr == nil {
		a.removeRunDir()
		a.dbSession = nil
	}
	a.sealMu.Unlock()

	if session != nil && sealErr != nil {
		// Keep the plaintext working copy so no data is lost; the operator
		// must resolve the seal failure before the copy disappears.
		slog.Error("failed to seal encrypted database container; unencrypted working copy preserved",
			slog.String("path", session.workingPath),
			slog.String("error", sealErr.Error()))
	}

	// The database is sealed and closed here, so starting the updated
	// binary now cannot race the old instance for the container.
	if a.pendingRestart.Swap(false) {
		if exe, err := os.Executable(); err != nil {
			slog.Error("update relaunch could not resolve executable path", slog.String("error", err.Error()))
		} else if err := update.SpawnRelaunch(exe); err != nil {
			slog.Error("update relaunch failed; start TameStack manually", slog.String("error", err.Error()))
		}
	}
}

// sealDatabaseLocked writes a plaintext snapshot and re-encrypts the container.
// Callers must hold a.sealMu.
func (a *App) sealDatabaseLocked() error {
	snap := filepath.Join(a.cfg.RunDir, "seal-snapshot.db")
	defer os.Remove(snap)
	if err := a.database.SnapshotToFile(context.Background(), snap); err != nil {
		return err
	}
	return dbcontainer.Seal(a.cfg.DatabaseContainerPath, snap, a.dbSession.password)
}

func (a *App) removeRunDir() {
	if err := os.RemoveAll(a.cfg.RunDir); err != nil {
		slog.Warn("failed to remove working database directory",
			slog.String("path", a.cfg.RunDir), slog.String("error", err.Error()))
	}
}

// dbFingerprint captures every signal that the working database changed:
// the DML counter plus file stats, because WAL auto-checkpoint can empty the
// WAL while the main file changed, and DDL/VACUUM do not bump total_changes.
type dbFingerprint struct {
	changes int64
	size    int64
	modTime time.Time
	walSize int64
}

func (a *App) readDBFingerprint(ctx context.Context) dbFingerprint {
	fp := dbFingerprint{}
	if a.database != nil {
		fp.changes, _ = a.database.ChangeCounter(ctx)
	}
	if st, err := os.Stat(a.cfg.WorkingDatabasePath); err == nil {
		fp.size, fp.modTime = st.Size(), st.ModTime()
	}
	if st, err := os.Stat(a.cfg.WorkingDatabasePath + "-wal"); err == nil {
		fp.walSize = st.Size()
	}
	return fp
}

func dbChangedSince(cur, last dbFingerprint) bool {
	return cur.changes != last.changes || cur.size != last.size ||
		!cur.modTime.Equal(last.modTime) || cur.walSize != last.walSize
}

// startContainerCheckpointWatcher bounds plaintext exposure after crashes by
// periodically re-sealing the container while the app is running.
func (a *App) startContainerCheckpointWatcher(ctx context.Context) {
	defer recoverBackground("container checkpoint watcher")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	last := a.readDBFingerprint(ctx)
	sealFailures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !dbChangedSince(a.readDBFingerprint(ctx), last) {
				continue
			}
			a.sealMu.Lock()
			if a.dbSession == nil || a.database == nil {
				a.sealMu.Unlock()
				return
			}
			// Fingerprint is taken before the seal: writes landing during the
			// seal keep the counter ahead, so the next tick re-seals them.
			pre := a.readDBFingerprint(ctx)
			err := a.sealDatabaseLocked()
			a.sealMu.Unlock()
			if err != nil {
				sealFailures++
				slog.Warn("periodic database container checkpoint failed",
					slog.Int("consecutive_failures", sealFailures),
					slog.String("error", err.Error()))
				if sealFailures == 3 && a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "db:seal_failed", map[string]string{
						"error": err.Error(),
					})
				}
				continue
			}
			sealFailures = 0
			last = pre
		}
	}
}

// BootStateInfo reports whether startup must be gated by the database password.
type BootStateInfo struct {
	Mode     string `json:"mode"` // "container" or "plain"
	DBLocked bool   `json:"db_locked"`
}

// requireUnlocked rejects data-dependent RPCs while the encrypted database
// container is still sealed. Services are nil in that state and Wails bindings
// do not recover panics, so an unguarded call would take the whole process down.
func (a *App) requireUnlocked() error {
	if a.dbLocked.Load() || a.database == nil {
		return fmt.Errorf("database is locked: unlock TameStack to use this action")
	}
	return nil
}

// withLock runs fn only while the database is unlocked; otherwise it returns
// the zero value of T and the lock error, so RPC bodies stay free of guards.
func withLock[T any](a *App, fn func() (T, error)) (T, error) {
	if err := a.requireUnlocked(); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}

// GetBootState is safe to call before the database is open.
func (a *App) GetBootState() *BootStateInfo {
	mode := "plain"
	if dbcontainer.Exists(a.cfg.DatabaseContainerPath) {
		mode = "container"
	}
	a.bootMu.Lock()
	defer a.bootMu.Unlock()
	return &BootStateInfo{Mode: mode, DBLocked: a.dbLocked.Load()}
}

// UnlockDatabase decrypts the database container into the working directory,
// wires all services and starts the background runtime. Called by the UI gate
// before any other data-dependent RPC is available.
func (a *App) UnlockDatabase(password string) error {
	a.bootMu.Lock()
	defer a.bootMu.Unlock()
	if !a.dbLocked.Load() {
		if a.database != nil {
			return nil
		}
		return fmt.Errorf("database is not in encrypted container mode")
	}
	container := a.cfg.DatabaseContainerPath
	if !dbcontainer.Exists(container) {
		return fmt.Errorf("database container is missing from %s", container)
	}
	workPath := a.cfg.WorkingDatabasePath
	if err := os.MkdirAll(a.cfg.RunDir, 0700); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	if _, err := os.Stat(workPath); err == nil {
		// Leftover working copy from an unclean shutdown is newer than the
		// container, so it is kept; the password is still required to seal it.
		if err := dbcontainer.VerifyPassword(container, password); err != nil {
			return err
		}
		slog.Warn("reusing leftover working database from unclean shutdown")
	} else if err := dbcontainer.Unlock(container, workPath, password); err != nil {
		a.removeRunDir()
		return err
	}

	if err := a.bootstrapServices(workPath); err != nil {
		a.removeRunDir()
		return fmt.Errorf("failed to start services after unlock: %w", err)
	}
	a.archiveLegacyPlaintextDatabase(container)

	// When secrets are protected by the same master password, unlock the vault
	// transparently; on mismatch the existing vault lock screen takes over.
	if a.vault.HasMasterPassword() {
		verifier, _ := a.database.GetVaultSetting("master_password_verifier")
		if verifier != "" {
			if err := a.vault.Unlock(password, verifier); err != nil {
				slog.Warn("vault remains locked after database unlock; UI will request the master password again")
			}
		}
	}

	a.sealMu.Lock()
	a.dbSession = &databaseSession{password: password, workingPath: workPath}
	a.sealMu.Unlock()
	a.dbLocked.Store(false)
	a.startRuntime()
	return nil
}

// EnableDatabaseEncryption seals the current plaintext database into a new
// encrypted container keyed by the existing vault master password. The
// container takes effect after an application restart.
func (a *App) EnableDatabaseEncryption(password string) error {
	a.bootMu.Lock()
	defer a.bootMu.Unlock()
	if password == "" {
		return fmt.Errorf("master password cannot be empty")
	}
	if a.dbLocked.Load() || dbcontainer.Exists(a.cfg.DatabaseContainerPath) {
		return fmt.Errorf("encrypted database container already exists; restart TameStack to activate it")
	}
	if a.database == nil {
		return fmt.Errorf("database is not open")
	}
	if !a.vault.HasMasterPassword() {
		return fmt.Errorf("set a master password in the vault settings first")
	}
	verifier, _ := a.database.GetVaultSetting("master_password_verifier")
	if verifier == "" {
		return fmt.Errorf("master password verifier is missing from the vault settings")
	}
	if err := a.vault.Unlock(password, verifier); err != nil {
		return fmt.Errorf("master password does not match the vault: %w", err)
	}

	snap := filepath.Join(a.cfg.DataDir, "encrypt-staging.db")
	defer os.Remove(snap)
	if err := a.database.SnapshotToFile(context.Background(), snap); err != nil {
		return err
	}
	if err := dbcontainer.Create(a.cfg.DatabaseContainerPath, snap, password); err != nil {
		_ = os.Remove(a.cfg.DatabaseContainerPath)
		return err
	}
	_ = os.Remove(a.cfg.DatabasePath + ".backup")
	slog.Info("encrypted database container created; restart required to activate")
	return nil
}

// archiveLegacyPlaintextDatabase moves the superseded plaintext database out of
// the way after a container boot, without deleting data the operator may need.
func (a *App) archiveLegacyPlaintextDatabase(container string) {
	legacy := a.cfg.DatabasePath
	info, err := os.Stat(legacy)
	if err != nil {
		return
	}
	if ci, cerr := os.Stat(container); cerr == nil && info.ModTime().After(ci.ModTime()) {
		slog.Warn("plaintext database was modified after encryption was enabled; those changes are NOT in the encrypted copy",
			slog.String("path", legacy))
	}
	stamp := time.Now().Format("20060102-150405")
	staleDir := filepath.Join(a.cfg.DataDir, "plaintext-legacy")
	if err := os.MkdirAll(staleDir, 0700); err != nil {
		slog.Warn("failed to create plaintext-legacy archive directory", slog.String("error", err.Error()))
		return
	}
	for _, suffix := range []string{"", "-wal", "-shm", ".backup"} {
		src := legacy + suffix
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := os.Rename(src, filepath.Join(staleDir, "tamestack.db"+suffix)); err != nil {
			slog.Warn("failed to archive unprotected database file", slog.String("path", src), slog.String("error", err.Error()))
		}
	}
	slog.Warn("unprotected legacy database files archived for manual review", slog.String("dir", staleDir),
		slog.String("archived_at", stamp))
}

func (a *App) getSSH(ctx context.Context, serverID string) (*ssh.SSHClient, *db.ServerRecord, error) {
	return module.GetSSHClient(ctx, a.cfg, a.database, a.vault, a.pool, serverID)
}
