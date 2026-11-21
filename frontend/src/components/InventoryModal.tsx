import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  RefreshCw,
  Server,
  Layers,
  Network,
  Users,
  HardDrive,
  Clock,
  Database,
  CheckCircle,
  AlertTriangle,
  Loader2,
  FileWarning,
  Cpu,
  Globe,
  ArrowRight,
  ShieldAlert,
  Wrench,
} from 'lucide-react';
import { ServerRecord } from '../types';
import { CollectServerInventory, GetLatestServerInventory } from '../../wailsjs/go/main/App';
import { inventory } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Tabs } from './ui';

interface InventoryModalProps {
  isOpen: boolean;
  onClose: () => void;
  server: ServerRecord;
}

export const InventoryModal: React.FC<InventoryModalProps> = ({
  isOpen,
  onClose,
  server,
}) => {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<inventory.ServerInventorySnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isCollecting, setIsCollecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'topology' | 'links' | 'apt' | 'units' | 'sockets' | 'users' | 'mounts' | 'cron' | 'databases'>('topology');
  const [filterQuery, setFilterQuery] = useState('');

  useEffect(() => {
    if (isOpen && server.id) {
      loadLatest();
    }
  }, [isOpen, server.id]);

  if (!isOpen) return null;

  const normalizeSnapshot = (snap: inventory.ServerInventorySnapshot | null): inventory.ServerInventorySnapshot | null => {
    if (!snap) return null;
    snap.links = snap.links || [];
    snap.sockets = snap.sockets || [];
    snap.units = snap.units || [];
    snap.users = snap.users || [];
    snap.mounts = snap.mounts || [];
    snap.cron_jobs = snap.cron_jobs || [];
    snap.containers = snap.containers || [];
    (snap as any).databases = (snap as any).databases || [];
    (snap as any).web_servers = (snap as any).web_servers || [];
    (snap as any).php_runtimes = (snap as any).php_runtimes || [];
    (snap as any).site_chains = (snap as any).site_chains || [];
    (snap as any).db_instances = (snap as any).db_instances || [];
    (snap as any).apt = (snap as any).apt || { sources: [], missing_gpg_keys: [], codename_mismatches: [], broken_packages: [], duplicate_sources: [], has_errors: false, summary: '' };
    return snap;
  };

  const loadLatest = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const snap = await GetLatestServerInventory(server.id);
      if (snap) {
        setSnapshot(normalizeSnapshot(snap));
      } else {
        // If none collected yet, trigger collection
        await triggerCollect();
      }
    } catch {
      // If fetching latest failed or no snapshot exists yet, trigger collection
      await triggerCollect();
    } finally {
      setIsLoading(false);
    }
  };

  const triggerCollect = async () => {
    setIsCollecting(true);
    setError(null);
    try {
      const snap = await CollectServerInventory(server.id);
      setSnapshot(normalizeSnapshot(snap));
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsCollecting(false);
    }
  };

  const links = snapshot?.links || [];
  const sockets = snapshot?.sockets || [];
  const units = snapshot?.units || [];
  const users = snapshot?.users || [];
  const mounts = snapshot?.mounts || [];
  const cronJobs = snapshot?.cron_jobs || [];
  const containers = snapshot?.containers || [];
  const databases: any[] = (snapshot as any)?.databases || [];
  const webServers: any[] = (snapshot as any)?.web_servers || [];
  const phpRuntimes: any[] = (snapshot as any)?.php_runtimes || [];
  const siteChains: any[] = (snapshot as any)?.site_chains || [];
  const aptDiag: any = (snapshot as any)?.apt || null;
  const osInfo: any = (snapshot as any)?.os || null;

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i];
  };

  return (
    <ModalShell
      onClose={onClose}
      size="6xl"
      className="h-[85vh]"
      icon={<Layers className="w-5 h-5" />}
      iconClassName="p-2.5 rounded-xl bg-indigo-600/20 text-indigo-400 border border-indigo-500/30 shrink-0"
      title={t('inventory.modalTitle')}
      headerExtra={
        <>
          <span className="text-xs font-mono px-2 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
            {server.name}
          </span>
          <span
            className={`text-[10px] uppercase font-bold px-2 py-0.5 rounded border ${
              server.mode === 'management'
                ? 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                : 'bg-sky-500/15 text-sky-300 border-sky-500/30'
            }`}
          >
            {server.mode === 'management' ? t('inventory.modeManagement') : t('inventory.modeAudit')}
          </span>
        </>
      }
      subtitle={
        snapshot
          ? t('inventory.snapshotFrom', { date: new Date(snapshot.created_at).toLocaleString() })
          : t('inventory.loadingTelemetry')
      }
      headerActions={
        <button
          onClick={triggerCollect}
          disabled={isCollecting || isLoading}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 text-xs font-medium transition disabled:opacity-50"
          title={t('inventory.rescanTooltip')}
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isCollecting ? 'animate-spin' : ''}`} />
          <span>{isCollecting ? t('inventory.collecting') : t('inventory.refreshSnapshot')}</span>
        </button>
      }
    >

        {/* Navigation Tabs */}
        <div className="px-6 py-2.5 border-b border-slate-800 bg-slate-950/90 shrink-0">
          <Tabs
            variant="pill"
            size="sm"
            activeTab={activeTab}
            onChange={(id) => setActiveTab(id as any)}
            className="flex-wrap bg-transparent border-0 p-0 gap-1.5"
            tabs={[
              {
                id: 'topology',
                label: t('inventory.tabTopology'),
                icon: <Layers className="w-3.5 h-3.5" />,
                badge: (webServers.length > 0 || siteChains.length > 0) ? (webServers.length + siteChains.length) : undefined,
              },
              {
                id: 'apt',
                label: t('inventory.tabApt'),
                icon: <ShieldAlert className={`w-3.5 h-3.5 ${aptDiag?.has_errors ? 'text-amber-400' : ''}`} />,
                badge: aptDiag?.has_errors ? '!' : undefined,
              },
              {
                id: 'links',
                label: t('inventory.tabLinks'),
                icon: <CheckCircle className="w-3.5 h-3.5" />,
                badge: links.length > 0 ? links.length : undefined,
              },
              {
                id: 'sockets',
                label: t('inventory.tabSockets'),
                icon: <Network className="w-3.5 h-3.5" />,
                badge: sockets.length > 0 ? sockets.length : undefined,
              },
              {
                id: 'units',
                label: t('inventory.tabServices'),
                icon: <Server className="w-3.5 h-3.5" />,
                badge: units.length > 0 ? units.length : undefined,
              },
              {
                id: 'users',
                label: t('inventory.tabUsers'),
                icon: <Users className="w-3.5 h-3.5" />,
                badge: users.length > 0 ? users.length : undefined,
              },
              {
                id: 'mounts',
                label: t('inventory.tabDisks'),
                icon: <HardDrive className="w-3.5 h-3.5" />,
                badge: mounts.length > 0 ? mounts.length : undefined,
              },
              {
                id: 'cron',
                label: t('inventory.tabCronDocker'),
                icon: <Clock className="w-3.5 h-3.5" />,
                badge: (cronJobs.length > 0 || containers.length > 0) ? (cronJobs.length + containers.length) : undefined,
              },
              {
                id: 'databases',
                label: t('inventory.tabDatabases'),
                icon: <Database className="w-3.5 h-3.5" />,
                badge: databases.length > 0 ? databases.length : undefined,
              },
            ]}
          />
        </div>

        {/* Error message - edge-to-edge */}
        {error && (
          <div className="px-6 py-2.5 bg-rose-500/15 border-b border-rose-500/30 text-rose-300 text-xs flex items-center gap-2 shrink-0">
            <AlertTriangle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Content Body */}
        <div className="p-6 overflow-y-auto flex-1 space-y-4 min-h-0">
          {isLoading && !snapshot ? (
            <div className="py-20 flex flex-col items-center justify-center text-slate-400 space-y-3">
              <Loader2 className="w-8 h-8 animate-spin text-indigo-500" />
              <p className="text-sm">{t('inventory.readingInventory')}</p>
            </div>
          ) : !snapshot ? (
            <div className="py-20 text-center text-slate-500">
              {t('inventory.noInventoryData')}
            </div>
          ) : (
            <>
              {/* Tab 0: Stack Topology & Architecture */}
              {activeTab === 'topology' && (
                <div className="space-y-6">
                  {/* OS & Host Summary */}
                  {osInfo && (
                    <div className="p-4 rounded-xl bg-slate-900/90 border border-slate-800 flex flex-wrap items-center justify-between gap-4 text-xs">
                      <div className="flex items-center gap-3">
                        <div className="p-2 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
                          <Cpu className="w-4 h-4" />
                        </div>
                        <div>
                          <div className="font-bold text-white text-sm flex items-center gap-2">
                            <span>{osInfo.distro || 'Linux'} {osInfo.version}</span>
                            {osInfo.codename && (
                              <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                                {osInfo.codename}
                              </span>
                            )}
                          </div>
                          <div className="text-slate-400 text-xs font-mono mt-0.5">
                            Kernel: {osInfo.kernel || 'unknown'} • Arch: {osInfo.architecture || 'unknown'}
                          </div>
                        </div>
                      </div>

                      {osInfo.current_user && (
                        <div className="flex items-center gap-2 bg-slate-950 px-3 py-1.5 rounded-lg border border-slate-800 font-mono text-xs">
                          <Users className="w-3.5 h-3.5 text-slate-400" />
                          <span className="text-slate-300">
                            {osInfo.current_user.username} (UID {osInfo.current_user.uid})
                          </span>
                          {osInfo.current_user.has_sudo ? (
                            <span className="px-1.5 py-0.2 rounded text-[10px] bg-emerald-500/15 text-emerald-400 font-bold border border-emerald-500/30">
                              SUDO
                            </span>
                          ) : (
                            <span className="px-1.5 py-0.2 rounded text-[10px] bg-slate-800 text-slate-400 font-bold border border-slate-700">
                              NON-ROOT
                            </span>
                          )}
                        </div>
                      )}
                    </div>
                  )}

                  {/* Section: Web Server Instances */}
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider flex items-center gap-2">
                        <Server className="w-3.5 h-3.5 text-indigo-400" />
                        <span>Экземпляры веб-серверов ({webServers.length})</span>
                      </h4>
                    </div>

                    {webServers.length === 0 ? (
                      <div className="p-6 text-center text-slate-500 border border-dashed border-slate-800 rounded-xl text-xs">
                        Веб-серверы не обнаружены
                      </div>
                    ) : (
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                        {webServers.map((ws: any, idx: number) => (
                          <div key={idx} className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-3">
                            <div className="flex items-start justify-between gap-2">
                              <div>
                                <div className="font-bold text-white text-sm flex items-center gap-2">
                                  <span>{ws.name || ws.engine}</span>
                                  {ws.version && (
                                    <span className="text-xs font-mono text-indigo-300 bg-indigo-500/10 px-1.5 py-0.5 rounded border border-indigo-500/20">
                                      v{ws.version}
                                    </span>
                                  )}
                                </div>
                                <div className="text-[11px] text-slate-400 font-mono mt-0.5 truncate" title={ws.binary_path}>
                                  {ws.binary_path}
                                </div>
                              </div>
                              <div className="flex items-center gap-1.5">
                                <span
                                  className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${
                                    ws.install_type === 'source'
                                      ? 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                                      : 'bg-sky-500/15 text-sky-300 border-sky-500/30'
                                  }`}
                                >
                                  {ws.install_type === 'source' ? 'Source /opt' : 'APT pkg'}
                                </span>
                                <span
                                  className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${
                                    ws.status === 'active'
                                      ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                                      : 'bg-slate-800 text-slate-400 border-slate-700'
                                  }`}
                                >
                                  {ws.status}
                                </span>
                              </div>
                            </div>

                            <div className="space-y-1.5 text-xs pt-2 border-t border-slate-800/70 font-mono text-slate-300">
                              {ws.prefix_dir && (
                                <div className="flex justify-between">
                                  <span className="text-slate-500">Prefix:</span>
                                  <span className="truncate max-w-[240px]" title={ws.prefix_dir}>{ws.prefix_dir}</span>
                                </div>
                              )}
                              {ws.config_path && (
                                <div className="flex justify-between">
                                  <span className="text-slate-500">Config:</span>
                                  <span className="truncate max-w-[240px]" title={ws.config_path}>{ws.config_path}</span>
                                </div>
                              )}
                              <div className="flex justify-between">
                                <span className="text-slate-500">Ports:</span>
                                <span>{(ws.listening_ports || []).join(', ') || 'none'}</span>
                              </div>
                              {ws.systemd_unit && (
                                <div className="flex justify-between">
                                  <span className="text-slate-500">Unit:</span>
                                  <span className="text-slate-400">{ws.systemd_unit}</span>
                                </div>
                              )}
                            </div>

                            {ws.test_command && (
                              <div className="pt-2 border-t border-slate-800/70">
                                <span className="text-[10px] text-slate-500 block uppercase font-mono">Test Command:</span>
                                <code className="text-[11px] text-emerald-400 font-mono block bg-slate-950 px-2 py-1 rounded border border-slate-800 truncate mt-1" title={ws.test_command}>
                                  {ws.test_command}
                                </code>
                              </div>
                            )}

                            {ws.limitations && ws.limitations.length > 0 && (
                              <div className="p-2 rounded bg-amber-500/10 border border-amber-500/20 text-amber-300 text-[11px] space-y-1">
                                {ws.limitations.map((lim: string, lIdx: number) => (
                                  <div key={lIdx} className="flex items-center gap-1.5">
                                    <AlertTriangle className="w-3 h-3 shrink-0" />
                                    <span>{lim}</span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Section: PHP Runtimes */}
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider flex items-center gap-2">
                        <Wrench className="w-3.5 h-3.5 text-indigo-400" />
                        <span>PHP Runtimes ({phpRuntimes.length})</span>
                      </h4>
                    </div>

                    {phpRuntimes.length === 0 ? (
                      <div className="p-6 text-center text-slate-500 border border-dashed border-slate-800 rounded-xl text-xs">
                        PHP runtimes не обнаружены
                      </div>
                    ) : (
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        {phpRuntimes.map((rt: any, idx: number) => (
                          <div
                            key={idx}
                            className={`p-3.5 rounded-xl border space-y-2 ${
                              rt.is_orphaned
                                ? 'bg-rose-950/20 border-rose-500/30'
                                : 'bg-slate-900 border-slate-800'
                            }`}
                          >
                            <div className="flex items-start justify-between gap-1.5">
                              <div>
                                <span className="font-bold text-white text-sm">
                                  PHP {rt.version || rt.major_minor}
                                </span>
                                <span className="text-[10px] block font-mono text-slate-400 uppercase mt-0.5">
                                  {rt.kind === 'mod_php' ? 'Apache mod_php' : rt.kind}
                                </span>
                              </div>
                              <span
                                className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${
                                  rt.is_orphaned
                                    ? 'bg-rose-500/20 text-rose-300 border-rose-500/40'
                                    : rt.status === 'active'
                                    ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                                    : 'bg-slate-800 text-slate-400 border-slate-700'
                                }`}
                              >
                                {rt.is_orphaned ? 'Оставшаяся служба' : rt.status}
                              </span>
                            </div>

                            {rt.is_orphaned ? (
                              <div className="p-2 rounded bg-rose-500/10 border border-rose-500/20 text-rose-300 text-[11px] flex items-start gap-1.5">
                                <AlertTriangle className="w-3.5 h-3.5 shrink-0 mt-0.5" />
                                <span>Служба {rt.systemd_unit} осталась после удаления пакетов (бинарник отсутствует)</span>
                              </div>
                            ) : (
                              <div className="text-xs font-mono space-y-1 pt-2 border-t border-slate-800/70 text-slate-300">
                                {rt.binary_path && (
                                  <div className="text-[11px] truncate text-slate-400" title={rt.binary_path}>
                                    {rt.binary_path}
                                  </div>
                                )}
                                {(rt.sockets || []).length > 0 && (
                                  <div className="text-[11px] text-emerald-400 truncate">
                                    Sock: {rt.sockets[0]}
                                  </div>
                                )}
                                {rt.systemd_unit && (
                                  <div className="text-[11px] text-slate-500">
                                    Unit: {rt.systemd_unit}
                                  </div>
                                )}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Section: Site Request Topology Chains */}
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider flex items-center gap-2">
                        <Globe className="w-3.5 h-3.5 text-indigo-400" />
                        <span>Сквозные маршруты запросов сайтов ({siteChains.length})</span>
                      </h4>
                    </div>

                    {siteChains.length === 0 ? (
                      <div className="p-6 text-center text-slate-500 border border-dashed border-slate-800 rounded-xl text-xs">
                        Маршруты сайтов еще не сопоставлены
                      </div>
                    ) : (
                      <div className="space-y-3">
                        {siteChains.map((chain: any, idx: number) => (
                          <div key={idx} className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-3">
                            <div className="flex items-start justify-between gap-2">
                              <div>
                                <span className="font-bold text-white text-base">{chain.domain}</span>
                                <div className="text-xs font-mono text-slate-400 mt-0.5">
                                  Vhost: {chain.vhost_path || 'не определен'}
                                </div>
                              </div>
                              <span
                                className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${
                                  chain.confidence === 'proven'
                                    ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                                    : chain.confidence === 'inferred'
                                    ? 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                                    : 'bg-rose-500/15 text-rose-300 border-rose-500/30'
                                }`}
                              >
                                {chain.confidence === 'proven' ? 'Связь доказана' : chain.confidence === 'inferred' ? 'Предположительно' : 'Не доказана'}
                              </span>
                            </div>

                            {/* Visual Chain Flow */}
                            <div className="p-3 rounded-lg bg-slate-950 border border-slate-800 flex flex-wrap items-center gap-2 text-xs font-mono">
                              <span className="px-2.5 py-1 rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                                {chain.web_server_engine || 'Web Server'} (:{ (chain.listen_ports || [80]).join('/') })
                              </span>
                              <ArrowRight className="w-3.5 h-3.5 text-slate-500" />
                              <span className="px-2.5 py-1 rounded bg-slate-800 text-slate-300 border border-slate-700">
                                {chain.backend_type === 'mod_php'
                                  ? 'Apache mod_php'
                                  : chain.backend_type === 'fastcgi_fpm'
                                  ? `FastCGI (${chain.backend_target})`
                                  : chain.backend_type === 'reverse_proxy'
                                  ? `Proxy (${chain.backend_target})`
                                  : 'Static'}
                              </span>
                              {chain.php_version && (
                                <>
                                  <ArrowRight className="w-3.5 h-3.5 text-slate-500" />
                                  <span className="px-2.5 py-1 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                                    PHP {chain.php_version}
                                  </span>
                                </>
                              )}
                            </div>

                            {chain.warnings && chain.warnings.length > 0 && (
                              <div className="p-2 rounded bg-amber-500/10 border border-amber-500/20 text-amber-300 text-xs space-y-1">
                                {chain.warnings.map((w: string, wIdx: number) => (
                                  <div key={wIdx} className="flex items-center gap-1.5">
                                    <AlertTriangle className="w-3.5 h-3.5 shrink-0" />
                                    <span>{w}</span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Tab: APT Diagnostics */}
              {activeTab === 'apt' && (
                <div className="space-y-4">
                  {/* Summary / Alerts */}
                  {aptDiag?.has_errors ? (
                    <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 space-y-2 text-xs">
                      <div className="flex items-center gap-2 font-bold text-sm text-rose-200">
                        <AlertTriangle className="w-4 h-4 shrink-0" />
                        <span>Обнаружены проблемы в конфигурации репозиториев пакетов</span>
                      </div>
                      {aptDiag.codename_mismatches?.length > 0 && (
                        <div className="pt-1">
                          <span className="font-bold text-amber-300 block">Несовпадение кодовых имен дистрибутива:</span>
                          <ul className="list-disc list-inside mt-0.5 space-y-0.5 text-amber-200 font-mono">
                            {aptDiag.codename_mismatches.map((m: string, idx: number) => (
                              <li key={idx}>{m}</li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {aptDiag.missing_gpg_keys?.length > 0 && (
                        <div className="pt-1">
                          <span className="font-bold text-rose-300 block">Отсутствующие открытые GPG-ключи:</span>
                          <div className="flex flex-wrap gap-1.5 mt-1 font-mono">
                            {aptDiag.missing_gpg_keys.map((k: string, idx: number) => (
                              <span key={idx} className="px-2 py-0.5 rounded bg-rose-950 text-rose-300 border border-rose-500/40">
                                NO_PUBKEY {k}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  ) : (
                    <div className="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs flex items-center gap-2">
                      <CheckCircle className="w-4 h-4 shrink-0" />
                      <span>{aptDiag?.summary || 'Все репозитории пакетов согласованы с версией операционной системы.'}</span>
                    </div>
                  )}

                  {/* Sources List Table */}
                  <div className="space-y-2">
                    <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider">
                      Источники APT ({aptDiag?.sources?.length || 0})
                    </h4>
                    {(!aptDiag?.sources || aptDiag.sources.length === 0) ? (
                      <div className="p-6 text-center text-slate-500 border border-dashed border-slate-800 rounded-xl text-xs">
                        Источники APT не прочитаны
                      </div>
                    ) : (
                      <div className="border border-slate-800 rounded-xl overflow-hidden bg-slate-900/60 text-xs">
                        <table className="w-full text-left font-mono">
                          <thead className="bg-slate-950 text-slate-400 text-[11px] uppercase border-b border-slate-800">
                            <tr>
                              <th className="px-3 py-2">URL репозитория</th>
                              <th className="px-3 py-2">Codename</th>
                              <th className="px-3 py-2">Компоненты</th>
                              <th className="px-3 py-2">Статус</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-800/60">
                            {aptDiag.sources.map((src: any, idx: number) => (
                              <tr key={idx} className="hover:bg-slate-800/30">
                                <td className="px-3 py-2 text-slate-200 truncate max-w-[280px]" title={src.url}>{src.url}</td>
                                <td className="px-3 py-2">
                                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold ${
                                    src.is_mismatch ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40' : 'text-slate-400'
                                  }`}>
                                    {src.codename}
                                    {src.is_mismatch && ' (mismatch!)'}
                                  </span>
                                </td>
                                <td className="px-3 py-2 text-slate-400">{src.component}</td>
                                <td className="px-3 py-2">
                                  <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase ${
                                    src.status === 'error'
                                      ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40'
                                      : src.status === 'warning'
                                      ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40'
                                      : 'bg-emerald-500/15 text-emerald-400'
                                  }`}>
                                    {src.status}
                                  </span>
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Tab 1: Links (Proven vs Unproven & Drift) */}
              {activeTab === 'links' && (
                <div className="space-y-3">
                  <div className="text-xs text-slate-400">
                    {t('inventory.linksExplanation')}
                  </div>

                  {links.length === 0 ? (
                    <div className="p-8 text-center text-slate-500 border border-dashed border-slate-800 rounded-xl">
                      {t('inventory.noSitesOnServer')}
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {links.map((link: any, idx) => (
                        <div
                          key={idx}
                          className={`p-4 rounded-xl border transition-all ${
                            link.has_drift
                              ? 'bg-rose-950/20 border-rose-500/40'
                              : link.status === 'proven'
                              ? 'bg-slate-900/80 border-slate-800'
                              : 'bg-amber-950/20 border-amber-500/30'
                          }`}
                        >
                          <div className="flex items-start justify-between gap-3">
                            <div className="space-y-1">
                              <div className="flex items-center gap-2">
                                <span className="font-bold text-white text-sm">{link.site_domain}</span>
                                <span
                                  className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider border ${
                                    link.status === 'proven'
                                      ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                                      : 'bg-amber-500/15 text-amber-400 border-amber-500/30'
                                  }`}
                                >
                                  {link.status === 'proven' ? t('inventory.provenLink') : t('inventory.unprovenLink')}
                                </span>
                                {link.has_drift && (
                                  <span className="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-rose-500/20 text-rose-300 border border-rose-500/40 flex items-center gap-1">
                                    <FileWarning className="w-3 h-3" />
                                    {t('inventory.driftVhost')}
                                  </span>
                                )}
                              </div>
                              <div className="text-xs text-slate-400 font-mono">
                                {t('inventory.vhost', { path: link.vhost_path || t('inventory.notDefined') })}
                              </div>
                            </div>
                          </div>

                          <div className="grid grid-cols-2 gap-3 mt-3 pt-3 border-t border-slate-800/80 text-xs">
                            <div>
                              <span className="text-slate-500 text-[11px] block">{t('inventory.fpmSocketAndPool')}</span>
                              <div className="font-mono text-slate-300 mt-0.5">
                                {link.fpm_socket ? `${link.fpm_socket} (${link.pool_name || t('inventory.noPool')})` : t('inventory.notUsed')}
                              </div>
                            </div>
                            <div>
                              <span className="text-slate-500 text-[11px] block">{t('inventory.sslCert')}</span>
                              <div className="font-mono text-slate-300 mt-0.5 truncate" title={link.cert_path}>
                                {link.cert_path || t('inventory.notAttached')}
                              </div>
                            </div>
                          </div>

                          {link.note && (
                            <div className="mt-2 text-xs text-amber-400/90 bg-amber-500/10 px-2.5 py-1.5 rounded-lg border border-amber-500/20">
                              {link.note}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* Tab 2: Sockets */}
              {activeTab === 'sockets' && (
                <div className="space-y-3">
                  <div className="overflow-x-auto rounded-xl border border-slate-800">
                    <table className="w-full text-xs text-left">
                      <thead className="bg-slate-950/80 text-slate-400 border-b border-slate-800">
                        <tr>
                          <th className="px-3.5 py-2.5">{t('inventory.colProtocol')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colPort')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colListenAddress')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colProcess')}</th>
                          <th className="px-3.5 py-2.5">PID</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800/60 font-mono">
                        {sockets.map((sock, i) => (
                          <tr key={i} className="hover:bg-slate-800/30">
                            <td className="px-3.5 py-2 uppercase text-slate-400">{sock.proto}</td>
                            <td className="px-3.5 py-2 font-bold text-indigo-400">{sock.port}</td>
                            <td className="px-3.5 py-2 text-slate-300">{sock.local}</td>
                            <td className="px-3.5 py-2 text-emerald-400">{sock.process || '-'}</td>
                            <td className="px-3.5 py-2 text-slate-400">{sock.pid > 0 ? sock.pid : '-'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* Tab 3: Units */}
              {activeTab === 'units' && (
                <div className="space-y-3">
                  <div className="overflow-x-auto rounded-xl border border-slate-800">
                    <table className="w-full text-xs text-left">
                      <thead className="bg-slate-950/80 text-slate-400 border-b border-slate-800">
                        <tr>
                          <th className="px-3.5 py-2.5">{t('inventory.colUnit')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colStatus')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colDescription')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800/60">
                        {units.map((u, i) => (
                          <tr key={i} className="hover:bg-slate-800/30">
                            <td className="px-3.5 py-2 font-mono text-indigo-300 font-semibold">{u.unit}</td>
                            <td className="px-3.5 py-2">
                              <span
                                className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase ${
                                  u.active === 'active'
                                    ? 'bg-emerald-500/15 text-emerald-400'
                                    : 'bg-slate-800 text-slate-400'
                                }`}
                              >
                                {u.active} ({u.sub})
                              </span>
                            </td>
                            <td className="px-3.5 py-2 text-slate-400 text-xs">{u.description}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* Tab 4: Users */}
              {activeTab === 'users' && (
                <div className="space-y-3">
                  <div className="overflow-x-auto rounded-xl border border-slate-800">
                    <table className="w-full text-xs text-left">
                      <thead className="bg-slate-950/80 text-slate-400 border-b border-slate-800">
                        <tr>
                          <th className="px-3.5 py-2.5">{t('inventory.colUser')}</th>
                          <th className="px-3.5 py-2.5">UID / GID</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colType')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colHomeDir')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colShell')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800/60 font-mono">
                        {users.map((user, i) => (
                          <tr key={i} className="hover:bg-slate-800/30">
                            <td className="px-3.5 py-2 font-bold text-white">{user.username}</td>
                            <td className="px-3.5 py-2 text-slate-400">{user.uid}:{user.gid}</td>
                            <td className="px-3.5 py-2">
                              <span
                                className={`px-1.5 py-0.5 rounded text-[10px] font-sans font-semibold ${
                                  user.is_system
                                    ? 'bg-slate-800 text-slate-400'
                                    : 'bg-emerald-500/15 text-emerald-400'
                                }`}
                              >
                                {user.is_system ? t('inventory.typeSystem') : t('inventory.typeUser')}
                              </span>
                            </td>
                            <td className="px-3.5 py-2 text-slate-300">{user.home}</td>
                            <td className="px-3.5 py-2 text-slate-400">{user.shell}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* Tab 5: Mounts */}
              {activeTab === 'mounts' && (
                <div className="space-y-3">
                  <div className="overflow-x-auto rounded-xl border border-slate-800">
                    <table className="w-full text-xs text-left">
                      <thead className="bg-slate-950/80 text-slate-400 border-b border-slate-800">
                        <tr>
                          <th className="px-3.5 py-2.5">{t('inventory.colMountPoint')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colFs')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colType')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colFree')}</th>
                          <th className="px-3.5 py-2.5">{t('inventory.colTotal')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800/60 font-mono">
                        {mounts.map((m, i) => (
                          <tr key={i} className="hover:bg-slate-800/30">
                            <td className="px-3.5 py-2 font-bold text-white">{m.mount}</td>
                            <td className="px-3.5 py-2 text-slate-400">{m.filesystem}</td>
                            <td className="px-3.5 py-2 text-indigo-400">{m.type}</td>
                            <td className="px-3.5 py-2 text-emerald-400">{formatBytes(m.free_bytes)}</td>
                            <td className="px-3.5 py-2 text-slate-300">{formatBytes(m.total_bytes)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* Tab 6: Cron & Containers */}
              {activeTab === 'cron' && (
                <div className="space-y-4">
                  <div>
                    <h3 className="text-xs font-bold text-slate-300 uppercase tracking-wider mb-2">
                      {t('inventory.cronRootHeading')}
                    </h3>
                    {cronJobs.length === 0 ? (
                      <p className="text-xs text-slate-500">{t('inventory.noCronTasks')}</p>
                    ) : (
                      <div className="space-y-1.5 font-mono text-xs">
                        {cronJobs.map((c, i) => (
                          <div key={i} className="p-2.5 rounded-lg bg-slate-950 border border-slate-800 flex items-center justify-between">
                            <span className="text-amber-400 font-bold shrink-0">{c.schedule}</span>
                            <span className="text-slate-300 flex-1 ml-4 truncate" title={c.command}>
                              {c.command}
                            </span>
                            <span className="ml-3 shrink-0 px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700 text-slate-400 text-[10px] font-mono">
                              {c.user}
                              {c.source && c.source !== 'crontab' ? ` · ${c.source}` : ''}
                            </span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div>
                    <h3 className="text-xs font-bold text-slate-300 uppercase tracking-wider mb-2">
                      {t('inventory.dockerContainersHeading')}
                    </h3>
                    {containers.length === 0 ? (
                      <p className="text-xs text-slate-500">{t('inventory.dockerNotRunning')}</p>
                    ) : (
                      <div className="overflow-x-auto rounded-xl border border-slate-800">
                        <table className="w-full text-xs text-left font-mono">
                          <thead className="bg-slate-950/80 text-slate-400 border-b border-slate-800">
                            <tr>
                              <th className="px-3 py-2">ID</th>
                              <th className="px-3 py-2">{t('inventory.colName')}</th>
                              <th className="px-3 py-2">{t('inventory.colImage')}</th>
                              <th className="px-3 py-2">{t('inventory.colStatus')}</th>
                              <th className="px-3 py-2">{t('inventory.colPorts')}</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-800/60">
                            {containers.map((c, i) => (
                              <tr key={i} className="hover:bg-slate-800/30">
                                <td className="px-3 py-1.5 text-slate-400">{(c.id || '').substring(0, 12)}</td>
                                <td className="px-3 py-1.5 text-emerald-400 font-bold">{c.names || '-'}</td>
                                <td className="px-3 py-1.5 text-slate-300">{c.image || '-'}</td>
                                <td className="px-3 py-1.5 text-indigo-300">{c.status || '-'}</td>
                                <td className="px-3 py-1.5 text-amber-300">{c.ports || '-'}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Tab 7: Databases */}
              {activeTab === 'databases' && (
                <div className="space-y-3">
                  {databases.length === 0 ? (
                    <div className="p-8 text-center text-slate-500 border border-dashed border-slate-800 rounded-xl">
                      {t('inventory.dbmsNotRunning')}
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {databases.map((db: any, i: number) => (
                        <div key={i} className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-2">
                          <div className="flex items-center justify-between">
                            <span className="font-bold text-white text-sm uppercase">{db.engine}</span>
                            <span className="text-xs font-mono text-slate-400">{db.version}</span>
                          </div>
                          <div className="pt-2 border-t border-slate-800">
                            <span className="text-xs text-slate-500 block mb-1">{t('inventory.detectedDbs')}</span>
                            <div className="flex flex-wrap gap-1.5">
                              {(db.databases || []).map((name: string, idx: number) => (
                                <span
                                  key={idx}
                                  className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono text-xs border border-slate-700"
                                >
                                  {name}
                                </span>
                              ))}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/90 flex justify-end shrink-0">
          <button
            onClick={onClose}
            className="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition"
          >{t('common.close')}</button>
        </div>
    </ModalShell>
  );
};
