package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/i18n"
	"tamestack/internal/module"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateEmail checks email syntax and rejects shell/newline metacharacters.
func ValidateEmail(email string) error {
	e := strings.TrimSpace(email)
	if !emailRegex.MatchString(e) {
		return i18n.Err("mail.invalidEmail", i18n.Args{"email": email})
	}
	return nil
}

// Service manages Postfix, Dovecot, OpenDKIM, virtual mail domains, mailboxes, and aliases.
type Service struct {
	module.Base
	engine *tasks.Engine
}

// NewService constructs a mail management service and registers task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}
	engine.RegisterHandler("mail_install_stack", s.handleInstallStackTask)
	engine.RegisterHandler("mail_add_domain", s.handleAddDomainTask)
	engine.RegisterHandler("mail_create_mailbox", s.handleCreateMailboxTask)
	engine.RegisterHandler("mail_delete_mailbox", s.handleDeleteMailboxTask)
	engine.RegisterHandler("mail_save_alias", s.handleSaveAliasTask)
	engine.RegisterHandler("mail_delete_alias", s.handleDeleteAliasTask)
	return s
}

func (s *Service) execCmd(ctx context.Context, client *ssh.SSHClient, cmd string, useSudo bool) (string, error) {
	return module.Exec(ctx, client, cmd, useSudo)
}

// GetMailStatus inspects Postfix, Dovecot, OpenDKIM, virtual domains, mailboxes, and aliases.
func (s *Service) GetMailStatus(ctx context.Context, serverID string) (*MailStatus, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	script := `echo "===STATUS==="
command -v postfix >/dev/null 2>&1 && echo "POSTFIX_BIN=1" || echo "POSTFIX_BIN=0"
systemctl is-active postfix 2>/dev/null || echo "inactive"
command -v dovecot >/dev/null 2>&1 && echo "DOVECOT_BIN=1" || echo "DOVECOT_BIN=0"
systemctl is-active dovecot 2>/dev/null || echo "inactive"
command -v opendkim >/dev/null 2>&1 && echo "DKIM_BIN=1" || echo "DKIM_BIN=0"
systemctl is-active opendkim 2>/dev/null || echo "inactive"
hostname -f 2>/dev/null || hostname
echo "===DOMAINS==="
cat /etc/postfix/virtual_domains 2>/dev/null || true
echo "===MAILBOXES==="
cat /etc/postfix/virtual_mailboxes 2>/dev/null || true
echo "===ALIASES==="
cat /etc/postfix/virtual_aliases 2>/dev/null || true
`
	out, err := s.execCmd(ctx, client, script, true)
	if err != nil {
		return nil, err
	}

	res := &MailStatus{}
	section := ""
	var domainNames []string

	for _, rawLine := range strings.Split(out, "\n") {
		line := strings.TrimSpace(rawLine)
		switch line {
		case "===STATUS===", "===DOMAINS===", "===MAILBOXES===", "===ALIASES===":
			section = line
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch section {
		case "===STATUS===":
			switch {
			case line == "POSTFIX_BIN=1":
				res.PostfixInstalled = true
			case line == "DOVECOT_BIN=1":
				res.DovecotInstalled = true
			case line == "DKIM_BIN=1":
				res.OpenDKIMInstalled = true
			case line == "active":
				if !res.PostfixActive && res.PostfixInstalled && !res.DovecotInstalled {
					res.PostfixActive = true
				}
			}
		case "===DOMAINS===":
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				domainNames = append(domainNames, fields[0])
			}
		case "===MAILBOXES===":
			fields := strings.Fields(line)
			if len(fields) >= 2 && strings.Contains(fields[0], "@") {
				parts := strings.SplitN(fields[0], "@", 2)
				res.Mailboxes = append(res.Mailboxes, MailboxItem{
					Email:     fields[0],
					LocalPart: parts[0],
					Domain:    parts[1],
					Maildir:   fields[1],
				})
			}
		case "===ALIASES===":
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				res.Aliases = append(res.Aliases, MailAlias{
					Source:      fields[0],
					Destination: strings.Join(fields[1:], " "),
				})
			}
		}
	}

	// Parse exact service status lines reliably.
	if idx := strings.Index(out, "===STATUS==="); idx != -1 {
		end := strings.Index(out, "===DOMAINS===")
		if end > idx {
			statusLines := strings.Split(strings.TrimSpace(out[idx+len("===STATUS==="):end]), "\n")
			if len(statusLines) >= 7 {
				res.PostfixActive = strings.TrimSpace(statusLines[1]) == "active"
				res.DovecotActive = strings.TrimSpace(statusLines[3]) == "active"
				res.OpenDKIMActive = strings.TrimSpace(statusLines[5]) == "active"
				res.Hostname = strings.TrimSpace(statusLines[6])
			}
		}
	}

	for _, dom := range domainNames {
		dkimKey := ""
		if b, err := client.ReadFile(ctx, fmt.Sprintf("/etc/opendkim/keys/%s/mail.txt", dom)); err == nil {
			dkimKey = strings.TrimSpace(string(b))
		}
		res.Domains = append(res.Domains, MailDomain{
			Domain:       dom,
			DKIMSelector: "mail",
			DKIMRecord:   dkimKey,
			SPFRecord:    fmt.Sprintf("v=spf1 mx a ~all"),
			DMARCRecord:  fmt.Sprintf("v=DMARC1; p=quarantine; rua=mailto:postmaster@%s", dom),
		})
	}

	return res, nil
}

