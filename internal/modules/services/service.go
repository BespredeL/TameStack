package services

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
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

type Service struct {
	module.Base
	engine *tasks.Engine
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}

	engine.RegisterHandler("service_control", s.handleServiceControlTask)
	engine.RegisterHandler("process_kill", s.handleProcessKillTask)

	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string, sudo bool) (string, error) {
	return module.Exec(ctx, client, cmd, sudo)
}

// ListUnits returns all systemd units (services, timers, sockets) matching optional filter.
func (s *Service) ListUnits(ctx context.Context, serverID string, unitType string) ([]SystemUnit, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := "systemctl list-units --all --no-pager --no-legend --plain"
	out, err := s.execCmd(ctx, client, cmd, false)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка служб: %w", err)
	}

	units, err := ParseSystemctlUnits(out)
	if err != nil {
		return nil, err
	}

	filter := strings.ToLower(strings.TrimSpace(unitType))
	if filter == "" || filter == "all" {
		return units, nil
	}

	var filtered []SystemUnit
	for _, u := range units {
		if u.UnitType == filter {
			filtered = append(filtered, u)
		}
	}
	return filtered, nil
}

// ControlUnit submits a task to start, stop, restart, reload, enable or disable a unit.
func (s *Service) ControlUnit(ctx context.Context, serverID string, unitName string, action string) (*db.TaskRecord, error) {
	cleanName, err := SanitizeUnitName(unitName)
	if err != nil {
		return nil, err
	}

	act := strings.ToLower(strings.TrimSpace(action))
	allowedActions := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
		"reload":  true,
		"enable":  true,
		"disable": true,
	}
	if !allowedActions[act] {
		return nil, fmt.Errorf("недопустимое действие: %s", action)
	}

	// Safety check: protect SSH/network.
	if err := IsProtectedUnit(cleanName, act); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(UnitActionRequest{
		ServerID: serverID,
		UnitName: cleanName,
		Action:   act,
	})

	taskName := fmt.Sprintf("Управление службой %s: %s", cleanName, act)
	return s.engine.Submit(serverID, "service_control", taskName, string(payload))
}

// GetUnitLogs fetches the recent journalctl logs for a unit.
func (s *Service) GetUnitLogs(ctx context.Context, serverID string, unitName string, lines int) (string, error) {
	cleanName, err := SanitizeUnitName(unitName)
	if err != nil {
		return "", err
	}

	if lines <= 0 {
		lines = 100
	}
	if lines > 2000 {
		lines = 2000
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return "", err
	}

	cmd := fmt.Sprintf("journalctl -u %s -n %d --no-pager", cleanName, lines)
	out, err := s.execCmd(ctx, client, cmd, true)
	if err != nil && out == "" {
		return "", fmt.Errorf("ошибка чтения журнала службы: %w", err)
	}
	return out, nil
}

// ListProcesses returns the current process list sorted by CPU usage.
func (s *Service) ListProcesses(ctx context.Context, serverID string) ([]ProcessItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := "ps -eo user:20,pid,%cpu,%mem,vsz,rss,stat,start,time,args --sort=-%cpu | head -n 250"
	out, err := s.execCmd(ctx, client, cmd, false)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения процессов: %w", err)
	}

	return ParsePsOutput(out)
}

// KillProcess submits a task to send a signal to a process.
func (s *Service) KillProcess(ctx context.Context, serverID string, pid int, signal string) (*db.TaskRecord, error) {
	if err := ValidateProcessKill(pid, signal); err != nil {
		return nil, err
	}

	sig := strings.ToUpper(strings.TrimSpace(signal))
	if sig == "" {
		sig = "SIGTERM"
	}

	payload, _ := json.Marshal(ProcessKillRequest{
		ServerID: serverID,
		PID:      pid,
		Signal:   sig,
	})

	taskName := fmt.Sprintf("Завершение процесса PID %d (%s)", pid, sig)
	return s.engine.Submit(serverID, "process_kill", taskName, string(payload))
}

// ListListeningPorts returns listening sockets from ss -tulpn.
func (s *Service) ListListeningPorts(ctx context.Context, serverID string) ([]ListeningPortItem, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := "ss -tulpn 2>/dev/null || netstat -tulpn 2>/dev/null"
	out, err := s.execCmd(ctx, client, cmd, true)
	if err != nil && out == "" {
		return nil, fmt.Errorf("ошибка получения портов: %w", err)
	}

	return ParseSsListening(out)
}

