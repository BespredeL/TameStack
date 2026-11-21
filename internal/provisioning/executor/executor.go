package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/osadapter"
	"tamestack/internal/provisioning/inventory"
	"tamestack/internal/provisioning/models"
	"tamestack/internal/provisioning/planner"
	"tamestack/internal/provisioning/validators"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// Executor coordinates the execution of provisioning plans.
type Executor struct {
	cfg      *config.AppConfig
	database *db.DB
	vault    *crypto.Vault
	pool     *ssh.Pool
}

// NewExecutor creates a new provisioning executor.
func NewExecutor(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool) *Executor {
	return &Executor{
		cfg:      cfg,
		database: database,
		vault:    vault,
		pool:     pool,
	}
}

// ProvisioningPayload contains data for server_provisioning task.
type ProvisioningPayload struct {
	ServerID    string           `json:"server_id"`
	BlueprintID string           `json:"blueprint_id,omitempty"`
	Blueprint   models.Blueprint `json:"blueprint"`
}

// HandleProvisioningTask runs the full idempotent provisioning workflow.
func (e *Executor) HandleProvisioningTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload ProvisioningPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("invalid provisioning payload: %w", err)
	}

	client, serverRecord, err := module.GetSSHClient(ctx, e.cfg, e.database, e.vault, e.pool, payload.ServerID)
	if err != nil {
		return "", fmt.Errorf("failed to connect via SSH to %s: %w", payload.ServerID, err)
	}

	startTime := time.Now()
	runRec := &db.ProvisioningRunRecord{
		ID:          task.ID,
		ServerID:    payload.ServerID,
		BlueprintID: payload.BlueprintID,
		Profile:     payload.Blueprint.Profile,
		Status:      "running",
		CreatedAt:   startTime,
	}
	_ = e.database.SaveProvisioningRun(ctx, runRec)

	// Step 1: Pre-flight & Discovery (10%)
	logger.SetStep("Сбор фактов и диагностика сервера", 10)
	logger.Info(fmt.Sprintf("Запуск первоначальной настройки сервера %s (%s)...", serverRecord.Name, serverRecord.Host))
	inv, err := inventory.CollectServerInventory(ctx, client, serverRecord.Port)
	if err != nil {
		return "", fmt.Errorf("ошибка сбора информации о сервере: %w", err)
	}
	logger.Info(fmt.Sprintf("Обнаружена ОС: %s %s (%s, %s), CPU: %d, RAM: %d МБ, Диск: %d ГБ свободно",
		inv.Distro, inv.Version, inv.Family, inv.Arch, inv.CPUCores, inv.RAMTotalMB, inv.DiskFreeGB))

	// Step 2: Planning & Validation (15%)
	logger.SetStep("Формирование и проверка плана установки", 15)
	plan, err := planner.GeneratePlan(payload.ServerID, inv, payload.Blueprint)
	if err != nil {
		return "", fmt.Errorf("ошибка формирования плана: %w", err)
	}
	planJSON, _ := json.Marshal(plan)
	runRec.PlanJSON = string(planJSON)
	_ = e.database.SaveProvisioningRun(ctx, runRec)

	if len(plan.Warnings) > 0 {
		for _, w := range plan.Warnings {
			logger.Warn(w)
		}
	}
	if len(plan.PotentialConflicts) > 0 {
		for _, c := range plan.PotentialConflicts {
			logger.Warn(fmt.Sprintf("Предупреждение о конфликте: %s", c))
		}
	}

	// Step 3: Repositories Configuration (25%)
	logger.SetStep("Настройка пакетов и репозиториев", 25)
	if inv.Family == "debian" {
		logger.Info("Обновление индексов APT и установка базовых утилит...")
		aptSetup := "export DEBIAN_FRONTEND=noninteractive && apt-get update -y && apt-get install -y --no-install-recommends software-properties-common apt-transport-https ca-certificates curl gnupg lsb-release"
		out, stderr, code, err := client.NewCommand(aptSetup).WithSudo().WithLock("pkg_mgr", 180).Exec(ctx)
		if err != nil || code != 0 {
			logger.Warn(fmt.Sprintf("apt-get update warning: %s %s", string(out), string(stderr)))
		}

		// Add PHP PPA/Sury if PHP requested
		if len(payload.Blueprint.PHP.Versions) > 0 {
			repoCheck := "grep -E 'ondrej/php|sury' /etc/apt/sources.list /etc/apt/sources.list.d/* 2>/dev/null || true"
			outRepo, _, _, _ := client.NewCommand(repoCheck).WithSudo().Exec(ctx)
			if len(strings.TrimSpace(string(outRepo))) == 0 {
				logger.Info("Подключение репозитория PHP (Ondrej PPA / packages.sury.org)...")
				addRepoCmd := "add-apt-repository -y ppa:ondrej/php 2>&1 || (curl -sSLo /etc/apt/trusted.gpg.d/php.gpg https://packages.sury.org/php/apt.gpg && echo 'deb https://packages.sury.org/php/ '$(lsb_release -sc)' main' > /etc/apt/sources.list.d/php.list)"
				client.NewCommand(addRepoCmd).WithSudo().WithLock("pkg_mgr", 180).WithEnv("DEBIAN_FRONTEND", "noninteractive").Exec(ctx)
				client.NewCommand("apt-get update -y").WithSudo().WithLock("pkg_mgr", 180).WithEnv("DEBIAN_FRONTEND", "noninteractive").Exec(ctx)
			}
		}
	} else if inv.Family == "rhel" {
		logger.Info("Подключение репозиториев EPEL и Remi...")
		rhelRepoCmd := "dnf install -y epel-release https://rpms.remirepo.net/enterprise/remi-release-$(rpm -E %rhel).rpm || true"
		client.NewCommand(rhelRepoCmd).WithSudo().WithLock("pkg_mgr", 180).Exec(ctx)
		client.NewCommand("dnf makecache -y || true").WithSudo().WithLock("pkg_mgr", 180).Exec(ctx)
	}

	// Step 4: Package Installation (40%)
	if len(plan.PackagesToInstall) > 0 {
		logger.SetStep(fmt.Sprintf("Установка пакетов (%d шт.)", len(plan.PackagesToInstall)), 40)
		logger.Info(fmt.Sprintf("Устанавливаемые пакеты: %s", strings.Join(plan.PackagesToInstall, " ")))

		var installCmd string
		if inv.Family == "debian" {
			installCmd = fmt.Sprintf("apt-get install -y --no-install-recommends %s", strings.Join(plan.PackagesToInstall, " "))
		} else {
			installCmd = fmt.Sprintf("dnf install -y %s", strings.Join(plan.PackagesToInstall, " "))
		}

		outInst, stderrInst, codeInst, errInst := client.NewCommand(installCmd).
			WithSudo().
			WithLock("pkg_mgr", 300).
			WithEnv("DEBIAN_FRONTEND", "noninteractive").
			Exec(ctx)
		logger.Info(string(outInst))
		if errInst != nil || codeInst != 0 {
			runRec.Status = "failed"
			runRec.ResultJSON = fmt.Sprintf(`{"error": "Package installation failed with exit code %d: %s"}`, codeInst, string(stderrInst))
			_ = e.database.SaveProvisioningRun(ctx, runRec)
			return "", fmt.Errorf("ошибка установки системных пакетов (exit %d): %s", codeInst, string(stderrInst))
		}
	} else {
		logger.Info("Все необходимые пакеты уже установлены на сервере (skip).")
	}

	// Step 5: System Users and Directories (60%)
	logger.SetStep("Создание каталогов и настройка пользователей", 60)
	userDirScript := `
if ! id -u www-data >/dev/null 2>&1; then
    groupadd -f www-data 2>/dev/null || true
    useradd -r -g www-data -s /usr/sbin/nologin -d /var/www www-data 2>/dev/null || true
fi
mkdir -p /var/www /var/log/tamestack
chown www-data:www-data /var/www 2>/dev/null || true
chmod 0755 /var/www 2>/dev/null || true
`
	client.NewCommand(userDirScript).WithSudo().Exec(ctx)
	logger.Info("Пользователь www-data и корневой каталог /var/www подготовлены без изменения прав существующих подкаталогов.")

	// Step 6: Security and Firewall (CRITICAL: SSH lockout prevention!) (75%)
	logger.SetStep("Настройка брандмауэра и безопасности", 75)
	sshPort := plan.SSHPortPreserved
	if sshPort <= 0 {
		sshPort = 22
	}

	fwType := strings.ToLower(payload.Blueprint.Security.Firewall)
	if fwType == "ufw" {
		logger.Info(fmt.Sprintf("Настройка UFW: разрешение SSH-порта %d и всех активных служб...", sshPort))
		var allowLines []string
		for _, p := range plan.PortsToOpen {
			if p > 0 && p < 65536 {
				allowLines = append(allowLines, fmt.Sprintf("ufw allow %d/tcp 2>/dev/null || true", p))
			}
		}
		ufwCmd := fmt.Sprintf(`
ufw default deny incoming
ufw default allow outgoing
ufw allow %d/tcp
%s
echo "y" | ufw enable
ufw status
`, sshPort, strings.Join(allowLines, "\n"))
		outUfw, _, _, _ := client.NewCommand(ufwCmd).WithSudo().Exec(ctx)
		logger.Info(string(outUfw))
	} else if fwType == "firewalld" {
		logger.Info(fmt.Sprintf("Настройка Firewalld: разрешение SSH-порта %d и всех активных служб...", sshPort))
		var fwLines []string
		for _, p := range plan.PortsToOpen {
			if p > 0 && p < 65536 {
				fwLines = append(fwLines, fmt.Sprintf("firewall-cmd --permanent --add-port=%d/tcp 2>/dev/null || true", p))
			}
		}
		fwCmd := fmt.Sprintf(`
systemctl enable --now firewalld
firewall-cmd --permanent --add-port=%d/tcp
%s
firewall-cmd --permanent --add-service=http
firewall-cmd --permanent --add-service=https
firewall-cmd --reload
`, sshPort, strings.Join(fwLines, "\n"))
		client.NewCommand(fwCmd).WithSudo().Exec(ctx)
	}

	if payload.Blueprint.Security.Fail2ban {
		logger.Info("Активация службы Fail2ban...")
		client.NewCommand("systemctl enable --now fail2ban 2>/dev/null || true").WithSudo().Exec(ctx)
	}

	// Step 7: Services Enable & Start (85%)
	logger.SetStep("Запуск и включение системных служб", 85)
	for _, srv := range plan.ServicesToStart {
		logger.Info(fmt.Sprintf("Проверка службы %s...", srv))
		startCmd := fmt.Sprintf(`
if systemctl is-active --quiet %s; then
    echo "active"
else
    systemctl unmask %s 2>/dev/null || true
    systemctl enable --now %s 2>/dev/null || true
    systemctl is-active %s
fi`, srv, srv, srv, srv)
		outSrv, _, codeSrv, _ := client.NewCommand(startCmd).WithSudo().Exec(ctx)
		if codeSrv != 0 || !strings.Contains(string(outSrv), "active") {
			logger.Warn(fmt.Sprintf("Служба %s не активна, запуск...", srv))
			client.NewCommand(fmt.Sprintf("systemctl restart %s 2>/dev/null || true", srv)).WithSudo().Exec(ctx)
		} else {
			logger.Info(fmt.Sprintf("Служба %s активна.", srv))
		}
	}

	// Step 8: Post-Provisioning Verification (95%)
	logger.SetStep("Проверка работоспособности окружения", 95)
	checks := validators.VerifyEnvironment(ctx, client, payload.Blueprint)
	allPassed := true
	for _, c := range checks {
		statusStr := "[OK]"
		if !c.Passed {
			statusStr = "[ВНИМАНИЕ]"
			allPassed = false
		}
		logger.Info(fmt.Sprintf("%s %s: %s", statusStr, c.Component, c.Message))
	}

	finishTime := time.Now()
	result := models.ProvisioningResult{
		Success:      allPassed,
		Profile:      payload.Blueprint.Profile,
		TotalSteps:   len(plan.Items),
		AppliedSteps: len(plan.PackagesToInstall) + len(plan.ServicesToStart),
		SkippedSteps: len(plan.Items) - len(plan.PackagesToInstall),
		DurationSec:  int(finishTime.Sub(startTime).Seconds()),
		Checks:       checks,
		FinishedAt:   finishTime,
	}

	resJSON, _ := json.Marshal(result)
	runRec.Status = "completed"
	runRec.ResultJSON = string(resJSON)
	runRec.CompletedAt = &finishTime
	_ = e.database.SaveProvisioningRun(ctx, runRec)

	// Update cached server diagnostics
	adapter, osInfo, _ := osadapter.DetectAndCreateAdapter(ctx, client)
	if adapter != nil && osInfo != nil {
		diag := &db.DiagnosticsRecord{
			ServerID:    serverRecord.ID,
			OSName:      osInfo.PrettyName,
			OSVersion:   osInfo.Version,
			OSFamily:    osInfo.Family,
			DiagnosedAt: finishTime,
		}
		_ = e.database.SaveDiagnostics(ctx, diag)
	}

	logger.SetStep("Первоначальная настройка успешно завершена", 100)
	msg := fmt.Sprintf("Сервер %s (%s) успешно настроен по профилю '%s' за %d сек.",
		serverRecord.Name, serverRecord.Host, payload.Blueprint.Profile, result.DurationSec)
	logger.Info(msg)
	return msg, nil
}
