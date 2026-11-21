package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

// --- Firewall (UFW / Firewalld) ---

// GetFirewallStatus detects the firewall type and reads its current status.
func (s *Service) GetFirewallStatus(ctx context.Context, serverID string) (*FirewallStatus, error) {
	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	checkUFW, _ := s.execCmd(ctx, client, "which ufw")
	if strings.TrimSpace(checkUFW) != "" {
		return s.parseUFWStatus(ctx, client)
	}

	checkFWD, _ := s.execCmd(ctx, client, "which firewall-cmd")
	if strings.TrimSpace(checkFWD) != "" {
		return s.parseFirewalldStatus(ctx, client)
	}

	return &FirewallStatus{
		Type:            "none",
		IsActive:        false,
		DefaultIncoming: "allow",
		DefaultOutgoing: "allow",
		Rules:           []FirewallRule{},
	}, nil
}

func (s *Service) parseUFWStatus(ctx context.Context, client *ssh.SSHClient) (*FirewallStatus, error) {
	status := &FirewallStatus{
		Type:            "ufw",
		IsActive:        false,
		DefaultIncoming: "deny",
		DefaultOutgoing: "allow",
		Rules:           []FirewallRule{},
	}

	outVerbose, _ := s.execCmd(ctx, client, "env LC_ALL=C ufw status verbose 2>/dev/null || ufw status verbose")
	lowerVerbose := strings.ToLower(outVerbose)
	if strings.Contains(lowerVerbose, "status: active") || strings.Contains(lowerVerbose, "активен") {
		status.IsActive = true
	} else {
		// Fallback check through systemctl unit state.
		outUnit, _ := s.execCmd(ctx, client, "systemctl is-active ufw 2>/dev/null || true")
		if strings.TrimSpace(outUnit) == "active" {
			status.IsActive = true
		}
	}

	// Default: deny (incoming), allow (outgoing), disabled (routed).
	if strings.Contains(lowerVerbose, "incoming") || strings.Contains(lowerVerbose, "входящие") {
		if strings.Contains(lowerVerbose, "deny (incoming)") || strings.Contains(lowerVerbose, "deny (входящие)") {
			status.DefaultIncoming = "deny"
		} else if strings.Contains(lowerVerbose, "reject (incoming)") || strings.Contains(lowerVerbose, "reject (входящие)") {
			status.DefaultIncoming = "reject"
		} else if strings.Contains(lowerVerbose, "allow (incoming)") || strings.Contains(lowerVerbose, "allow (входящие)") {
			status.DefaultIncoming = "allow"
		}
	}

	// Read the numbered rules.
	outNumbered, _ := s.execCmd(ctx, client, "env LC_ALL=C ufw status numbered 2>/dev/null || ufw status numbered")
	scanner := bufio.NewScanner(strings.NewReader(outNumbered))

	// Regular expression for a rule line: [ 1] 22/tcp  ALLOW IN  Anywhere  # comment.
	reRule := regexp.MustCompile(`^\[\s*(\d+)\]\s+(\S+)\s+(ALLOW|DENY|REJECT|LIMIT)\s*(IN|OUT)?\s+(\S+)(?:\s+#\s*(.*))?$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") {
			continue
		}

		matches := reRule.FindStringSubmatch(line)
		if len(matches) > 0 {
			num, _ := strconv.Atoi(matches[1])
			portProto := matches[2]
			action := matches[3]
			direction := matches[4]
			if direction == "" {
				direction = "IN"
			}
			fromIP := matches[5]
			comment := ""
			if len(matches) > 6 {
				comment = strings.TrimSpace(matches[6])
			}

			port := portProto
			proto := "any"
			if strings.Contains(portProto, "/") {
				parts := strings.SplitN(portProto, "/", 2)
				port = parts[0]
				proto = parts[1]
			}

			status.Rules = append(status.Rules, FirewallRule{
				Number:    num,
				Port:      port,
				Protocol:  proto,
				Action:    action,
				FromIP:    fromIP,
				Direction: direction,
				Comment:   comment,
			})
		}
	}

	return status, nil
}

func (s *Service) parseFirewalldStatus(ctx context.Context, client *ssh.SSHClient) (*FirewallStatus, error) {
	status := &FirewallStatus{
		Type:            "firewalld",
		IsActive:        false,
		DefaultIncoming: "deny",
		DefaultOutgoing: "allow",
		Rules:           []FirewallRule{},
	}

	outState, _ := s.execCmd(ctx, client, "env LC_ALL=C firewall-cmd --state 2>/dev/null || firewall-cmd --state")
	if strings.TrimSpace(outState) == "running" {
		status.IsActive = true
	} else {
		outUnit, _ := s.execCmd(ctx, client, "systemctl is-active firewalld 2>/dev/null || true")
		if strings.TrimSpace(outUnit) == "active" {
			status.IsActive = true
		}
	}

	outPorts, _ := s.execCmd(ctx, client, "firewall-cmd --list-ports")
	ports := strings.Fields(strings.TrimSpace(outPorts))

	num := 1
	for _, p := range ports {
		parts := strings.Split(p, "/")
		port := parts[0]
		proto := "tcp"
		if len(parts) > 1 {
			proto = parts[1]
		}
		status.Rules = append(status.Rules, FirewallRule{
			Number:    num,
			Port:      port,
			Protocol:  proto,
			Action:    "ALLOW",
			FromIP:    "Anywhere",
			Direction: "IN",
			Comment:   "Firewalld standard port",
		})
		num++
	}

	return status, nil
}

// ToggleFirewall enables or disables the firewall while protecting the SSH port.
func (s *Service) ToggleFirewall(ctx context.Context, serverID string, enable bool) (*db.TaskRecord, error) {
	title := "Включение брандмауэра"
	if !enable {
		title = "Отключение брандмауэра"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"server_id": serverID,
		"enable":    enable,
	})

	return s.engine.Submit(serverID, "security_firewall_toggle", title, string(payload))
}

func (s *Service) handleFirewallToggleTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID string `json:"server_id"`
		Enable   bool   `json:"enable"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("неверные параметры задачи: %w", err)
	}

	client, record, err := s.getSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Проверка типа брандмауэра", 20)
	checkUFW, _ := s.execCmd(ctx, client, "which ufw")
	isUFW := strings.TrimSpace(checkUFW) != ""

	if isUFW {
		if payload.Enable {
			// CRITICAL PROTECTION: before enabling the firewall, SSH and the web ports (80, 443) must be opened.
			sshPort := record.Port
			if sshPort == 0 {
				sshPort = 22
			}
			logger.Info(fmt.Sprintf("Гарантируем сетевой доступ: открытие портов SSH (%d/tcp), HTTP (80/tcp) и HTTPS (443/tcp)...", sshPort))
			_, _ = s.execCmd(ctx, client, fmt.Sprintf("ufw allow %d/tcp comment 'TameStack SSH' && ufw allow 80/tcp comment 'HTTP Web' && ufw allow 443/tcp comment 'HTTPS Web'", sshPort))

			logger.SetStep("Включение UFW брандмауэра", 60)
			out, err := s.execCmd(ctx, client, "ufw --force enable")
			if err != nil {
				return "", fmt.Errorf("ошибка включения ufw: %w: %s", err, out)
			}
			logger.Info("UFW брандмауэр успешно включен, порты SSH, 80 и 443 защищены и открыты.")
		} else {
			logger.SetStep("Отключение UFW брандмауэра", 60)
			out, err := s.execCmd(ctx, client, "ufw disable")
			if err != nil {
				return "", fmt.Errorf("ошибка отключения ufw: %w: %s", err, out)
			}
			logger.Info("UFW брандмауэр отключен.")
		}
	} else {
		// Firewalld
		if payload.Enable {
			logger.SetStep("Запуск firewalld службы", 50)
			_, _ = s.execCmd(ctx, client, fmt.Sprintf("firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --permanent --add-service=http && firewall-cmd --permanent --add-service=https 2>/dev/null || true", record.Port))
			out, err := s.execCmd(ctx, client, "systemctl enable --now firewalld && firewall-cmd --reload")
			if err != nil {
				return "", fmt.Errorf("ошибка запуска firewalld: %w: %s", err, out)
			}
		} else {
			logger.SetStep("Остановка firewalld службы", 50)
			out, err := s.execCmd(ctx, client, "systemctl stop firewalld && systemctl disable firewalld")
			if err != nil {
				return "", fmt.Errorf("ошибка остановки firewalld: %w: %s", err, out)
			}
		}
	}

	logger.SetStep("Готово", 100)
	return "Статус брандмауэра успешно изменен", nil
}

