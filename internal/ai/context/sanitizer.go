package context

import (
	"regexp"
	"strings"

	"tamestack/internal/crypto"
)

var (
	// SSH and TLS Private Keys.
	rePrivateKey = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	rePuttyKey   = regexp.MustCompile(`(?s)PuTTY-User-Key-File-[0-9]+:.*?Private-Lines:.*`)

	// Sensitive key-value assignments: password, secret, token, api_key.
	reKeyValueSecret = regexp.MustCompile(`(?i)(password|passwd|pwd|db_pass|db_password|secret|client_secret|api_key|auth_token)[\s:=]+['"]?([a-zA-Z0-9!@#$%^&*()_+=\-{}\[\]:;<>,.?/~]{4,})['"]?`)

	// Connection strings: proto://user:password@host.
	reConnString = regexp.MustCompile(`(?i)([a-z0-9+]+://[^:\s]+:)([^@\s]+)(@)`)

	// Bearer & JWT tokens.
	reJWT    = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.?[a-zA-Z0-9_.-]*\b`)
	reBearer = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]{20,}`)

	// Shadow password hashes ($6$, $y$, $1$).
	reShadowHash = regexp.MustCompile(`\$[156y]\$[a-zA-Z0-9./]+\$[a-zA-Z0-9./]{20,}`)
)

// Sanitizer provides high-rigor data masking for LLM prompts and context.
type Sanitizer struct{}

// NewSanitizer creates a new sanitizer instance.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{}
}

// Sanitize removes credentials, private keys, hashes and sensitive tokens from text.
func (s *Sanitizer) Sanitize(input string) string {
	if input == "" {
		return ""
	}

	// 1. Core scrubber from TameStack crypto vault.
	out := crypto.ScrubSecrets(input)

	// 2. Private Keys.
	out = rePrivateKey.ReplaceAllString(out, "[REDACTED_PRIVATE_KEY]")
	out = rePuttyKey.ReplaceAllString(out, "[REDACTED_PUTTY_KEY]")

	// 3. Shadow password hashes.
	out = reShadowHash.ReplaceAllString(out, "$[REDACTED_PASSWORD_HASH]")

	// 4. JWT and Bearer tokens.
	out = reJWT.ReplaceAllString(out, "[REDACTED_JWT_TOKEN]")
	out = reBearer.ReplaceAllString(out, "Bearer [REDACTED_BEARER_TOKEN]")

	// 5. Connection strings.
	out = reConnString.ReplaceAllString(out, "${1}[REDACTED_PASSWORD]${3}")

	// 6. Generic password / secret assignments.
	out = reKeyValueSecret.ReplaceAllStringFunc(out, func(match string) string {
		parts := reKeyValueSecret.FindStringSubmatch(match)
		if len(parts) >= 3 {
			key := parts[1]
			sep := ":"
			if strings.Contains(match, "=") {
				sep = "="
			}
			return key + sep + " \"[REDACTED_SECRET]\""
		}
		return match
	})

	return out
}