// InstallMailStack submits a task to install and configure Postfix + Dovecot + OpenDKIM.
func (s *Service) InstallMailStack(ctx context.Context, serverID, hostname string) (*db.TaskRecord, error) {
	h := strings.TrimSpace(hostname)
	if h != "" {
		if err := ssh.ValidateHostToken(h); err != nil {
			return nil, fmt.Errorf("недопустимый hostname: %w", err)
		}
	}
	payload, _ := json.Marshal(InstallMailStackRequest{ServerID: serverID, Hostname: h})
	return s.engine.Submit(serverID, "mail_install_stack", "Установка почтового сервера (Postfix + Dovecot + OpenDKIM)", string(payload))
}

// AddMailDomain submits a task to register a virtual mail domain and generate its DKIM key.
func (s *Service) AddMailDomain(ctx context.Context, serverID, domain string) (*db.TaskRecord, error) {
	d := strings.TrimSpace(strings.ToLower(domain))
	if err := ssh.ValidateHostToken(d); err != nil {
		return nil, i18n.Errf("mail.invalidDomain", nil, err)
	}
	payload, _ := json.Marshal(MailDomainRequest{ServerID: serverID, Domain: d})
	return s.engine.Submit(serverID, "mail_add_domain", i18n.Tr("mail.taskAddDomainTitle", i18n.Args{"domain": d}), string(payload))
}

// CreateMailbox submits a task to create or update a virtual mailbox.
func (s *Service) CreateMailbox(ctx context.Context, serverID, email, password string, quotaMB int) (*db.TaskRecord, error) {
	e := strings.TrimSpace(strings.ToLower(email))
	if err := ValidateEmail(e); err != nil {
		return nil, err
	}
	if len(password) < 8 {
		return nil, i18n.Err("mail.passMinLength")
	}
	if strings.ContainsAny(password, "\r\n") {
		return nil, i18n.Err("mail.passNewline")
	}

	encPass := password
	if s.Vault != nil {
		var err error
		encPass, err = s.Vault.Encrypt(password)
		if err != nil {
			return nil, err
		}
		encPass = "vault:" + encPass
	}

	payload, _ := json.Marshal(CreateMailboxRequest{
		ServerID: serverID,
		Email:    e,
		Password: encPass,
		QuotaMB:  quotaMB,
	})
	return s.engine.Submit(serverID, "mail_create_mailbox", i18n.Tr("mail.taskCreateMailboxTitle", i18n.Args{"email": e}), string(payload))
}

