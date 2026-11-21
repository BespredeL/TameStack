package main

import (
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/modules/database"
	"tamestack/internal/tasks"
)

// --- Database Manager RPC ---

func (a *App) InspectDatabases(serverID string) ([]database.DatabaseInstanceInfo, error) {
	return withLock(a, func() ([]database.DatabaseInstanceInfo, error) {
		return a.dbService.InspectDatabases(a.ctx, serverID)
	})
}

func (a *App) ListDatabases(serverID string, engine string) ([]database.DatabaseItem, error) {
	return withLock(a, func() ([]database.DatabaseItem, error) {
		return a.dbService.ListDatabases(a.ctx, serverID, database.EngineType(engine))
	})
}

func (a *App) ListDatabaseUsers(serverID string, engine string) ([]database.DatabaseUser, error) {
	return withLock(a, func() ([]database.DatabaseUser, error) {
		return a.dbService.ListUsers(a.ctx, serverID, database.EngineType(engine))
	})
}

func (a *App) CreateDatabase(req database.CreateDatabaseRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dbService.CreateDatabase(a.ctx, req)
	})
}

func (a *App) CreateDatabaseUser(req database.CreateUserRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dbService.CreateUser(a.ctx, req)
	})
}

func (a *App) DeleteDatabase(serverID string, engine string, dbName string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dbService.DeleteDatabase(a.ctx, serverID, database.EngineType(engine), dbName)
	})
}

func (a *App) DeleteDatabaseUser(serverID string, engine string, username string, host string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	payload, err := tasks.MarshalPayload(database.DeleteUserPayload{ServerID: serverID, Engine: engine, Username: username, Host: host})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, serverID, "db_user_delete", fmt.Sprintf("Удаление пользователя БД %s", username), payload)
}

func (a *App) BackupDatabase(req database.BackupDatabaseRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dbService.BackupDatabase(a.ctx, req)
	})
}

func (a *App) RestoreDatabase(req database.RestoreDatabaseRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dbService.RestoreDatabase(a.ctx, req)
	})
}

func (a *App) ControlDatabaseService(serverID string, engine string, action string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dbService.ServiceControl(a.ctx, serverID, database.EngineType(engine), action)
	})
}

// PlanDatabaseInstall previews a DBMS installation (packages, service, OS compatibility).
func (a *App) PlanDatabaseInstall(serverID, engine, variant string) (*database.DBInstallPlan, error) {
	return withLock(a, func() (*database.DBInstallPlan, error) {
		return a.dbService.PlanInstall(a.ctx, serverID, engine, variant)
	})
}

// InstallDatabase queues an asynchronous DBMS installation task.
func (a *App) InstallDatabase(serverID, engine, variant string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dbService.InstallDatabase(serverID, engine, variant)
	})
}

// GetDatabaseConfig reads a DBMS configuration file.
func (a *App) GetDatabaseConfig(serverID, remotePath string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.dbService.GetConfig(a.ctx, serverID, remotePath)
	})
}

// SaveDatabaseConfig saves a DBMS configuration with backup, service restart and rollback on failure.
func (a *App) SaveDatabaseConfig(serverID, engine, remotePath, content string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	_, err := withLock(a, func() (struct{}, error) {
		return struct{}{}, a.dbService.SaveConfig(a.ctx, serverID, database.EngineType(engine), remotePath, content)
	})
	return err
}
