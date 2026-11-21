package php

import (
	"context"
	"fmt"
	"strings"

	"tamestack/internal/module"
	"tamestack/internal/platform/capabilities"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// InstallVersionPackages configures repositories, installs PHP-FPM packages and
// enables the FPM service. Shared by the php_install task and site provisioning.
func InstallVersionPackages(ctx context.Context, client *ssh.SSHClient, osFamily, version string, extensions []string, logger tasks.TaskLogger) error {
	logger.SetStep("Checking and configuring repositories", 25)
	if osFamily == "debian" {
		outOS, _, _, _ := client.Exec(ctx, "cat /etc/os-release 2>/dev/null || true")
		if ok, reason := capabilities.AllowsPHPInstall(string(outOS), version); !ok {
			return fmt.Errorf("%s", reason)
		}
		compat := DetectDistroCompatibility(string(outOS))
		archiveOnly := compat.DistroID == "ubuntu" && strings.HasPrefix(compat.VersionID, "18.04")

		if archiveOnly {
			logger.Info(fmt.Sprintf("Ubuntu %s: installing PHP from the distro archive only.", compat.VersionID))
			if err := module.RunPrivileged(ctx, client, "apt-get update -y"); err != nil {
				return err
			}
		} else {
			logger.Info("Checking software-properties-common and apt-transport-https...")
			if err := module.RunPrivileged(ctx, client, "apt-get update -y && apt-get install -y --no-install-recommends software-properties-common apt-transport-https ca-certificates curl gnupg"); err != nil {
				return err
			}

			// Check if PPA / sury repository is present.
			repoCheckCmd := "grep -E 'ondrej/php|sury' /etc/apt/sources.list /etc/apt/sources.list.d/* 2>/dev/null"
			out, _, _, _ := client.NewCommand(repoCheckCmd).WithSudo().Exec(ctx)
			if len(out) == 0 {
				logger.Info("Adding Ondrej PHP PPA repository for multi-version PHP support...")
				addCmd := "add-apt-repository -y ppa:ondrej/php 2>&1 || (curl -sSLo /etc/apt/trusted.gpg.d/php.gpg https://packages.sury.org/php/apt.gpg && echo 'deb https://packages.sury.org/php/ '$(lsb_release -sc)' main' > /etc/apt/sources.list.d/php.list)"
				outAdd, stderrAdd, codeAdd, errAdd := client.NewCommand(addCmd).
					WithSudo().
					WithLock("pkg_mgr", 180).
					WithEnv("DEBIAN_FRONTEND", "noninteractive").
					Exec(ctx)
				logger.Info(string(outAdd))
				if errAdd != nil || codeAdd != 0 {
					return fmt.Errorf("adding PHP repository failed (exit %d): %s", codeAdd, strings.TrimSpace(string(stderrAdd)))
				}
				logger.Info("Updating package index...")
				if err := module.RunPrivileged(ctx, client, "apt-get update -y"); err != nil {
					return err
				}
			} else {
				logger.Info("PHP repository is already configured.")
			}
		}
	} else if osFamily == "rhel" {
		logger.Info("Configuring EPEL and Remi RPM repositories...")
		if err := module.RunPrivileged(ctx, client, "dnf install -y epel-release https://rpms.remirepo.net/enterprise/remi-release-$(rpm -E %rhel).rpm"); err != nil {
			return err
		}
	}

	logger.SetStep("Preparing package dependencies", 40)
	var packages []string
	if osFamily == "debian" {
		packages = []string{
			fmt.Sprintf("php%s-fpm", version),
			fmt.Sprintf("php%s-cli", version),
			fmt.Sprintf("php%s-common", version),
			fmt.Sprintf("php%s-opcache", version),
		}
		for _, ext := range extensions {
			ext = strings.TrimSpace(ext)
			if ext != "" && ext != "opcache" {
				packages = append(packages, fmt.Sprintf("php%s-%s", version, ext))
			}
		}
	} else {
		verCompact := strings.ReplaceAll(version, ".", "")
		packages = []string{
			fmt.Sprintf("php%s-php-fpm", verCompact),
			fmt.Sprintf("php%s-php-cli", verCompact),
		}
	}

	logger.Info(fmt.Sprintf("Packages to install: %s", strings.Join(packages, " ")))
	logger.SetStep(fmt.Sprintf("Installing PHP %s and extensions via package manager", version), 60)

	var installBuilder *ssh.CommandBuilder
	if osFamily == "debian" {
		installCmd := fmt.Sprintf("apt-get install -y --no-install-recommends %s", strings.Join(packages, " "))
		installBuilder = client.NewCommand(installCmd).
			WithSudo().
			WithLock("pkg_mgr", 180).
			WithEnv("DEBIAN_FRONTEND", "noninteractive")
	} else {
		installCmd := fmt.Sprintf("dnf install -y %s", strings.Join(packages, " "))
		installBuilder = client.NewCommand(installCmd).
			WithSudo().
			WithLock("pkg_mgr", 180)
	}

	outInstall, errInstall, code, err := installBuilder.Exec(ctx)
	logger.Info(string(outInstall))
	if err != nil || code != 0 {
		return fmt.Errorf("package installation failed with exit code %d: %s", code, string(errInstall))
	}

	logger.SetStep("Enabling and starting PHP-FPM systemd service", 85)
	serviceName := fmt.Sprintf("php%s-fpm", version)
	if osFamily == "rhel" {
		serviceName = fmt.Sprintf("php%s-php-fpm", strings.ReplaceAll(version, ".", ""))
	}
	srvCmd := fmt.Sprintf("systemctl enable --now %s && systemctl is-active %s", serviceName, serviceName)
	outSrv, _, codeSrv, _ := client.NewCommand(srvCmd).WithSudo().Exec(ctx)
	if codeSrv != 0 || !strings.Contains(string(outSrv), "active") {
		logger.Warn(fmt.Sprintf("PHP-FPM service state: %s. Attempting restart...", strings.TrimSpace(string(outSrv))))
		outRestart, stderrRestart, codeRestart, errRestart := client.NewCommand("systemctl restart " + ssh.ShellEscape(serviceName) + " && systemctl is-active " + ssh.ShellEscape(serviceName)).WithSudo().Exec(ctx)
		if errRestart != nil || codeRestart != 0 || !strings.Contains(string(outRestart), "active") {
			return fmt.Errorf("PHP-FPM %s did not become active: %s", serviceName, strings.TrimSpace(string(stderrRestart)))
		}
	} else {
		logger.Info(fmt.Sprintf("Service %s is ACTIVE and listening on socket.", serviceName))
	}

	verCheckCmd := fmt.Sprintf("php%s -v | head -n 1", version)
	outVer, _, _, _ := client.Exec(ctx, verCheckCmd)
	logger.Info(fmt.Sprintf("Verified PHP binary: %s", strings.TrimSpace(string(outVer))))

	return nil
}
