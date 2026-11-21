package capabilities

import (
	"strings"
)

// OSRelease holds structured release identifiers from /etc/os-release.
type OSRelease struct {
	ID        string `json:"id"`
	VersionID string `json:"version_id"`
	Codename  string `json:"codename"`
	Name      string `json:"name"`
	IDLike    string `json:"id_like"`
}

// LTS versions that receive install profiles. Other Ubuntu releases are read-only.
var ubuntuLTS = []string{"18.04", "20.04", "22.04", "24.04", "26.04"}

// IsUbuntuLTS reports whether version is one of the 5 supported LTS releases.
func IsUbuntuLTS(version string) bool {
	for _, lts := range ubuntuLTS {
		if strings.HasPrefix(version, lts) {
			return true
		}
	}
	return false
}

// AllowsPackageInstall reports whether packages can be installed on this OS.
// Non-LTS releases (e.g. 23.10, 24.10) block package installation.
func AllowsPackageInstall(osReleaseContent string) (bool, string) {
	rel := ParseOSRelease(osReleaseContent)
	return AllowsPackageInstallByVersion(rel.ID, rel.VersionID)
}

// AllowsPackageInstallByVersion checks if package install is allowed for osID and versionID.
func AllowsPackageInstallByVersion(osID, versionID string) (bool, string) {
	if strings.EqualFold(osID, "ubuntu") {
		if !IsUbuntuLTS(versionID) && versionID != "" && versionID != "0" {
			return false, "Установка пакетов поддерживается только на Ubuntu LTS (18.04, 20.04, 22.04, 24.04, 26.04). Промежуточные релизы поддерживают только осмотр (read-only)."
		}
	}
	return true, ""
}

// AllowsPHPInstall reports whether a PHP version may be installed on this OS.
// The empty Ondřej index for Ubuntu 18.04 is not used.
func AllowsPHPInstall(osReleaseContent, phpVersion string) (bool, string) {
	rel := ParseOSRelease(osReleaseContent)
	return AllowsPHPInstallByVersion(rel.ID, rel.VersionID, phpVersion)
}

// AllowsPHPInstallByVersion checks if PHP version is allowed given osID (e.g. "ubuntu") and versionID (e.g. "18.04").
func AllowsPHPInstallByVersion(osID, versionID, phpVersion string) (bool, string) {
	if strings.EqualFold(osID, "ubuntu") {
		if !IsUbuntuLTS(versionID) && versionID != "" && versionID != "0" {
			return false, "Установка пакетов поддерживается только на Ubuntu LTS (18.04, 20.04, 22.04, 24.04, 26.04). Этот выпуск можно осматривать."
		}
		if strings.HasPrefix(versionID, "18.04") && phpVersion != "7.2" {
			return false, "Версия " + phpVersion + " требует более нового релиза Ubuntu (20.04+)."
		}
		if strings.HasPrefix(versionID, "20.04") {
			switch phpVersion {
			case "7.4", "8.0", "8.1", "8.2", "8.3":
				// Поддерживается в архиве (7.4) и PPA Ondřej (8.0-8.3)
			default:
				return false, "Версия " + phpVersion + " требует более нового релиза Ubuntu (22.04+)."
			}
		}
		if strings.HasPrefix(versionID, "26.04") && (phpVersion == "" || phpVersion == "php") {
			return false, "На Ubuntu 26.04 метапакет php тянет Apache. Ставьте php8.5-fpm."
		}
	}
	return true, ""
}

// AllowsMySQLCreateUserIfNotExists checks if MySQL supports CREATE USER IF NOT EXISTS.
// MySQL 5.7 does not support it (false), MySQL 8.0+ and MariaDB support it (true).
func AllowsMySQLCreateUserIfNotExists(versionStr string) bool {
	v := strings.ToLower(versionStr)
	if strings.Contains(v, "mariadb") {
		return true
	}
	if strings.Contains(v, "5.7") || strings.Contains(v, "5.6") || strings.Contains(v, "5.5") {
		return false
	}
	return true
}

// ParseOSRelease reads ID, VERSION_ID, CODENAME, NAME, ID_LIKE from /etc/os-release.
func ParseOSRelease(content string) OSRelease {
	rel := OSRelease{
		ID:        "unknown",
		VersionID: "",
		Codename:  "",
		Name:      "",
		IDLike:    "",
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch key {
		case "ID":
			rel.ID = strings.ToLower(val)
		case "VERSION_ID":
			rel.VersionID = val
		case "VERSION_CODENAME":
			rel.Codename = strings.ToLower(val)
		case "NAME":
			rel.Name = val
		case "ID_LIKE":
			rel.IDLike = strings.ToLower(val)
		}
	}
	return rel
}
