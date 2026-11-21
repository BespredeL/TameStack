package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/users"
)

// --- Users & Permissions ---

func (a *App) ListSystemUsers(serverID string) ([]users.SystemUserInfo, error) {
	return withLock(a, func() ([]users.SystemUserInfo, error) {
		return a.usersService.ListUsers(a.ctx, serverID)
	})
}

func (a *App) CreateSiteUser(serverID string, username string, homeDir string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.usersService.CreateSiteUser(a.ctx, serverID, username, homeDir)
	})
}

func (a *App) SetDirectoryOwner(serverID string, dirPath string, username string, group string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.usersService.SetDirectoryOwner(a.ctx, serverID, dirPath, username, group)
	})
}

func (a *App) GetUserAuthorizedKeys(serverID string, username string) ([]users.AuthorizedKeyItem, error) {
	return withLock(a, func() ([]users.AuthorizedKeyItem, error) {
		return a.usersService.GetAuthorizedKeys(a.ctx, serverID, username)
	})
}

func (a *App) AddUserAuthorizedKey(serverID string, username string, pubKey string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.usersService.AddAuthorizedKey(a.ctx, serverID, username, pubKey)
	})
}
