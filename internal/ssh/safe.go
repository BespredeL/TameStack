package ssh

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var (
	phpVersionPattern  = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
	tokenPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]{0,127}$`)
	chmodPattern       = regexp.MustCompile(`^[0-7]{3,4}$`)
	portPattern        = regexp.MustCompile(`^[0-9]{1,5}(:[0-9]{1,5})?$`)
	identPattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)
	charsetPattern     = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	hostPattern        = regexp.MustCompile(`^[A-Za-z0-9_.:%-]{1,255}$`)
	imagePattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./:@-]{0,255}$`)
	ipPattern          = regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}(/[0-9]{1,2})?$|^[A-Za-z0-9:./]{1,64}$`)
)

// policyUnsafePathChars are shell-significant bytes that must never appear in a path the
// AI subsystem manages. Quoting already neutralises them; the check is defence in depth for
// every consumer of the path, including ones that treat it as plain text.
const policyUnsafePathChars = "\r\n\t\x00'\"`$\\|&;<>(){}*?[]"

var logTypes = map[string]bool{"error": true, "access": true}

var systemctlActions = map[string]bool{
	"start": true, "stop": true, "restart": true, "reload": true, "status": true, "enable": true, "disable": true,
}

var firewallActions = map[string]bool{
	"allow": true, "deny": true, "reject": true, "limit": true, "delete": true,
}

var firewallProtos = map[string]bool{
	"tcp": true, "udp": true,
}

// ValidatePHPVersion accepts dotted versions such as 8.2.
func ValidatePHPVersion(version string) error {
	if !phpVersionPattern.MatchString(version) {
		return fmt.Errorf("invalid PHP version %q", version)
	}
	return nil
}

// ValidateToken accepts extension names, container ids, and similar shell tokens.
func ValidateToken(value, label string) error {
	if !tokenPattern.MatchString(value) {
		return fmt.Errorf("invalid %s %q", label, value)
	}
	return nil
}

// ValidateIdent accepts database names, cron users, and similar identifiers.
func ValidateIdent(value, label string) error {
	if !identPattern.MatchString(value) {
		return fmt.Errorf("invalid %s %q", label, value)
	}
	return nil
}

// ValidateCharset accepts MySQL charset and collation names.
func ValidateCharset(value string) error {
	if value == "" || !charsetPattern.MatchString(value) {
		return fmt.Errorf("invalid charset or collation %q", value)
	}
	return nil
}

// ValidateHostToken accepts database host values such as localhost or %.
func ValidateHostToken(value string) error {
	if !hostPattern.MatchString(value) || strings.Contains(value, "..") {
		return fmt.Errorf("invalid host %q", value)
	}
	return nil
}

// ValidateServiceName accepts systemd unit names such as nginx, php8.1-fpm or app@worker.
func ValidateServiceName(name string) error {
	if !serviceNamePattern.MatchString(name) {
		return fmt.Errorf("invalid service name %q", name)
	}
	return nil
}

// ValidateLogType restricts log selectors to the read-only categories TameStack exposes.
func ValidateLogType(logType string) error {
	if !logTypes[logType] {
		return fmt.Errorf("invalid log type %q, expected error or access", logType)
	}
	return nil
}

// ValidatePolicyPath rejects paths that a shell, a glob or a tilde expansion could reinterpret.
func ValidatePolicyPath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "~") {
		return fmt.Errorf("path must not start with ~: %s", p)
	}
	if strings.ContainsAny(p, policyUnsafePathChars) {
		return fmt.Errorf("path contains characters that are not allowed: %s", p)
	}
	return nil
}

// ValidateSystemctlAction restricts service actions to a fixed set.
func ValidateSystemctlAction(action string) error {
	if !systemctlActions[action] {
		return fmt.Errorf("invalid service action %q", action)
	}
	return nil
}

// ValidateChmodMode accepts an octal mode.
func ValidateChmodMode(mode string) error {
	if !chmodPattern.MatchString(mode) {
		return fmt.Errorf("invalid chmod mode %q", mode)
	}
	return nil
}

// ValidatePortSpec accepts a port or port range.
func ValidatePortSpec(port string) error {
	if !portPattern.MatchString(port) {
		return fmt.Errorf("invalid port %q", port)
	}
	return nil
}

// ValidateFirewallAction restricts firewall verbs.
func ValidateFirewallAction(action string) error {
	if !firewallActions[strings.ToLower(action)] {
		return fmt.Errorf("invalid firewall action %q", action)
	}
	return nil
}

// ValidateFirewallProto restricts protocols.
func ValidateFirewallProto(proto string) error {
	if proto == "" || strings.EqualFold(proto, "any") {
		return nil
	}
	if !firewallProtos[strings.ToLower(proto)] {
		return fmt.Errorf("invalid protocol %q", proto)
	}
	return nil
}

// ValidateSource allows an IP, CIDR, or the words any/anywhere.
func ValidateSource(source string) error {
	if source == "" || source == "any" || source == "anywhere" {
		return nil
	}
	if strings.ContainsAny(source, "\r\n;|&$`") || !ipPattern.MatchString(source) {
		return fmt.Errorf("invalid source %q", source)
	}
	return nil
}

// ValidateImageName accepts a docker image reference without shell metacharacters.
func ValidateImageName(image string) error {
	if !imagePattern.MatchString(image) || strings.Contains(image, "..") {
		return fmt.Errorf("invalid image %q", image)
	}
	return nil
}

// ValidateRemotePath requires an absolute path without newlines or parent segments.
// allowRoot permits "/" itself; destructive operations should pass false.
func ValidateRemotePath(p string, allowRoot bool) error {
	if p == "" || strings.ContainsAny(p, "\r\n\x00") {
		return fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must be absolute: %s", p)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path must not contain ..: %s", p)
	}
	clean := path.Clean(p)
	if clean != p && !(strings.HasSuffix(p, "/") && clean+"/" == p) {
		return fmt.Errorf("path is not canonical: %s", p)
	}
	if !allowRoot && (clean == "/" || clean == "/etc" || clean == "/var" || clean == "/usr" || clean == "/home" || clean == "/root" || clean == "/boot" || clean == "/bin" || clean == "/tmp") {
		return fmt.Errorf("path %s is too broad", clean)
	}
	return nil
}

// ValidateComposeAction restricts docker compose verbs.
func ValidateComposeAction(action string) error {
	switch action {
	case "up", "down", "restart", "pull":
		return nil
	default:
		return fmt.Errorf("invalid compose action %q", action)
	}
}

// ValidateDockerAction restricts container verbs.
func ValidateDockerAction(action string) error {
	switch action {
	case "start", "stop", "restart", "kill", "rm", "remove":
		return nil
	default:
		return fmt.Errorf("invalid docker action %q", action)
	}
}
