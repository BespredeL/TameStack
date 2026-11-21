package profiles

import "tamestack/internal/provisioning/models"

type ProfileInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Recommended bool   `json:"recommended"`
}

// ListProfiles returns metadata for all available built-in profiles.
func ListProfiles() []ProfileInfo {
	return []ProfileInfo{
		{
			ID:          models.ProfileMinimalPHP,
			Name:        "Minimal PHP Server",
			Description: "Легковесный стек: Nginx, PHP 8.3 FPM, базовые расширения и Composer.",
			Icon:        "Zap",
		},
		{
			ID:          models.ProfileFullPHP,
			Name:        "Full PHP Web Server",
			Description: "Полный веб-сервер: Nginx, PHP 8.2 & 8.3, MariaDB, Redis, SSL Certbot, UFW и Fail2ban.",
			Icon:        "Server",
			Recommended: true,
		},
		{
			ID:          models.ProfileApachePHP,
			Name:        "Apache PHP Server",
			Description: "Классический стек: Apache2 с PHP-FPM / mod_proxy_fcgi, расширения и Certbot SSL.",
			Icon:        "Layers",
		},
		{
			ID:          models.ProfileLaravel,
			Name:        "Laravel Server",
			Description: "Оптимизирован для Laravel: Nginx, PHP 8.3, Redis, MariaDB, Composer, Node.js, права www-data.",
			Icon:        "Flame",
		},
		{
			ID:          models.ProfileWordPress,
			Name:        "WordPress Server",
			Description: "Готов для WordPress: Nginx, PHP 8.2 FPM, расширения GD/Imagick/Intl, MariaDB, UFW и SSL.",
			Icon:        "Globe",
		},
		{
			ID:          models.ProfileDockerHost,
			Name:        "Docker Host",
			Description: "Сервер контейнеров: Docker Engine, Docker Compose Plugin, базовая безопасность.",
			Icon:        "Box",
		},
		{
			ID:          models.ProfileCustom,
			Name:        "Custom Setup",
			Description: "Индивидуальный выбор компонентов, версий PHP, СУБД и брандмауэра.",
			Icon:        "Sliders",
		},
	}
}

