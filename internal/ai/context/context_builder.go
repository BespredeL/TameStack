package context

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"tamestack/internal/ai/domain"
	"tamestack/internal/db"
)

// UIContext captures where the user currently is in the TameStack UI and what they are viewing.
type UIContext struct {
	ActiveTab          string `json:"active_tab"`
	ActiveTabTitle     string `json:"active_tab_title"`
	SelectedServerID   string `json:"selected_server_id,omitempty"`
	SelectedServerName string `json:"selected_server_name,omitempty"`
	SelectedServerHost string `json:"selected_server_host,omitempty"`
	ScreenSummary      string `json:"screen_summary,omitempty"`
}

// BuilderOptions defines options for constructing the system prompt.
type BuilderOptions struct {
	ServerID       *string
	IncludeMemory  bool
	CustomPrompt   string
	LanguageLocale string // "ru" or "en"
	UIContext      *UIContext
	AvailableTools []domain.ToolDefinition
}

// ContextBuilder builds a complete, sanitized system prompt for LLM conversations.
type ContextBuilder struct {
	database  *db.DB
	collector *ServerCollector
	sanitizer *Sanitizer
}

// NewContextBuilder creates a new ContextBuilder.
func NewContextBuilder(database *db.DB, sanitizer *Sanitizer) *ContextBuilder {
	if sanitizer == nil {
		sanitizer = NewSanitizer()
	}
	collector := NewServerCollector(database, sanitizer)
	return &ContextBuilder{
		database:  database,
		collector: collector,
		sanitizer: sanitizer,
	}
}

// DefaultSystemPromptTextRU defines the Russian baseline persona and rules.
const DefaultSystemPromptTextRU = `Ты — TameStack AI Assistant, Senior DevOps инженер и системный архитектор.
Твоя задача — проактивно помогать администратору в диагностике, поиске причин сбоев и обслуживании серверов (Nginx, Apache, PHP-FPM, MySQL/PostgreSQL, SSL, Docker, почта, DNS, безопасность).

### КРИТИЧЕСКИ ВАЖНЫЕ ПРАВИЛА АВТОНОМНОЙ ДИАГНОСТИКИ:
1. НИКОГДА НЕ ПРОСИ ПОЛЬЗОВАТЕЛЯ вручную выполнять команды в консоли (например: "sudo tail -n 50 /var/log/apache2/error.log", "sudo systemctl status ...", "cat /etc/..."), если ты можешь вызвать доступный инструмент чтения сам!
2. Актуальный список доступных тебе инструментов передаётся ниже в разделе «ДОСТУПНЫЕ ИНСТРУМЕНТЫ» и через схему вызова функций (function calling).
3. Если пользователь жалуется на ошибку (502 Bad Gateway, 500 Internal Server Error, сайт не открывается, служба не стартует) — ТЫ ОБЯЗАН СНАЧАЛА САМ вызвать соответствующие read-only инструменты:
   - Проверить логи ошибок веб-сервера через read_webserver_logs.
   - Проверить статус веб-сервера, PHP-FPM или приложения через get_service_status, get_service_logs или get_app_process_status.
   - Проверить стек и режим PHP через check_web_topology (или inspect_site_app для конкретного сайта).
4. Только после получения реальных данных из инструментов чтения давай пользователю исчерпывающий технический анализ первопричины и конкретный пошаговый рецепт исправления.
5. Отвечай кратко, профессионально и по существу на том языке, на котором обращается пользователь (по умолчанию: русский).`

