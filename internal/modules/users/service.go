package users

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

var (
	usernameRegex  = regexp.MustCompile(`^[a-z_][a-z0-9_-]{1,31}$`)
	sshKeyPrefixes = []string{
		"ssh-rsa ",
		"ssh-ed25519 ",
		"ecdsa-sha2-nistp256 ",
		"ecdsa-sha2-nistp384 ",
		"ecdsa-sha2-nistp521 ",
		"sk-ssh-ed25519@openssh.com ",
		"sk-ecdsa-sha2-nistp256@openssh.com ",
	}
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

	engine.RegisterHandler("user_create_site", s.handleCreateSiteUserTask)
	engine.RegisterHandler("user_chown_site", s.handleChownTask)
	engine.RegisterHandler("user_add_ssh_key", s.handleAddSSHKeyTask)
	engine.RegisterHandler("user_enable_sftp_chroot", s.handleEnableSFTPChrootTask)
	engine.RegisterHandler("user_disable_sftp_chroot", s.handleDisableSFTPChrootTask)
	engine.RegisterHandler("user_set_password", s.handleSetUserPasswordTask)

	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string, sudo bool) (string, error) {
	return module.Exec(ctx, client, cmd, sudo)
}

// ValidateUsername checks if a username is valid and not a reserved system user.
func ValidateUsername(username string) error {
	u := strings.TrimSpace(username)
	if u == "" {
		return fmt.Errorf("имя пользователя не может быть пустым")
	}
	if !usernameRegex.MatchString(u) {
		return fmt.Errorf("недопустимое имя пользователя '%s': должно начинаться с буквы и содержать только строчные буквы, цифры, дефис или подчеркивание (до 32 символов)", u)
	}

	reserved := map[string]bool{
		"root": true, "daemon": true, "bin": true, "sys": true, "sync": true,
		"games": true, "man": true, "lp": true, "mail": true, "news": true,
		"uucp": true, "proxy": true, "www-data": true, "backup": true,
		"list": true, "irc": true, "gnats": true, "nobody": true,
		"systemd": true, "syslog": true, "sshd": true, "sudo": true,
	}
	if reserved[u] {
		return fmt.Errorf("имя '%s' является зарезервированным системным именем пользователя", u)
	}
	return nil
}

// ValidateSSHKey checks that the provided public key has a valid OpenSSH prefix.
func ValidateSSHKey(key string) error {
	k := strings.TrimSpace(key)
	if k == "" {
		return fmt.Errorf("SSH-ключ не может быть пустым")
	}
	for _, p := range sshKeyPrefixes {
		if strings.HasPrefix(k, p) {
			return nil
		}
	}
	return fmt.Errorf("недопустимый формат открытого ключа SSH: поддерживаются ssh-rsa, ssh-ed25519, ecdsa-sha2-*")
}

// ListUsers returns users from /etc/passwd on the remote server.
func (s *Service) ListUsers(ctx context.Context, serverID string) ([]SystemUserInfo, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	out, err := s.execCmd(ctx, client, "cat /etc/passwd", false)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения /etc/passwd: %w", err)
	}

	chrootMap := make(map[string]string)
	chrootOut, _ := s.execCmd(ctx, client, `grep -Hn "ChrootDirectory" /etc/ssh/sshd_config.d/tamestack_sftp_*.conf 2>/dev/null || true`, true)
	for _, line := range strings.Split(chrootOut, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, "tamestack_sftp_")
		if idx == -1 {
			continue
		}
		rest := line[idx+len("tamestack_sftp_"):]
		dot := strings.Index(rest, ".conf:")
		if dot == -1 {
			continue
		}
		uname := rest[:dot]
		fields := strings.Fields(rest[dot+len(".conf:"):])
		if len(fields) >= 2 && strings.EqualFold(fields[len(fields)-2], "ChrootDirectory") {
			chrootMap[uname] = fields[len(fields)-1]
		} else if len(fields) >= 3 && strings.EqualFold(fields[1], "ChrootDirectory") {
			chrootMap[uname] = fields[2]
		}
	}

	var users []SystemUserInfo
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}

		uid, _ := strconv.Atoi(parts[2])
		gid, _ := strconv.Atoi(parts[3])

		cDir, hasChroot := chrootMap[parts[0]]
		users = append(users, SystemUserInfo{
			Username:   parts[0],
			UID:        uid,
			GID:        gid,
			HomeDir:    parts[5],
			Shell:      parts[6],
			IsSystem:   uid < 1000 && parts[0] != "root",
			SFTPChroot: hasChroot,
			ChrootDir:  cDir,
		})
	}

	return users, nil
}

