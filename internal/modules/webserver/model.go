package webserver

// WebServerStatus represents status and health of Nginx or Apache service.
type WebServerStatus struct {
	Name        string `json:"name"` // "nginx", "apache2"
	DisplayName string `json:"display_name"`
	IsInstalled bool   `json:"is_installed"`
	Status      string `json:"status"` // "active", "inactive", "failed", "not_installed"
	Version     string `json:"version"`
	ConfigPath  string `json:"config_path"`
	TestOutput  string `json:"test_output"`
	TestPassed  bool   `json:"test_passed"`
}
