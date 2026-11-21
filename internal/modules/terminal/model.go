package terminal

// TerminalSessionInfo holds metadata about an open terminal session.
type TerminalSessionInfo struct {
	SessionID string `json:"session_id"`
	ServerID  string `json:"server_id"`
	CreatedAt string `json:"created_at"`
}

// StartSessionRequest is a request to start an interactive terminal session.
type StartSessionRequest struct {
	ServerID string `json:"server_id"`
	Cols     int    `json:"cols"`
	Rows     int    `json:"rows"`
}

// ResizeTerminalRequest is a request to resize the terminal PTY.
type ResizeTerminalRequest struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}
