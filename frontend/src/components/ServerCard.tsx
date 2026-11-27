import React, { useEffect, useRef, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  Server,
  Activity,
  Cpu,
  HardDrive,
  Trash2,
  Play,
  CheckCircle,
  XCircle,
  HelpCircle,
  Loader2,
  Terminal,
  Edit3,
  Shield,
  ShieldAlert,
  Layers,
} from 'lucide-react';
import { ServerRecord, DiagnosticsRecord, TaskRecord } from '../types';
import { TestConnection, TriggerDiagnostics, DeleteServer, SetServerMode } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { InventoryModal } from './InventoryModal';
import { Badge, Button } from './ui';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { usePrivacy } from '../context/PrivacyContext';

interface ServerCardProps {
  server: ServerRecord;
  diagnostics?: DiagnosticsRecord;
  onRefresh: () => void;
  onSelectServer?: (server: ServerRecord) => void;
  onEditServer?: (server: ServerRecord) => void;
}

export const ServerCard: React.FC<ServerCardProps> = ({
  server,
  diagnostics,
  onRefresh,
  onSelectServer,
  onEditServer,
}) => {
  const { t, language } = useTranslation();
  const confirm = useConfirm();
  const { maskServerName, maskHost, maskUser } = usePrivacy();
  const [isTesting, setIsTesting] = useState(false);
  const [isDiagnosing, setIsDiagnosing] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isInventoryOpen, setIsInventoryOpen] = useState(false);
  const [isTogglingMode, setIsTogglingMode] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [diagResult, setDiagResult] = useState<{ kind: 'success' | 'error' | 'info'; message: string } | null>(null);
  const diagUnsubRef = useRef<(() => void) | null>(null);
  const diagTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const showDiagResult = (kind: 'success' | 'error' | 'info', message: string, autoClearMs?: number) => {
    if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
    setDiagResult({ kind, message });
    if (autoClearMs) {
      diagTimerRef.current = setTimeout(() => setDiagResult(null), autoClearMs);
    }
  };

  useEffect(() => {
    return () => {
      diagUnsubRef.current?.();
      if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
    };
  }, []);

  const handleToggleMode = async (e: React.MouseEvent) => {
    e.stopPropagation();
    const targetMode = server.mode === 'management' ? 'audit' : 'management';
    const confirmMessage = targetMode === 'audit'
      ? t('servers.card.switchAuditConfirm', { name: server.name })
      : t('servers.card.switchManagementConfirm', { name: server.name });
    if (!(await confirm(confirmMessage, { danger: targetMode === 'management' }))) {
      return;
    }
    setIsTogglingMode(true);
    try {
      await SetServerMode(server.id, targetMode);
      onRefresh();
    } catch (err: unknown) {
      console.error('Failed to set server mode:', err);
    } finally {
      setIsTogglingMode(false);
    }
  };

  const handleTestConnection = async (e: React.MouseEvent) => {
    e.stopPropagation();
    setIsTesting(true);
    setTestResult(null);
    try {
      const res = await TestConnection(server.id);
      if (res.success) {
        setTestResult({ success: true, message: t('servers.card.connectionSuccess') });
      } else {
        setTestResult({ success: false, message: res.message || t('servers.card.connectionFailed') });
      }
      onRefresh();
    } catch (err: unknown) {
      setTestResult({ success: false, message: t('servers.card.connectionError', { error: getErrorMessage(err) }) });
    } finally {
      setIsTesting(false);
      setTimeout(() => setTestResult(null), 4000);
    }
  };

  const handleRunDiagnostics = async (e: React.MouseEvent) => {
    e.stopPropagation();
    setIsDiagnosing(true);
    setDiagResult(null);
    try {
      const task = await TriggerDiagnostics(server.id);
      showDiagResult('info', t('servers.card.diagQueued'));
      onRefresh();
      // The RPC only enqueues the task; keep the spinner until the engine
      // reports it finished, otherwise the button looks idle while it runs.
      diagUnsubRef.current?.();
      diagUnsubRef.current = EventsOn('task:finished', (finished: TaskRecord) => {
        if (!task || finished.id !== task.id) return;
        diagUnsubRef.current?.();
        diagUnsubRef.current = null;
        setIsDiagnosing(false);
        if (finished.status === 'completed') {
          showDiagResult('success', t('servers.card.diagCompleted'), 6000);
        } else {
          showDiagResult('error', finished.error_message || t('servers.card.diagFailed'));
        }
      });
    } catch (err: unknown) {
      setIsDiagnosing(false);
      showDiagResult('error', t('servers.card.diagError', { error: getErrorMessage(err) }));
    }
  };

  const handleDelete = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!(await confirm(t('servers.deleteConfirm', { name: server.name, host: server.host }), { danger: true, confirmLabel: t('common.delete') }))) {
      return;
    }
    setIsDeleting(true);
    try {
      await DeleteServer(server.id);
      onRefresh();
    } catch (err) {
      console.error(err);
    } finally {
      setIsDeleting(false);
    }
  };

  const getStatusIcon = () => {
    switch (server.status) {
      case 'online':
        return <CheckCircle className="w-3.5 h-3.5 text-emerald-400" />;
      case 'offline':
        return <XCircle className="w-3.5 h-3.5 text-rose-400" />;
      default:
        return <HelpCircle className="w-3.5 h-3.5 text-amber-400" />;
    }
  };

  const formatUptime = (seconds: number) => {
    if (!seconds) return '—';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    if (days > 0) return `${days}${t('common.dayShort')} ${hours}${t('common.hourShort')}`;
    if (hours > 0) return `${hours}${t('common.hourShort')}`;
    const mins = Math.floor((seconds % 3600) / 60);
    return `${mins}${t('common.minShort')}`;
  };

  const ramGB = diagnostics ? (diagnostics.ram_total_bytes / (1024 * 1024 * 1024)).toFixed(1) : null;
  const diskTotalGB = diagnostics ? (diagnostics.disk_total_bytes / (1024 * 1024 * 1024)).toFixed(0) : null;
  const diskUsedGB = diagnostics ? (diagnostics.disk_used_bytes / (1024 * 1024 * 1024)).toFixed(0) : null;
  const diskPercent = diagnostics && diagnostics.disk_total_bytes > 0
    ? Math.round((diagnostics.disk_used_bytes / diagnostics.disk_total_bytes) * 100)
    : 0;

  return (
    <>
      <div
        onClick={() => onSelectServer && onSelectServer(server)}
        className="bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 shadow-lg transition-all hover:shadow-indigo-500/5 group flex flex-col justify-between"
      >
      <div>
        {/* Top Server Header: Icon, Name & Connection, Status Pill */}
        <div className="flex items-start justify-between gap-3 mb-2.5">
          <div className="flex items-center gap-3 min-w-0 flex-1">
            <div className="relative w-10 h-10 rounded-xl bg-slate-800 border border-slate-700/80 flex items-center justify-center text-slate-300 group-hover:border-indigo-500/50 group-hover:text-indigo-400 transition-colors shrink-0">
              <Server className="w-5 h-5" />
              <span
                className={`absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full ring-2 ring-surface-850 ${
                  server.status === 'online'
                    ? 'bg-emerald-400'
                    : server.status === 'offline'
                    ? 'bg-rose-400'
                    : 'bg-amber-400'
                }`}
              />
            </div>
            <div className="min-w-0 flex-1">
              <h3
                className="font-bold text-white text-base tracking-wide truncate group-hover:text-indigo-300 transition-colors leading-snug"
                title={maskServerName(server.name)}
              >
                {maskServerName(server.name)}
              </h3>
              <div
                className="text-xs text-slate-400 font-mono mt-0.5 truncate select-all"
                title={`${maskUser(server.username)}@${maskHost(server.host)}:${server.port}`}
              >
                {maskUser(server.username)}@{maskHost(server.host)}:{server.port}
              </div>
            </div>
          </div>

          <div className="shrink-0">
            <Badge
              variant={
                server.status === 'online'
                  ? 'success'
                  : server.status === 'offline'
                  ? 'danger'
                  : 'warning'
              }
            >
              {getStatusIcon()}
              <span>
                {server.status === 'online' ? t('servers.statusOnline') : server.status === 'offline' ? t('servers.statusOffline') : t('servers.statusUnknown')}
              </span>
            </Badge>
          </div>
        </div>

        {/* Server Attributes & Tags: Group, Mode, OS, Tags */}
        <div className="flex flex-wrap items-center gap-1.5 mb-3">
          {server.group_name && (
            <Badge variant="neutral" mono size="xs">
              {server.group_name}
            </Badge>
          )}

          <button
            onClick={handleToggleMode}
            disabled={isTogglingMode}
            className={`px-2 py-0.5 rounded text-[10px] font-semibold flex items-center gap-1 border transition-all shrink-0 ${
              server.mode === 'management'
                ? 'bg-amber-500/15 text-amber-300 border-amber-500/40 hover:bg-amber-500/25'
                : 'bg-sky-500/15 text-sky-300 border-sky-500/40 hover:bg-sky-500/25'
            }`}
            title={
              server.mode === 'management'
                ? t('servers.modeManagementTooltip')
                : t('servers.modeAuditTooltip')
            }
          >
            {isTogglingMode ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : server.mode === 'management' ? (
              <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />
            ) : (
              <Shield className="w-3.5 h-3.5 text-sky-400" />
            )}
            <span>{server.mode === 'management' ? t('servers.modeManagement') : t('servers.modeAudit')}</span>
          </button>

          {diagnostics?.os_name && (
            <Badge variant="indigo" mono size="xs">
              {diagnostics.os_name}
            </Badge>
          )}

          {server.tags && server.tags.map((tag, i) => (
            <span
              key={i}
              className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800/80 text-slate-400 font-medium border border-slate-700/50"
            >
              #{tag}
            </span>
          ))}
        </div>

        {/* Hardware & Diagnostics stats */}
        {diagnostics ? (
          <div className="grid grid-cols-3 gap-2 p-3 rounded-lg bg-slate-900/60 border border-slate-800/80 mb-3 text-xs min-w-0">
            <div>
              <div className="text-slate-400 flex items-center gap-1 mb-1">
                <Cpu className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
                <span className="text-[11px]">CPU</span>
              </div>
              <div className="font-semibold text-white font-mono text-xs tabular-nums">
                {diagnostics.cpu_cores} {t('servers.card.cores')}
              </div>
              <div className="text-[10px] text-slate-400 truncate mt-0.5" title={diagnostics.cpu_model || t('servers.card.unknown')}>
                {diagnostics.cpu_model || t('servers.card.unknown')}
              </div>
            </div>

            <div>
              <div className="text-slate-400 flex items-center gap-1 mb-1">
                <Activity className="w-3.5 h-3.5 text-cyan-400 shrink-0" />
                <span className="text-[11px]">RAM</span>
              </div>
              <div className="font-semibold text-white font-mono text-xs tabular-nums">
                {ramGB} {t('servers.card.gb')}
              </div>
              <div className="text-[10px] text-slate-400 truncate mt-0.5 tabular-nums" title={diagnostics.uptime_seconds ? `${t('servers.card.uptimePrefix')}${formatUptime(diagnostics.uptime_seconds)}` : ''}>
                {diagnostics.uptime_seconds ? formatUptime(diagnostics.uptime_seconds) : (diagnostics.os_family || 'RAM')}
              </div>
            </div>

            <div>
              <div className="text-slate-400 flex items-center gap-1 mb-1">
                <HardDrive className="w-3.5 h-3.5 text-amber-400 shrink-0" />
                <span className="text-[11px]">{t('servers.card.disk')}</span>
              </div>
              <div className="font-semibold text-white font-mono text-xs tabular-nums">
                {diskPercent}% {t('servers.card.used')}
              </div>
              <div className="text-[10px] text-slate-400 truncate mt-0.5 tabular-nums">
                {diskUsedGB} / {diskTotalGB} {t('servers.card.gb')}
              </div>
            </div>
          </div>
        ) : (
          <div className="p-3 rounded-lg bg-slate-900/40 border border-slate-800/50 mb-3 text-xs text-slate-400 text-center">
            {t('servers.card.noTelemetry')}
          </div>
        )}

        {/* Installed software badges */}
        {diagnostics && (((diagnostics.installed_webservers?.length || 0) > 0) || ((diagnostics.installed_php_versions?.length || 0) > 0) || ((diagnostics.installed_db_versions?.length || 0) > 0) || !!diagnostics.docker_version) && (
          <div className="flex flex-wrap gap-1.5 mb-3">
            {(diagnostics.installed_webservers || []).map((ws, i) => (
              <span key={`ws-${i}`} className="text-[10px] font-mono px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">
                {ws}
              </span>
            ))}
            {(diagnostics.installed_php_versions || []).map((php, i) => (
              <span key={`php-${i}`} className="text-[10px] font-mono px-2 py-0.5 rounded bg-purple-500/10 text-purple-300 border border-purple-500/20">
                PHP {php}
              </span>
            ))}
            {(diagnostics.installed_db_versions || []).map((db, i) => (
              <span key={`db-${i}`} className="text-[10px] font-mono px-2 py-0.5 rounded bg-cyan-500/10 text-cyan-300 border border-cyan-500/20">
                {db}
              </span>
            ))}
            {diagnostics.docker_version && (
              <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-blue-500/10 text-blue-300 border border-blue-500/20">
                Docker
              </span>
            )}
          </div>
        )}

        {testResult && (
          <div className={`mt-2 mb-2 p-2 rounded-lg text-xs text-center font-mono border ${
            testResult.success
              ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
              : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
          }`}>
            {testResult.message}
          </div>
        )}

        {diagResult && (
          <div className={`mt-2 mb-2 p-2 rounded-lg text-xs text-center font-mono border ${
            diagResult.kind === 'success'
              ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
              : diagResult.kind === 'error'
              ? 'bg-rose-500/10 border-rose-500/30 text-rose-300'
              : 'bg-indigo-500/10 border-indigo-500/30 text-indigo-300'
          }`}>
            {diagResult.message}
          </div>
        )}
      </div>

      {/* Action Footer */}
      <div className="pt-3 border-t border-slate-800/80 flex flex-col gap-2">
        {/* Main server actions toolbar */}
        <div className="grid grid-cols-3 gap-1.5">
          <Button
            variant="secondary"
            size="xs"
            onClick={handleTestConnection}
            disabled={isTesting}
            loading={isTesting}
            title={t('servers.card.testConnectionTooltip')}
            icon={!isTesting ? <Terminal className="w-3.5 h-3.5 text-slate-400" /> : undefined}
          >
            <span className="truncate">SSH</span>
          </Button>

          <Button
            variant="secondary"
            size="xs"
            onClick={handleRunDiagnostics}
            disabled={isDiagnosing}
            loading={isDiagnosing}
            title={t('servers.runDiagnostics')}
            icon={!isDiagnosing ? <Play className="w-3.5 h-3.5 text-indigo-400" /> : undefined}
          >
            <span className="truncate">{t('servers.runDiagnostics')}</span>
          </Button>

          <Button
            variant="secondary"
            size="xs"
            onClick={(e) => {
              e.stopPropagation();
              setIsInventoryOpen(true);
            }}
            title={t('servers.openInventory')}
            icon={<Layers className="w-3.5 h-3.5 text-indigo-400" />}
          >
            <span className="truncate">{t('servers.openInventory')}</span>
          </Button>
        </div>

        {/* Sub-bar: Connection port & Edit/Delete actions */}
        <div className="flex items-center justify-between pt-1.5 text-[11px] text-slate-400 border-t border-slate-800/40">
          <div className="flex items-center gap-1.5 font-mono text-[11px] text-slate-400 truncate">
            <span className="text-slate-500">port:</span>
            <span className="text-slate-300 font-semibold tabular-nums">{server.port}</span>
            {server.auth_type && (
              <>
                <span className="text-slate-600">•</span>
                <span className="text-slate-400">{server.auth_type === 'key' ? 'SSH key' : 'password'}</span>
              </>
            )}
          </div>

          <div className="flex items-center gap-1 shrink-0">
            {onEditServer && (
              <Button
                variant="ghost"
                size="xs"
                onClick={(e) => {
                  e.stopPropagation();
                  onEditServer(server);
                }}
                title={t('servers.editServer')}
                aria-label={t('servers.editServer')}
                icon={<Edit3 className="w-3 h-3 text-cyan-400" />}
              >
                <span>{t('common.edit')}</span>
              </Button>
            )}

            <Button
              variant="danger"
              size="xs"
              onClick={handleDelete}
              disabled={isDeleting}
              loading={isDeleting}
              title={t('servers.deleteServer')}
              aria-label={t('servers.deleteServer')}
              icon={!isDeleting ? <Trash2 className="w-3 h-3 text-rose-400" /> : undefined}
            >
              <span>{t('common.delete')}</span>
            </Button>
          </div>
        </div>
      </div>
    </div>

    <InventoryModal
      isOpen={isInventoryOpen}
      onClose={() => setIsInventoryOpen(false)}
      server={server}
    />
  </>
);
};