// DefaultSystemPromptTextEN defines the English baseline persona and rules.
const DefaultSystemPromptTextEN = `You are TameStack AI Assistant, a Senior DevOps engineer and system architect.
Your mission is to proactively assist the administrator in diagnosing, troubleshooting, and maintaining servers (Nginx, Apache, PHP-FPM, MySQL/PostgreSQL, SSL, Docker, mail, DNS, security).

### CRITICAL AUTONOMOUS DIAGNOSIS RULES:
1. NEVER ASK THE USER to run manual console commands (e.g. "sudo tail -n 50 /var/log/apache2/error.log", "sudo systemctl status ...", "cat /etc/...") if you can invoke an available inspection tool yourself!
2. The up-to-date list of tools enabled for this session is provided below under "AVAILABLE TOOLS" and in the function calling schema.
3. If the user reports an issue (502 Bad Gateway, 500 Internal Server Error, site unreachable, service failed to start) — YOU MUST FIRST call the appropriate read-only tools yourself:
   - Check web server error logs via read_webserver_logs.
   - Check web server, PHP-FPM, or app process status via get_service_status, get_service_logs, or get_app_process_status.
   - Check stack and PHP mode via check_web_topology (or inspect_site_app for a specific site).
4. Only after obtaining real data from inspection tools, provide the user with a thorough technical root-cause analysis and a concrete step-by-step resolution.
5. Keep responses concise, professional, and directly to the point in the language used by the user (default: English).`

// DefaultSystemPromptText defines the standard baseline persona and rules (Russian fallback).
const DefaultSystemPromptText = DefaultSystemPromptTextRU

// GetDefaultSystemPrompt returns the default system prompt for the specified language.
func GetDefaultSystemPrompt(lang string) string {
	if strings.ToLower(strings.TrimSpace(lang)) == "en" {
		return DefaultSystemPromptTextEN
	}
	return DefaultSystemPromptTextRU
}

