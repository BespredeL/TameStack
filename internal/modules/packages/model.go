package packages

import "time"

// PackageChange represents a single package modification detected during apt simulation.
type PackageChange struct {
	Name       string `json:"name"`
	Action     string `json:"action"` // "install", "upgrade", "remove", "configure"
	OldVersion string `json:"old_version,omitempty"`
	NewVersion string `json:"new_version,omitempty"`
	Arch       string `json:"arch,omitempty"`
}

// AptSimulationResult represents the outcome of an apt-get -s command.
type AptSimulationResult struct {
	Success        bool            `json:"success"`
	Command        string          `json:"command"`
	Changes        []PackageChange `json:"changes"`
	UpgradedCount  int             `json:"upgraded_count"`
	InstalledCount int             `json:"installed_count"`
	RemovedCount   int             `json:"removed_count"`
	HeldCount      int             `json:"held_count"`
	RawOutput      string          `json:"raw_output"`
	ErrorMessage   string          `json:"error_message,omitempty"`
}

// PackageItem represents an installed or searchable system package.
type PackageItem struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
	Status       string `json:"status"` // "installed", "upgradable", etc.
	Description  string `json:"description,omitempty"`
}

// PackageActionRequest holds inputs for install, upgrade, or remove tasks.
type PackageActionRequest struct {
	ServerID string   `json:"server_id"`
	Packages []string `json:"packages"`
	Action   string   `json:"action"` // "install", "remove", "upgrade"
}

// PackageTaskHistoryItem records details of past package tasks.
type PackageTaskHistoryItem struct {
	ID            string     `json:"id"`
	ServerID      string     `json:"server_id"`
	OperationType string     `json:"operation_type"`
	Status        string     `json:"status"`
	Packages      []string   `json:"packages"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
}