// AddFirewallRule adds a new firewall rule with security validation.
func (s *Service) AddFirewallRule(ctx context.Context, req AddFirewallRuleRequest) (*db.TaskRecord, error) {
	if strings.TrimSpace(req.Port) == "" {
		return nil, fmt.Errorf("не указан порт или диапазон портов")
	}

	// SSH protection validation: refuse to block the active SSH port.
	record, err := s.database.GetServer(ctx, req.ServerID)
	if err == nil {
		actionUpper := strings.ToUpper(req.Action)
		if actionUpper == "DENY" || actionUpper == "REJECT" {
			portNum, _ := strconv.Atoi(req.Port)
			if portNum == record.Port || req.Port == strconv.Itoa(record.Port) || (record.Port == 0 && portNum == 22) {
				from := strings.ToLower(strings.TrimSpace(req.FromIP))
				if from == "" || from == "any" || from == "anywhere" || from == "0.0.0.0/0" {
					return nil, fmt.Errorf("ОПАСНОСТЬ: Запрещено блокировать порт %d для всех IP! Это приведет к немедленной потере SSH-доступа к серверу.", record.Port)
				}
			}
		}
	}

	payload, _ := json.Marshal(req)
	title := fmt.Sprintf("Добавление правила Firewall: %s %s", req.Action, req.Port)
	return s.engine.Submit(req.ServerID, "security_firewall_add_rule", title, string(payload))
}

func (s *Service) handleFirewallAddRuleTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req AddFirewallRuleRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}
	if err := ssh.ValidateFirewallAction(req.Action); err != nil {
		return "", err
	}
	if err := ssh.ValidatePortSpec(req.Port); err != nil {
		return "", err
	}
	if err := ssh.ValidateFirewallProto(req.Protocol); err != nil {
		return "", err
	}
	if err := ssh.ValidateSource(req.FromIP); err != nil {
		return "", err
	}

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Формирование команды брандмауэра", 30)

	checkUFW, _ := s.execCmd(ctx, client, "which ufw")
	isUFW := strings.TrimSpace(checkUFW) != ""

	actionLower := strings.ToLower(req.Action)
	if actionLower == "" {
		actionLower = "allow"
	}

	proto := strings.ToLower(req.Protocol)
	if proto == "" || proto == "any" {
		proto = ""
	}

	if isUFW {
		cmdParts := []string{"ufw"}
		cmdParts = append(cmdParts, actionLower)

		if proto != "" {
			cmdParts = append(cmdParts, "proto", proto)
		}

		if req.FromIP != "" && strings.ToLower(req.FromIP) != "any" && strings.ToLower(req.FromIP) != "anywhere" {
			cmdParts = append(cmdParts, "from", req.FromIP, "to", "any")
		}

		cmdParts = append(cmdParts, "port", req.Port)

		if req.Comment != "" {
			cleanComment := strings.ReplaceAll(req.Comment, "'", "")
			cmdParts = append(cmdParts, "comment", fmt.Sprintf("'%s'", cleanComment))
		}

		fullCmd := strings.Join(cmdParts, " ")
		logger.Info("Выполнение команды: " + fullCmd)
		logger.SetStep("Применение правила UFW", 70)

		out, err := s.execCmd(ctx, client, fullCmd)
		if err != nil {
			return "", fmt.Errorf("ошибка добавления правила: %w: %s", err, out)
		}
	} else {
		// Firewalld
		rulePort := req.Port
		if proto != "" {
			rulePort = fmt.Sprintf("%s/%s", req.Port, proto)
		} else {
			rulePort = fmt.Sprintf("%s/tcp", req.Port)
		}

		cmd := fmt.Sprintf("firewall-cmd --permanent --add-port=%s && firewall-cmd --reload", rulePort)
		logger.Info("Выполнение команды firewalld: " + cmd)
		logger.SetStep("Применение правила firewalld", 70)

		out, err := s.execCmd(ctx, client, cmd)
		if err != nil {
			return "", fmt.Errorf("ошибка firewalld: %w: %s", err, out)
		}
	}

	logger.SetStep("Правило успешно добавлено", 100)
	return fmt.Sprintf("Правило %s для порта %s успешно применено", req.Action, req.Port), nil
}