// CreateSiteUser submits a task to create an unprivileged website user with home directory.
func (s *Service) CreateSiteUser(ctx context.Context, serverID string, username string, homeDir string) (*db.TaskRecord, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}

	h := strings.TrimSpace(homeDir)
	if h == "" {
		h = fmt.Sprintf("/var/www/%s", username)
	}
	if err := ssh.ValidateRemotePath(h, false); err != nil {
		return nil, fmt.Errorf("недопустимый домашний каталог: %w", err)
	}

	payload, _ := json.Marshal(CreateSiteUserRequest{
		ServerID: serverID,
		Username: username,
		HomeDir:  path.Clean(h),
	})

	taskName := fmt.Sprintf("Создание изолированного пользователя сайта: %s", username)
	return s.engine.Submit(serverID, "user_create_site", taskName, string(payload))
}

// SetDirectoryOwner submits a task to change ownership of a site directory.
func (s *Service) SetDirectoryOwner(ctx context.Context, serverID string, dirPath string, username string, group string) (*db.TaskRecord, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	p := strings.TrimSpace(dirPath)
	if err := ssh.ValidateRemotePath(p, false); err != nil {
		return nil, err
	}
	cleanPath := path.Clean(p)

	// Guard against root or system directories.
	if cleanPath == "/" || cleanPath == "/etc" || cleanPath == "/bin" || cleanPath == "/usr" || cleanPath == "/var" {
		return nil, fmt.Errorf("изменение владельца системного каталога %s запрещено", cleanPath)
	}

	grp := strings.TrimSpace(group)
	if grp == "" {
		grp = username
	}
	if err := ssh.ValidateIdent(grp, "group"); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(ChownRequest{
		ServerID: serverID,
		Path:     cleanPath,
		Username: username,
		Group:    grp,
	})

	taskName := fmt.Sprintf("Установка владельца %s:%s для %s", username, grp, cleanPath)
	return s.engine.Submit(serverID, "user_chown_site", taskName, string(payload))
}

// GetAuthorizedKeys reads the user's authorized_keys.
func (s *Service) GetAuthorizedKeys(ctx context.Context, serverID string, username string) ([]AuthorizedKeyItem, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}

	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := fmt.Sprintf("getent passwd %s | cut -d: -f6", username)
	homeOut, err := s.execCmd(ctx, client, cmd, false)
	homeDir := strings.TrimSpace(homeOut)
	if homeDir == "" {
		return nil, fmt.Errorf("пользователь %s не найден", username)
	}

	keyPath := path.Join(homeDir, ".ssh", "authorized_keys")
	out, err := s.execCmd(ctx, client, fmt.Sprintf("cat %s 2>/dev/null || true", keyPath), true)
	if err != nil {
		return nil, err
	}

	var keys []AuthorizedKeyItem
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		keyType := fields[0]
		keyData := fields[1]
		comment := ""
		if len(fields) >= 3 {
			comment = strings.Join(fields[2:], " ")
		}

		keys = append(keys, AuthorizedKeyItem{
			KeyType: keyType,
			KeyData: keyData,
			Comment: comment,
			Raw:     line,
		})
	}

	return keys, nil
}

// AddAuthorizedKey appends a public key to ~/.ssh/authorized_keys with backup.
func (s *Service) AddAuthorizedKey(ctx context.Context, serverID string, username string, pubKey string) (*db.TaskRecord, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	if err := ValidateSSHKey(pubKey); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(AddSSHKeyRequest{
		ServerID: serverID,
		Username: username,
		Key:      strings.TrimSpace(pubKey),
	})

	taskName := fmt.Sprintf("Добавление SSH-ключа для пользователя %s", username)
	return s.engine.Submit(serverID, "user_add_ssh_key", taskName, string(payload))
}