// BuildSystemPrompt generates the system prompt including role, security rules, server state and memory.
func (b *ContextBuilder) BuildSystemPrompt(ctx context.Context, opts BuilderOptions) (string, error) {
	var sb strings.Builder

	lang := strings.ToLower(strings.TrimSpace(opts.LanguageLocale))
	if lang == "" && b.database != nil {
		savedLang, _ := b.database.GetAIGlobalSetting(ctx, "app_language")
		lang = strings.ToLower(strings.TrimSpace(savedLang))
	}
	if lang != "en" {
		lang = "ru"
	}

	basePrompt := GetDefaultSystemPrompt(lang)
	if b.database != nil {
		customPrompt, _ := b.database.GetAIGlobalSetting(ctx, "system_prompt_"+lang)
		if strings.TrimSpace(customPrompt) == "" {
			legacyPrompt, _ := b.database.GetAIGlobalSetting(ctx, "system_prompt")
			trimmedLegacy := strings.TrimSpace(legacyPrompt)
			if trimmedLegacy != "" &&
				trimmedLegacy != strings.TrimSpace(DefaultSystemPromptTextRU) &&
				trimmedLegacy != strings.TrimSpace(DefaultSystemPromptTextEN) {
				customPrompt = legacyPrompt
			}
		}
		if strings.TrimSpace(customPrompt) != "" {
			basePrompt = strings.TrimSpace(customPrompt)
		}
	}

	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")

	if len(opts.AvailableTools) > 0 {
		sortedTools := make([]domain.ToolDefinition, len(opts.AvailableTools))
		copy(sortedTools, opts.AvailableTools)
		sort.Slice(sortedTools, func(i, j int) bool {
			return sortedTools[i].Name < sortedTools[j].Name
		})

		if lang == "en" {
			sb.WriteString("### AVAILABLE TOOLS:\n")
			for _, td := range sortedTools {
				mode := "read-only"
				if td.IsMutating || td.ApprovalViaTask {
					mode = "requires approval"
				}
				sb.WriteString(fmt.Sprintf("- `%s` [%s]: %s\n", td.Name, mode, td.Description))
			}
		} else {
			sb.WriteString("### ДОСТУПНЫЕ ИНСТРУМЕНТЫ:\n")
			for _, td := range sortedTools {
				mode := "read-only"
				if td.IsMutating || td.ApprovalViaTask {
					mode = "требует подтверждения"
				}
				sb.WriteString(fmt.Sprintf("- `%s` [%s]: %s\n", td.Name, mode, td.Description))
			}
		}
		sb.WriteString("\n")
	}

	if lang == "en" {
		sb.WriteString(tameStackProductGuideEN)
	} else {
		sb.WriteString(tameStackProductGuideRU)
	}
	sb.WriteString("\n\n")

	if opts.UIContext != nil && (opts.UIContext.ActiveTab != "" || opts.UIContext.ScreenSummary != "") {
		if lang == "en" {
			sb.WriteString("### Current User Screen Context:\n")
			if opts.UIContext.ActiveTab != "" {
				sb.WriteString(fmt.Sprintf("- Active section: `%s` (%s)\n", opts.UIContext.ActiveTab, opts.UIContext.ActiveTabTitle))
			}
			if opts.UIContext.SelectedServerID != "" {
				sb.WriteString(fmt.Sprintf("- Selected server in UI: %s (%s), ID: `%s`\n", opts.UIContext.SelectedServerName, opts.UIContext.SelectedServerHost, opts.UIContext.SelectedServerID))
			}
			if opts.UIContext.ScreenSummary != "" {
				sb.WriteString(fmt.Sprintf("- Screen details: %s\n", opts.UIContext.ScreenSummary))
			}
			if guide, ok := tameStackTabGuideEN[opts.UIContext.ActiveTab]; ok {
				sb.WriteString(fmt.Sprintf("- Section capabilities & UI controls: %s\n", guide))
			}
			sb.WriteString("Use this context to answer questions about what the user sees, how to use this section in TameStack, or to inspect the selected server.\n\n")
		} else {
			sb.WriteString("### Текущий контекст экрана пользователя:\n")
			if opts.UIContext.ActiveTab != "" {
				sb.WriteString(fmt.Sprintf("- Открытый раздел: `%s` (%s)\n", opts.UIContext.ActiveTab, opts.UIContext.ActiveTabTitle))
			}
			if opts.UIContext.SelectedServerID != "" {
				sb.WriteString(fmt.Sprintf("- Выбранный сервер в интерфейсе: %s (%s), ID: `%s`\n", opts.UIContext.SelectedServerName, opts.UIContext.SelectedServerHost, opts.UIContext.SelectedServerID))
			}
			if opts.UIContext.ScreenSummary != "" {
				sb.WriteString(fmt.Sprintf("- Детали экрана: %s\n", opts.UIContext.ScreenSummary))
			}
			if guide, ok := tameStackTabGuideRU[opts.UIContext.ActiveTab]; ok {
				sb.WriteString(fmt.Sprintf("- Возможности и элементы этого раздела: %s\n", guide))
			}
			sb.WriteString("Используй этот контекст, чтобы понимать, где находится пользователь, подсказывать шаги по интерфейсу TameStack или диагностировать выбранный сервер.\n\n")
		}
	}

	// 2. Additional Session Instructions (if set specifically for this conversation).
	if opts.CustomPrompt != "" &&
		strings.TrimSpace(opts.CustomPrompt) != strings.TrimSpace(basePrompt) &&
		strings.TrimSpace(opts.CustomPrompt) != strings.TrimSpace(DefaultSystemPromptTextRU) &&
		strings.TrimSpace(opts.CustomPrompt) != strings.TrimSpace(DefaultSystemPromptTextEN) {
		if lang == "en" {
			sb.WriteString("### Additional Session Instructions:\n")
		} else {
			sb.WriteString("### Дополнительные инструкции сессии:\n")
		}
		sb.WriteString(opts.CustomPrompt)
		sb.WriteString("\n\n")
	}

	serverIDToUse := opts.ServerID
	if (serverIDToUse == nil || *serverIDToUse == "") && opts.UIContext != nil && opts.UIContext.SelectedServerID != "" {
		sID := opts.UIContext.SelectedServerID
		serverIDToUse = &sID
	}
	if (serverIDToUse == nil || *serverIDToUse == "") && b.database != nil {
		servers, _ := b.database.ListServers(ctx)
		if len(servers) == 1 {
			sID := servers[0].ID
			serverIDToUse = &sID
		} else if len(servers) > 1 {
			if lang == "en" {
				sb.WriteString("### Available Servers in TameStack:\n")
				for _, s := range servers {
					sb.WriteString(fmt.Sprintf("- Server ID: `%s`, Name: %s, Host: %s, Status: %s\n", s.ID, s.Name, s.Host, s.Status))
				}
				sb.WriteString("\nWhen calling tools for a specific server, pass its ID in the `server_id` argument.\n\n")
			} else {
				sb.WriteString("### Доступные серверы в TameStack:\n")
				for _, s := range servers {
					sb.WriteString(fmt.Sprintf("- Сервер ID: `%s`, Имя: %s, Хост: %s, Статус: %s\n", s.ID, s.Name, s.Host, s.Status))
				}
				sb.WriteString("\nПри вызове инструментов для конкретного сервера передавай его ID в аргумент `server_id`.\n\n")
			}
		}
	}

	if serverIDToUse != nil && *serverIDToUse != "" {
		if lang == "en" {
			sb.WriteString(fmt.Sprintf("Active target server for tool execution: `server_id` = `%s`.\n\n", *serverIDToUse))
		} else {
			sb.WriteString(fmt.Sprintf("Активный целевой сервер для вызова инструментов: `server_id` = `%s`.\n\n", *serverIDToUse))
		}
		snapshot, err := b.collector.CollectServerSnapshot(ctx, *serverIDToUse)
		if err == nil && snapshot != nil {
			if lang == "en" {
				sb.WriteString("### Infrastructure Context:\n")
			} else {
				sb.WriteString("### Контекст инфраструктуры:\n")
			}
			sb.WriteString(b.collector.FormatContextMarkdownLang(snapshot, lang))
			sb.WriteString("\n\n")
		}
	}

	if opts.IncludeMemory {
		facts, err := b.database.ListAIMemoryFacts(ctx, serverIDToUse)
		if err == nil && len(facts) > 0 {
			if lang == "en" {
				sb.WriteString("### Infrastructure Long-term Memory:\n")
			} else {
				sb.WriteString("### Долговременная память об инфраструктуре:\n")
			}
			for _, f := range facts {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", f.Category, f.FactKey, f.FactValue))
			}
			sb.WriteString("\n")
		}
	}

	return b.sanitizer.Sanitize(sb.String()), nil
}

