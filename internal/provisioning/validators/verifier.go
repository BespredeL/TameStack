package validators

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/provisioning/models"
	"tamestack/internal/ssh"
)

// VerifyEnvironment runs validation checks on all requested components.
func VerifyEnvironment(ctx context.Context, client *ssh.SSHClient, bp models.Blueprint) []models.VerificationCheck {
	var checks []models.VerificationCheck

	// 1. Web server check
	wsType := strings.ToLower(bp.Webserver.Type)
	if wsType == "nginx" || wsType == "both" {
		out, _, code, _ := client.Exec(ctx, "systemctl is-active nginx 2>/dev/null")
		status := strings.TrimSpace(string(out))
		checks = append(checks, models.VerificationCheck{
			Component: "Nginx Web Server",
			Passed:    code == 0 && status == "active",
			Message:   fmt.Sprintf("Статус службы Nginx: %s", status),
		})
	}
	if wsType == "apache" || wsType == "both" {
		out, _, code, _ := client.Exec(ctx, "systemctl is-active apache2 2>/dev/null || systemctl is-active httpd 2>/dev/null")
		status := strings.TrimSpace(string(out))
		checks = append(checks, models.VerificationCheck{
			Component: "Apache Web Server",
			Passed:    code == 0 && status == "active",
			Message:   fmt.Sprintf("Статус службы Apache: %s", status),
		})
	}

	// 2. PHP Runtimes check
	for _, v := range bp.PHP.Versions {
		ver := v.Version
		outCli, _, codeCli, _ := client.Exec(ctx, fmt.Sprintf("php%s -v 2>/dev/null | head -n 1 || php -v 2>/dev/null | head -n 1", ver))
		cliPassed := codeCli == 0 && len(strings.TrimSpace(string(outCli))) > 0
		checks = append(checks, models.VerificationCheck{
			Component: fmt.Sprintf("PHP %s CLI", ver),
			Passed:    cliPassed,
			Message:   strings.TrimSpace(string(outCli)),
		})

		outFpm, _, codeFpm, _ := client.Exec(ctx, fmt.Sprintf("systemctl is-active php%s-fpm 2>/dev/null || systemctl is-active php%s-php-fpm 2>/dev/null || systemctl is-active php-fpm 2>/dev/null", ver, strings.ReplaceAll(ver, ".", "")))
		fpmStatus := strings.TrimSpace(string(outFpm))
		checks = append(checks, models.VerificationCheck{
			Component: fmt.Sprintf("PHP %s FPM Service", ver),
			Passed:    codeFpm == 0 && fpmStatus == "active",
			Message:   fmt.Sprintf("Статус FPM: %s", fpmStatus),
		})
	}

	// 3. Database check
	dbType := strings.ToLower(bp.Database.Type)
	switch dbType {
	case "mariadb", "mysql":
		out, _, code, _ := client.Exec(ctx, "systemctl is-active mariadb 2>/dev/null || systemctl is-active mysql 2>/dev/null")
		status := strings.TrimSpace(string(out))
		checks = append(checks, models.VerificationCheck{
			Component: "Database Engine (" + dbType + ")",
			Passed:    code == 0 && status == "active",
			Message:   fmt.Sprintf("Статус службы СУБД: %s", status),
		})
	case "postgres":
		out, _, code, _ := client.Exec(ctx, "systemctl is-active postgresql 2>/dev/null")
		status := strings.TrimSpace(string(out))
		checks = append(checks, models.VerificationCheck{
			Component: "PostgreSQL Database Engine",
			Passed:    code == 0 && status == "active",
			Message:   fmt.Sprintf("Статус службы PostgreSQL: %s", status),
		})
	}

	// 4. Redis cache check
	if strings.ToLower(bp.Cache.Type) == "redis" {
		out, _, code, _ := client.Exec(ctx, "systemctl is-active redis-server 2>/dev/null || systemctl is-active redis 2>/dev/null")
		status := strings.TrimSpace(string(out))
		checks = append(checks, models.VerificationCheck{
			Component: "Redis In-Memory Cache",
			Passed:    code == 0 && status == "active",
			Message:   fmt.Sprintf("Статус службы Redis: %s", status),
		})
	}

	// 5. Docker check
	if bp.Containers.Docker {
		out, _, code, _ := client.Exec(ctx, "docker --version 2>/dev/null")
		status := strings.TrimSpace(string(out))
		checks = append(checks, models.VerificationCheck{
			Component: "Docker Engine",
			Passed:    code == 0 && status != "",
			Message:   status,
		})
	}

	// 6. Security (SSH & Firewall)
	fwType := strings.ToLower(bp.Security.Firewall)
	if fwType == "ufw" {
		out, _, _, _ := client.Exec(ctx, "ufw status 2>/dev/null | head -n 1")
		checks = append(checks, models.VerificationCheck{
			Component: "UFW Firewall",
			Passed:    true,
			Message:   strings.TrimSpace(string(out)),
		})
	} else if fwType == "firewalld" {
		out, _, code, _ := client.Exec(ctx, "systemctl is-active firewalld 2>/dev/null")
		checks = append(checks, models.VerificationCheck{
			Component: "Firewalld",
			Passed:    code == 0,
			Message:   strings.TrimSpace(string(out)),
		})
	}

	return checks
}
