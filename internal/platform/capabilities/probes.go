package capabilities

import (
	"regexp"
	"strings"
)

var (
	reNginxVersion   = regexp.MustCompile(`nginx/([0-9\.]+)`)
	rePHPVersion     = regexp.MustCompile(`PHP\s+([0-9\.]+)`)
	reCertbotVersion = regexp.MustCompile(`certbot\s+([0-9\.]+)`)
	reDockerVersion  = regexp.MustCompile(`Docker version\s+([0-9\.]+)`)
	reSystemdVersion = regexp.MustCompile(`systemd\s+([0-9]+)`)
)

type NginxProbeResult struct {
	Version  string `json:"version"`
	HasHTTP2 bool   `json:"has_http2"`
	HasSSL   bool   `json:"has_ssl"`
}

// ParseNginxV parses output of `nginx -V` for version and compiled modules.
func ParseNginxV(output string) NginxProbeResult {
	res := NginxProbeResult{}
	if m := reNginxVersion.FindStringSubmatch(output); len(m) > 1 {
		res.Version = m[1]
	}
	res.HasHTTP2 = strings.Contains(output, "--with-http_v2_module")
	res.HasSSL = strings.Contains(output, "--with-http_ssl_module")
	return res
}

// ParsePHPV parses output of `php -v`.
func ParsePHPV(output string) string {
	if m := rePHPVersion.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}
	return ""
}

// ParseCertbotV parses output of `certbot --version`.
func ParseCertbotV(output string) string {
	if m := reCertbotVersion.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}
	return ""
}

// ParseDockerV parses output of `docker version` or `docker --version`.
func ParseDockerV(output string) string {
	if m := reDockerVersion.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}
	return ""
}

// ParseSystemctlV parses output of `systemctl --version`.
func ParseSystemctlV(output string) string {
	if m := reSystemdVersion.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}
	return ""
}
