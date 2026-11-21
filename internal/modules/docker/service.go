package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

type Service struct {
	module.Base
	engine *tasks.Engine
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}

	engine.RegisterHandler("docker_container_action", s.handleContainerActionTask)
	engine.RegisterHandler("docker_image_pull", s.handleImagePullTask)
	engine.RegisterHandler("docker_compose_action", s.handleComposeActionTask)
	engine.RegisterHandler("docker_prune", s.handlePruneTask)

	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string) (string, string, int, error) {
	stdout, stderr, code, err := client.Exec(ctx, cmd)
	return string(stdout), string(stderr), code, err
}

// GetDockerStatus collects overview of Docker daemon.
func (s *Service) GetDockerStatus(ctx context.Context, serverID string) (*DockerStatus, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	status := &DockerStatus{}

	vOut, _, vCode, _ := s.execCmd(ctx, client, "docker --version 2>/dev/null")
	if vCode != 0 || len(strings.TrimSpace(vOut)) == 0 {
		return status, nil // Not installed
	}
	status.Installed = true
	status.Version = strings.TrimSpace(vOut)

	cOut, _, _, _ := s.execCmd(ctx, client, "docker compose version 2>/dev/null || docker-compose --version 2>/dev/null")
	status.ComposeVersion = strings.TrimSpace(cOut)

	sOut, _, _, _ := s.execCmd(ctx, client, "systemctl is-active docker 2>/dev/null")
	status.ServiceStatus = strings.TrimSpace(sOut)
	if status.ServiceStatus == "" {
		status.ServiceStatus = "inactive"
	}

	infoOut, _, code, _ := s.execCmd(ctx, client, "docker info --format '{{.Containers}}|{{.ContainersRunning}}|{{.Images}}' 2>/dev/null")
	if code == 0 && len(strings.TrimSpace(infoOut)) > 0 {
		parts := strings.Split(strings.TrimSpace(infoOut), "|")
		if len(parts) >= 3 {
			status.ContainersCount, _ = strconv.Atoi(parts[0])
			status.RunningCount, _ = strconv.Atoi(parts[1])
			status.ImagesCount, _ = strconv.Atoi(parts[2])
		}
	}

	volOut, _, volCode, _ := s.execCmd(ctx, client, "docker volume ls -q 2>/dev/null | wc -l")
	if volCode == 0 {
		status.VolumesCount, _ = strconv.Atoi(strings.TrimSpace(volOut))
	}

	return status, nil
}

// ListContainers queries existing containers with resource usage.
func (s *Service) ListContainers(ctx context.Context, serverID string, all bool) ([]ContainerItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	flags := "-a"
	if !all {
		flags = ""
	}

	cmd := fmt.Sprintf("docker ps %s --format '{{.ID}};;;{{.Names}};;;{{.Image}};;;{{.Command}};;;{{.CreatedAt}};;;{{.Status}};;;{{.State}};;;{{.Ports}};;;{{.Size}}' 2>/dev/null", flags)
	stdout, _, code, err := s.execCmd(ctx, client, cmd)
	if code == 127 {
		return []ContainerItem{}, nil
	}
	if err != nil || code != 0 {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	// Fetch stats in parallel for running containers.
	statsMap := make(map[string][2]string)
	statsOut, _, sCode, _ := s.execCmd(ctx, client, "docker stats --no-stream --format '{{.ID}};;;{{.CPUPerc}};;;{{.MemUsage}}' 2>/dev/null")
	if sCode == 0 {
		lines := strings.Split(statsOut, "\n")
		for _, line := range lines {
			parts := strings.Split(strings.TrimSpace(line), ";;;")
			if len(parts) >= 3 {
				statsMap[parts[0]] = [2]string{parts[1], parts[2]}
			}
		}
	}

	var list []ContainerItem
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";;;")
		if len(parts) >= 9 {
			cid := parts[0]
			cpuVal := "-"
			memVal := "-"
			// Try full and short prefix.
			for k, v := range statsMap {
				if strings.HasPrefix(cid, k) || strings.HasPrefix(k, cid) {
					cpuVal = v[0]
					memVal = v[1]
					break
				}
			}

			list = append(list, ContainerItem{
				ID:         cid,
				Names:      parts[1],
				Image:      parts[2],
				Command:    parts[3],
				CreatedAt:  parts[4],
				Status:     parts[5],
				State:      parts[6],
				Ports:      parts[7],
				Size:       parts[8],
				CPUPercent: cpuVal,
				MemUsage:   memVal,
			})
		}
	}

	return list, nil
}