// DeleteMailbox submits a task to delete a virtual mailbox entry.
func (s *Service) DeleteMailbox(ctx context.Context, serverID, email string) (*db.TaskRecord, error) {
	e := strings.TrimSpace(strings.ToLower(email))
	if err := ValidateEmail(e); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(DeleteMailboxRequest{ServerID: serverID, Email: e})
	return s.engine.Submit(serverID, "mail_delete_mailbox", i18n.Tr("mail.taskDeleteMailboxTitle", i18n.Args{"email": e}), string(payload))
}

// SaveMailAlias submits a task to add or update a Postfix virtual alias.
func (s *Service) SaveMailAlias(ctx context.Context, serverID, source, destination string) (*db.TaskRecord, error) {
	src := strings.TrimSpace(strings.ToLower(source))
	dst := strings.TrimSpace(strings.ToLower(destination))
	if err := ValidateEmail(src); err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	if err := ValidateEmail(dst); err != nil {
		return nil, fmt.Errorf("destination: %w", err)
	}
	payload, _ := json.Marshal(MailAliasRequest{ServerID: serverID, Source: src, Destination: dst})
	return s.engine.Submit(serverID, "mail_save_alias", i18n.Tr("mail.taskSaveAliasTitle", i18n.Args{"source": src, "dest": dst}), string(payload))
}

// DeleteMailAlias submits a task to remove a Postfix virtual alias.
func (s *Service) DeleteMailAlias(ctx context.Context, serverID, source string) (*db.TaskRecord, error) {
	src := strings.TrimSpace(strings.ToLower(source))
	if err := ValidateEmail(src); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(MailAliasRequest{ServerID: serverID, Source: src})
	return s.engine.Submit(serverID, "mail_delete_alias", i18n.Tr("mail.taskDeleteAliasTitle", i18n.Args{"source": src}), string(payload))
}

