package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/docker"
)

// --- Docker Manager RPC ---

func (a *App) GetDockerStatus(serverID string) (*docker.DockerStatus, error) {
	return withLock(a, func() (*docker.DockerStatus, error) {
		return a.dockerService.GetDockerStatus(a.ctx, serverID)
	})
}

func (a *App) ListDockerContainers(serverID string, all bool) ([]docker.ContainerItem, error) {
	return withLock(a, func() ([]docker.ContainerItem, error) {
		return a.dockerService.ListContainers(a.ctx, serverID, all)
	})
}

func (a *App) GetDockerContainerLogs(serverID, containerID string, lines int) (string, error) {
	return withLock(a, func() (string, error) {
		return a.dockerService.GetContainerLogs(a.ctx, serverID, containerID, lines)
	})
}

func (a *App) ListDockerImages(serverID string) ([]docker.ImageItem, error) {
	return withLock(a, func() ([]docker.ImageItem, error) {
		return a.dockerService.ListImages(a.ctx, serverID)
	})
}

func (a *App) ListDockerVolumes(serverID string) ([]docker.VolumeItem, error) {
	return withLock(a, func() ([]docker.VolumeItem, error) {
		return a.dockerService.ListVolumes(a.ctx, serverID)
	})
}

func (a *App) ListDockerNetworks(serverID string) ([]docker.NetworkItem, error) {
	return withLock(a, func() ([]docker.NetworkItem, error) {
		return a.dockerService.ListNetworks(a.ctx, serverID)
	})
}

func (a *App) ListDockerComposeProjects(serverID string) ([]docker.ComposeProject, error) {
	return withLock(a, func() ([]docker.ComposeProject, error) {
		return a.dockerService.ListComposeProjects(a.ctx, serverID)
	})
}

func (a *App) ReadDockerComposeFile(serverID, workingDir string) (string, error) {
	return withLock(a, func() (string, error) {
		return a.dockerService.ReadComposeFile(a.ctx, serverID, workingDir)
	})
}

func (a *App) DockerContainerAction(req docker.ContainerActionRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dockerService.ContainerAction(a.ctx, req)
	})
}

func (a *App) DockerPullImage(req docker.PullImageRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dockerService.PullImage(a.ctx, req)
	})
}

func (a *App) DockerComposeAction(req docker.ComposeActionRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dockerService.ComposeAction(a.ctx, req)
	})
}

func (a *App) DockerPrune(req docker.DockerPruneRequest) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dockerService.Prune(a.ctx, req)
	})
}
