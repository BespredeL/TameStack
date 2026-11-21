package docker

// DockerStatus contains general information about the Docker daemon on the server.
type DockerStatus struct {
	Installed       bool   `json:"installed"`
	Version         string `json:"version"`
	ComposeVersion  string `json:"compose_version"`
	ServiceStatus   string `json:"service_status"` // active, inactive
	ContainersCount int    `json:"containers_count"`
	RunningCount    int    `json:"running_count"`
	ImagesCount     int    `json:"images_count"`
	VolumesCount    int    `json:"volumes_count"`
}

// ContainerItem represents a single container.
type ContainerItem struct {
	ID         string `json:"id"`
	Names      string `json:"names"`
	Image      string `json:"image"`
	Command    string `json:"command"`
	CreatedAt  string `json:"created_at"`
	Status     string `json:"status"` // e.g. "Up 2 hours", "Exited (0) 5 mins ago"
	State      string `json:"state"`  // running, exited, paused, restarting
	Ports      string `json:"ports"`
	Size       string `json:"size"`
	CPUPercent string `json:"cpu_percent"`
	MemUsage   string `json:"mem_usage"`
}

// ImageItem represents a Docker image.
type ImageItem struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	CreatedAt  string `json:"created_at"`
}

// VolumeItem represents a Docker volume.
type VolumeItem struct {
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Scope     string `json:"scope"`
	SizeBytes int64  `json:"size_bytes"`
}

// NetworkItem represents a Docker network.
type NetworkItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

// ComposeProject represents a detected Docker Compose project.
type ComposeProject struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	ConfigFile string `json:"config_file"`
	WorkingDir string `json:"working_dir"`
	Containers int    `json:"containers"`
}

// ContainerActionRequest for start, stop, restart, remove.
type ContainerActionRequest struct {
	ServerID    string `json:"server_id"`
	ContainerID string `json:"container_id"`
	Action      string `json:"action"` // start, stop, restart, remove, kill
	Force       bool   `json:"force"`
}

// PullImageRequest for pulling a new image.
type PullImageRequest struct {
	ServerID string `json:"server_id"`
	Image    string `json:"image"` // e.g. "nginx:alpine", "redis:latest"
}

// ComposeActionRequest for up, down, restart.
type ComposeActionRequest struct {
	ServerID   string `json:"server_id"`
	WorkingDir string `json:"working_dir"`
	Action     string `json:"action"`            // up, down, restart, pull
	Content    string `json:"content,omitempty"` // Updated docker-compose.yml if saving
}

// DockerPruneRequest for cleanup.
type DockerPruneRequest struct {
	ServerID     string `json:"server_id"`
	PruneType    string `json:"prune_type"` // containers, images, volumes, all
	PruneVolumes bool   `json:"prune_volumes"`
}
