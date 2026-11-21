package php

import (
	"fmt"
	"strings"
)

type DistroCompatibility struct {
	DistroID      string `json:"distro_id"`
	VersionID     string `json:"version_id"`
	IsEOL         bool   `json:"is_eol"`
	ArchiveRepo   string `json:"archive_repo,omitempty"`
	PPAKeyringCmd string `json:"ppa_keyring_cmd,omitempty"`
}

// DetectDistroCompatibility parses /etc/os-release and identifies legacy/EOL distros.
func DetectDistroCompatibility(osReleaseContent string) *DistroCompatibility {
	compat := &DistroCompatibility{
		DistroID:  "unknown",
		VersionID: "0",
	}

	lines := strings.Split(osReleaseContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			val := strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			compat.DistroID = strings.ToLower(val)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			val := strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
			compat.VersionID = strings.ToLower(val)
		}
	}

	// Ubuntu 18.04 (Bionic) and 20.04 (Focal) are EOL; PHP installs come from the distro archive only.
	if compat.DistroID == "ubuntu" && strings.HasPrefix(compat.VersionID, "18.04") {
		compat.IsEOL = true
	} else if compat.DistroID == "ubuntu" && strings.HasPrefix(compat.VersionID, "20.04") {
		compat.IsEOL = true
	} else if compat.DistroID == "ubuntu" && strings.HasPrefix(compat.VersionID, "16.04") {
		compat.IsEOL = true
	} else if compat.DistroID == "debian" && (compat.VersionID == "8" || compat.VersionID == "9") {
		compat.IsEOL = true
	}

	return compat
}

// GenerateApacheProxyFCGIBlock generates isolated VirtualHost proxy directives for Apache,
// allowing a site to run on modern PHP-FPM without conflicting with or disabling mod_php.
func GenerateApacheProxyFCGIBlock(domain string, phpVersion string) string {
	socketPath := fmt.Sprintf("/run/php/php%s-fpm.sock", phpVersion)
	return fmt.Sprintf(`    # TameStack PHP-FPM isolation for %s
    <FilesMatch \.php$>
        SetHandler "proxy:unix:%s|fcgi://localhost"
    </FilesMatch>
`, domain, socketPath)
}
