package main

import (
	"tamestack/internal/modules/cron"
)

// --- Cron Manager RPC ---

func (a *App) ListCronJobs(serverID, user string) ([]cron.CronJob, error) {
	return withLock(a, func() ([]cron.CronJob, error) {
		return a.cronService.ListCronJobs(a.ctx, serverID, user)
	})
}

func (a *App) SaveCronJob(req cron.SaveCronJobRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.cronService.SaveCronJob(a.ctx, req)
}

func (a *App) DeleteCronJob(req cron.DeleteCronJobRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.cronService.DeleteCronJob(a.ctx, req)
}

func (a *App) ToggleCronJob(req cron.ToggleCronJobRequest) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.cronService.ToggleCronJob(a.ctx, req)
}