func (s *Service) handleCreateSiteUserTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req CreateSiteUserRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep(fmt.Sprintf("Проверка существования пользователя %s", req.Username), 20)
	checkOut, _ := s.execCmd(ctx, client, fmt.Sprintf("id -u %s 2>/dev/null || true", req.Username), false)
	if strings.TrimSpace(checkOut) != "" {
		return fmt.Sprintf("Пользователь %s уже существует", req.Username), nil
	}

	logger.SetStep(fmt.Sprintf("Создание пользователя %s с каталогом %s (без sudo)", req.Username, req.HomeDir), 60)
	// Create user with bash shell and dedicated home dir, no password, NO sudo!
	safeHome := ssh.ShellEscape(req.HomeDir)
	safeUser := ssh.ShellEscape(req.Username)
	createCmd := fmt.Sprintf("useradd -m -d %s -s /bin/bash %s", safeHome, safeUser)
	out, err := s.execCmd(ctx, client, createCmd, true)
	if err != nil {
		return "", fmt.Errorf("ошибка создания пользователя: %w\n%s", err, out)
	}

	// Create .ssh directory with 700 permissions.
	safeSSHDir := ssh.ShellEscape(path.Join(req.HomeDir, ".ssh"))
	sshDirCmd := fmt.Sprintf("mkdir -p %s && chmod 700 %s && chown %s:%s %s", safeSSHDir, safeSSHDir, safeUser, safeUser, safeSSHDir)
	_, _ = s.execCmd(ctx, client, sshDirCmd, true)

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Пользователь %s успешно создан (каталог: %s)", req.Username, req.HomeDir), nil
}

func (s *Service) handleChownTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req ChownRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}
	if err := ValidateUsername(req.Username); err != nil {
		return "", err
	}
	if err := ssh.ValidateIdent(req.Group, "group"); err != nil {
		return "", err
	}
	if err := ssh.ValidateRemotePath(req.Path, false); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep(fmt.Sprintf("Установка владельца %s:%s для %s", req.Username, req.Group, req.Path), 50)
	cmd := fmt.Sprintf("chown -R %s:%s %s", ssh.ShellEscape(req.Username), ssh.ShellEscape(req.Group), ssh.ShellEscape(req.Path))
	out, err := s.execCmd(ctx, client, cmd, true)
	if err != nil {
		return "", fmt.Errorf("ошибка смены владельца каталога: %w\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Владелец каталога %s успешно изменен на %s:%s", req.Path, req.Username, req.Group), nil
}