func (s *Service) handleInstallStackTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req InstallMailStackRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Установка пакетов Postfix, Dovecot и OpenDKIM", 25)
	installScript := `if command -v apt-get >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq postfix dovecot-core dovecot-imapd dovecot-pop3d dovecot-lmtpd opendkim opendkim-tools
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q postfix dovecot opendkim opendkim-tools
else
    echo "Unsupported package manager" >&2; exit 1
fi`
	if out, err := s.execCmd(ctx, client, installScript, true); err != nil {
		return "", fmt.Errorf("ошибка установки почтовых пакетов: %w\n%s", err, out)
	}

	logger.SetStep("Создание пользователя vmail и таблиц виртуальных доменов", 50)
	hostnameArg := req.Hostname
	if hostnameArg == "" {
		hostnameArg = "$(hostname -f 2>/dev/null || hostname)"
	} else {
		hostnameArg = ssh.ShellEscape(hostnameArg)
	}
	setupScript := fmt.Sprintf(`groupadd -g 5000 vmail 2>/dev/null || true
useradd -u 5000 -g vmail -s /usr/sbin/nologin -d /var/mail/vhosts -m vmail 2>/dev/null || true
mkdir -p /var/mail/vhosts /etc/opendkim/keys
chown -R vmail:vmail /var/mail/vhosts
touch /etc/postfix/virtual_domains /etc/postfix/virtual_mailboxes /etc/postfix/virtual_aliases /etc/dovecot/users
touch /etc/opendkim/KeyTable /etc/opendkim/SigningTable /etc/opendkim/TrustedHosts
chmod 640 /etc/dovecot/users
chown root:dovecot /etc/dovecot/users 2>/dev/null || true
postmap /etc/postfix/virtual_domains /etc/postfix/virtual_mailboxes /etc/postfix/virtual_aliases
MAIL_HOST=%s
postconf -e "myhostname = $MAIL_HOST"
postconf -e "virtual_mailbox_domains = /etc/postfix/virtual_domains"
postconf -e "virtual_mailbox_base = /var/mail/vhosts"
postconf -e "virtual_mailbox_maps = hash:/etc/postfix/virtual_mailboxes"
postconf -e "virtual_alias_maps = hash:/etc/postfix/virtual_aliases"
postconf -e "virtual_minimum_uid = 5000"
postconf -e "virtual_uid_maps = static:5000"
postconf -e "virtual_gid_maps = static:5000"
postconf -e "virtual_transport = lmtp:unix:private/dovecot-lmtp"
postconf -e "smtpd_sasl_type = dovecot"
postconf -e "smtpd_sasl_path = private/auth"
postconf -e "smtpd_sasl_auth_enable = yes"
postconf -e "milter_protocol = 6"
postconf -e "milter_default_action = accept"
postconf -e "smtpd_milters = inet:localhost:8891"
postconf -e "non_smtpd_milters = inet:localhost:8891"
`, hostnameArg)

	if out, err := s.execCmd(ctx, client, setupScript, true); err != nil {
		return "", fmt.Errorf("ошибка настройки Postfix: %w\n%s", err, out)
	}

	logger.SetStep("Настройка Dovecot LMTP/Auth и OpenDKIM", 75)
	dovecotCfg := `# Managed by TameStack
protocols = imap pop3 lmtp
mail_location = maildir:/var/mail/vhosts/%d/%n
auth_mechanisms = plain login
passdb {
  driver = passwd-file
  args = scheme=SHA512-CRYPT username_format=%u /etc/dovecot/users
}
userdb {
  driver = static
  args = uid=vmail gid=vmail home=/var/mail/vhosts/%d/%n
}
service lmtp {
  unix_listener /var/spool/postfix/private/dovecot-lmtp {
    mode = 0600
    user = postfix
    group = postfix
  }
}
service auth {
  unix_listener /var/spool/postfix/private/auth {
    mode = 0666
    user = postfix
    group = postfix
  }
}
`
	if err := client.WriteFile(ctx, "/etc/dovecot/conf.d/99-tamestack.conf", []byte(dovecotCfg), 0644); err != nil {
		return "", fmt.Errorf("ошибка записи конфигурации Dovecot: %w", err)
	}

	opendkimCfg := `# Managed by TameStack
Syslog                  yes
UMask                   002
Canonicalization        relaxed/simple
Mode                    sv
SubDomains              no
AutoRestart             yes
Background              yes
DNSTimeout              5
SignatureAlgorithm      rsa-sha256
Socket                  inet:8891@localhost
PidFile                 /run/opendkim/opendkim.pid
UserID                  opendkim
KeyTable                refile:/etc/opendkim/KeyTable
SigningTable            refile:/etc/opendkim/SigningTable
ExternalIgnoreList      /etc/opendkim/TrustedHosts
InternalHosts           /etc/opendkim/TrustedHosts
`
	if err := client.WriteFile(ctx, "/etc/opendkim.conf", []byte(opendkimCfg), 0644); err != nil {
		return "", fmt.Errorf("ошибка записи /etc/opendkim.conf: %w", err)
	}

	logger.SetStep("Запуск служб Postfix, Dovecot и OpenDKIM", 90)
	startCmd := `printf '127.0.0.1\nlocalhost\n' > /etc/opendkim/TrustedHosts
chown -R opendkim:opendkim /etc/opendkim 2>/dev/null || true
systemctl enable postfix dovecot opendkim 2>/dev/null || true
systemctl restart opendkim postfix dovecot`
	if out, err := s.execCmd(ctx, client, startCmd, true); err != nil {
		return "", fmt.Errorf("ошибка запуска почтовых служб: %w\n%s", err, out)
	}

	logger.SetStep("Готово", 100)
	return "Почтовый сервер (Postfix + Dovecot + OpenDKIM) установлен и настроен", nil
}

