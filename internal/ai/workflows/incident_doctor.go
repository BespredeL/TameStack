package workflows

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type DoctorDiagnosis struct {
	Title          string            `json:"title"`
	IncidentType   string            `json:"incident_type"`
	RootCause      string            `json:"root_cause"`
	RecommendedFix string            `json:"recommended_fix"`
	RemediationCmd string            `json:"remediation_cmd,omitempty"`
	CanAutoFix     bool              `json:"can_auto_fix"`
	Report         *DiagnosticReport `json:"report,omitempty"`
}

var (
	fpmSocketRegex = regexp.MustCompile(`unix:(/run/php/php\d+\.\d+-fpm\.sock|/var/run/php/php\d+\.\d+-fpm\.sock)`)
	fpmVerRegex    = regexp.MustCompile(`php(\d+\.\d+)-fpm`)
)

// AnalyzeIncidentContext parses error logs, signals, and context to produce a structured diagnosis.
func AnalyzeIncidentContext(incidentType, details, logs string) *DoctorDiagnosis {
	combined := strings.ToLower(fmt.Sprintf("%s\n%s\n%s", incidentType, details, logs))

	// 1. Case: 502 Bad Gateway / Upstream Connection Refused.
	if strings.Contains(combined, "502") || strings.Contains(combined, "upstream") || strings.Contains(combined, "bad gateway") {
		diag := &DoctorDiagnosis{
			Title:        "Сбой подключения веб-сервера к бэкенду (502 Bad Gateway)",
			IncidentType: "502_bad_gateway",
			CanAutoFix:   true,
		}

		if m := fpmSocketRegex.FindStringSubmatch(logs); len(m) > 1 {
			sock := m[1]
			ver := "8.2"
			if vm := fpmVerRegex.FindStringSubmatch(sock); len(vm) > 1 {
				ver = vm[1]
			}
			diag.RootCause = fmt.Sprintf("Веб-сервер не может подключиться к сокету PHP-FPM (%s). Служба php%s-fpm остановлена либо сокет отсутствует.", sock, ver)
			diag.RecommendedFix = fmt.Sprintf("Запустить или перезапустить службу php%s-fpm.", ver)
			diag.RemediationCmd = fmt.Sprintf("systemctl restart php%s-fpm", ver)
		} else if strings.Contains(combined, "111: connection refused") || strings.Contains(combined, "127.0.0.1:") {
			diag.RootCause = "Бэкенд-сервис (Node.js/Python/Go) на локальном порту не отвечает или аварийно завершил работу."
			diag.RecommendedFix = "Проверить статус фонового процесса (PM2/systemd) и перезапустить сервис."
			diag.RemediationCmd = "systemctl restart sh-*"
		} else {
			diag.RootCause = "Веб-сервер получил недопустимый ответ от вышестоящего сервера (upstream)."
			diag.RecommendedFix = "Проверить статус активных PHP-FPM сокетов и перезагрузить веб-сервер."
			diag.RemediationCmd = "systemctl reload nginx 2>/dev/null || systemctl reload apache2 2>/dev/null"
		}
		return diag
	}

	// 2. Case: Disk Space Exhaustion (No space left on device).
	if strings.Contains(combined, "space") || strings.Contains(combined, "disk") || strings.Contains(combined, "enospc") {
		return &DoctorDiagnosis{
			Title:          "Критическое переполнение дискового пространства",
			IncidentType:   "disk_full",
			RootCause:      "На диске закончилось свободное место (0 байт), из-за чего базы данных и службы не могут писать временные файлы и логи.",
			RecommendedFix: "Выполнить безопасную очистку старых системных логов journalctl, кэшей пакетов apt/dnf и неиспользуемых Docker слоев.",
			RemediationCmd: "journalctl --vacuum-time=3d && apt-get clean 2>/dev/null && docker system prune -f 2>/dev/null || true",
			CanAutoFix:     true,
		}
	}

	// 3. Case: Database Failure (MySQL/PostgreSQL down).
	if strings.Contains(combined, "mysql") || strings.Contains(combined, "postgres") || strings.Contains(combined, "can't connect to local mysql server") {
		return &DoctorDiagnosis{
			Title:          "Отказ службы сервера баз данных",
			IncidentType:   "database_down",
			RootCause:      "Сервер баз данных остановлен или завершил работу из-за нехватки памяти (OOM Killer).",
			RecommendedFix: "Перезапустить службу базы данных и проверить доступную оперативную память.",
			RemediationCmd: "systemctl restart mysql 2>/dev/null || systemctl restart mariadb 2>/dev/null || systemctl restart postgresql 2>/dev/null",
			CanAutoFix:     true,
		}
	}

	// Default diagnosis
	return &DoctorDiagnosis{
		Title:          "Системный инцидент",
		IncidentType:   incidentType,
		RootCause:      fmt.Sprintf("Обнаружена ошибка: %s", details),
		RecommendedFix: "Изучить системные журналы службы и выполнить перезапуск.",
		CanAutoFix:     false,
		Report: &DiagnosticReport{
			WorkflowType: "incident_doctor",
			Status:       "warning",
			Summary:      details,
			Timestamp:    time.Now(),
		},
	}
}