// GetContainerLogs retrieves stdout/stderr tail logs.
func (s *Service) GetContainerLogs(ctx context.Context, serverID string, containerID string, lines int) (string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	if lines <= 0 {
		lines = 200
	}
	cmd := fmt.Sprintf("docker logs --tail %d %s 2>&1", lines, containerID)
	stdout, _, _, err := s.execCmd(ctx, client, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}
	return stdout, nil
}

// ListImages queries all local Docker images.
func (s *Service) ListImages(ctx context.Context, serverID string) ([]ImageItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := "docker images --format '{{.ID}};;;{{.Repository}};;;{{.Tag}};;;{{.Size}};;;{{.CreatedAt}}' 2>/dev/null"
	stdout, _, code, err := s.execCmd(ctx, client, cmd)
	if code == 127 {
		return []ImageItem{}, nil
	}
	if err != nil || code != 0 {
		return nil, fmt.Errorf("failed to list docker images: %v", err)
	}

	var list []ImageItem
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";;;")
		if len(parts) >= 5 {
			list = append(list, ImageItem{
				ID:         parts[0],
				Repository: parts[1],
				Tag:        parts[2],
				Size:       parts[3],
				CreatedAt:  parts[4],
			})
		}
	}
	return list, nil
}

// ListVolumes queries all Docker volumes.
func (s *Service) ListVolumes(ctx context.Context, serverID string) ([]VolumeItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := "docker volume ls --format '{{.Name}};;;{{.Driver}};;;{{.Scope}}' 2>/dev/null"
	stdout, _, code, err := s.execCmd(ctx, client, cmd)
	if code == 127 {
		return []VolumeItem{}, nil
	}
	if err != nil || code != 0 {
		return nil, fmt.Errorf("failed to list docker volumes: %v", err)
	}

	var list []VolumeItem
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";;;")
		if len(parts) >= 3 {
			list = append(list, VolumeItem{
				Name:   parts[0],
				Driver: parts[1],
				Scope:  parts[2],
			})
		}
	}
	return list, nil
}

// ListNetworks queries all Docker networks.
func (s *Service) ListNetworks(ctx context.Context, serverID string) ([]NetworkItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := "docker network ls --format '{{.ID}};;;{{.Name}};;;{{.Driver}};;;{{.Scope}}' 2>/dev/null"
	stdout, _, code, err := s.execCmd(ctx, client, cmd)
	if code == 127 {
		return []NetworkItem{}, nil
	}
	if err != nil || code != 0 {
		return nil, fmt.Errorf("failed to list docker networks: %v", err)
	}

	var list []NetworkItem
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";;;")
		if len(parts) >= 4 {
			list = append(list, NetworkItem{
				ID:     parts[0],
				Name:   parts[1],
				Driver: parts[2],
				Scope:  parts[3],
			})
		}
	}
	return list, nil
}

