// Package secretredact centralizes the secret-masking patterns used by the vault
// and the logger so the two definitions live side by side and cannot silently
// drift apart. The two scrubbers keep distinct output contracts: callers of
// ScrubSecrets (task payloads, AI context, site-audit rules) depend on its exact
// format, while ScrubLogText additionally masks auth headers for log output.
package secretredact

import (
	"fmt"
	"regexp"
	"strings"
)

// privateKeyRegex matches PEM private-key blocks; identical for every caller.
var privateKeyRegex = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)

var passwordRegex = regexp.MustCompile(`(?i)"?(password|secret|passphrase|token|api_key)"?\s*[:=]\s*["']?([^"' \n\r,}]+)["']?`)

// ScrubSecrets masks passwords, private keys and tokens from a string,
// normalizing the separator to '=' and emitting an unquoted marker.
func ScrubSecrets(input string) string {
	if input == "" {
		return ""
	}
	out := privateKeyRegex.ReplaceAllString(input, "[REDACTED_PRIVATE_KEY]")
	out = passwordRegex.ReplaceAllString(out, "$1=[REDACTED_SECRET]")
	return out
}

var (
	secretKeyRegex  = regexp.MustCompile(`(?i)(password|secret|passphrase|token|api_key|sudo_secret)\s*[:=]\s*["']?([^"' \n\r,]+)["']?`)
	authHeaderRegex = regexp.MustCompile(`(?i)(Bearer|Basic)\s+[A-Za-z0-9_\-\.~+\/=]+`)
)

// ScrubLogText masks passwords, private keys, tokens and auth headers for log
// output, preserving the original separator and quoting the marker.
func ScrubLogText(input string) string {
	if input == "" {
		return ""
	}
	out := privateKeyRegex.ReplaceAllString(input, "[REDACTED_PRIVATE_KEY]")
	out = authHeaderRegex.ReplaceAllString(out, "$1 [REDACTED_TOKEN]")
	out = secretKeyRegex.ReplaceAllStringFunc(out, func(m string) string {
		parts := strings.SplitN(m, ":", 2)
		sep := ":"
		if len(parts) < 2 {
			parts = strings.SplitN(m, "=", 2)
			sep = "="
		}
		if len(parts) == 2 {
			return fmt.Sprintf("%s%s\"[REDACTED_SECRET]\"", parts[0], sep)
		}
		return m
	})
	return out
}
