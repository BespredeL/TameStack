package logs

// LogTailResult contains the lines read from a log file.
type LogTailResult struct {
	FilePath  string   `json:"file_path"`
	Lines     []string `json:"lines"`
	LineCount int      `json:"line_count"`
	Truncated bool     `json:"truncated"`
	SizeBytes int64    `json:"size_bytes"`
}