// ListComposeProjects lists active Docker Compose projects.
func (s *Service) ListComposeProjects(ctx context.Context, serverID string) ([]ComposeProject, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := "docker compose ls --format '{{.Name}};;;{{.Status}};;;{{.ConfigFiles}}' 2>/dev/null"
	stdout, _, code, _ := s.execCmd(ctx, client, cmd)
	if code != 0 || len(strings.TrimSpace(stdout)) == 0 {
		return []ComposeProject{}, nil
	}

	var projects []ComposeProject
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";;;")
		if len(parts) >= 3 {
			configFile := parts[2]
			workingDir := ""
			idx := strings.LastIndex(configFile, "/")
			if idx > 0 {
				workingDir = configFile[:idx]
			}
			projects = append(projects, ComposeProject{
				Name:       parts[0],
				Status:     parts[1],
				ConfigFile: configFile,
				WorkingDir: workingDir,
			})
		}
	}
	return projects, nil
}

// ReadComposeFile reads docker-compose.yml in directory.
func (s *Service) ReadComposeFile(ctx context.Context, serverID string, workingDir string) (string, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	cmd := fmt.Sprintf("cat %s/compose.yaml 2>/dev/null || cat %s/docker-compose.yml 2>/dev/null", workingDir, workingDir)
	stdout, stderr, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		return "", fmt.Errorf("failed to read compose file: %s %v", stderr, err)
	}
	return stdout, nil
}

// Enqueue Task operations.

func (s *Service) ContainerAction(ctx context.Context, req ContainerActionRequest) (*db.TaskRecord, error) {
	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"docker_container_action",
		fmt.Sprintf("Docker %s container: %s", strings.Title(req.Action), req.ContainerID),
		string(payload),
	)
}

func (s *Service) PullImage(ctx context.Context, req PullImageRequest) (*db.TaskRecord, error) {
	if req.Image == "" {
		return nil, fmt.Errorf("image name is required")
	}
	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"docker_image_pull",
		fmt.Sprintf("Docker pull image: %s", req.Image),
		string(payload),
	)
}

func (s *Service) ComposeAction(ctx context.Context, req ComposeActionRequest) (*db.TaskRecord, error) {
	if req.WorkingDir == "" {
		return nil, fmt.Errorf("compose working directory is required")
	}
	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"docker_compose_action",
		fmt.Sprintf("Docker Compose %s (%s)", req.Action, req.WorkingDir),
		string(payload),
	)
}

func (s *Service) Prune(ctx context.Context, req DockerPruneRequest) (*db.TaskRecord, error) {
	payload, _ := json.Marshal(req)
	return s.engine.Submit(
		req.ServerID,
		"docker_prune",
		fmt.Sprintf("Docker prune %s", req.PruneType),
		string(payload),
	)
}

// --- Task Handlers ---

func (s *Service) handleContainerActionTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, _, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req ContainerActionRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidateDockerAction(req.Action); err != nil {
		return "", err
	}
	if err := ssh.ValidateToken(req.ContainerID, "container id"); err != nil {
		return "", err
	}

	logger.Info(fmt.Sprintf("Executing docker %s on container %s...", req.Action, req.ContainerID))
	logger.SetStep("executing_container_action", 30)

	var cmd string
	id := ssh.ShellEscape(req.ContainerID)
	switch req.Action {
	case "start":
		cmd = fmt.Sprintf("docker start %s", id)
	case "stop":
		cmd = fmt.Sprintf("docker stop %s", id)
	case "restart":
		cmd = fmt.Sprintf("docker restart %s", id)
	case "kill":
		cmd = fmt.Sprintf("docker kill %s", id)
	case "remove":
		forceFlag := ""
		if req.Force {
			forceFlag = "-f"
		}
		cmd = fmt.Sprintf("docker rm %s %s", forceFlag, id)
	default:
		return "", fmt.Errorf("unsupported action: %s", req.Action)
	}

	stdout, stderr, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		logger.Error(fmt.Sprintf("Command failed: %s %v", stderr, err))
		return "", fmt.Errorf("docker action error: %s", stderr)
	}

	logger.Info(fmt.Sprintf("Container %s %s successfully: %s", req.ContainerID, req.Action, strings.TrimSpace(stdout)))
	logger.SetStep("completed", 100)
	return fmt.Sprintf("Container %s %sed successfully", req.ContainerID, req.Action), nil
}

