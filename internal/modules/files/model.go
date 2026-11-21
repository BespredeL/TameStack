package files

import "time"

// FileEntry represents a file or directory on the remote server.
type FileEntry struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	HumanSize   string    `json:"human_size"`
	IsDir       bool      `json:"is_dir"`
	Permissions string    `json:"permissions"` // e.g. "0755" or "-rwxr-xr-x"
	Owner       string    `json:"owner"`
	Group       string    `json:"group"`
	ModTime     time.Time `json:"mod_time"`
	Extension   string    `json:"extension"`
}

// DirectoryListing contains files in current directory and parent path.
type DirectoryListing struct {
	CurrentPath string      `json:"current_path"`
	ParentPath  string      `json:"parent_path"`
	TotalItems  int         `json:"total_items"`
	Entries     []FileEntry `json:"entries"`
}

// FileContentResult returns content of a text file.
type FileContentResult struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	SizeBytes   int64  `json:"size_bytes"`
	IsBinary    bool   `json:"is_binary"`
	Permissions string `json:"permissions"`
}

// SaveFileRequest payload to save content back to server.
type SaveFileRequest struct {
	ServerID     string `json:"server_id"`
	Path         string `json:"path"`
	Content      string `json:"content"`
	CreateBackup bool   `json:"create_backup"`
}

// CreateEntryRequest for touch or mkdir.
type CreateEntryRequest struct {
	ServerID string `json:"server_id"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
}

// DeleteEntryRequest for rm / rmdir.
type DeleteEntryRequest struct {
	ServerID string `json:"server_id"`
	Path     string `json:"path"`
}

// RenameEntryRequest for mv.
type RenameEntryRequest struct {
	ServerID string `json:"server_id"`
	OldPath  string `json:"old_path"`
	NewPath  string `json:"new_path"`
}

// ChmodRequest for changing file permissions.
type ChmodRequest struct {
	ServerID string `json:"server_id"`
	Path     string `json:"path"`
	Mode     string `json:"mode"` // e.g. "0755"
}

// DiskPartition represents a mounted filesystem partition.
type DiskPartition struct {
	Filesystem  string  `json:"filesystem"`
	MountPoint  string  `json:"mount_point"`
	TotalBytes  int64   `json:"total_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
	FreeBytes   int64   `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
	HumanTotal  string  `json:"human_total"`
	HumanUsed   string  `json:"human_used"`
	HumanFree   string  `json:"human_free"`
}

// DiskUsageItem represents a directory or file size item in the analyzer.
type DiskUsageItem struct {
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	SizeBytes int64   `json:"size_bytes"`
	HumanSize string  `json:"human_size"`
	IsDir     bool    `json:"is_dir"`
	Percent   float64 `json:"percent"`
}

// LargestFileItem represents one of the largest files found on disk.
type LargestFileItem struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	SizeBytes int64     `json:"size_bytes"`
	HumanSize string    `json:"human_size"`
	ModTime   time.Time `json:"mod_time"`
}

// CleanupCategory represents a cleanable space category.
type CleanupCategory struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SizeBytes   int64  `json:"size_bytes"`
	HumanSize   string `json:"human_size"`
	Safe        bool   `json:"safe"`
	Available   bool   `json:"available"`
}

// DiskAnalysisReport is the complete result of disk analysis.
type DiskAnalysisReport struct {
	ServerID       string            `json:"server_id"`
	AnalyzedPath   string            `json:"analyzed_path"`
	Partitions     []DiskPartition   `json:"partitions"`
	TopDirectories []DiskUsageItem   `json:"top_directories"`
	LargestFiles   []LargestFileItem `json:"largest_files"`
	CleanupOptions []CleanupCategory `json:"cleanup_options"`
	TotalPathBytes int64             `json:"total_path_bytes"`
	HumanTotalPath string            `json:"human_total_path"`
	AnalyzedAt     time.Time         `json:"analyzed_at"`
}

// CleanDiskRequest is request to clean a category.
type CleanDiskRequest struct {
	ServerID   string `json:"server_id"`
	CategoryID string `json:"category_id"`
}