func (s *Service) handleAddSSHKeyTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req AddSSHKeyRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}
	if err := ValidateUsername(req.Username); err != nil {
		return "", err
	}
	if err := ValidateSSHKey(req.Key); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Определение домашнего каталога", 20)
	safeUser := ssh.ShellEscape(req.Username)
	homeOut, err := s.execCmd(ctx, client, fmt.Sprintf("getent passwd %s | cut -d: -f6", safeUser), false)
	homeDir := strings.TrimSpace(homeOut)
	if homeDir == "" {
		return "", fmt.Errorf("пользователь %s не найден", req.Username)
	}

	sshDir := path.Join(homeDir, ".ssh")
	keyFile := path.Join(sshDir, "authorized_keys")
	safeSSHDir := ssh.ShellEscape(sshDir)
	safeKeyFile := ssh.ShellEscape(keyFile)

	logger.SetStep("Создание резервной копии authorized_keys", 40)
	ts := time.Now().Format("20060102_150405")
	safeBackup := ssh.ShellEscape(fmt.Sprintf("%s.bak_%s", keyFile, ts))
	backupCmd := fmt.Sprintf("[ -f %s ] && cp %s %s || true", safeKeyFile, safeKeyFile, safeBackup)
	_, _ = s.execCmd(ctx, client, backupCmd, true)

	logger.SetStep("Добавление открытого ключа", 70)
	appendScript := fmt.Sprintf(`mkdir -p %s
chmod 700 %s
touch %s
chmod 600 %s
printf '%%s\n' %s >> %s
chown -R %s:%s %s
`, safeSSHDir, safeSSHDir, safeKeyFile, safeKeyFile, ssh.ShellEscape(req.Key), safeKeyFile, safeUser, safeUser, safeSSHDir)

	out, err := s.execCmd(ctx, client, appendScript, true)
	if err != nil {
		return "", fmt.Errorf("ошибка добавления ключа: %w\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("SSH-ключ успешно добавлен для пользователя %s", req.Username), nil
}

// EnableSFTPChroot submits a task to isolate a user in an SFTP-only chroot jail.
func (s *Service) EnableSFTPChroot(ctx context.Context, serverID, username, chrootDir, writableSubdir string) (*db.TaskRecord, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	if username == "root" {
		return nil, fmt.Errorf("нельзя изолировать пользователя root в SFTP chroot")
	}
	dir := strings.TrimSpace(chrootDir)
	if dir == "" {
		dir = fmt.Sprintf("/var/www/%s", username)
	}
	if err := ssh.ValidateRemotePath(dir, false); err != nil {
		return nil, err
	}
	sub := strings.TrimSpace(writableSubdir)
	if sub != "" {
		if err := ssh.ValidateToken(sub, "writable subdir"); err != nil {
			return nil, err
		}
	}

	payload, _ := json.Marshal(SFTPChrootRequest{
		ServerID:       serverID,
		Username:       username,
		ChrootDir:      path.Clean(dir),
		WritableSubdir: sub,
	})
	return s.engine.Submit(serverID, "user_enable_sftp_chroot", fmt.Sprintf("SFTP-изоляция (chroot) для %s", username), string(payload))
}

// DisableSFTPChroot submits a task to remove SFTP chroot isolation for a user.
func (s *Service) DisableSFTPChroot(ctx context.Context, serverID, username string) (*db.TaskRecord, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(SFTPChrootRequest{
		ServerID: serverID,
		Username: username,
	})
	return s.engine.Submit(serverID, "user_disable_sftp_chroot", fmt.Sprintf("Отключение SFTP-изоляции для %s", username), string(payload))
}

// SetUserPassword submits a task to update a system/SFTP user's password via chpasswd stdin.
func (s *Service) SetUserPassword(ctx context.Context, serverID, username, password string) (*db.TaskRecord, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	if username == "root" {
		return nil, fmt.Errorf("изменение пароля root через модуль пользователей сайтов запрещено")
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("пароль должен содержать минимум 8 символов")
	}
	if strings.ContainsAny(password, "\r\n") {
		return nil, fmt.Errorf("пароль содержит недопустимые символы перевода строки")
	}

	encPass := password
	if s.Vault != nil {
		var err error
		encPass, err = s.Vault.Encrypt(password)
		if err != nil {
			return nil, fmt.Errorf("ошибка шифрования пароля: %w", err)
		}
		encPass = "vault:" + encPass
	}

	payload, _ := json.Marshal(SetUserPasswordRequest{
		ServerID: serverID,
		Username: username,
		Password: encPass,
	})
	return s.engine.Submit(serverID, "user_set_password", fmt.Sprintf("Установка пароля для %s", username), string(payload))
}

func (s *Service) handleEnableSFTPChrootTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req SFTPChrootRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}
	if err := ValidateUsername(req.Username); err != nil {
		return "", err
	}
	if err := ssh.ValidateRemotePath(req.ChrootDir, false); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	safeUser := ssh.ShellEscape(req.Username)
	safeChroot := ssh.ShellEscape(req.ChrootDir)

	logger.SetStep("Настройка прав chroot-каталога (root:root 755)", 25)
	prepCmd := fmt.Sprintf("mkdir -p %s && chown root:root %s && chmod 755 %s", safeChroot, safeChroot, safeChroot)
	if req.WritableSubdir != "" {
		subPath := path.Join(req.ChrootDir, req.WritableSubdir)
		safeSub := ssh.ShellEscape(subPath)
		prepCmd += fmt.Sprintf(" && mkdir -p %s && chown %s:%s %s && chmod 755 %s", safeSub, safeUser, safeUser, safeSub, safeSub)
	}
	// Restrict interactive shell login for chrooted SFTP-only account.
	prepCmd += fmt.Sprintf(" && NOLOGIN=$(command -v nologin || echo /usr/sbin/nologin) && usermod -s \"$NOLOGIN\" %s", safeUser)
	if out, err := s.execCmd(ctx, client, prepCmd, true); err != nil {
		return "", fmt.Errorf("ошибка подготовки каталога/пользователя: %w\n%s", err, out)
	}

	logger.SetStep("Запись конфигурации OpenSSH Match User", 60)
	confPath := fmt.Sprintf("/etc/ssh/sshd_config.d/tamestack_sftp_%s.conf", req.Username)
	confBody := fmt.Sprintf(`# Managed by TameStack: SFTP Chroot for %s
Match User %s
    ChrootDirectory %s
    ForceCommand internal-sftp
    AllowTcpForwarding no
    X11Forwarding no
    PermitTunnel no
`, req.Username, req.Username, req.ChrootDir)

	if err := client.WriteFile(ctx, confPath, []byte(confBody), 0600); err != nil {
		return "", fmt.Errorf("ошибка записи %s: %w", confPath, err)
	}

	logger.SetStep("Проверка конфигурации sshd -t и перезапуск", 85)
	// Ensure sshd_config includes sshd_config.d/*.conf; if sshd -t fails, roll back immediately.
	testAndReload := fmt.Sprintf(`if ! grep -qE '^\s*Include\s+/etc/ssh/sshd_config\.d/\*\.conf' /etc/ssh/sshd_config 2>/dev/null; then
    mkdir -p /etc/ssh/sshd_config.d
    printf '\nInclude /etc/ssh/sshd_config.d/*.conf\n' >> /etc/ssh/sshd_config
fi
if ! sshd -t; then
    rm -f %s
    exit 1
fi
systemctl reload ssh 2>/dev/null || systemctl reload sshd 2>/dev/null || service ssh reload 2>/dev/null || true`, ssh.ShellEscape(confPath))

	if out, err := s.execCmd(ctx, client, testAndReload, true); err != nil {
		return "", fmt.Errorf("ошибка проверки sshd -t (изменения откатаны): %w\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("SFTP-изоляция (chroot: %s) включена для %s", req.ChrootDir, req.Username), nil
}

func (s *Service) handleDisableSFTPChrootTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req SFTPChrootRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}
	if err := ValidateUsername(req.Username); err != nil {
		return "", err
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	confPath := fmt.Sprintf("/etc/ssh/sshd_config.d/tamestack_sftp_%s.conf", req.Username)
	logger.SetStep("Удаление правила SFTP chroot и перезапуск sshd", 60)
	cmd := fmt.Sprintf("rm -f %s && sshd -t && (systemctl reload ssh 2>/dev/null || systemctl reload sshd 2>/dev/null || service ssh reload 2>/dev/null || true)", ssh.ShellEscape(confPath))
	if out, err := s.execCmd(ctx, client, cmd, true); err != nil {
		return "", fmt.Errorf("ошибка отключения SFTP chroot: %w\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("SFTP-изоляция для %s отключена", req.Username), nil
}

func (s *Service) handleSetUserPasswordTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req SetUserPasswordRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", fmt.Errorf("ошибка парсинга payload: %w", err)
	}
	if err := ValidateUsername(req.Username); err != nil {
		return "", err
	}
	pass := req.Password
	if strings.HasPrefix(pass, "vault:") && s.Vault != nil {
		plain, err := s.Vault.Decrypt(strings.TrimPrefix(pass, "vault:"))
		if err != nil {
			return "", fmt.Errorf("ошибка расшифровки пароля: %w", err)
		}
		pass = plain
	}
	if strings.ContainsAny(pass, "\r\n") {
		return "", fmt.Errorf("пароль содержит перевод строки")
	}

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Обновление пароля через chpasswd", 60)
	cmd := client.SudoHeader() + "$SUDO chpasswd\n"
	_, stderr, code, err := client.ExecStdin(ctx, cmd, strings.NewReader(req.Username+":"+pass+"\n"))
	if err != nil || code != 0 {
		return "", fmt.Errorf("ошибка chpasswd: %s %v", strings.TrimSpace(string(stderr)), err)
	}

	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Пароль для %s успешно обновлён", req.Username), nil
}
