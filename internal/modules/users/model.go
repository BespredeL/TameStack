package users

// SystemUserInfo contains basic user account metadata.
type SystemUserInfo struct {
	Username string `json:"username"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	HomeDir  string `json:"home_dir"`
	Shell    string `json:"shell"`
	IsSystem bool   `json:"is_system"`
}

// CreateSiteUserRequest holds input to create an isolated website user.
type CreateSiteUserRequest struct {
	ServerID string `json:"server_id"`
	Username string `json:"username"`
	HomeDir  string `json:"home_dir"`
}

// ChownRequest holds input to set file/directory ownership.
type ChownRequest struct {
	ServerID string `json:"server_id"`
	Path     string `json:"path"`
	Username string `json:"username"`
	Group    string `json:"group"`
}

// AuthorizedKeyItem represents a public key entry.
type AuthorizedKeyItem struct {
	KeyType string `json:"key_type"`
	KeyData string `json:"key_data"`
	Comment string `json:"comment"`
	Raw     string `json:"raw"`
}

// AddSSHKeyRequest holds input to append a public key.
type AddSSHKeyRequest struct {
	ServerID string `json:"server_id"`
	Username string `json:"username"`
	Key      string `json:"key"`
}