const tameStackProductGuideRU = `### СПРАВОЧНИК ПО ВОЗМОЖНОСТЯМ TAMESTACK (ПОМОГАЙ ПОЛЬЗОВАТЕЛЮ ОРИЕНТИРОВАТЬСЯ В ПРОГРАММЕ):
TameStack — десктопная панель управления серверами по SSH без установки агента или веб-панели на сервер. Разделы интерфейса:
- Обзор: «Дашборд» (сводка), «Серверы» (добавление SSH-узлов, тест, диагностика, режим audit/management), «Мониторинг» (live-метрики CPU/RAM/диска/сети, проверка URL), «Задачи» (очередь и логи фоновых операций).
- Веб и приложения: «Сайты» (Nginx/Apache vhost, Laravel/WordPress, Reverse Proxy + менеджер systemd-процессов для Node.js/Python/Go во вкладке приложения, переключение PHP), «Веб-серверы» (установка/конфиги Nginx/Apache), «PHP Менеджер» (версии PHP, расширения, php.ini, FPM-пулы), «SSL Сертификаты» (Certbot Let's Encrypt), «Docker» (контейнеры, образы, Compose).
- Данные и сеть: «Базы данных» (MySQL/MariaDB/PostgreSQL, БД, пользователи, дампы), «Почтовый сервер» (Postfix + Dovecot + OpenDKIM, домены, DKIM/SPF/DMARC, ящики, алиасы), «DNS (BIND9)» (мастер-зоны и записи A/AAAA/CNAME/MX/TXT/NS/SRV/CAA).
- Система и файлы: «Файловый менеджер» (SFTP-проводник, редактор, chmod, анализ и очистка диска), «Cron Задачи», «Бэкапы» (сайты/БД/конфиги, инспекция и восстановление), «Службы и процессы» (systemd, порты, проверка DNS/TCP), «Пакеты APT» (симуляция и установка/обновление), «Мастер настройки» (первичный provisioning нового VPS).
- Безопасность: «Безопасность» (UFW/firewalld, Fail2ban, hardening, сканирование файлов), «Пользователи и SFTP» (создание пользователей сайтов, SFTP chroot-изоляция, SSH-ключи, пароли, chown).
- Инструменты: «SSH Терминал» (интерактивный терминал + AI-панель), «AI Ассистент» (полноэкранный чат, маршрутизация моделей, память, воркфлоу), «Настройки» (шифрование Vault/БД, логи, автообновление).
Горячие клавиши: Ctrl+K — быстрый переход, Ctrl+I — вызов AI-помощника в любом месте.`

