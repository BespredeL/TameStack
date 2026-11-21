package planner

import (
	"fmt"
	"strings"

	"tamestack/internal/provisioning/inventory"
	"tamestack/internal/provisioning/models"
)

// GeneratePlan produces a granular pre-flight execution plan comparing inventory with blueprint.
func GeneratePlan(serverID string, inv *inventory.ServerInventory, bp models.Blueprint) (*models.ProvisioningPlan, error) {
	plan := &models.ProvisioningPlan{
		ServerID:         serverID,
		Profile:          bp.Profile,
		OSDistro:         inv.Distro,
		OSFamily:         inv.Family,
		OSVersion:        inv.Version,
		Arch:             inv.Arch,
		SSHPortPreserved: inv.SSHPort,
		IsCompatible:     true,
		EstimatedTimeMin: 5,
	}

	if plan.SSHPortPreserved <= 0 {
		plan.SSHPortPreserved = 22
	}

	// 0. Server existing stack & safety checks
	if inv.IsAlreadyConfigured {
		var existingComponents []string
		if len(inv.InstalledWeb) > 0 {
			existingComponents = append(existingComponents, fmt.Sprintf("Web: %s", strings.Join(inv.InstalledWeb, ", ")))
		}
		if len(inv.InstalledPHP) > 0 {
			existingComponents = append(existingComponents, fmt.Sprintf("PHP: %s", strings.Join(inv.InstalledPHP, ", ")))
		}
		if len(inv.InstalledDB) > 0 {
			existingComponents = append(existingComponents, fmt.Sprintf("DB: %s", strings.Join(inv.InstalledDB, ", ")))
		}
		if inv.ExistingSitesCount > 0 {
			existingComponents = append(existingComponents, fmt.Sprintf("Сайты в /var/www: %d", inv.ExistingSitesCount))
		}
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"Обнаружен уже настроенный сервер (%s). Мастер активировал безопасный режим дополнения (Safe Additive Mode): существующие сайты, виртуальные хосты, базы данных и файлы не будут перезаписаны.",
			strings.Join(existingComponents, "; "),
		))
	}

	// 1. Hardware checks & warnings
	if inv.RAMTotalMB > 0 && inv.RAMTotalMB < 512 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("Низкий объём оперативной памяти (%d МБ). Для стабильной работы рекомендуется минимум 1024 МБ.", inv.RAMTotalMB))
	} else if inv.RAMTotalMB > 0 && inv.RAMTotalMB < 1024 && bp.Database.Type != "none" {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("Объём оперативной памяти %d МБ может быть недостаточен для одновременной работы PHP и СУБД.", inv.RAMTotalMB))
	}

	if inv.DiskFreeGB > 0 && inv.DiskFreeGB < 2 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("Мало свободного места на диске (%d ГБ). Рекомендуется освободить место перед установкой пакетов.", inv.DiskFreeGB))
	}

	// 2. Webserver planning
	wsType := strings.ToLower(bp.Webserver.Type)
	if wsType == "nginx" || wsType == "both" {
		isInstalled := contains(inv.InstalledWeb, "nginx")
		pkg := "nginx"
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      pkg,
			Description: "Установка Nginx веб-сервера",
			Skip:        isInstalled,
		})
		if !isInstalled {
			plan.PackagesToInstall = append(plan.PackagesToInstall, pkg)
		}
		plan.ServicesToStart = append(plan.ServicesToStart, "nginx")
		plan.PortsToOpen = append(plan.PortsToOpen, 80, 443)

		// Conflict check: if Apache is already listening on port 80
		if containsInt(inv.ListeningPorts, 80) && contains(inv.InstalledWeb, "apache") && !isInstalled {
			plan.PotentialConflicts = append(plan.PotentialConflicts, "Порт 80 уже занят (возможно, Apache). Перед запуском Nginx потребуется освободить порт 80 или настроить проксирование.")
		}
	}

	if wsType == "apache" || wsType == "both" {
		isInstalled := contains(inv.InstalledWeb, "apache")
		pkg := "apache2"
		srv := "apache2"
		if inv.Family == "rhel" {
			pkg = "httpd"
			srv = "httpd"
		}
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      pkg,
			Description: "Установка Apache HTTP Server",
			Skip:        isInstalled,
		})
		if !isInstalled {
			plan.PackagesToInstall = append(plan.PackagesToInstall, pkg)
		}
		plan.ServicesToStart = append(plan.ServicesToStart, srv)
		plan.PortsToOpen = append(plan.PortsToOpen, 80, 443)

		if wsType == "apache" && containsInt(inv.ListeningPorts, 80) && contains(inv.InstalledWeb, "nginx") && !isInstalled {
			plan.PotentialConflicts = append(plan.PotentialConflicts, "Порт 80 уже занят (возможно, Nginx). Перед запуском Apache потребуется освободить порт 80 или настроить проксирование.")
		}
	}

	// 3. PHP Planning
	if len(bp.PHP.Versions) > 0 {
		// Repositories
		if inv.Family == "debian" {
			plan.RepositoriesNeeded = append(plan.RepositoriesNeeded, "ppa:ondrej/php (или packages.sury.org)")
		} else if inv.Family == "rhel" {
			plan.RepositoriesNeeded = append(plan.RepositoriesNeeded, "epel-release", "remi-release")
		}

		for _, v := range bp.PHP.Versions {
			cleanVer := v.Version
			isInstalled := contains(inv.InstalledPHP, cleanVer)

			var pkgs []string
			if inv.Family == "debian" {
				pkgs = append(pkgs, fmt.Sprintf("php%s-fpm", cleanVer), fmt.Sprintf("php%s-cli", cleanVer), fmt.Sprintf("php%s-common", cleanVer))
				for _, ext := range v.Extensions {
					if ext == "" || ext == "opcache" {
						pkgs = append(pkgs, fmt.Sprintf("php%s-opcache", cleanVer))
						continue
					}
					pkgs = append(pkgs, fmt.Sprintf("php%s-%s", cleanVer, ext))
				}
			} else {
				compact := strings.ReplaceAll(cleanVer, ".", "")
				pkgs = append(pkgs, fmt.Sprintf("php%s-php-fpm", compact), fmt.Sprintf("php%s-php-cli", compact))
				for _, ext := range v.Extensions {
					if ext != "" {
						pkgs = append(pkgs, fmt.Sprintf("php%s-php-%s", compact, ext))
					}
				}
			}

			plan.Items = append(plan.Items, models.PlanItem{
				Category:    "package",
				Action:      "install",
				Target:      fmt.Sprintf("php%s (fpm + extensions)", cleanVer),
				Description: fmt.Sprintf("PHP %s и %d расширений", cleanVer, len(v.Extensions)),
				Skip:        isInstalled,
			})

			if !isInstalled {
				plan.PackagesToInstall = append(plan.PackagesToInstall, pkgs...)
			}

			srvName := fmt.Sprintf("php%s-fpm", cleanVer)
			if inv.Family == "rhel" {
				srvName = fmt.Sprintf("php%s-php-fpm", strings.ReplaceAll(cleanVer, ".", ""))
			}
			plan.ServicesToStart = append(plan.ServicesToStart, srvName)
		}
	}

	// 4. Database Planning
	dbType := strings.ToLower(bp.Database.Type)
	switch dbType {
	case "mariadb":
		isInstalled := contains(inv.InstalledDB, "mariadb")
		pkg := "mariadb-server"
		srv := "mariadb"
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      pkg,
			Description: "Установка СУБД MariaDB Server",
			Skip:        isInstalled,
		})
		if !isInstalled {
			plan.PackagesToInstall = append(plan.PackagesToInstall, pkg)
		}
		plan.ServicesToStart = append(plan.ServicesToStart, srv)

	case "mysql":
		isInstalled := contains(inv.InstalledDB, "mysql")
		pkg := "mysql-server"
		srv := "mysql"
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      pkg,
			Description: "Установка СУБД MySQL Server",
			Skip:        isInstalled,
		})
		if !isInstalled {
			plan.PackagesToInstall = append(plan.PackagesToInstall, pkg)
		}
		plan.ServicesToStart = append(plan.ServicesToStart, srv)

	case "postgres":
		isInstalled := contains(inv.InstalledDB, "postgres")
		pkg := "postgresql"
		srv := "postgresql"
		if inv.Family == "rhel" {
			pkg = "postgresql-server"
		}
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      pkg,
			Description: "Установка СУБД PostgreSQL",
			Skip:        isInstalled,
		})
		if !isInstalled {
			plan.PackagesToInstall = append(plan.PackagesToInstall, pkg)
		}
		plan.ServicesToStart = append(plan.ServicesToStart, srv)
	}

	// 5. Cache Planning (Redis)
	if strings.ToLower(bp.Cache.Type) == "redis" {
		isInstalled := contains(inv.InstalledDB, "redis")
		pkg := "redis-server"
		srv := "redis-server"
		if inv.Family == "rhel" {
			pkg = "redis"
			srv = "redis"
		}
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      pkg,
			Description: "Установка In-Memory Cache Redis",
			Skip:        isInstalled,
		})
		if !isInstalled {
			plan.PackagesToInstall = append(plan.PackagesToInstall, pkg)
		}
		plan.ServicesToStart = append(plan.ServicesToStart, srv)
	}

	// 6. Tools (Composer, Git, Node.js)
	if bp.Tools.Git {
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      "git",
			Description: "Система контроля версий Git",
			Skip:        inv.HasGit,
		})
		if !inv.HasGit {
			plan.PackagesToInstall = append(plan.PackagesToInstall, "git")
		}
	}

	if bp.Tools.Composer {
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      "composer",
			Description: "Пакетный менеджер PHP Composer",
			Skip:        inv.HasComposer,
		})
		if !inv.HasComposer {
			plan.PackagesToInstall = append(plan.PackagesToInstall, "composer")
		}
	}

	if bp.Tools.NodeJS {
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      "nodejs",
			Description: "Node.js & npm runtime",
			Skip:        inv.HasNode,
		})
		if !inv.HasNode {
			if inv.Family == "debian" {
				plan.PackagesToInstall = append(plan.PackagesToInstall, "nodejs", "npm")
			} else {
				plan.PackagesToInstall = append(plan.PackagesToInstall, "nodejs")
			}
		}
	}

	// 7. Containers (Docker & Compose)
	if bp.Containers.Docker {
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      "docker",
			Description: "Docker Engine и Docker Compose",
			Skip:        inv.HasDocker,
		})
		if !inv.HasDocker {
			if inv.Family == "debian" {
				plan.PackagesToInstall = append(plan.PackagesToInstall, "docker.io", "docker-compose-plugin")
			} else {
				plan.PackagesToInstall = append(plan.PackagesToInstall, "docker-ce", "docker-compose-plugin")
			}
		}
		plan.ServicesToStart = append(plan.ServicesToStart, "docker")
	}

	// 8. Security & Firewall
	fwType := strings.ToLower(bp.Security.Firewall)
	if fwType == "ufw" {
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "firewall",
			Action:      "allow",
			Target:      fmt.Sprintf("SSH port %d", plan.SSHPortPreserved),
			Description: "Обязательное разрешение порта SSH перед включением брандмауэра",
			Skip:        false,
		})
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      "ufw",
			Description: "UFW брандмауэр",
			Skip:        inv.HasUFW,
		})
		if !inv.HasUFW {
			plan.PackagesToInstall = append(plan.PackagesToInstall, "ufw")
		}
	} else if fwType == "firewalld" {
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "firewall",
			Action:      "allow",
			Target:      fmt.Sprintf("SSH port %d", plan.SSHPortPreserved),
			Description: "Обязательное разрешение порта SSH",
			Skip:        false,
		})
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      "firewalld",
			Description: "Firewalld брандмауэр",
			Skip:        inv.HasFirewalld,
		})
		if !inv.HasFirewalld {
			plan.PackagesToInstall = append(plan.PackagesToInstall, "firewalld")
		}
		plan.ServicesToStart = append(plan.ServicesToStart, "firewalld")
	}

	if bp.Security.Fail2ban {
		plan.Items = append(plan.Items, models.PlanItem{
			Category:    "package",
			Action:      "install",
			Target:      "fail2ban",
			Description: "Защита от подбора паролей Fail2ban",
			Skip:        inv.HasFail2ban,
		})
		if !inv.HasFail2ban {
			plan.PackagesToInstall = append(plan.PackagesToInstall, "fail2ban")
		}
		plan.ServicesToStart = append(plan.ServicesToStart, "fail2ban")
	}

	// 9. Directories & Users
	plan.Items = append(plan.Items, models.PlanItem{
		Category:    "user",
		Action:      "create",
		Target:      "www-data / nginx",
		Description: "Системный пользователь для веб-сервера и сайтов",
		Skip:        false,
	})
	plan.Items = append(plan.Items, models.PlanItem{
		Category:    "dir",
		Action:      "create",
		Target:      "/var/www",
		Description: "Корневой каталог для веб-сайтов с правами 0755",
		Skip:        false,
	})

	// Preserve all currently listening ports in PortsToOpen so firewall rules don't block existing services
	for _, p := range inv.ListeningPorts {
		if p > 0 && p < 65536 && !containsInt(plan.PortsToOpen, p) {
			plan.PortsToOpen = append(plan.PortsToOpen, p)
		}
	}
	if !containsInt(plan.PortsToOpen, plan.SSHPortPreserved) {
		plan.PortsToOpen = append(plan.PortsToOpen, plan.SSHPortPreserved)
	}

	// Estimated time
	plan.EstimatedTimeMin = 2 + len(plan.PackagesToInstall)/4
	if plan.EstimatedTimeMin < 3 {
		plan.EstimatedTimeMin = 3
	}

	return plan, nil
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return true
		}
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