// GetBlueprintForProfile returns default blueprint settings for a profile.
func GetBlueprintForProfile(profileID string, defaultSSHPort int) models.Blueprint {
	if defaultSSHPort <= 0 {
		defaultSSHPort = 22
	}

	baseSec := models.SecuritySpec{
		Firewall:     "ufw",
		Fail2ban:     true,
		SSHHarden:    true,
		DisableRoot:  false,
		AllowSSHPort: defaultSSHPort,
	}

	switch profileID {
	case models.ProfileMinimalPHP:
		return models.Blueprint{
			Profile: models.ProfileMinimalPHP,
			Webserver: models.WebserverSpec{
				Type:        "nginx",
				EnableHTTP2: true,
			},
			PHP: models.PHPSpec{
				Versions: []models.VersionSpec{
					{
						Version:    "8.3",
						Handler:    "fpm",
						Extensions: []string{"curl", "mbstring", "mysql", "xml", "zip", "opcache"},
						IsDefault:  true,
					},
				},
			},
			Database: models.DatabaseSpec{Type: "none"},
			Cache:    models.CacheSpec{Type: "none"},
			Security: baseSec,
			SSL:      models.SSLSpec{Provider: "certbot"},
			Tools:    models.ToolsSpec{Composer: true, Git: true},
		}

	case models.ProfileFullPHP:
		return models.Blueprint{
			Profile: models.ProfileFullPHP,
			Webserver: models.WebserverSpec{
				Type:        "nginx",
				EnableHTTP2: true,
			},
			PHP: models.PHPSpec{
				Versions: []models.VersionSpec{
					{
						Version:    "8.3",
						Handler:    "fpm",
						Extensions: []string{"curl", "mbstring", "mysql", "xml", "zip", "gd", "intl", "redis", "opcache"},
						IsDefault:  true,
					},
					{
						Version:    "8.2",
						Handler:    "fpm",
						Extensions: []string{"curl", "mbstring", "mysql", "xml", "zip", "gd", "intl", "redis", "opcache"},
						IsDefault:  false,
					},
				},
			},
			Database:   models.DatabaseSpec{Type: "mariadb"},
			Cache:      models.CacheSpec{Type: "redis"},
			Security:   baseSec,
			SSL:        models.SSLSpec{Provider: "certbot"},
			Tools:      models.ToolsSpec{Composer: true, Git: true, NodeJS: true},
			Containers: models.ContainersSpec{Docker: false},
		}

	case models.ProfileApachePHP:
		return models.Blueprint{
			Profile: models.ProfileApachePHP,
			Webserver: models.WebserverSpec{
				Type:        "apache",
				EnableHTTP2: true,
			},
			PHP: models.PHPSpec{
				Versions: []models.VersionSpec{
					{
						Version:    "8.3",
						Handler:    "fpm",
						Extensions: []string{"curl", "mbstring", "mysql", "xml", "zip", "opcache"},
						IsDefault:  true,
					},
				},
			},
			Database: models.DatabaseSpec{Type: "mariadb"},
			Cache:    models.CacheSpec{Type: "none"},
			Security: baseSec,
			SSL:      models.SSLSpec{Provider: "certbot"},
			Tools:    models.ToolsSpec{Composer: true, Git: true},
		}

	case models.ProfileLaravel:
		return models.Blueprint{
			Profile: models.ProfileLaravel,
			Webserver: models.WebserverSpec{
				Type:        "nginx",
				EnableHTTP2: true,
			},
			PHP: models.PHPSpec{
				Versions: []models.VersionSpec{
					{
						Version:    "8.3",
						Handler:    "fpm",
						Extensions: []string{"curl", "mbstring", "mysql", "xml", "zip", "bcmath", "fileinfo", "redis", "opcache"},
						IsDefault:  true,
					},
				},
			},
			Database:   models.DatabaseSpec{Type: "mariadb"},
			Cache:      models.CacheSpec{Type: "redis"},
			Security:   baseSec,
			SSL:        models.SSLSpec{Provider: "certbot"},
			Tools:      models.ToolsSpec{Composer: true, Git: true, NodeJS: true},
			Containers: models.ContainersSpec{Docker: false},
		}

	case models.ProfileWordPress:
		return models.Blueprint{
			Profile: models.ProfileWordPress,
			Webserver: models.WebserverSpec{
				Type:        "nginx",
				EnableHTTP2: true,
			},
			PHP: models.PHPSpec{
				Versions: []models.VersionSpec{
					{
						Version:    "8.2",
						Handler:    "fpm",
						Extensions: []string{"curl", "mbstring", "mysql", "xml", "zip", "gd", "imagick", "intl", "fileinfo", "opcache"},
						IsDefault:  true,
					},
				},
			},
			Database: models.DatabaseSpec{Type: "mariadb"},
			Cache:    models.CacheSpec{Type: "none"},
			Security: baseSec,
			SSL:      models.SSLSpec{Provider: "certbot"},
			Tools:    models.ToolsSpec{Git: true},
		}

	case models.ProfileDockerHost:
		return models.Blueprint{
			Profile: models.ProfileDockerHost,
			Webserver: models.WebserverSpec{
				Type: "none",
			},
			PHP:        models.PHPSpec{Versions: []models.VersionSpec{}},
			Database:   models.DatabaseSpec{Type: "none"},
			Cache:      models.CacheSpec{Type: "none"},
			Security:   baseSec,
			SSL:        models.SSLSpec{Provider: "none"},
			Tools:      models.ToolsSpec{Git: true},
			Containers: models.ContainersSpec{Docker: true, Compose: true},
		}

	default: // Custom
		return models.Blueprint{
			Profile: models.ProfileCustom,
			Webserver: models.WebserverSpec{
				Type:        "nginx",
				EnableHTTP2: true,
			},
			PHP: models.PHPSpec{
				Versions: []models.VersionSpec{
					{
						Version:    "8.3",
						Handler:    "fpm",
						Extensions: []string{"curl", "mbstring", "mysql", "xml", "zip", "opcache"},
						IsDefault:  true,
					},
				},
			},
			Database: models.DatabaseSpec{Type: "mariadb"},
			Cache:    models.CacheSpec{Type: "none"},
			Security: baseSec,
			SSL:      models.SSLSpec{Provider: "certbot"},
			Tools:    models.ToolsSpec{Composer: true, Git: true},
		}
	}
}