func (s *Service) handleAddDomainTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req MailDomainRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ssh.ValidateHostToken(req.Domain); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	safeDom := ssh.ShellEscape(req.Domain)
	logger.SetStep("Добавление домена в Postfix и генерация DKIM-ключа", 50)
	script := fmt.Sprintf(`touch /etc/postfix/virtual_domains
if ! grep -qxF %s /etc/postfix/virtual_domains; then
    echo %s >> /etc/postfix/virtual_domains
fi
mkdir -p /etc/opendkim/keys/%s
if [ ! -f /etc/opendkim/keys/%s/mail.private ]; then
    opendkim-genkey -b 2048 -d %s -D /etc/opendkim/keys/%s -s mail -v
fi
chown -R opendkim:opendkim /etc/opendkim/keys/%s 2>/dev/null || true
if ! grep -q "mail._domainkey.%s" /etc/opendkim/KeyTable 2>/dev/null; then
    printf 'mail._domainkey.%s %s:mail:/etc/opendkim/keys/%s/mail.private\n' >> /etc/opendkim/KeyTable
fi
if ! grep -q "*@%s" /etc/opendkim/SigningTable 2>/dev/null; then
    printf '*@%s mail._domainkey.%s\n' >> /etc/opendkim/SigningTable
fi
systemctl reload opendkim 2>/dev/null || systemctl restart opendkim 2>/dev/null || true
systemctl reload postfix 2>/dev/null || systemctl restart postfix`,
		safeDom, safeDom, req.Domain, req.Domain, safeDom, req.Domain, req.Domain,
		req.Domain, req.Domain, req.Domain, req.Domain,
		req.Domain, req.Domain, req.Domain)

	if out, err := s.execCmd(ctx, client, script, true); err != nil {
		return "", fmt.Errorf("ошибка настройки почтового домена %s: %w\n%s", req.Domain, err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Почтовый домен %s добавлен, DKIM-ключ сгенерирован", req.Domain), nil
}

func (s *Service) handleCreateMailboxTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req CreateMailboxRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ValidateEmail(req.Email); err != nil {
		return "", err
	}
	pass := req.Password
	if strings.HasPrefix(pass, "vault:") && s.Vault != nil {
		plain, err := s.Vault.Decrypt(strings.TrimPrefix(pass, "vault:"))
		if err != nil {
			return "", err
		}
		pass = plain
	}
	parts := strings.SplitN(req.Email, "@", 2)
	localPart, domain := parts[0], parts[1]

	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Хеширование пароля через doveadm", 30)
	hashCmd := client.SudoHeader() + "doveadm pw -s SHA512-CRYPT\n"
	outHash, stderr, code, err := client.ExecStdin(ctx, hashCmd, strings.NewReader(pass+"\n"+pass+"\n"))
	if err != nil || code != 0 {
		return "", fmt.Errorf("ошибка генерации хеша пароля doveadm: %s %v", strings.TrimSpace(string(stderr)), err)
	}
	lines := strings.Split(strings.TrimSpace(string(outHash)), "\n")
	pwHash := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(pwHash, "{SHA512-CRYPT}") {
		return "", fmt.Errorf("неожиданный формат хеша doveadm")
	}

	logger.SetStep("Обновление /etc/dovecot/users и /etc/postfix/virtual_mailboxes", 70)
	maildir := fmt.Sprintf("%s/%s/", domain, localPart)
	safeEmail := ssh.ShellEscape(req.Email)
	userLine := fmt.Sprintf("%s:%s", req.Email, pwHash)
	mboxLine := fmt.Sprintf("%s %s", req.Email, maildir)

	script := fmt.Sprintf(`touch /etc/dovecot/users /etc/postfix/virtual_mailboxes
grep -v "^%s:" /etc/dovecot/users > /etc/dovecot/users.tmp || true
printf '%%s\n' %s >> /etc/dovecot/users.tmp
mv /etc/dovecot/users.tmp /etc/dovecot/users
chmod 640 /etc/dovecot/users
chown root:dovecot /etc/dovecot/users 2>/dev/null || true
grep -v "^%s " /etc/postfix/virtual_mailboxes > /etc/postfix/virtual_mailboxes.tmp || true
printf '%%s\n' %s >> /etc/postfix/virtual_mailboxes.tmp
mv /etc/postfix/virtual_mailboxes.tmp /etc/postfix/virtual_mailboxes
mkdir -p /var/mail/vhosts/%s/%s
chown -R vmail:vmail /var/mail/vhosts/%s
postmap /etc/postfix/virtual_mailboxes
systemctl reload postfix 2>/dev/null || true`,
		req.Email, ssh.ShellEscape(userLine),
		req.Email, ssh.ShellEscape(mboxLine),
		domain, localPart, domain)
	_ = safeEmail

	if out, err := s.execCmd(ctx, client, script, true); err != nil {
		return "", fmt.Errorf("ошибка создания ящика %s: %w\n%s", req.Email, err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Почтовый ящик %s создан", req.Email), nil
}

func (s *Service) handleDeleteMailboxTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req DeleteMailboxRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ValidateEmail(req.Email); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Удаление ящика из Postfix и Dovecot", 60)
	script := fmt.Sprintf(`grep -v "^%s:" /etc/dovecot/users > /etc/dovecot/users.tmp 2>/dev/null || true
mv /etc/dovecot/users.tmp /etc/dovecot/users 2>/dev/null || true
grep -v "^%s " /etc/postfix/virtual_mailboxes > /etc/postfix/virtual_mailboxes.tmp 2>/dev/null || true
mv /etc/postfix/virtual_mailboxes.tmp /etc/postfix/virtual_mailboxes 2>/dev/null || true
postmap /etc/postfix/virtual_mailboxes
systemctl reload postfix 2>/dev/null || true`, req.Email, req.Email)
	if out, err := s.execCmd(ctx, client, script, true); err != nil {
		return "", fmt.Errorf("ошибка удаления ящика %s: %w\n%s", req.Email, err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Почтовый ящик %s удалён", req.Email), nil
}

func (s *Service) handleSaveAliasTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req MailAliasRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ValidateEmail(req.Source); err != nil {
		return "", err
	}
	if err := ValidateEmail(req.Destination); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Сохранение виртуального алиаса", 60)
	line := fmt.Sprintf("%s %s", req.Source, req.Destination)
	script := fmt.Sprintf(`touch /etc/postfix/virtual_aliases
grep -v "^%s " /etc/postfix/virtual_aliases > /etc/postfix/virtual_aliases.tmp || true
printf '%%s\n' %s >> /etc/postfix/virtual_aliases.tmp
mv /etc/postfix/virtual_aliases.tmp /etc/postfix/virtual_aliases
postmap /etc/postfix/virtual_aliases
systemctl reload postfix 2>/dev/null || true`, req.Source, ssh.ShellEscape(line))
	if out, err := s.execCmd(ctx, client, script, true); err != nil {
		return "", fmt.Errorf("ошибка сохранения алиаса: %w\n%s", err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Алиас %s -> %s сохранён", req.Source, req.Destination), nil
}

func (s *Service) handleDeleteAliasTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var req MailAliasRequest
	if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
		return "", err
	}
	if err := ValidateEmail(req.Source); err != nil {
		return "", err
	}
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep("Удаление виртуального алиаса", 60)
	script := fmt.Sprintf(`grep -v "^%s " /etc/postfix/virtual_aliases > /etc/postfix/virtual_aliases.tmp 2>/dev/null || true
mv /etc/postfix/virtual_aliases.tmp /etc/postfix/virtual_aliases 2>/dev/null || true
postmap /etc/postfix/virtual_aliases
systemctl reload postfix 2>/dev/null || true`, req.Source)
	if out, err := s.execCmd(ctx, client, script, true); err != nil {
		return "", fmt.Errorf("ошибка удаления алиаса: %w\n%s", err, out)
	}
	logger.SetStep("Готово", 100)
	return fmt.Sprintf("Алиас %s удалён", req.Source), nil
}
