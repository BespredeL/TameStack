package services

// SystemUnit represents a systemd service, timer, socket or target.
type SystemUnit struct {
	Unit        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
	UnitType    string `json:"unit_type"`
}

// UnitActionRequest represents a control command for a systemd unit.
type UnitActionRequest struct {
	ServerID string `json:"server_id"`
	UnitName string `json:"unit_name"`
	Action   string `json:"action"` // "start", "stop", "restart", "reload", "enable", "disable"
}

// ProcessItem represents an OS process running on the host.
type ProcessItem struct {
	User    string  `json:"user"`
	PID     int     `json:"pid"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	VSZ     int64   `json:"vsz"`
	RSS     int64   `json:"rss"`
	Stat    string  `json:"stat"`
	Start   string  `json:"start"`
	Time    string  `json:"time"`
	Command string  `json:"command"`
}

// ProcessKillRequest holds parameters to kill or signal a process.
type ProcessKillRequest struct {
	ServerID string `json:"server_id"`
	PID      int    `json:"pid"`
	Signal   string `json:"signal"` // "SIGTERM", "SIGKILL", "SIGHUP"
}

// ListeningPortItem represents a listening network socket.
type ListeningPortItem struct {
	Proto       string `json:"proto"`
	RecvQ       int    `json:"recv_q"`
	SendQ       int    `json:"send_q"`
	LocalAddr   string `json:"local_addr"`
	LocalPort   int    `json:"local_port"`
	ForeignAddr string `json:"foreign_addr"`
	ForeignPort int    `json:"foreign_port"`
	Process     string `json:"process"`
	PID         int    `json:"pid"`
}

// DNSCheckResult holds the result of a DNS lookup.
type DNSCheckResult struct {
	Domain     string   `json:"domain"`
	RecordType string   `json:"record_type"`
	Records    []string `json:"records"`
	Success    bool     `json:"success"`
	Error      string   `json:"error,omitempty"`
}

// PortCheckResult holds the result of a remote TCP port connection test.
type PortCheckResult struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Open      bool   `json:"open"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// NetworkOverview provides read-only network configuration facts.
type NetworkOverview struct {
	Interfaces string `json:"interfaces"`
	Routes     string `json:"routes"`
	DNSResolv  string `json:"dns_resolv"`
	Notice     string `json:"notice"`
}
