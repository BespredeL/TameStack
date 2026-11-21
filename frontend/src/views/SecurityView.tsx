import React, { useState, useEffect } from 'react';
import {
  Shield,
  ShieldCheck,
  ShieldAlert,
  AlertTriangle,
  Lock,
  Unlock,
  Plus,
  Pencil,
  Trash2,
  RefreshCw,
  CheckCircle2,
  XCircle,
  Zap,
  Radio,
  Play,
  Square,
  RotateCw,
  Ban,
  Wrench,
  FileSearch,
  Download,
  Power,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import {
  GetFirewallStatus,
  ToggleFirewall,
  DeleteFirewallRule,
  GetFail2banStatus,
  UnbanIP,
  InstallFail2ban,
  RunSecurityAudit,
  ApplyHardeningFix,
  ControlFail2ban,
  RepairWebAccess,
  DeleteFail2banJail,
  ToggleFail2banJail,
} from '../../wailsjs/go/main/App';
import { AddFirewallRuleModal } from '../components/AddFirewallRuleModal';
import { InstallFirewallModal } from '../components/InstallFirewallModal';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { BanIPModal } from '../components/BanIPModal';
import { ConfigureFail2banJailModal } from '../components/security/ConfigureFail2banJailModal';
import { FileSecurityScanner } from '../components/security/FileSecurityScanner';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { PageToolbar, Button, Tabs } from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';

interface SecurityViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

const localizeAuditCheck = (chk: any, t: any) => {
  const isPass = chk.status === 'pass';
  let title = chk.title;
  let details = chk.details;
  let rec = chk.recommendation;

  switch (chk.key) {
    case 'ssh_root_login':
      if (isPass) {
        title = t('security.auditChecks.ssh_root_login.passTitle') || title;
        const m = (chk.details || '').match(/PermitRootLogin установлен в '(.*?)'/);
        details = t('security.auditChecks.ssh_root_login.passDetails', { value: m ? m[1] : 'no' }) || details;
        rec = t('security.auditChecks.ssh_root_login.passRec') || rec;
      } else {
        title = t('security.auditChecks.ssh_root_login.dangerTitle') || title;
        details = t('security.auditChecks.ssh_root_login.dangerDetails') || details;
        rec = t('security.auditChecks.ssh_root_login.dangerRec') || rec;
      }
      break;

    case 'ssh_password_auth':
      if (isPass) {
        title = t('security.auditChecks.ssh_password_auth.passTitle') || title;
        details = t('security.auditChecks.ssh_password_auth.passDetails') || details;
        rec = t('security.auditChecks.ssh_password_auth.passRec') || rec;
      } else {
        title = t('security.auditChecks.ssh_password_auth.warnTitle') || title;
        details = t('security.auditChecks.ssh_password_auth.warnDetails') || details;
        rec = t('security.auditChecks.ssh_password_auth.warnRec') || rec;
      }
      break;

    case 'firewall_active':
      if (isPass) {
        const mType = (chk.title || '').match(/Брандмауэр активен \((.*?)\)/);
        title = t('security.auditChecks.firewall_active.passTitle', { type: mType ? mType[1] : 'ufw' }) || title;
        const mDet = (chk.details || '').match(/Активно правил фильтрации: (\d+)\. Входящий трафик по умолчанию: (.*?)\./);
        details = t('security.auditChecks.firewall_active.passDetails', {
          count: mDet ? mDet[1] : 0,
          defaultIncoming: mDet ? mDet[2] : 'deny',
        }) || details;
        rec = t('security.auditChecks.firewall_active.passRec') || rec;
      } else {
        title = t('security.auditChecks.firewall_active.dangerTitle') || title;
        details = t('security.auditChecks.firewall_active.dangerDetails') || details;
        rec = t('security.auditChecks.firewall_active.dangerRec') || rec;
      }
      break;

    case 'fail2ban_active':
      if (isPass) {
        title = t('security.auditChecks.fail2ban_active.passTitle') || title;
        const mDet = (chk.details || '').match(/Активных джейлов: (\d+)\. Заблокировано IP в данный момент: (\d+)\./);
        details = t('security.auditChecks.fail2ban_active.passDetails', {
          jails: mDet ? mDet[1] : 0,
          banned: mDet ? mDet[2] : 0,
        }) || details;
        rec = t('security.auditChecks.fail2ban_active.passRec') || rec;
      } else {
        title = t('security.auditChecks.fail2ban_active.warnTitle') || title;
        details = t('security.auditChecks.fail2ban_active.warnDetails') || details;
        rec = t('security.auditChecks.fail2ban_active.warnRec') || rec;
      }
      break;

    case 'exposed_databases':
      if (isPass) {
        title = t('security.auditChecks.exposed_databases.passTitle') || title;
        details = t('security.auditChecks.exposed_databases.passDetails') || details;
        rec = t('security.auditChecks.exposed_databases.passRec') || rec;
      } else {
        title = t('security.auditChecks.exposed_databases.dangerTitle') || title;
        const mDet = (chk.details || '').match(/Обнаружены службы СУБД, слушающие внешний интерфейс: (.*?)\./);
        details = t('security.auditChecks.exposed_databases.dangerDetails', { list: mDet ? mDet[1] : '' }) || details;
        rec = t('security.auditChecks.exposed_databases.dangerRec') || rec;
      }
      break;

    case 'security_updates':
      if (isPass) {
        title = t('security.auditChecks.security_updates.passTitle') || title;
        const mDet = (chk.details || '').match(/Все критические патчи установлены \(всего ожидающих обновлений: (\d+)\)\./);
        details = t('security.auditChecks.security_updates.passDetails', { total: mDet ? mDet[1] : 0 }) || details;
        rec = t('security.auditChecks.security_updates.passRec') || rec;
      } else {
        const mTitle = (chk.title || '').match(/Доступно критических обновлений безопасности: (\d+)/);
        title = t('security.auditChecks.security_updates.warnTitle', { count: mTitle ? mTitle[1] : 0 }) || title;
        const mDet = (chk.details || '').match(/Обнаружено (\d+) пакетов с исправлениями уязвимостей безопасности \(всего обновлений: (\d+)\)\./);
        details = t('security.auditChecks.security_updates.warnDetails', {
          count: mDet ? mDet[1] : 0,
          total: mDet ? mDet[2] : 0,
        }) || details;
        rec = t('security.auditChecks.security_updates.warnRec') || rec;
      }
      break;
  }

  return { title, details, recommendation: rec };
};

export const SecurityView: React.FC<SecurityViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const { t } = useTranslation();
  const confirm = useConfirm();
  const { selectedServerId, setSelectedServerId, setPageLoading } = useServer();
  const [activeTab, setActiveTab] = useState<'firewall' | 'fail2ban' | 'audit' | 'filescan'>('firewall');

  // Firewall state
  const [firewallStatus, setFirewallStatus] = useState<any>(null);
  const [isFirewallLoading, setIsFirewallLoading] = useState(false);
  const [isAddRuleOpen, setIsAddRuleOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<any | null>(null);
  const [isInstallFirewallOpen, setIsInstallFirewallOpen] = useState(false);

  // Fail2ban state
  const [fail2banStatus, setFail2banStatus] = useState<any>(null);
  const [isFail2banLoading, setIsFail2banLoading] = useState(false);
  const [isControlLoading, setIsControlLoading] = useState<string | null>(null);
  const [isBanModalOpen, setIsBanModalOpen] = useState(false);
  const [selectedJailForBan, setSelectedJailForBan] = useState<string | undefined>(undefined);
  const [configureJailModal, setConfigureJailModal] = useState<{
    isOpen: boolean;
    jail?: any | null;
  }>({ isOpen: false, jail: null });

  // Security audit state
  const [auditReport, setAuditReport] = useState<any>(null);
  const [isAuditLoading, setIsAuditLoading] = useState(false);

  const [isRepairing, setIsRepairing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const currentServer = servers.find((s) => s.id === selectedServerId);

  // Load data based on active tab
  useEffect(() => {
    if (!selectedServerId) return;
    setError(null);

    if (activeTab === 'firewall') {
      loadFirewall();
    } else if (activeTab === 'fail2ban') {
      loadFail2ban();
    } else if (activeTab === 'audit') {
      loadAudit();
    }
  }, [selectedServerId, activeTab]);

  useEffect(() => {
    const cleanup = EventsOn('task:finished', (data: TaskRecord) => {
      if (
        data &&
        data.server_id === selectedServerId &&
        data.status === 'completed' &&
        (data.operation_type === 'security_firewall_install' ||
          data.operation_type === 'security_firewall_toggle' ||
          data.operation_type === 'security_firewall_add_rule' ||
          data.operation_type === 'security_firewall_update_rule' ||
          data.operation_type === 'security_firewall_del_rule')
      ) {
        loadFirewall();
      }
    });
    return () => {
      if (typeof cleanup === 'function') cleanup();
    };
  }, [selectedServerId]);

  const loadFirewall = async () => {
    if (!selectedServerId) return;
    try {
      setIsFirewallLoading(true);
      setPageLoading(true, t('security.readingFirewall'));
      const res = await GetFirewallStatus(selectedServerId);
      setFirewallStatus(res);
    } catch (err: unknown) {
      setError(t('security.errorLoadingFirewall', { error: err?.toString() || '' }));
    } finally {
      setIsFirewallLoading(false);
      setPageLoading(false);
    }
  };

  const loadFail2ban = async () => {
    if (!selectedServerId) return;
    try {
      setIsFail2banLoading(true);
      setPageLoading(true, t('security.pollingFail2ban'));
      const res = await GetFail2banStatus(selectedServerId);
      setFail2banStatus(res);
    } catch (err: unknown) {
      setError(t('security.errorLoadingFail2ban', { error: err?.toString() || '' }));
    } finally {
      setIsFail2banLoading(false);
      setPageLoading(false);
    }
  };

  const loadAudit = async () => {
    if (!selectedServerId) return;
    try {
      setIsAuditLoading(true);
      setPageLoading(true, t('security.runningAudit'));
      const res = await RunSecurityAudit(selectedServerId);
      setAuditReport(res);
    } catch (err: unknown) {
      setError(t('security.errorRunningAudit', { error: err?.toString() || '' }));
    } finally {
      setIsAuditLoading(false);
      setPageLoading(false);
    }
  };

  const handleToggleFirewall = async (enable: boolean) => {
    if (!selectedServerId) return;
    if (!enable && !(await confirm(t('security.disableFirewallConfirm'), { danger: true }))) {
      return;
    }
    try {
      const task = await ToggleFirewall(selectedServerId, enable);
      onOpenTask(task as any);
      setTimeout(loadFirewall, 2000);
    } catch (err: unknown) {
      setError(t('security.toggleFirewallError', { error: err?.toString() || '' }));
    }
  };

  const handleDeleteRule = async (ruleNum: number, port: string, proto: string) => {
    if (!selectedServerId) return;
    if (!(await confirm(t('security.deleteRuleConfirm', { ruleNum, port, proto }), { danger: true, confirmLabel: t('common.delete') }))) {
      return;
    }
    try {
      const task = await DeleteFirewallRule(selectedServerId, ruleNum, port, proto);
      onOpenTask(task as any);
      setTimeout(loadFirewall, 2000);
    } catch (err: unknown) {
      setError(t('security.deleteRuleError', { error: err?.toString() || '' }));
    }
  };

  const handleUnban = async (jail: string, ip: string) => {
    if (!selectedServerId) return;
    try {
      const task = await UnbanIP({
        server_id: selectedServerId,
        jail,
        ip,
      } as any);
      onOpenTask(task as any);
      setTimeout(loadFail2ban, 1500);
    } catch (err: unknown) {
      setError(t('security.unbanError', { error: err?.toString() || '' }));
    }
  };

  const handleInstallFail2ban = async () => {
    if (!selectedServerId) return;
    try {
      const task = await InstallFail2ban(selectedServerId);
      onOpenTask(task as any);
      setTimeout(loadFail2ban, 3000);
    } catch (err: unknown) {
      setError(t('security.installFail2banError', { error: err?.toString() || '' }));
    }
  };

  const handleControlFail2ban = async (action: string) => {
    if (!selectedServerId) return;
    if (action === 'stop' && !(await confirm(t('security.stopFail2banConfirm'), { danger: true }))) {
      return;
    }
    try {
      setIsControlLoading(action);
      const task = await ControlFail2ban(selectedServerId, action);
      onOpenTask(task as any);
      setTimeout(loadFail2ban, 2500);
    } catch (err: unknown) {
      setError(t('security.controlFail2banError', { error: err?.toString() || '' }));
    } finally {
      setIsControlLoading(null);
    }
  };

  const handleToggleJail = async (jail: any) => {
    if (!selectedServerId) return;
    try {
      const task = await ToggleFail2banJail({
        server_id: selectedServerId,
        name: jail.name,
        enabled: !jail.enabled,
      });
      onOpenTask(task as any);
      setTimeout(loadFail2ban, 1500);
    } catch (err: unknown) {
      setError(err?.toString() || 'Failed to toggle jail');
    }
  };

  const handleDeleteJail = async (jailName: string) => {
    if (!selectedServerId) return;
    if (
      !(await confirm(t('security.deleteJailConfirm', { name: jailName }), {
        danger: true,
        title: t('security.deleteJailTitle'),
      }))
    ) {
      return;
    }
    try {
      const task = await DeleteFail2banJail({
        server_id: selectedServerId,
        name: jailName,
      });
      onOpenTask(task as any);
      setTimeout(loadFail2ban, 1500);
    } catch (err: unknown) {
      setError(err?.toString() || 'Failed to delete jail');
    }
  };

  const handleApplyFix = async (fixType: string) => {
    if (!selectedServerId) return;
    if (!(await confirm(t('security.applyFixConfirm'), { danger: true }))) {
      return;
    }
    try {
      const task = await ApplyHardeningFix(selectedServerId, fixType);
      onOpenTask(task as any);
      setTimeout(loadAudit, 3000);
    } catch (err: unknown) {
      setError(t('security.applyFixError', { error: err?.toString() || '' }));
    }
  };

  const handleRepairWebAccess = async () => {
    if (!selectedServerId) return;
    if (!(await confirm(t('security.repairWebAccessConfirm'), { danger: true }))) {
      return;
    }
    try {
      setIsRepairing(true);
      const task = await RepairWebAccess(selectedServerId);
      onOpenTask(task as any);
      setTimeout(() => {
        loadFirewall();
        loadFail2ban();
      }, 3000);
    } catch (err: unknown) {
      setError(t('security.repairWebAccessError', { error: err?.toString() || '' }));
    } finally {
      setIsRepairing(false);
    }
  };



  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Shield}
        description={t('empty.securityDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        serverLabel={t('security.targetServer')}
        icon={<ShieldCheck className="w-5 h-5 text-emerald-400" />}
        servers={servers}
        selectedServerId={selectedServerId}
        onSelectServer={setSelectedServerId}
        actions={
          <>
            <Button
              variant="outline"
              size="md"
              onClick={handleRepairWebAccess}
              disabled={isRepairing}
              loading={isRepairing}
              icon={!isRepairing ? <Wrench className="w-4 h-4 text-amber-400" /> : undefined}
              title={t('security.repairWebAccessTitle')}
              className="border-amber-500/30 text-amber-300 hover:bg-amber-500/10"
            >
              {t('security.repairWebAccessBtn')}
            </Button>
            <Button
              variant="secondary"
              size="md"
              onClick={() => {
                if (activeTab === 'firewall') loadFirewall();
                else if (activeTab === 'fail2ban') loadFail2ban();
                else loadAudit();
              }}
              disabled={isFirewallLoading || isFail2banLoading || isAuditLoading}
              loading={isFirewallLoading || isFail2banLoading || isAuditLoading}
              icon={<RefreshCw className="w-4 h-4" />}
              title={t('common.refresh')}
            >
              {t('common.refresh')}
            </Button>
          </>
        }
      />

      {error && (
        <div className="p-4 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-center justify-between text-sm text-rose-300">
          <div className="flex items-center gap-3">
            <AlertTriangle className="w-5 h-5 text-rose-400 shrink-0" />
            <span>{error}</span>
          </div>
          <button onClick={() => setError(null)} className="text-xs hover:underline text-rose-400">
            {t('common.close')}
          </button>
        </div>
      )}

      {/* Navigation Tabs */}
      <Tabs
        tabs={[
          { id: 'firewall', label: t('security.tabFirewall'), icon: <Shield className="w-4 h-4" /> },
          { id: 'fail2ban', label: 'Fail2ban', icon: <Lock className="w-4 h-4" /> },
          { id: 'audit', label: t('security.tabAudit'), icon: <Zap className="w-4 h-4" /> },
          { id: 'filescan', label: t('security.tabScanner'), icon: <FileSearch className="w-4 h-4" /> },
        ]}
        activeTab={activeTab}
        onChange={(tabId) => setActiveTab(tabId as any)}
        variant="line"
      />

      {/* TAB 1: FIREWALL */}
      {activeTab === 'firewall' && (
        isFirewallLoading && !firewallStatus ? (
          <LoadingState
            type="cards"
            title={t('security.pollFirewallTitle')}
            subtitle={t('security.pollFirewallSubtitle', { server: currentServer?.name || '' })}
          />
        ) : firewallStatus?.type === 'none' ? (
          <div className="bg-surface-850 border border-amber-500/20 rounded-xl p-8 text-center space-y-4 shadow-sm">
            <div className="w-16 h-16 rounded-2xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400 mx-auto">
              <ShieldAlert className="w-8 h-8" />
            </div>
            <h3 className="text-lg font-bold text-slate-100">{t('security.firewallNotInstalledTitle')}</h3>
            <p className="text-sm text-slate-400 max-w-lg mx-auto">
              {t('security.firewallNotInstalledDesc')}
            </p>
            <button
              onClick={() => setIsInstallFirewallOpen(true)}
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-emerald-900/20"
            >
              <Download className="w-4 h-4" />
              {t('security.installFirewallBtn')}
            </button>
          </div>
        ) : (
          <div className="space-y-6">
            {/* Status Bar */}
          <div className="bg-surface-850 border border-slate-800 rounded-2xl p-5 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-sm">
            <div className="flex items-center gap-4">
              <div
                className={`w-12 h-12 rounded-xl flex items-center justify-center ${
                  firewallStatus?.is_active
                    ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                    : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                }`}
              >
                {firewallStatus?.is_active ? (
                  <ShieldCheck className="w-6 h-6" />
                ) : (
                  <ShieldAlert className="w-6 h-6" />
                )}
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h3 className="text-base font-semibold text-slate-100">
                    {t('security.firewallStatusLabel', { type: firewallStatus?.type?.toUpperCase() || 'UFW' })}
                  </h3>
                  <span
                    className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                      firewallStatus?.is_active
                        ? 'bg-emerald-500/20 text-emerald-300'
                        : 'bg-rose-500/20 text-rose-300'
                    }`}
                  >
                    {firewallStatus?.is_active ? t('security.active') : t('security.inactive')}
                  </span>
                </div>
                <p className="text-xs text-slate-400 mt-0.5">
                  {t('security.defaultIncoming')}{' '}
                  <strong className="text-slate-200">{firewallStatus?.default_incoming || 'deny'}</strong> | {t('security.defaultOutgoing')}{' '}
                  <strong className="text-slate-200">{firewallStatus?.default_outgoing || 'allow'}</strong>
                </p>
              </div>
            </div>

            <div className="flex items-center gap-3">
              {firewallStatus?.is_active ? (
                <button
                  onClick={() => handleToggleFirewall(false)}
                  className="flex items-center gap-2 px-4 py-2 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 text-rose-300 text-xs font-medium rounded-lg transition"
                >
                  <Unlock className="w-4 h-4" />
                  {t('security.disableFirewallBtn')}
                </button>
              ) : (
                <button
                  onClick={() => handleToggleFirewall(true)}
                  className="flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium rounded-lg transition shadow-lg shadow-emerald-900/20"
                >
                  <Lock className="w-4 h-4" />
                  {t('security.enableFirewallBtn')}
                </button>
              )}

              <button
                onClick={() => {
                  setEditingRule(null);
                  setIsAddRuleOpen(true);
                }}
                className="flex items-center gap-2 px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-100 text-xs font-medium rounded-lg border border-slate-700/60 transition"
              >
                <Plus className="w-4 h-4 text-emerald-400" />
                {t('security.addRule')}
              </button>
            </div>
          </div>

          {/* Rules Table */}
          <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
            <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between">
              <div>
                <h4 className="text-sm font-semibold text-slate-200">{t('security.rulesTableTitle')}</h4>
                <p className="text-xs text-slate-400">
                  {t('security.totalRules')}{' '}
                  <span className="font-semibold text-slate-200">
                    {firewallStatus?.rules?.length || 0}
                  </span>
                </p>
              </div>
            </div>

            {isFirewallLoading ? (
              <div className="p-12 text-center text-slate-400 flex flex-col items-center justify-center gap-3">
                <RefreshCw className="w-6 h-6 animate-spin text-emerald-400" />
                <span className="text-sm">{t('security.readingConfig')}</span>
              </div>
            ) : !firewallStatus?.rules || firewallStatus.rules.length === 0 ? (
              <div className="p-12 text-center text-slate-400">
                <Shield className="w-10 h-10 mx-auto text-slate-500 mb-2 opacity-50" />
                <p className="text-sm">{t('security.noRulesYet')}</p>
                <p className="text-xs text-slate-500 mt-1">
                  {t('security.noRulesHint')}
                </p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-slate-900/60 text-slate-400 border-b border-slate-800">
                    <tr>
                      <th className="py-3 px-4 font-medium">#</th>
                      <th className="py-3 px-4 font-medium">{t('security.colPort')}</th>
                      <th className="py-3 px-4 font-medium">{t('security.colProto')}</th>
                      <th className="py-3 px-4 font-medium">{t('security.colAction')}</th>
                      <th className="py-3 px-4 font-medium">{t('security.colFrom')}</th>
                      <th className="py-3 px-4 font-medium">{t('security.colDirection')}</th>
                      <th className="py-3 px-4 font-medium">{t('security.colComment')}</th>
                      <th className="py-3 px-4 font-medium text-right">{t('security.colActions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60">
                    {firewallStatus.rules.map((rule: any, idx: number) => {
                      const isSSHPort =
                        rule.port === String(currentServer?.port) ||
                        (currentServer?.port === 0 && rule.port === '22');

                      return (
                        <tr key={idx} className="hover:bg-slate-800/40 transition">
                          <td className="py-3 px-4 font-mono text-slate-400">
                            {rule.number || idx + 1}
                          </td>
                          <td className="py-3 px-4 font-mono font-medium text-slate-200">
                            <span className="flex items-center gap-2">
                              {rule.port}
                              {isSSHPort && (
                                <span className="px-1.5 py-0.5 text-[10px] bg-blue-500/20 text-blue-300 rounded font-sans">
                                  SSH
                                </span>
                              )}
                            </span>
                          </td>
                          <td className="py-3 px-4 text-slate-300 uppercase font-mono">
                            {rule.protocol || 'any'}
                          </td>
                          <td className="py-3 px-4">
                            <span
                              className={`px-2 py-0.5 rounded text-[11px] font-semibold ${
                                rule.action === 'ALLOW'
                                  ? 'bg-emerald-500/20 text-emerald-300'
                                  : rule.action === 'LIMIT'
                                  ? 'bg-amber-500/20 text-amber-300'
                                  : 'bg-rose-500/20 text-rose-300'
                              }`}
                            >
                              {rule.action}
                            </span>
                          </td>
                          <td className="py-3 px-4 text-slate-300 font-mono">
                            {rule.from_ip || t('security.sourceAnywhere')}
                          </td>
                          <td className="py-3 px-4 text-slate-400 uppercase">
                            {rule.direction || 'IN'}
                          </td>
                          <td className="py-3 px-4 text-slate-400 italic">
                            {rule.comment || '—'}
                          </td>
                          <td className="py-3 px-4 text-right">
                            <div className="flex items-center justify-end gap-1">
                              <button
                                onClick={() => {
                                  setEditingRule(rule);
                                  setIsAddRuleOpen(true);
                                }}
                                className="p-1 text-slate-400 hover:text-indigo-400 hover:bg-indigo-500/10 rounded transition"
                                title={t('security.editRuleTooltip')}
                              >
                                <Pencil className="w-4 h-4" />
                              </button>
                              <button
                                onClick={() =>
                                  handleDeleteRule(rule.number, rule.port, rule.protocol)
                                }
                                className="p-1 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded transition"
                                title={t('security.deleteRuleTooltip')}
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
        )
      )}

      {/* TAB 2: FAIL2BAN */}
      {activeTab === 'fail2ban' && (
        <div className="space-y-6">
          {isFail2banLoading ? (
            <LoadingState
              type="cards"
              title={t('security.pollFail2banTitle')}
              subtitle={t('security.pollFail2banSubtitle', { server: currentServer?.name || '' })}
            />
          ) : !fail2banStatus?.is_installed ? (
            <div className="bg-surface-850 border border-amber-500/20 rounded-xl p-8 text-center space-y-4 shadow-sm">
              <div className="w-16 h-16 rounded-2xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400 mx-auto">
                <Lock className="w-8 h-8" />
              </div>
              <h3 className="text-lg font-bold text-slate-100">{t('security.notInstalledTitle')}</h3>
              <p className="text-sm text-slate-400 max-w-lg mx-auto">
                {t('security.notInstalledDesc')}
              </p>
              <button
                onClick={handleInstallFail2ban}
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-emerald-900/20"
              >
                <Zap className="w-4 h-4" />
                {t('security.installFail2banBtn')}
              </button>
            </div>
          ) : (
            <div className="space-y-6">
              {/* Fail2ban Status Overview */}
              <div className="bg-surface-850 border border-slate-800 rounded-2xl p-5 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-sm">
                <div className="flex items-center gap-4">
                  <div
                    className={`w-12 h-12 rounded-xl flex items-center justify-center border ${
                      fail2banStatus?.is_active
                        ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                        : 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                    }`}
                  >
                    {fail2banStatus?.is_active ? (
                      <ShieldCheck className="w-6 h-6" />
                    ) : (
                      <ShieldAlert className="w-6 h-6" />
                    )}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="text-base font-semibold text-slate-100">{t('security.serviceTitle')}</h3>
                      <span
                        className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                          fail2banStatus?.is_active
                            ? 'bg-emerald-500/20 text-emerald-300'
                            : 'bg-rose-500/20 text-rose-300'
                        }`}
                      >
                        {fail2banStatus?.is_active ? t('security.activeScanning') : t('security.stopped')}
                      </span>
                    </div>
                    <p className="text-xs text-slate-400 mt-0.5">
                      {t('security.versionAndJails', { version: fail2banStatus?.version || '1.0', jails: fail2banStatus?.jails_count || 0 })}{' '}
                      <strong>{fail2banStatus?.jails?.length || 0}</strong>
                    </p>
                  </div>
                </div>

                {/* Management Controls */}
                <div className="flex flex-wrap items-center gap-2">
                  {fail2banStatus?.is_active ? (
                    <>
                      <button
                        onClick={() => handleControlFail2ban('restart')}
                        disabled={!!isControlLoading}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 border border-slate-700/60 text-slate-200 rounded-lg text-xs font-medium transition disabled:opacity-50"
                        title={t('security.restartServiceTooltip')}
                      >
                        <RotateCw
                          className={`w-3.5 h-3.5 ${isControlLoading === 'restart' ? 'animate-spin' : ''}`}
                        />
                        {t('security.restartService')}
                      </button>
                      <button
                        onClick={() => handleControlFail2ban('reload')}
                        disabled={!!isControlLoading}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 border border-slate-700/60 text-slate-200 rounded-lg text-xs font-medium transition disabled:opacity-50"
                        title={t('security.reloadServiceTooltip')}
                      >
                        <RefreshCw
                          className={`w-3.5 h-3.5 ${isControlLoading === 'reload' ? 'animate-spin' : ''}`}
                        />
                        {t('security.reloadService')}
                      </button>
                      <button
                        onClick={() => handleControlFail2ban('stop')}
                        disabled={!!isControlLoading}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 text-rose-300 rounded-lg text-xs font-medium transition disabled:opacity-50"
                        title={t('security.stopServiceTooltip')}
                      >
                        <Square className="w-3.5 h-3.5" />
                        {t('security.stopService')}
                      </button>
                    </>
                  ) : (
                    <button
                      onClick={() => handleControlFail2ban('start')}
                      disabled={!!isControlLoading}
                      className="inline-flex items-center gap-1.5 px-4 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition disabled:opacity-50 shadow-sm"
                      title={t('security.startServiceTooltip')}
                    >
                      <Play className="w-3.5 h-3.5 fill-current" />
                      {t('security.startService')}
                    </button>
                  )}

                  <button
                    onClick={() => {
                      setSelectedJailForBan(undefined);
                      setIsBanModalOpen(true);
                    }}
                    className="inline-flex items-center gap-1.5 px-3.5 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-medium transition shadow-sm ml-1"
                    title={t('security.banIPTooltip')}
                  >
                    <Ban className="w-3.5 h-3.5" />
                    {t('security.banIP')}
                  </button>

                  <button
                    onClick={() => setConfigureJailModal({ isOpen: true, jail: null })}
                    className="inline-flex items-center gap-1.5 px-3.5 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg text-xs font-medium transition shadow-sm ml-1 cursor-pointer"
                    title={t('security.addJailTooltip')}
                  >
                    <Plus className="w-3.5 h-3.5" />
                    {t('security.addJailBtn')}
                  </button>
                </div>
              </div>

              {/* Jails Grid */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {fail2banStatus?.jails?.map((jail: any) => (
                  <div
                    key={jail.name}
                    className={`bg-surface-850 border rounded-2xl p-5 space-y-4 shadow-sm transition ${
                      jail.enabled ? 'border-slate-800 hover:border-slate-700' : 'border-slate-800/60 opacity-75'
                    }`}
                  >
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-800 pb-3">
                      <div className="flex items-center gap-2.5 flex-wrap">
                        <Radio
                          className={`w-4 h-4 ${
                            jail.enabled ? 'text-emerald-400' : 'text-slate-500'
                          }`}
                        />
                        <h4 className="text-sm font-semibold text-slate-100 font-mono">
                          {jail.name}
                        </h4>
                        <span
                          className={`px-2 py-0.5 text-[10px] font-medium rounded-full ${
                            jail.enabled
                              ? 'bg-emerald-500/20 text-emerald-300'
                              : 'bg-slate-700/50 text-slate-400'
                          }`}
                        >
                          {jail.enabled ? t('security.jailActive') : t('security.jailDisabled')}
                        </span>
                        {jail.is_managed && (
                          <span className="px-1.5 py-0.5 text-[10px] bg-indigo-500/15 border border-indigo-500/30 text-indigo-300 rounded font-mono">
                            TameStack
                          </span>
                        )}
                      </div>

                      <div className="flex items-center gap-1.5 self-end sm:self-auto">
                        <button
                          onClick={() => handleToggleJail(jail)}
                          className={`p-1.5 rounded-lg border text-xs transition cursor-pointer ${
                            jail.enabled
                              ? 'text-emerald-400 hover:bg-emerald-500/10 border-emerald-500/20'
                              : 'text-slate-400 hover:bg-slate-700/50 border-slate-700'
                          }`}
                          title={t('security.toggleJailTooltip', {
                            action: jail.enabled ? t('security.stopService') : t('security.startService'),
                            name: jail.name,
                          })}
                        >
                          <Power className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => setConfigureJailModal({ isOpen: true, jail })}
                          className="p-1.5 rounded-lg text-slate-400 hover:text-indigo-400 hover:bg-slate-800 transition cursor-pointer"
                          title={t('security.editJailTooltip', { name: jail.name })}
                        >
                          <Pencil className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => handleDeleteJail(jail.name)}
                          className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition cursor-pointer"
                          title={t('security.deleteJailTooltip', { name: jail.name })}
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => {
                            setSelectedJailForBan(jail.name);
                            setIsBanModalOpen(true);
                          }}
                          className="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 transition px-2 py-1 rounded hover:bg-rose-500/10 ml-1"
                          title={t('security.banIPInJailTooltip', { jail: jail.name })}
                        >
                          <Ban className="w-3 h-3" />
                          <span className="hidden lg:inline">{t('security.banIPInJail')}</span>
                        </button>
                      </div>
                    </div>

                    {/* Jail Configuration Details Badges */}
                    <div className="space-y-1.5">
                      <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
                        {jail.port && !jail.port.toLowerCase().includes('invalid') && !jail.port.includes('(') && (
                          <span className="px-2 py-0.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 font-mono">
                            <span className="text-slate-500">{t('security.jailPort')}:</span> {jail.port}
                          </span>
                        )}
                        {jail.filter && (
                          <span className="px-2 py-0.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 font-mono">
                            <span className="text-slate-500">{t('security.jailFilter')}:</span> {jail.filter}
                          </span>
                        )}
                        {jail.max_retry > 0 && (
                          <span className="px-2 py-0.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 font-mono">
                            <span className="text-slate-500">{t('security.jailMaxRetry')}:</span> {jail.max_retry}
                          </span>
                        )}
                        {jail.find_time && (
                          <span className="px-2 py-0.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 font-mono">
                            <span className="text-slate-500">{t('security.jailFindTime')}:</span> {jail.find_time}
                          </span>
                        )}
                        {jail.ban_time && (
                          <span className="px-2 py-0.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 font-mono">
                            <span className="text-slate-500">{t('security.jailBanTime')}:</span> {jail.ban_time}
                          </span>
                        )}
                      </div>
                      {jail.log_path && !jail.log_path.includes('Current monitored') && !jail.log_path.toLowerCase().includes('invalid') && (
                        <div className="text-[11px] text-slate-400 font-mono truncate" title={jail.log_path}>
                          <span className="text-slate-500">{t('security.jailLogPath')}:</span> {jail.log_path}
                        </div>
                      )}
                    </div>

                    <div className="grid grid-cols-2 gap-3 text-xs">
                      <div className="p-3 bg-slate-900 border border-slate-800/80 rounded-lg">
                        <span className="text-slate-400 block">{t('security.failedAttemptsNow')}</span>
                        <span className="text-base font-bold text-amber-400">
                          {jail.currently_failed}
                        </span>
                      </div>
                      <div className="p-3 bg-slate-900 border border-slate-800/80 rounded-lg">
                        <span className="text-slate-400 block">{t('security.bannedNow')}</span>
                        <span className="text-base font-bold text-rose-400">
                          {jail.currently_banned}
                        </span>
                      </div>
                    </div>

                    {/* Banned IP List */}
                    <div>
                      <span className="text-xs font-medium text-slate-300 block mb-2">
                        {t('security.bannedIPsLabel')}
                      </span>
                      {jail.banned_ip_list && jail.banned_ip_list.length > 0 ? (
                        <div className="flex flex-wrap gap-2">
                          {jail.banned_ip_list.map((ip: string) => (
                            <span
                              key={ip}
                              className="inline-flex items-center gap-1.5 px-2.5 py-1 bg-slate-900 border border-rose-500/30 text-rose-300 text-xs rounded-lg font-mono"
                            >
                              {ip}
                              <button
                                onClick={() => handleUnban(jail.name, ip)}
                                className="text-slate-400 hover:text-white transition ml-1"
                                title={t('security.unbanIPTooltip')}
                              >
                                ×
                              </button>
                            </span>
                          ))}
                        </div>
                      ) : (
                        <p className="text-xs text-slate-500 italic">
                          {t('security.noBannedIPs')}
                        </p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* TAB 3: SECURITY AUDIT & HARDENING */}
      {activeTab === 'audit' && (
        <div className="space-y-6">
          {isAuditLoading ? (
            <LoadingState
              type="table"
              title={t('security.auditTitle')}
              subtitle={t('security.auditSubtitle', { server: currentServer?.name || '' })}
            />
          ) : !auditReport ? (
            <div className="bg-surface-850 border border-slate-800 rounded-2xl p-8 text-center space-y-4 shadow-sm">
              <Zap className="w-12 h-12 mx-auto text-emerald-400" />
              <h3 className="text-lg font-bold text-slate-100">{t('security.noAuditYetTitle')}</h3>
              <button
                onClick={loadAudit}
                className="px-5 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-emerald-900/20"
              >
                {t('security.runAuditBtn')}
              </button>
            </div>
          ) : (
            <div className="space-y-6">
              {/* Score Bar */}
              <div className="bg-surface-850 border border-slate-800 rounded-2xl p-6 flex flex-col md:flex-row md:items-center justify-between gap-6 shadow-sm">
                <div className="flex items-center gap-5">
                  <div
                    className={`w-20 h-20 rounded-2xl flex flex-col items-center justify-center border font-bold ${
                      auditReport.score >= 80
                        ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                        : auditReport.score >= 50
                        ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                        : 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                    }`}
                  >
                    <span className="text-2xl">{auditReport.score}%</span>
                    <span className="text-[10px] font-normal tracking-wide uppercase">{t('security.ratingLabel')}</span>
                  </div>
                  <div>
                    <h3 className="text-lg font-bold text-slate-100">
                      {t('security.ratingLabel')}: {auditReport.score >= 80 ? t('security.protectionHigh') : auditReport.score >= 50 ? t('security.protectionMedium') : t('security.protectionCritical')}
                    </h3>
                    <p className="text-xs text-slate-400 mt-1 max-w-lg">
                      {t('security.protectionDesc')}
                    </p>
                  </div>
                </div>

                <button
                  onClick={loadAudit}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/60 text-xs font-medium rounded-lg transition shrink-0"
                >
                  {t('security.recheckBtn')}
                </button>
              </div>

              {/* Checklist */}
              <div className="space-y-3">
                <h4 className="text-sm font-semibold text-slate-200">
                  {t('security.checkResults', { count: auditReport.checks?.length || 0 })}
                </h4>

                <div className="grid grid-cols-1 gap-3">
                  {auditReport.checks?.map((chk: any) => {
                    const localized = localizeAuditCheck(chk, t);
                    return (
                      <div
                        key={chk.key}
                        className="bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-xl p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-sm transition-all"
                      >
                        <div className="flex items-start gap-3.5">
                          <div className="mt-0.5">
                            {chk.status === 'pass' ? (
                              <CheckCircle2 className="w-5 h-5 text-emerald-400" />
                            ) : chk.status === 'warning' ? (
                              <AlertTriangle className="w-5 h-5 text-amber-400" />
                            ) : (
                              <XCircle className="w-5 h-5 text-rose-400" />
                            )}
                          </div>
                          <div>
                            <h5 className="text-sm font-medium text-slate-200">{localized.title}</h5>
                            <p className="text-xs text-slate-400 mt-0.5">{localized.details}</p>
                            {localized.recommendation && (
                              <p className="text-xs text-emerald-400/90 mt-1 font-mono">
                                {t('security.tipPrefix')} {localized.recommendation}
                              </p>
                            )}
                          </div>
                        </div>

                        {chk.can_auto_fix && chk.status !== 'pass' && (
                          <button
                            onClick={() => handleApplyFix(chk.key)}
                            className="flex items-center gap-1.5 px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition shadow-md shrink-0"
                          >
                            <Zap className="w-3.5 h-3.5" />
                            {t('security.fix1Click')}
                          </button>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* Listening Ports Table */}
              <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm mt-6">
                <div className="px-6 py-4 border-b border-slate-800">
                  <h4 className="text-sm font-semibold text-slate-200">
                    {t('security.listeningPortsTitle')}
                  </h4>
                  <p className="text-xs text-slate-400">
                    {t('security.listeningPortsDesc')}
                  </p>
                </div>

                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-slate-900/60 text-slate-400 border-b border-slate-800">
                      <tr>
                        <th className="py-3 px-4 font-medium">{t('security.colProto')}</th>
                        <th className="py-3 px-4 font-medium">{t('security.colListeningPort')}</th>
                        <th className="py-3 px-4 font-medium">{t('security.colLocalAddress')}</th>
                        <th className="py-3 px-4 font-medium">{t('security.colProcess')}</th>
                        <th className="py-3 px-4 font-medium">PID</th>
                        <th className="py-3 px-4 font-medium">{t('security.colExposure')}</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60 font-mono">
                      {auditReport.listening_ports?.map((p: any, idx: number) => (
                        <tr key={idx} className="hover:bg-slate-800/40 transition">
                          <td className="py-2.5 px-4 uppercase text-slate-400">{p.protocol}</td>
                          <td className="py-2.5 px-4 font-bold text-slate-200">{p.port}</td>
                          <td className="py-2.5 px-4 text-slate-300">{p.local_address}</td>
                          <td className="py-2.5 px-4 text-emerald-400 font-sans font-medium">
                            {p.process}
                          </td>
                          <td className="py-2.5 px-4 text-slate-400">{p.pid}</td>
                          <td className="py-2.5 px-4 font-sans">
                            {p.is_public ? (
                              <span className="px-2 py-0.5 rounded text-[11px] bg-rose-500/20 text-rose-300 font-medium">
                                {t('security.publicExposure')}
                              </span>
                            ) : (
                              <span className="px-2 py-0.5 rounded text-[11px] bg-slate-700 text-slate-300 font-medium">
                                {t('security.localExposure')}
                              </span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* TAB 4: FILE SECURITY & WEBSHELL SCANNER */}
      {activeTab === 'filescan' && selectedServerId && (
        <FileSecurityScanner serverId={selectedServerId} />
      )}

      {/* Add / Edit Rule Modal */}
      {currentServer && (
        <AddFirewallRuleModal
          isOpen={isAddRuleOpen}
          onClose={() => {
            setIsAddRuleOpen(false);
            setEditingRule(null);
          }}
          server={currentServer}
          initialRule={editingRule}
          onRuleAdded={(task) => {
            onOpenTask(task);
            setTimeout(loadFirewall, 2000);
          }}
          onRuleUpdated={(task) => {
            onOpenTask(task);
            setTimeout(loadFirewall, 2000);
          }}
        />
      )}

      {/* Ban IP Modal */}
      {currentServer && (
        <BanIPModal
          isOpen={isBanModalOpen}
          onClose={() => setIsBanModalOpen(false)}
          server={currentServer}
          jails={fail2banStatus?.jails?.map((j: any) => j.name) || ['sshd']}
          initialJail={selectedJailForBan}
          onBanned={(task) => {
            onOpenTask(task);
            setTimeout(loadFail2ban, 1500);
          }}
        />
      )}

      {/* Configure Fail2ban Jail Modal */}
      {selectedServerId && configureJailModal.isOpen && (
        <ConfigureFail2banJailModal
          isOpen={configureJailModal.isOpen}
          onClose={() => setConfigureJailModal({ isOpen: false, jail: null })}
          serverID={selectedServerId}
          initialJail={configureJailModal.jail}
          availableFilters={fail2banStatus?.available_filters || []}
          onSaved={() => {
            setTimeout(loadFail2ban, 1500);
          }}
        />
      )}

      {/* Install Firewall Modal */}
      {currentServer && (
        <InstallFirewallModal
          isOpen={isInstallFirewallOpen}
          serverID={currentServer.id}
          serverName={currentServer.name}
          onClose={() => setIsInstallFirewallOpen(false)}
          onTaskStarted={(task) => onOpenTask(task)}
        />
      )}
    </div>
  );
};