// CheckDNS runs DNS resolution query on the remote host.
func (s *Service) CheckDNS(ctx context.Context, serverID string, domain string, recordType string) (*DNSCheckResult, error) {
	dom := strings.TrimSpace(domain)
	if dom == "" {
		return nil, fmt.Errorf("доменное имя не может быть пустым")
	}
	recType := strings.ToUpper(strings.TrimSpace(recordType))
	if recType == "" {
		recType = "A"
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// Try dig, then host, then getent.
	cmd := fmt.Sprintf("dig +short %s %s 2>/dev/null || host -t %s %s 2>/dev/null || getent hosts %s 2>/dev/null || true", recType, dom, recType, dom, dom)
	out, _ := s.execCmd(ctx, client, cmd, false)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var records []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			records = append(records, trimmed)
		}
	}

	return &DNSCheckResult{
		Domain:     dom,
		RecordType: recType,
		Records:    records,
		Success:    len(records) > 0,
	}, nil
}

// CheckPort verifies remote TCP port reachability from the server.
func (s *Service) CheckPort(ctx context.Context, serverID string, host string, port int) (*PortCheckResult, error) {
	h := strings.TrimSpace(host)
	if h == "" {
		return nil, fmt.Errorf("хост не может быть пустым")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("недопустимый порт: %d", port)
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	cmd := fmt.Sprintf("nc -z -w 3 %s %d 2>/dev/null || timeout 3 bash -c '</dev/tcp/%s/%d' 2>/dev/null", h, port, h, port)
	_, err = s.execCmd(ctx, client, cmd, false)
	duration := time.Since(start).Milliseconds()

	return &PortCheckResult{
		Host:      h,
		Port:      port,
		Open:      err == nil,
		LatencyMs: duration,
	}, nil
}

// GetNetworkOverview returns read-only network details with security explanation.
func (s *Service) GetNetworkOverview(ctx context.Context, serverID string) (*NetworkOverview, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	ifaces, _ := s.execCmd(ctx, client, "ip -br addr 2>/dev/null || ifconfig -a 2>/dev/null", false)
	routes, _ := s.execCmd(ctx, client, "ip route 2>/dev/null || route -n 2>/dev/null", false)
	dns, _ := s.execCmd(ctx, client, "cat /etc/resolv.conf 2>/dev/null | grep -v '^#' || true", false)

	return &NetworkOverview{
		Interfaces: strings.TrimSpace(ifaces),
		Routes:     strings.TrimSpace(routes),
		DNSResolv:  strings.TrimSpace(dns),
		Notice:     "Изменение сетевых интерфейсов и маршрутов намеренно заблокировано в интерфейсе, чтобы исключить риск потери SSH-соединения с сервером.",
	}, nil
}

func (s *Service) handleServiceControlTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req UnitActionRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	cleanName, err := SanitizeUnitName(req.UnitName)
	if err != nil {
		return "", err
	}

	if err := IsProtectedUnit(cleanName, req.Action); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep(fmt.Sprintf("Выполнение %s для %s", req.Action, cleanName), 40)
	cmd := fmt.Sprintf("systemctl %s %s", req.Action, cleanName)
	out, err := s.execCmd(ctx, client, cmd, true)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения systemctl %s %s: %w\n%s", req.Action, cleanName, err, out)
	}

	logger.SetStep("Проверка статуса", 80)
	statusOut, _ := s.execCmd(ctx, client, fmt.Sprintf("systemctl is-active %s || true", cleanName), false)

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Команда %s для службы %s успешно выполнена. Текущий статус: %s", req.Action, cleanName, strings.TrimSpace(statusOut)), nil
}

func (s *Service) handleProcessKillTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req ProcessKillRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	if err := ValidateProcessKill(req.PID, req.Signal); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep(fmt.Sprintf("Отправка сигнала %s процессу PID %d", req.Signal, req.PID), 50)
	cmd := fmt.Sprintf("kill -s %s %d", req.Signal, req.PID)
	out, err := s.execCmd(ctx, client, cmd, true)
	if err != nil {
		return "", fmt.Errorf("ошибка завершения процесса PID %d: %w\n%s", req.PID, err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Сигнал %s успешно отправлен процессу PID %d", req.Signal, req.PID), nil
}
