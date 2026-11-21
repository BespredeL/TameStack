package main

import (
	"context"
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/inventory"
	"tamestack/internal/modules/database"
	"tamestack/internal/modules/php"
	"tamestack/internal/tasks"
)

// registerInventoryTaskHandler registers the cross-domain inventory task; all
// single-domain mutation handlers live in their owning modules.
func (a *App) registerInventoryTaskHandler() {
	a.engine.RegisterHandler("inventory_snapshot", func(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
		logger.Info("Сбор полного инвентаря сервера в режиме чтения...")
		client, record, err := a.getSSH(ctx, task.ServerID)
		if err != nil {
			return "", err
		}
		sites, _ := a.database.ListSites(ctx, record.ID)
		var pools []php.FpmPoolConfig
		phpVersions, _ := a.phpService.InspectPHPVersions(ctx, record.ID)
		for _, v := range phpVersions {
			if v.IsInstalled {
				p, _ := a.phpService.ListFPMPools(ctx, record.ID, v.Version)
				pools = append(pools, p...)
			}
		}
		certs, _ := a.sslService.ListCertificates(ctx, record.ID)
		dbFacts := collectDatabaseFacts(ctx, a.dbService, record.ID)
		snap, err := a.inventoryService.Collect(ctx, client, record.ID, sites, pools, certs, dbFacts)
		if err != nil {
			return "", err
		}
		logger.Info(fmt.Sprintf("Инвентарь собран: %d веб-серверов, %d PHP runtimes, %d цепочек сайтов, %d сокетов, %d cron-задач, %d БД.",
			len(snap.WebServers), len(snap.PHPRuntimes), len(snap.SiteChains), len(snap.Sockets), len(snap.CronJobs), len(snap.Databases)))
		return "Инвентарь собран", nil
	})
}

// collectDatabaseFacts folds DBMS inspection results into inventory facts.
// An engine that is installed but unlistable is still reported with its
// version — the empty name list is the honest observation.
func collectDatabaseFacts(ctx context.Context, svc *database.Service, serverID string) []inventory.DatabaseFacts {
	instances, err := svc.InspectDatabases(ctx, serverID)
	if err != nil {
		return nil
	}
	var facts []inventory.DatabaseFacts
	for _, inst := range instances {
		if !inst.Installed {
			continue
		}
		fact := inventory.DatabaseFacts{
			Engine:  string(inst.Engine),
			Version: inventory.ShortVersion(inst.Version),
		}
		if items, lErr := svc.ListDatabases(ctx, serverID, inst.Engine); lErr == nil {
			for _, it := range items {
				fact.Databases = append(fact.Databases, it.Name)
			}
		}
		facts = append(facts, fact)
	}
	return facts
}

// --- Task Engine RPC ---

func (a *App) ListTasks(limit int) ([]*db.TaskRecord, error) {
	return withLock(a, func() ([]*db.TaskRecord, error) {
		return a.database.ListTasks(a.ctx, limit)
	})
}

func (a *App) GetTaskLogs(taskID string) ([]*db.TaskLogRecord, error) {
	return withLock(a, func() ([]*db.TaskLogRecord, error) {
		return a.database.GetTaskLogs(a.ctx, taskID)
	})
}

func (a *App) CancelTask(taskID string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.engine.Cancel(taskID)
}
