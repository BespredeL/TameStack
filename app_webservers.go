package main

import (
	"fmt"

	"tamestack/internal/db"
	"tamestack/internal/modules/webserver"
	"tamestack/internal/tasks"
)

// --- Web Server Manager RPC ---

func (a *App) InspectWebServers(serverID string) ([]webserver.WebServerStatus, error) {
	return withLock(a, func() ([]webserver.WebServerStatus, error) {
		return a.webserverService.InspectWebServers(a.ctx, serverID)
	})
}

func (a *App) TestWebserverConfig(serverID, webserverType string) (map[string]interface{}, error) {
	return withLock(a, func() (map[string]interface{}, error) {
		passed, out, err := a.webserverService.TestConfig(a.ctx, serverID, webserverType)
		return map[string]interface{}{
			"passed": passed,
			"output": out,
		}, err
	})
}

func (a *App) ControlWebserver(serverID, webserverType, action string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	payload, err := tasks.MarshalPayload(webserver.ControlPayload{ServerID: serverID, WebServerType: webserverType, Action: action})
	if err != nil {
		return err
	}
	return a.engine.SubmitAndWait(a.ctx, serverID, "webserver_control", fmt.Sprintf("%s %s", action, webserverType), payload)
}

// PlanWebserverInstall previews the installation (packages, service, OS compatibility) without changes.
func (a *App) PlanWebserverInstall(serverID, webserverType string) (*webserver.InstallPlan, error) {
	return withLock(a, func() (*webserver.InstallPlan, error) {
		return a.webserverService.PlanInstall(a.ctx, serverID, webserverType)
	})
}

// InstallWebserver queues an asynchronous web server installation task.
func (a *App) InstallWebserver(serverID, webserverType string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.webserverService.InstallWebServer(serverID, webserverType)
	})
}

// GetWebserverConfig reads the main configuration file of the web server.
func (a *App) GetWebserverConfig(serverID, remotePath string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.webserverService.GetConfig(a.ctx, serverID, remotePath)
	})
}

// SaveWebserverConfig saves the configuration with backup, syntax test, rollback on failure and reload.
func (a *App) SaveWebserverConfig(serverID, remotePath, content, webserverType string) error {
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	_, err := withLock(a, func() (struct{}, error) {
		return struct{}{}, a.webserverService.SaveConfig(a.ctx, serverID, remotePath, content, webserverType)
	})
	return err
}
