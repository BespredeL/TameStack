package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/mail"
)

func (a *App) GetMailStatus(serverID string) (*mail.MailStatus, error) {
	return withLock(a, func() (*mail.MailStatus, error) {
		return a.mailService.GetMailStatus(a.ctx, serverID)
	})
}

func (a *App) InstallMailStack(serverID, hostname string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.mailService.InstallMailStack(a.ctx, serverID, hostname)
	})
}

func (a *App) AddMailDomain(serverID, domain string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.mailService.AddMailDomain(a.ctx, serverID, domain)
	})
}

func (a *App) CreateMailbox(serverID, email, password string, quotaMB int) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.mailService.CreateMailbox(a.ctx, serverID, email, password, quotaMB)
	})
}

func (a *App) DeleteMailbox(serverID, email string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.mailService.DeleteMailbox(a.ctx, serverID, email)
	})
}

func (a *App) SaveMailAlias(serverID, source, destination string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.mailService.SaveMailAlias(a.ctx, serverID, source, destination)
	})
}

func (a *App) DeleteMailAlias(serverID, source string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.mailService.DeleteMailAlias(a.ctx, serverID, source)
	})
}
