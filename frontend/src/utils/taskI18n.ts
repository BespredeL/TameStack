import { TaskRecord } from '../types';

type TFunc = (path: string, params?: Record<string, string | number>) => string;

const parsePayload = (task: TaskRecord): Record<string, any> => {
  if (!task.payload) return {};
  try {
    const parsed = JSON.parse(task.payload);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
};

const str = (v: any): string => (v == null ? '' : String(v));

const ACTIONS = [
  'start', 'stop', 'restart', 'reload', 'enable', 'disable',
  'remove', 'kill', 'up', 'down', 'status', 'pause', 'unpause', 'pull',
];

const actionWord = (action: string, t: TFunc): string => {
  const a = (action || '').toLowerCase();
  return ACTIONS.includes(a) ? t(`tasks.action.${a}`) : action || '';
};

const packagesLabel = (pkgs: any): string => {
  const list: string[] = Array.isArray(pkgs) ? pkgs.map((x) => String(x)) : [];
  if (list.length === 0) return '';
  const joined = list.join(', ');
  return joined.length > 60 ? joined.slice(0, 57) + '…' : joined;
};

// Reconstructs a task title in the active UI language from operation_type +
// payload. Falls back to the stored (backend-generated) title when the operation
// is unknown or required payload fields are missing (e.g. historical site tasks
// recorded before `domain` was added to the payload).
export const localizeTaskTitle = (task: TaskRecord, t: TFunc): string => {
  const p = parsePayload(task);
  const fallback = task.title;
  const siteTitle = (key: string, params: Record<string, string | number>): string =>
    p.domain ? t(`tasks.op.${key}`, params) : t(`tasks.op.${key}_generic`);

  switch (task.operation_type) {
    case 'php_ini_save':
      return t('tasks.op.php_ini_save', { version: str(p.version) });
    case 'php_fpm_restart':
      return t('tasks.op.php_fpm_restart', { version: str(p.version) });
    case 'php_fpm_toggle':
      return t(p.enable ? 'tasks.op.php_fpm_toggle_start' : 'tasks.op.php_fpm_toggle_stop', {
        version: str(p.version),
      });
    case 'php_install':
      return t('tasks.op.php_install', { version: str(p.version) });
    case 'php_ext_install':
      return t('tasks.op.php_ext_install', { version: str(p.version) });
    case 'php_ext_toggle':
      return t(p.enable ? 'tasks.op.php_ext_toggle_enable' : 'tasks.op.php_ext_toggle_disable', {
        version: str(p.version),
        extension: str(p.extension),
      });

    case 'db_install':
      return t('tasks.op.db_install', { engine: str(p.engine) });
    case 'db_create':
      return t('tasks.op.db_create', { engine: str(p.engine), name: str(p.name) });
    case 'db_delete':
      return t('tasks.op.db_delete', { engine: str(p.engine), name: str(p.name) });
    case 'db_user_create':
      return t('tasks.op.db_user_create', { engine: str(p.engine), username: str(p.username) });
    case 'db_user_delete':
      return t('tasks.op.db_user_delete', { engine: str(p.engine), username: str(p.username) });
    case 'db_backup':
      return t('tasks.op.db_backup', { engine: str(p.engine), name: str(p.database_name) });
    case 'db_restore':
      return t('tasks.op.db_restore', { engine: str(p.engine), name: str(p.database_name) });
    case 'db_service_control':
      return t('tasks.op.db_service_control', { engine: str(p.engine), action: actionWord(str(p.action), t) });

    case 'backup_create':
      return p.target_name
        ? t('tasks.op.backup_create', { name: str(p.target_name) })
        : t('tasks.op.backup_create_generic');
    case 'backup_restore':
      return p.target_name
        ? t('tasks.op.backup_restore', { name: str(p.target_name) })
        : t('tasks.op.backup_restore_generic');
    case 'backup_delete':
      return t('tasks.op.backup_delete');

    case 'inventory_snapshot':
      return t('tasks.op.inventory_snapshot');
    case 'system_diag':
      return t('tasks.op.system_diag');

    case 'site_create':
      return p.domain ? t('tasks.op.site_create', { domain: str(p.domain) }) : t('tasks.op.site_create_generic');
    case 'site_blueprint': {
      const domain = str(p.spec?.domain);
      return domain ? t('tasks.op.site_blueprint', { domain }) : t('tasks.op.site_blueprint_generic');
    }
    case 'site_delete':
      return siteTitle('site_delete', { domain: str(p.domain) });
    case 'site_php_switch':
      return siteTitle('site_php_switch', { domain: str(p.domain), version: str(p.target_php_version) });
    case 'site_config_save':
      return siteTitle('site_config_save', { domain: str(p.domain) });
    case 'site_settings_update':
      return siteTitle('site_settings_update', { domain: str(p.domain) });
    case 'site_status_toggle':
      return siteTitle(p.enable ? 'site_status_toggle_enable' : 'site_status_toggle_disable', { domain: str(p.domain) });
    case 'site_file_quarantine':
      return t('tasks.op.site_file_quarantine', { path: str(p.path) });

    case 'webserver_install':
      return t('tasks.op.webserver_install', { type: str(p.webserver_type) });
    case 'webserver_control':
      return t('tasks.op.webserver_control', { type: str(p.webserver_type), action: actionWord(str(p.action), t) });

    case 'service_control':
      return t('tasks.op.service_control', { unit: str(p.unit_name), action: actionWord(str(p.action), t) });
    case 'process_kill':
      return t('tasks.op.process_kill', { pid: str(p.pid) });

    case 'package_install':
      return t('tasks.op.package_install', { packages: packagesLabel(p.packages) });
    case 'package_remove':
      return t('tasks.op.package_remove', { packages: packagesLabel(p.packages) });
    case 'package_upgrade': {
      const label = packagesLabel(p.packages);
      return label ? t('tasks.op.package_upgrade', { packages: label }) : t('tasks.op.package_upgrade_all');
    }

    case 'user_create_site':
      return t('tasks.op.user_create_site', { username: str(p.username) });
    case 'user_chown_site':
      return t('tasks.op.user_chown_site', { path: str(p.path) });
    case 'user_add_ssh_key':
      return t('tasks.op.user_add_ssh_key', { username: str(p.username) });

    case 'security_firewall_install':
      return t('tasks.op.security_firewall_install', { type: str(p.firewall_type) });
    case 'security_firewall_toggle':
      return t(p.enable ? 'tasks.op.security_firewall_toggle_enable' : 'tasks.op.security_firewall_toggle_disable');
    case 'security_firewall_add_rule':
      return t('tasks.op.security_firewall_add_rule', { port: str(p.port), protocol: str(p.protocol) });
    case 'security_firewall_update_rule':
      return t('tasks.op.security_firewall_update_rule', { port: str(p.port), protocol: str(p.protocol) });
    case 'security_firewall_del_rule':
      return t('tasks.op.security_firewall_del_rule', { port: str(p.port), protocol: str(p.proto) });
    case 'security_fail2ban_install':
      return t('tasks.op.security_fail2ban_install');
    case 'security_fail2ban_control':
      return t('tasks.op.security_fail2ban_control', { action: actionWord(str(p.action), t) });
    case 'security_fail2ban_ban':
      return t('tasks.op.security_fail2ban_ban', { ip: str(p.ip) });
    case 'security_fail2ban_unban':
      return t('tasks.op.security_fail2ban_unban', { ip: str(p.ip) });
    case 'security_fail2ban_save_jail':
      return t('tasks.op.security_fail2ban_save_jail', { name: str(p.name) });
    case 'security_fail2ban_delete_jail':
      return t('tasks.op.security_fail2ban_delete_jail', { name: str(p.name) });
    case 'security_fail2ban_toggle_jail':
      return t('tasks.op.security_fail2ban_toggle_jail', { name: str(p.name) });
    case 'security_apply_hardening': {
      const fix = str(p.fix_type);
      const known = ['ssh_root_login', 'ssh_password_auth', 'security_updates', 'firewall_active', 'fail2ban_active'];
      return known.includes(fix)
        ? t('tasks.op.security_apply_hardening', { fix: t(`tasks.hardening.${fix}`) })
        : fallback;
    }
    case 'security_repair_web_access':
      return t('tasks.op.security_repair_web_access');

    case 'ssl_issue':
      return t('tasks.op.ssl_issue', { domain: str(p.primary_domain) });
    case 'ssl_renew':
      return t('tasks.op.ssl_renew', { domain: str(p.primary_domain) });
    case 'ssl_install_certbot':
      return t('tasks.op.ssl_install_certbot');

    case 'docker_container_action':
      return t('tasks.op.docker_container_action', { id: str(p.container_id), action: actionWord(str(p.action), t) });
    case 'docker_image_pull':
      return t('tasks.op.docker_image_pull', { image: str(p.image) });
    case 'docker_compose_action':
      return t('tasks.op.docker_compose_action', { action: actionWord(str(p.action), t) });
    case 'docker_prune':
      return t('tasks.op.docker_prune', { type: str(p.prune_type) });

    case 'files_disk_cleanup':
      return t('tasks.op.files_disk_cleanup', { category: str(p.category_id) });

    default:
      return fallback;
  }
};

// Exact-match dictionary: raw backend step string (RU / EN) -> i18n key suffix.
const STATIC_STEPS: Record<string, string> = {
  'Подключение к серверу по SSH': 'connect',
  'Подключение к удаленному серверу по SSH': 'connect',
  'Подключение по SSH': 'connect',
  'Connecting to server': 'connect',
  'Connecting to server via SSH': 'connect',
  'Обновление кэша репозиториев': 'update_index',
  'Обновление списков пакетов': 'update_index',
  'Updating package index': 'update_index',
  'Готово': 'completed',
  'Завершено': 'completed',
  'Completed': 'completed',
  'Установка завершена': 'install_done',
  'PHP installation successfully completed': 'install_done',
  'Diagnostics completed': 'diag_done',
  'Renewal completed': 'renew_done',
  'Blueprint applied successfully': 'blueprint_done',
  'SSL certificate issued successfully': 'ssl_issued',
  'Website successfully provisioned and online': 'site_provisioned',
  'Проверка свободного места на диске': 'check_disk',
  'Checking free disk space': 'check_disk',
  'Проверка конфигурации': 'check_config',
  'Проверка установки': 'check_install',
  'Проверка статуса': 'check_status',
  'Проверка статуса службы': 'check_status',
  'Инициализация кластера PostgreSQL': 'pg_init',
  'Подключение репозитория EPEL': 'epel',
  'Включение UFW': 'ufw_enable',
  'Запуск службы firewalld': 'firewalld_start',
  'Проверка типа брандмауэра': 'fw_detect',
  'Включение UFW брандмауэра': 'ufw_on',
  'Отключение UFW брандмауэра': 'ufw_off',
  'Запуск firewalld службы': 'firewalld_on',
  'Остановка firewalld службы': 'firewalld_off',
  'Формирование команды брандмауэра': 'fw_cmd',
  'Применение правила UFW': 'fw_apply_ufw',
  'Применение правила firewalld': 'fw_apply_firewalld',
  'Правило успешно добавлено': 'fw_rule_added',
  'Удаление правила': 'fw_rule_del',
  'Правило удалено': 'fw_rule_deleted',
  'Анализ состояния Fail2ban': 'f2b_analyze',
  'Применение команды': 'apply_cmd',
  'Блокировка IP': 'ban_ip',
  'Разблокировка IP': 'unban_ip',
  'Определение пакетного менеджера': 'pkg_mgr_detect',
  'Установка Fail2ban через APT': 'f2b_apt',
  'Установка Fail2ban через DNF/YUM': 'f2b_dnf',
  'Создание базовой конфигурации jail.local': 'f2b_jail',
  'Запуск и включение службы Fail2ban': 'f2b_start',
  'Создание резервной копии sshd_config': 'sshd_backup',
  'Применение PermitRootLogin no': 'permit_root',
  'Применение PasswordAuthentication no': 'password_auth',
  'Проверка синтаксиса sshd и перезагрузка': 'sshd_check',
  'Установка обновлений безопасности': 'sec_updates',
  'Включение брандмауэра с сохранением SSH порта': 'fw_ssh_preserve',
  'Включение автообновления (certbot.timer)': 'certbot_timer',
  'Разблокировка портов 80 и 443 в брандмауэре': 'ports_80_443',
  'Снятие возможных блокировок Fail2ban': 'f2b_unban_all',
  'Диагностика и запуск веб-серверов': 'web_diag',
  'Определение домашнего каталога': 'home_detect',
  'Создание резервной копии authorized_keys': 'authkeys_backup',
  'Добавление открытого ключа': 'add_pubkey',
  'Разрешение символьных ссылок и повторная проверка политикой': 'symlink_resolve',
  'Полное обновление системы': 'full_upgrade',
  'Checking and configuring repositories': 'check_repos',
  'Preparing package dependencies': 'prep_deps',
  'Enabling and starting PHP-FPM systemd service': 'fpm_enable',
  'Enabling extensions': 'enable_ext',
  'Reloading PHP-FPM': 'fpm_reload',
  'Detecting Operating System distribution': 'detect_os',
  'Collecting hardware metrics': 'hw_metrics',
  'Scanning installed software & services': 'scan_software',
  'Connecting to server and checking Certbot installation': 'certbot_check',
  'Verifying generated certificate files and expiration': 'cert_verify',
  'Binding SSL certificate to Apache VirtualHost': 'ssl_bind_apache',
  'Binding SSL certificate to Nginx VirtualHost': 'ssl_bind_nginx',
  'Saving certificate record in database': 'cert_save_db',
  'Reloading web servers': 'web_reload',
  'Connecting to server and checking web server availability': 'web_check',
  'Creating DocumentRoot directory': 'docroot_create',
  'Configuring filesystem permissions and ownership': 'fs_perms',
  'Placing default welcome index file': 'index_file',
  'Provisioning MySQL/MariaDB database and user': 'db_provision',
  'Preparing work directory and permissions': 'workdir_prep',
  'Configuring systemd service unit': 'systemd_unit',
  'Configuring Nginx Reverse Proxy': 'nginx_proxy',
};

interface DynamicStep {
  re: RegExp;
  key: string;
  build: (m: RegExpMatchArray, t: TFunc) => Record<string, string | number>;
}

const DYNAMIC_STEPS: DynamicStep[] = [
  { re: /^Включение и запуск службы (.+)$/, key: 'svc_enable_start', build: (m) => ({ service: m[1] }) },
  { re: /^Установка пакетов: (.+)$/, key: 'install_packages', build: (m) => ({ packages: m[1] }) },
  { re: /^Удаление пакетов: (.+)$/, key: 'remove_packages', build: (m) => ({ packages: m[1] }) },
  { re: /^Обновление выбранных пакетов \((\d+)\)$/, key: 'upgrade_packages_count', build: (m) => ({ count: m[1] }) },
  { re: /^Открытие портов SSH \((\d+)\/tcp\), HTTP \(80\/tcp\), HTTPS \(443\/tcp\)$/, key: 'open_ports_full', build: (m) => ({ ssh: m[1] }) },
  { re: /^Открытие портов SSH \((\d+)\/tcp\), HTTP, HTTPS$/, key: 'open_ports', build: (m) => ({ ssh: m[1] }) },
  { re: /^Выполнение (.+) для (.+)$/, key: 'exec_for', build: (m, t) => ({ action: actionWord(m[1], t), target: m[2] }) },
  { re: /^Отправка сигнала (.+) процессу PID (\d+)$/, key: 'kill_signal', build: (m) => ({ signal: m[1], pid: m[2] }) },
  { re: /^Проверка существования пользователя (.+)$/, key: 'check_user', build: (m) => ({ username: m[1] }) },
  { re: /^Создание пользователя (.+) с каталогом (.+) \(без sudo\)$/, key: 'create_user', build: (m) => ({ username: m[1], home: m[2] }) },
  { re: /^Установка владельца (.+):(.+) для (.+)$/, key: 'chown', build: (m) => ({ owner: m[1], group: m[2], path: m[3] }) },
  { re: /^Создание резервной копии в (.+)$/, key: 'backup_to', build: (m) => ({ path: m[1] }) },
  { re: /^Запись обновленной конфигурации в (.+)$/, key: 'write_config', build: (m) => ({ path: m[1] }) },
  { re: /^Проверка синтаксиса: (.+)$/, key: 'syntax_check', build: (m) => ({ cmd: m[1] }) },
  { re: /^Перезагрузка службы: (.+)$/, key: 'reload_service', build: (m) => ({ service: m[1] }) },
  { re: /^Выполнение перезапуска службы (.+)$/, key: 'restart_service', build: (m) => ({ service: m[1] }) },
  { re: /^Installing extensions for PHP (.+)$/, key: 'install_ext_for', build: (m) => ({ version: m[1] }) },
  { re: /^Installing PHP (.+) and extensions via package manager$/, key: 'install_php_ext', build: (m) => ({ version: m[1] }) },
];

// Localizes a progress step. Tries the exact dictionary, then dynamic patterns,
// then snake_case machine keys (backup/database/docker). Falls back to the raw
// step when nothing matches, so unknown steps still render readably.
export const localizeTaskStep = (step: string | null | undefined, t: TFunc): string => {
  if (!step) return '';
  const staticKey = STATIC_STEPS[step];
  if (staticKey) return t(`tasks.step.${staticKey}`);
  for (const { re, key, build } of DYNAMIC_STEPS) {
    const m = step.match(re);
    if (m) return t(`tasks.step.${key}`, build(m, t));
  }
  if (/^[a-z0-9_]+$/.test(step)) {
    const path = `tasks.step.${step}`;
    const res = t(path);
    if (res !== path) return res;
  }
  return step;
};