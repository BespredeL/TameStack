package main

import (
	"strings"
	"testing"

	"tamestack/internal/config"
)

// In encrypted-container mode the app boots with every service field nil. Wails
// binds RPC calls without recovering panics, so a call made before the unlock
// gate must be refused with an error instead of crashing the process.
func TestLockedAppRefusesDataDependentRPC(t *testing.T) {
	app := &App{cfg: &config.AppConfig{}}
	app.dbLocked.Store(true)

	check := func(name string, run func() error) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("%s panicked while the database is locked: %v", name, p)
				}
			}()
			err := run()
			if err == nil || !strings.Contains(err.Error(), "database is locked") {
				t.Fatalf("%s returned %v, want the locked-database refusal", name, err)
			}
		})
	}

	check("ListServers", func() error { _, err := app.ListServers(); return err })
	check("ListTasks", func() error { _, err := app.ListTasks(10); return err })
	check("ListSites", func() error { _, err := app.ListSites("srv-1"); return err })
	check("GetDashboardStats", func() error { _, err := app.GetDashboardStats(); return err })
	check("GetAppSettings", func() error { _, err := app.GetAppSettings(); return err })
	check("GetMailStatus", func() error { _, err := app.GetMailStatus("srv-1"); return err })
	check("GetDNSStatus", func() error { _, err := app.GetDNSStatus("srv-1"); return err })
	check("EnableSFTPChroot", func() error { _, err := app.EnableSFTPChroot("srv-1", "u", "/var/www/u", ""); return err })
	check("GetAppProcessStatus", func() error { _, err := app.GetAppProcessStatus("site-1"); return err })
	check("AIStreamMessageWithContext", func() error { return app.AIStreamMessageWithContext("c1", "hi", nil) })

	// Slice-only RPCs cannot report an error; they must come back empty.
	if tools := app.AIListTools(); tools != nil {
		t.Errorf("AIListTools returned %d tools while locked, want nil", len(tools))
	}
	if statuses := app.AIListAgents(); statuses != nil {
		t.Errorf("AIListAgents returned %d agents while locked, want nil", len(statuses))
	}
}

// GetBootState must stay callable while locked — it is the only RPC the unlock
// screen itself needs.
func TestLockedAppStillAnswersBootState(t *testing.T) {
	app := &App{cfg: &config.AppConfig{}}
	app.dbLocked.Store(true)

	if state := app.GetBootState(); !state.DBLocked {
		t.Fatalf("GetBootState reported DBLocked=false while the flag is set")
	}
}