const tameStackProductGuideEN = `### TAMESTACK PRODUCT GUIDE (HELP THE USER NAVIGATE THE APP):
TameStack is a desktop SSH server control panel with zero server-side web panel/agent. UI sections:
- Overview: Dashboard, Servers (SSH nodes, diagnostics, audit/management mode), Monitoring (live CPU/RAM/disk/network, URL checks), Tasks (background job queue & logs).
- Web & Apps: Sites (Nginx/Apache vhosts, Laravel/WordPress, Reverse Proxy + Node.js/Python/Go systemd process manager in the App tab, PHP switching), Web Servers, PHP Manager (versions, extensions, php.ini, FPM pools), SSL Certificates (Certbot), Docker (containers, images, Compose).
- Data & Network: Databases (MySQL/MariaDB/PostgreSQL, users, dumps), Mail Server (Postfix + Dovecot + OpenDKIM, domains, DKIM/SPF/DMARC, mailboxes, aliases), DNS (BIND9) (master zones and A/AAAA/CNAME/MX/TXT/NS/SRV/CAA records).
- System & Files: File Manager (SFTP, editor, chmod, disk cleanup), Cron Jobs, Backups, Services & Processes (systemd, ports, DNS/TCP checks), APT Packages, Server Setup Wizard (initial VPS provisioning).
- Security & Access: Security (UFW/firewalld, Fail2ban, hardening, file scanner), Users & SFTP (site users, SFTP chroot isolation, SSH keys, passwords, chown).
- Tools: SSH Terminal, AI Assistant, Settings (Vault/DB encryption, logs, updates).
Shortcuts: Ctrl+K for Command Palette, Ctrl+I to toggle the global AI Copilot anywhere.`

var tameStackTabGuideRU = map[string]string{
	"dashboard":    "Сводка по подключённым серверам, ресурсам и активным задачам.",
	"servers":      "Добавление и редактирование SSH-серверов, проверка соединения, диагностика ОС и переключение режима audit/management.",
	"monitoring":   "Мониторинг CPU, RAM, диска, аптайма и проверка доступности сайтов/эндпоинтов в реальном времени.",
	"tasks":        "Очередь фоновых задач, статус выполнения, прогресс и логи SSH-операций.",
	"sites":        "Создание сайтов (PHP/Laravel/WordPress/Static/Reverse Proxy), привязка доменов и алиасов, редактирование vhost Nginx/Apache с откатом, переключение версий PHP, а во вкладке «Приложение» — Artisan, .env, Queue/Scheduler и менеджер systemd-процессов для Node.js/Python/Go.",
	"webservers":   "Установка и настройка Nginx/Apache, проверка синтаксиса конфигов, перезапуск и просмотр логов веб-сервера.",
	"mail":         "Установка стека Postfix + Dovecot + OpenDKIM, добавление почтовых доменов, автоматическая генерация DKIM-ключей и готовых DNS-записей (DKIM/SPF/DMARC), создание и удаление почтовых ящиков, настройка переадресаций (алиасов).",
	"dns":          "Установка BIND9, создание и удаление мастер-зон, редактирование ресурсных записей (A, AAAA, CNAME, MX, TXT, NS, SRV, CAA) с проверкой named-checkzone и автоматическим откатом при ошибке.",
	"users":        "Список системных пользователей, создание изолированных пользователей сайтов, включение/отключение SFTP chroot (Match User + ForceCommand internal-sftp), управление SSH authorized_keys, смена пароля (chpasswd) и владельца каталогов (chown).",
	"security":     "Управление фаерволом (UFW/firewalld), Fail2ban (jails, бан/разбан IP), аудит безопасности, hardening и сканер файлов.",
	"databases":    "Инспекция MySQL/MariaDB/PostgreSQL, создание и удаление баз данных и пользователей, выдача прав, дампы и восстановление.",
	"php":          "Установка версий PHP и расширений, редактирование php.ini с бэкапами, управление пулами PHP-FPM, перезапуск и логи.",
	"docker":       "Управление контейнерами, образами, томами, сетями, проектами Docker Compose и просмотр логов контейнеров.",
	"ssl":          "Установка Certbot, выпуск и продление SSL-сертификатов Let's Encrypt, мониторинг сроков действия.",
	"filemanager":  "SFTP-проводник, встроенный редактор файлов, изменение прав (chmod), анализ занятого места и очистка диска.",
	"cron":         "Просмотр, создание, включение/выключение и удаление cron-задач пользователей и системы.",
	"services":     "Управление systemd-службами, просмотр процессов, открытых портов и сетевая диагностика (DNS/TCP).",
	"packages":     "Поиск, симуляция обновления (apt/dnf) и установка/обновление системных пакетов.",
	"backups":      "Создание резервных копий сайтов, БД, конфигов и полного бэкапа, инспекция архивов, скачивание и восстановление.",
	"provisioning": "Мастер первичной настройки чистого VPS: выбор эталонного профиля, расчёт плана установки и развёртывание стека.",
	"terminal":     "Интерактивный SSH-терминал со встроенной AI-панелью помощи по командам и ошибкам.",
	"ai":           "Полноэкранный AI-чат с историей, маршрутизацией моделей, инструментами, автономными агентами и настройками провайдеров.",
	"settings":     "Настройки приложения, шифрования Vault/БД, языка интерфейса, логов и проверки обновлений.",
}