func (s *Service) handleImagePullTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, _, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req PullImageRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	logger.Info(fmt.Sprintf("Pulling Docker image '%s'...", req.Image))
	logger.SetStep("pulling_layers", 20)

	if err := ssh.ValidateImageName(req.Image); err != nil {
		return "", err
	}
	cmd := fmt.Sprintf("docker pull %s", ssh.ShellEscape(req.Image))
	stdout, stderr, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		return "", fmt.Errorf("image pull failed: %s", stderr)
	}

	logger.Info(stdout)
	logger.SetStep("completed", 100)
	return fmt.Sprintf("Image '%s' pulled successfully", req.Image), nil
}

func (s *Service) handleComposeActionTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, _, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req ComposeActionRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}
	if err := ssh.ValidateRemotePath(req.WorkingDir, false); err != nil {
		return "", err
	}
	if err := ssh.ValidateComposeAction(req.Action); err != nil {
		return "", err
	}

	// If content is provided, save it first.
	if len(req.Content) > 0 {
		logger.Info("Updating compose configuration file...")
		marker := "TameStack_COMPOSE"
		for strings.Contains(req.Content, marker) {
			marker += "X"
		}
		saveCmd := fmt.Sprintf("cat << '%s' > %s\n%s\n%s", marker, ssh.ShellEscape(strings.TrimRight(req.WorkingDir, "/")+"/compose.yaml"), req.Content, marker)
		_, _, sCode, sErr := s.execCmd(ctx, client, saveCmd)
		if sCode != 0 || sErr != nil {
			return "", fmt.Errorf("failed to save compose file: %v", sErr)
		}
	}

	logger.Info(fmt.Sprintf("Running docker compose %s in %s...", req.Action, req.WorkingDir))
	logger.SetStep("running_compose", 40)

	var cmd string
	dir := ssh.ShellEscape(req.WorkingDir)
	switch req.Action {
	case "up":
		cmd = fmt.Sprintf("cd %s && (docker compose up -d 2>&1 || docker-compose up -d 2>&1)", dir)
	case "down":
		cmd = fmt.Sprintf("cd %s && (docker compose down 2>&1 || docker-compose down 2>&1)", dir)
	case "restart":
		cmd = fmt.Sprintf("cd %s && (docker compose restart 2>&1 || docker-compose restart 2>&1)", dir)
	case "pull":
		cmd = fmt.Sprintf("cd %s && (docker compose pull 2>&1 || docker-compose pull 2>&1)", dir)
	default:
		return "", fmt.Errorf("unsupported compose action: %s", req.Action)
	}

	stdout, stderr, code, err := s.execCmd(ctx, client, cmd)
	if err != nil || code != 0 {
		return "", fmt.Errorf("compose action failed: %s %s", stdout, stderr)
	}

	logger.Info(stdout)
	logger.SetStep("completed", 100)
	return fmt.Sprintf("Compose %s executed successfully", req.Action), nil
}

func (s *Service) handlePruneTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	if task.ServerID == "" {
		return "", fmt.Errorf("task has no associated server")
	}
	client, _, err := s.GetSSHClient(ctx, task.ServerID)
	if err != nil {
		return "", err
	}

	var req DockerPruneRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	logger.Info(fmt.Sprintf("Running Docker prune (%s)...", req.PruneType))
	logger.SetStep("pruning", 50)

	var cmd string
	switch req.PruneType {
	case "containers":
		cmd = "docker container prune -f"
	case "images":
		cmd = "docker image prune -a -f"
	case "volumes":
		cmd = "docker volume prune -f"
	default:
		cmd = "docker system prune -a -f"
		if req.PruneVolumes {
			cmd += " --volumes"
		}
	}

	stdout, _, _, err := s.execCmd(ctx, client, cmd)
	if err != nil {
		return "", fmt.Errorf("prune failed: %w", err)
	}

	logger.Info(stdout)
	logger.SetStep("completed", 100)
	return "Docker prune completed", nil
}
