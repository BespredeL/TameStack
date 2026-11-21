package policies

import (
	"fmt"
	"path"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/ssh"
)

// AllowedConfigPrefixes lists system directories where AI is allowed to manage configs.
var AllowedConfigPrefixes = []string{
	"/etc/nginx",
	"/etc/apache2",
	"/etc/httpd",
	"/etc/mysql",
	"/etc/php",
	"/var/www",
}

// BlockedCriticalPaths lists directories and files where AI modification is strictly prohibited.
var BlockedCriticalPaths = []string{
	"/boot",
	"/root",
	"/bin",
	"/sbin",
	"/usr/bin",
	"/usr/sbin",
	"/etc/shadow",
	"/etc/gshadow",
	"/etc/sudoers",
	"/etc/passwd",
	"/etc/pam.d",
	"/etc/ssh/sshd_config",
}

// SecurityGate enforces boundary constraints, path protection and human approval.
type SecurityGate struct{}

// NewSecurityGate creates a new SecurityGate instance.
func NewSecurityGate() *SecurityGate {
	return &SecurityGate{}
}

// ValidatePath checks if targetPath is safe to modify by AI. It runs on the requested name;
// callers that write remotely must re-run it on the symlink-resolved target.
func (g *SecurityGate) ValidatePath(targetPath string) error {
	trimmed := strings.TrimSpace(targetPath)
	if err := ssh.ValidatePolicyPath(trimmed); err != nil {
		return fmt.Errorf("путь '%s' отклонен политикой безопасности: %w", targetPath, err)
	}
	if strings.Contains(trimmed, "..") {
		return fmt.Errorf("путь '%s' содержит недопустимые элементы перехода '..'", trimmed)
	}
	clean := path.Clean(trimmed)
	if clean != trimmed && !(strings.HasSuffix(trimmed, "/") && clean+"/" == trimmed) {
		return fmt.Errorf("путь '%s' должен быть в канонической форме ('%s')", trimmed, clean)
	}

	// Check against strictly blocked paths.
	for _, blocked := range BlockedCriticalPaths {
		if clean == blocked || strings.HasPrefix(clean, blocked+"/") {
			return fmt.Errorf("модификация пути '%s' строго запрещена политикой безопасности TameStack", clean)
		}
	}

	// Must match at least one allowed prefix.
	isAllowed := false
	for _, allowed := range AllowedConfigPrefixes {
		if clean == allowed || strings.HasPrefix(clean, allowed+"/") {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return fmt.Errorf("путь '%s' находится вне разрешенных каталогов конфигурации веб-серверов или сайтов (%s)", clean, strings.Join(AllowedConfigPrefixes, ", "))
	}

	return nil
}

// RequiresHumanApproval determines if an action must be gated behind human confirmation.
// Only tools explicitly classified as safe read-only inspection run without approval, so a
// tool that touches the server but is mislabelled as non-mutating still reaches the user.
// Proposal tools (ApprovalViaTask) are exempt: they only create a requires-approval task,
// and the actual mutation is gated by the user's approval of that task.
func (g *SecurityGate) RequiresHumanApproval(toolDef domain.ToolDefinition) bool {
	if toolDef.ApprovalViaTask {
		return false
	}
	return toolDef.IsMutating || toolDef.RiskLevel != domain.RiskSafe
}
