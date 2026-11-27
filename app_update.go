package main

import (
	"context"
	"fmt"

	"tamestack/internal/update"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// These RPCs deliberately do not touch the database: they must keep working
// while the encrypted container is still sealed and all other services are
// offline, so the user can update the app before unlocking it.

// CheckForUpdates compares the running AppVersion with the latest GitHub
// release of the project repository.
func (a *App) CheckForUpdates() (*update.CheckResult, error) {
	if a.updateService == nil {
		return nil, fmt.Errorf("служба обновления не инициализирована")
	}
	return a.updateService.Check(context.Background())
}

// InstallUpdate downloads the verified release archive, swaps the executable
// and restarts the application. Progress arrives as update:progress events;
// a successful swap is followed by update:ready and the shutdown sequence.
func (a *App) InstallUpdate() error {
	if a.updateService == nil {
		return fmt.Errorf("служба обновления не инициализирована")
	}
	if !a.updateInstalling.CompareAndSwap(false, true) {
		return fmt.Errorf("установка обновления уже выполняется")
	}
	defer a.updateInstalling.Store(false)

	err := a.updateService.Install(context.Background(), func(p update.Progress) {
		wailsRuntime.EventsEmit(a.ctx, "update:progress", p)
	})
	if err != nil {
		wailsRuntime.EventsEmit(a.ctx, "update:error", map[string]string{"message": err.Error()})
		return err
	}

	if !a.updateService.IsElevatedPending() {
		a.pendingRestart.Store(true)
	}
	wailsRuntime.EventsEmit(a.ctx, "update:ready", map[string]string{})
	wailsRuntime.Quit(a.ctx)
	return nil
}
