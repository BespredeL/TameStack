package capabilities

import (
	"strings"
)

// AptPolicyResult represents parsed apt-cache policy output.
type AptPolicyResult struct {
	Package   string `json:"package"`
	Installed string `json:"installed"`
	Candidate string `json:"candidate"`
	Found     bool   `json:"found"`
}

// ParseAptCachePolicy parses the standard output of `apt-cache policy <package>`.
// Example:
// php8.2-fpm:
//
//	Installed: (none)
//	Candidate: 8.2.20-1+ubuntu22.04.1+deb.sury.org+1
//	Version table:
//	   ...
func ParseAptCachePolicy(output string) AptPolicyResult {
	res := AptPolicyResult{
		Installed: "",
		Candidate: "",
		Found:     false,
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			res.Package = strings.TrimSuffix(trimmed, ":")
			continue
		}
		if strings.HasPrefix(trimmed, "Installed:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "Installed:"))
			if val != "(none)" && val != "" {
				res.Installed = val
			}
		} else if strings.HasPrefix(trimmed, "Candidate:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "Candidate:"))
			if val != "(none)" && val != "" {
				res.Candidate = val
				res.Found = true
			}
		}
	}
	return res
}