// UpdateFirewallRule updates an existing firewall rule with security validation.
func (s *Service) UpdateFirewallRule(ctx context.Context, req UpdateFirewallRuleRequest) (*db.TaskRecord, error) {
	if strings.TrimSpace(req.Port) == "" {
		return nil, fmt.Errorf("не указан порт или диапазон портов")
	}

	// SSH protection validation: refuse to block the active SSH port.
	record, err := s.database.GetServer(ctx, req.ServerID)
	if err == nil {
		actionUpper := strings.ToUpper(req.Action)
		if actionUpper == "DENY" || actionUpper == "REJECT" {
			portNum, _ := strconv.Atoi(req.Port)
			sshPort := record.Port
			if sshPort == 0 {
				sshPort = 22
			}
			if portNum == sshPort || req.Port == strconv.Itoa(sshPort) {
				from := strings.ToLower(strings.TrimSpace(req.FromIP))
				if from == "" || from == "any" || from == "anywhere" || from == "0.0.0.0/0" {
					return nil, fmt.Errorf("ОПАСНОСТЬ: Запрещено блокировать порт %d для всех IP! Это приведет к немедленной потере SSH-доступа к серверу.", sshPort)
				}
			}
		}
	}

	payload, _ := json.Marshal(req)
	title := fmt.Sprintf("Изменение правила Firewall #%d: %s %s", req.RuleNum, req.Action, req.Port)
	return s.engine.Submit(req.ServerID, "security_firewall_update_rule", title, string(payload))
}

func (s *Service) handleFirewallUpdateRuleTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req UpdateFirewallRuleRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}
	if err := ssh.ValidateFirewallAction(req.Action); err != nil {
		return "", err
	}
	if err := ssh.ValidatePortSpec(req.Port); err != nil {
		return "", err
	}
	if err := ssh.ValidateFirewallProto(req.Protocol); err != nil {
		return "", err
	}
	if err := ssh.ValidateSource(req.FromIP); err != nil {
		return "", err
	}

	client, _, err := s.getSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Формирование команды брандмауэра", 20)

	checkUFW, _ := s.execCmd(ctx, client, "which ufw")
	isUFW := strings.TrimSpace(checkUFW) != ""

	actionLower := strings.ToLower(req.Action)
	if actionLower == "" {
		actionLower = "allow"
	}

	proto := strings.ToLower(req.Protocol)
	if proto == "" || proto == "any" {
		proto = ""
	}

	if isUFW {
		cmdParts := []string{"ufw"}
		if req.RuleNum > 0 {
			cmdParts = append(cmdParts, "insert", strconv.Itoa(req.RuleNum))
		}
		cmdParts = append(cmdParts, actionLower)

		if proto != "" {
			cmdParts = append(cmdParts, "proto", proto)
		}

		if req.FromIP != "" && strings.ToLower(req.FromIP) != "any" && strings.ToLower(req.FromIP) != "anywhere" {
			cmdParts = append(cmdParts, "from", req.FromIP, "to", "any")
		}

		cmdParts = append(cmdParts, "port", req.Port)

		if req.Comment != "" {
			cleanComment := strings.ReplaceAll(req.Comment, "'", "")
			cmdParts = append(cmdParts, "comment", fmt.Sprintf("'%s'", cleanComment))
		}

		fullCmd := strings.Join(cmdParts, " ")
		logger.Info("Выполнение команды: " + fullCmd)
		logger.SetStep("Применение обновленного правила UFW", 50)

		out, err := s.execCmd(ctx, client, fullCmd)
		if err != nil {
			return "", fmt.Errorf("ошибка применения правила: %w: %s", err, out)
		}

		if req.RuleNum > 0 {
			delCmd := fmt.Sprintf("echo y | ufw delete %d", req.RuleNum+1)
			logger.Info("Удаление старой версии правила: " + delCmd)
			logger.SetStep("Удаление старой версии правила UFW", 80)
			delOut, delErr := s.execCmd(ctx, client, delCmd)
			if delErr != nil {
				logger.Warn(fmt.Sprintf("предупреждение при удалении старого правила #%d: %v (%s)", req.RuleNum+1, delErr, delOut))
			}
		}
	} else {
		// Firewalld
		oldPortProto := req.OldPort
		if req.OldProto != "" && !strings.Contains(req.OldPort, "/") && strings.ToLower(req.OldProto) != "any" {
			oldPortProto = fmt.Sprintf("%s/%s", req.OldPort, strings.ToLower(req.OldProto))
		} else if !strings.Contains(req.OldPort, "/") {
			oldPortProto = fmt.Sprintf("%s/tcp", req.OldPort)
		}

		newPortProto := req.Port
		if proto != "" {
			newPortProto = fmt.Sprintf("%s/%s", req.Port, proto)
		} else {
			newPortProto = fmt.Sprintf("%s/tcp", req.Port)
		}

		var cmd string
		if oldPortProto != newPortProto && req.OldPort != "" {
			cmd = fmt.Sprintf("firewall-cmd --permanent --remove-port=%s ; firewall-cmd --permanent --add-port=%s && firewall-cmd --reload", oldPortProto, newPortProto)
		} else {
			cmd = fmt.Sprintf("firewall-cmd --permanent --add-port=%s && firewall-cmd --reload", newPortProto)
		}
		logger.Info("Выполнение команды firewalld: " + cmd)
		logger.SetStep("Применение правила firewalld", 70)

		out, err := s.execCmd(ctx, client, cmd)
		if err != nil {
			return "", fmt.Errorf("ошибка firewalld: %w: %s", err, out)
		}
	}

	logger.SetStep("Правило успешно обновлено", 100)
	return fmt.Sprintf("Правило #%d (%s %s) успешно обновлено", req.RuleNum, req.Action, req.Port), nil
}

// DeleteFirewallRule removes a firewall rule.
func (s *Service) DeleteFirewallRule(ctx context.Context, serverID string, ruleNum int, port string, proto string) (*db.TaskRecord, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"server_id": serverID,
		"rule_num":  ruleNum,
		"port":      port,
		"proto":     proto,
	})
	title := fmt.Sprintf("Удаление правила Firewall #%d (%s)", ruleNum, port)
	return s.engine.Submit(serverID, "security_firewall_del_rule", title, string(payload))
}

func (s *Service) handleFirewallDelRuleTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		ServerID string `json:"server_id"`
		RuleNum  int    `json:"rule_num"`
		Port     string `json:"port"`
		Proto    string `json:"proto"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.getSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Удаление правила", 40)
	checkUFW, _ := s.execCmd(ctx, client, "which ufw")
	isUFW := strings.TrimSpace(checkUFW) != ""

	if isUFW {
		cmd := fmt.Sprintf("echo y | ufw delete %d", payload.RuleNum)
		out, err := s.execCmd(ctx, client, cmd)
		if err != nil {
			return "", fmt.Errorf("ошибка удаления правила UFW: %w: %s", err, out)
		}
	} else {
		portProto := payload.Port
		if payload.Proto != "" && !strings.Contains(payload.Port, "/") {
			portProto = fmt.Sprintf("%s/%s", payload.Port, payload.Proto)
		}
		cmd := fmt.Sprintf("firewall-cmd --permanent --remove-port=%s && firewall-cmd --reload", portProto)
		out, err := s.execCmd(ctx, client, cmd)
		if err != nil {
			return "", fmt.Errorf("ошибка firewalld: %w: %s", err, out)
		}
	}

	logger.SetStep("Правило удалено", 100)
	return fmt.Sprintf("Правило #%d успешно удалено", payload.RuleNum), nil
}