var tameStackTabGuideEN = map[string]string{
	"dashboard":    "Overview of connected servers, resource usage, and active tasks.",
	"servers":      "Add/edit SSH servers, test connectivity, run OS diagnostics, and switch audit/management mode.",
	"monitoring":   "Real-time CPU, RAM, disk, uptime metrics, and website/endpoint health checks.",
	"tasks":        "Background job queue, execution progress, and detailed SSH task logs.",
	"sites":        "Create sites (PHP/Laravel/WordPress/Static/Reverse Proxy), edit Nginx/Apache vhosts with rollback, switch PHP versions, and in the App tab manage Laravel Artisan/.env/Queues/Scheduler or Node.js/Python/Go systemd processes.",
	"webservers":   "Install and configure Nginx/Apache, validate config syntax, reload services, and inspect logs.",
	"mail":         "Install Postfix + Dovecot + OpenDKIM, add mail domains, generate DKIM keys and copy-ready DKIM/SPF/DMARC records, create/delete mailboxes, and configure forwarding aliases.",
	"dns":          "Install BIND9, create/delete master zones, edit A/AAAA/CNAME/MX/TXT/NS/SRV/CAA records with named-checkzone validation and rollback.",
	"users":        "List OS users, create isolated site users, toggle SFTP chroot (Match User + internal-sftp), manage SSH authorized_keys, set passwords, and change directory ownership.",
	"security":     "Manage UFW/firewalld, Fail2ban jails and bans, security audit, hardening, and file scanner.",
	"databases":    "Inspect MySQL/MariaDB/PostgreSQL, create/drop databases and users, dump and restore.",
	"php":          "Install PHP versions and extensions, edit php.ini with backups, manage PHP-FPM pools and logs.",
	"docker":       "Manage Docker containers, images, volumes, networks, Compose projects, and container logs.",
	"ssl":          "Install Certbot, issue and renew Let's Encrypt certificates, monitor expiry.",
	"filemanager":  "SFTP file explorer, file editor, chmod, disk usage analyzer, and cleanup.",
	"cron":         "Inspect, create, toggle, and delete user and system cron jobs.",
	"services":     "Manage systemd units, inspect running processes, listening ports, and network DNS/TCP checks.",
	"packages":     "Search, simulate upgrades (apt/dnf), and install or update OS packages.",
	"backups":      "Create site/DB/config/full backups, inspect archives, download, and restore.",
	"provisioning": "Initial VPS setup wizard: choose stack blueprint, preview pre-flight plan, and provision.",
	"terminal":     "Interactive SSH terminal with an embedded AI assistant panel for commands and errors.",
	"ai":           "Full-screen AI chat with conversation history, model routing, tool manager, autonomous agents, and provider settings.",
	"settings":     "Application settings, Vault/DB encryption, UI language, logs, and update checks.",
}
