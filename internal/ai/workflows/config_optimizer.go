package workflows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/modules/security"
	"tamestack/internal/ssh"
)

// ConfigOptimizerWorkflow calculates resource-based tuning recommendations.
type ConfigOptimizerWorkflow struct {
	workflowBase
}

// NewConfigOptimizerWorkflow creates a new configuration optimizer.
func NewConfigOptimizerWorkflow(
	database *db.DB,
	secService *security.Service,
	vault *crypto.Vault,
	pool *ssh.Pool,
	knownHosts string,
) *ConfigOptimizerWorkflow {
	return &ConfigOptimizerWorkflow{workflowBase{
		database:   database,
		secService: secService,
		vault:      vault,
		pool:       pool,
		knownHosts: knownHosts,
	}}
}

// Optimize analyzes system capacity and generates tuning recommendations.
func (w *ConfigOptimizerWorkflow) Optimize(ctx context.Context, serverID string) (*DiagnosticReport, error) {
	srv, err := w.lookupServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	report := w.newReport("config_optimizer", serverID, srv, srv.Host)

	client, err := w.dialSSH(ctx, serverID, srv, "ключа")
	if err != nil {
		return nil, err
	}

	cpuOut, _, _, _ := client.Exec(ctx, "nproc 2>/dev/null || echo '1'")
	cpuCores, _ := strconv.Atoi(strings.TrimSpace(string(cpuOut)))
	if cpuCores <= 0 {
		cpuCores = 1
	}
	report.Metrics["cpu_cores"] = cpuCores

	memOut, _, _, _ := client.Exec(ctx, "grep -E 'MemTotal|MemAvailable' /proc/meminfo 2>/dev/null")
	var memTotalKB, memAvailKB int64
	for _, line := range strings.Split(string(memOut), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			val, _ := strconv.ParseInt(parts[1], 10, 64)
			if strings.HasPrefix(parts[0], "MemTotal:") {
				memTotalKB = val
			} else if strings.HasPrefix(parts[0], "MemAvailable:") {
				memAvailKB = val
			}
		}
	}

	memTotalMB := memTotalKB / 1024
	memAvailMB := memAvailKB / 1024
	report.Metrics["mem_total_mb"] = memTotalMB
	report.Metrics["mem_available_mb"] = memAvailMB

	nginxActive, _, _, _ := client.Exec(ctx, "systemctl is-active nginx 2>/dev/null")
	if strings.TrimSpace(string(nginxActive)) == "active" {
		nginxConf, _, _, _ := client.Exec(ctx, "cat /etc/nginx/nginx.conf 2>/dev/null")
		confStr := string(nginxConf)

		// Check worker_processes.
		if strings.Contains(confStr, "worker_processes") && !strings.Contains(confStr, "worker_processes auto;") {
			report.Issues = append(report.Issues, Issue{
				ID:             "opt-nginx-workers",
				Severity:       SeverityInfo,
				Component:      "webserver",
				Title:          "Оптимизация worker_processes в Nginx",
				Description:    fmt.Sprintf("Текущее значение worker_processes не использует директиву 'auto' на сервере с %d ядрами CPU.", cpuCores),
				Recommendation: "Укажите 'worker_processes auto;', чтобы Nginx автоматически утилизировал все доступные ядра CPU.",
				ProposedAction: "prepare_config_diff",
				ActionParams: map[string]any{
					"server_id": serverID,
					"file_path": "/etc/nginx/nginx.conf",
				},
			})
		}

		// Check gzip.
		if !strings.Contains(confStr, "gzip on;") {
			report.Issues = append(report.Issues, Issue{
				ID:             "opt-nginx-gzip",
				Severity:       SeverityInfo,
				Component:      "webserver",
				Title:          "Сжатие Gzip отключено в Nginx",
				Description:    "Gzip-сжатие снижает объем передаваемого HTTP-трафика для текстовых ассетов (HTML, CSS, JS) на 60-80%.",
				Recommendation: "Включите директиву 'gzip on;' и настройте 'gzip_types text/plain text/css application/json application/javascript'.",
			})
		}
	}

	phpSvcOut, _, _, _ := client.Exec(ctx, "systemctl list-units --type=service --state=running | grep -o 'php[0-9.]*-fpm' | head -n 1")
	phpSvc := strings.TrimSpace(string(phpSvcOut))
	if phpSvc != "" {
		report.Metrics["active_php_fpm"] = phpSvc
		// Estimate the optimal pm.max_children:
		// an average PHP process is ~45MB, and 60% of memory is allocated to PHP on a web server.
		targetRAMForPHP := float64(memTotalMB) * 0.6
		recommendedMaxChildren := int(targetRAMForPHP / 45.0)
		if recommendedMaxChildren < 4 {
			recommendedMaxChildren = 4
		} else if recommendedMaxChildren > 128 {
			recommendedMaxChildren = 128
		}

		report.Issues = append(report.Issues, Issue{
			ID:             "opt-php-fpm-pool",
			Severity:       SeverityInfo,
			Component:      "php",
			Title:          fmt.Sprintf("Расчет пула процессов PHP-FPM (%s)", phpSvc),
			Description:    fmt.Sprintf("При объеме оперативной памяти %d МБ оптимальное значение pm.max_children для пула составляет ~%d процессов (из расчета ~45 МБ на процесс с защитным запасом под кэши ядра).", memTotalMB, recommendedMaxChildren),
			Recommendation: fmt.Sprintf("Установите в /etc/php/*/fpm/pool.d/www.conf параметры: pm.max_children = %d, pm.max_requests = 1000.", recommendedMaxChildren),
		})
	}

	mysqlActive, _, _, _ := client.Exec(ctx, "systemctl is-active mysql 2>/dev/null || systemctl is-active mariadb 2>/dev/null")
	if strings.TrimSpace(string(mysqlActive)) == "active" {
		// Recommended InnoDB Buffer Pool size:
		// RAM < 2GB: 256MB-512MB
		// RAM >= 2GB up to 8GB: 40% of RAM
		// RAM > 8GB: 50% of RAM
		var recommendedBufferPoolMB int64
		if memTotalMB < 2048 {
			recommendedBufferPoolMB = 256
		} else if memTotalMB <= 8192 {
			recommendedBufferPoolMB = int64(float64(memTotalMB) * 0.4)
		} else {
			recommendedBufferPoolMB = int64(float64(memTotalMB) * 0.5)
		}

		// Round down to a multiple of 128M.
		recommendedBufferPoolMB = (recommendedBufferPoolMB / 128) * 128
		if recommendedBufferPoolMB < 128 {
			recommendedBufferPoolMB = 128
		}

		report.Metrics["recommended_innodb_buffer_mb"] = recommendedBufferPoolMB

		report.Issues = append(report.Issues, Issue{
			ID:             "opt-mysql-innodb-buffer",
			Severity:       SeverityInfo,
			Component:      "database",
			Title:          "Оптимизация MySQL InnoDB Buffer Pool",
			Description:    fmt.Sprintf("Для сервера с %d МБ RAM расчетный оптимальный размер innodb_buffer_pool_size составляет %dM. Это обеспечит удержание горячих индексов и данных в памяти, минимизируя задержки дискового I/O.", memTotalMB, recommendedBufferPoolMB),
			Recommendation: fmt.Sprintf("Добавьте или обновите в /etc/mysql/my.cnf директиву: innodb_buffer_pool_size = %dM", recommendedBufferPoolMB),
			ProposedAction: "prepare_config_diff",
			ActionParams: map[string]any{
				"server_id": serverID,
				"file_path": "/etc/mysql/my.cnf",
			},
		})
	}

	swapOut, _, _, _ := client.Exec(ctx, "grep -E 'SwapTotal|SwapFree' /proc/meminfo 2>/dev/null")
	var swapTotalKB int64
	for _, line := range strings.Split(string(swapOut), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && strings.HasPrefix(parts[0], "SwapTotal:") {
			swapTotalKB, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}
	swapTotalMB := swapTotalKB / 1024
	report.Metrics["swap_total_mb"] = swapTotalMB

	if swapTotalMB == 0 && memTotalMB <= 4096 {
		report.Issues = append(report.Issues, Issue{
			ID:             "opt-swap-missing",
			Severity:       SeverityWarning,
			Component:      "system",
			Title:          "Отсутствует раздел подкачки (Swap)",
			Description:    fmt.Sprintf("На сервере с объемом RAM %d МБ отсутствует Swap. При пиковых нагрузках ядро Linux активирует OOM Killer и принудительно завершит критические процессы (MySQL/PHP).", memTotalMB),
			Recommendation: "Создайте swap-файл размером не менее 2 ГБ ('fallocate -l 2G /swapfile && mkswap /swapfile && swapon /swapfile').",
		})
	}

	if len(report.Issues) > 0 {
		report.Status = "warning"
		report.Summary = fmt.Sprintf("Анализ ресурсов сервера (%d ядер CPU, %d МБ RAM) сформировал %d рекомендаций по оптимизации производительности.", cpuCores, memTotalMB, len(report.Issues))
	} else {
		report.Status = "healthy"
		report.Summary = "Конфигурация сервера хорошо сбалансирована под имеющиеся аппаратные ресурсы."
	}

	return report, nil
}
