package main

import (
	"fmt"
	"strconv"

	"tamestack/internal/vaultops"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type VaultStatusInfo struct {
	HasMasterPassword bool `json:"has_master_password"`
	IsLocked          bool `json:"is_locked"`
	AutoLockMinutes   int  `json:"auto_lock_minutes"`
}

func (a *App) GetVaultStatus() (*VaultStatusInfo, error) {
	return withLock(a, func() (*VaultStatusInfo, error) {
		if a.database == nil {
			return nil, fmt.Errorf("database is locked")
		}
		hasPass, _ := a.database.GetVaultSetting("has_master_password")
		autoLockStr, _ := a.database.GetVaultSetting("auto_lock_minutes")
		autoLock, _ := strconv.Atoi(autoLockStr)
		if autoLock <= 0 {
			autoLock = 15
		}

		return &VaultStatusInfo{
			HasMasterPassword: hasPass == "true",
			IsLocked:          a.vault.IsLocked(),
			AutoLockMinutes:   autoLock,
		}, nil
	})
}

func (a *App) SetMasterPassword(newPass string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	if err := vaultops.SetMasterPassword(a.ctx, a.database, a.vault, newPass); err != nil {
		return err
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "vault:unlocked")
	}
	return nil
}

func (a *App) UnlockVault(password string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	verifierToken, err := a.database.GetVaultSetting("master_password_verifier")
	if err != nil || verifierToken == "" {
		return fmt.Errorf("master password is not set or verifier is missing")
	}

	err = a.vault.Unlock(password, verifierToken)
	if err != nil {
		return err
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "vault:unlocked")
	}
	return nil
}

func (a *App) LockVault() error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	a.vault.Lock()
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "vault:locked")
	}
	return nil
}

func (a *App) SetAutoLockMinutes(minutes int) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	if minutes < 0 {
		minutes = 0
	}
	return a.database.SetVaultSetting("auto_lock_minutes", strconv.Itoa(minutes))
}
