import React, { useState, useMemo } from 'react';
import {
  Server,
  Activity,
  Cpu,
  HardDrive,
  CheckCircle2,
  XCircle,
  Clock,
  ListTodo,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Layers,
  ArrowUpRight,
  LayoutDashboard,
  Globe,
  Terminal,
  Folder,
  Database,
  RefreshCw,
  Search,
  Plus,
  Check,
  Copy,
  Eye,
  Box,
  Code,
} from 'lucide-react';
import { DashboardStats, ServerRecord, TaskRecord, DiagnosticsRecord, NavigationTab } from '../types';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import {
  Button,
  Badge,
  SearchInput,
  Tabs,
  Select,
  PageToolbar,
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
  TableEmpty,
} from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';
import { localizeTaskTitle, localizeTaskStep } from '../utils/taskI18n';
import { useServer } from '../context/ServerContext';
import { usePrivacy } from '../context/PrivacyContext';

interface DashboardViewProps {
  stats: DashboardStats | null;
  servers: ServerRecord[];
  diagnosticsMap?: Record<string, DiagnosticsRecord>;
  isRefreshing?: boolean;
  onRefresh?: () => void;
  onSelectTab: (tab: NavigationTab) => void;
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
  onEditServer?: (server: ServerRecord) => void;
}

export const DashboardView: React.FC<DashboardViewProps> = ({
  stats,
  servers,
  diagnosticsMap = {},
  isRefreshing = false,
  onRefresh,
  onSelectTab,
  onOpenTask,
  onOpenAddServer,
  onEditServer,
}) => {
  const { t } = useTranslation();
  const { selectedServerId, setSelectedServerId, selectedServer } = useServer();
  const { maskServerName, maskHost } = usePrivacy();

  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'online' | 'offline'>('all');
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const handleCopy = (text: string, id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const formatUptime = (seconds?: number) => {
    if (!seconds) return '—';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    if (days > 0) return `${days}${t('common.dayShort')} ${hours}${t('common.hourShort')}`;
    if (hours > 0) return `${hours}${t('common.hourShort')}`;
    const mins = Math.floor((seconds % 3600) / 60);
    return `${mins}${t('common.minShort')}`;
  };

  const getDiskInfo = (diag?: DiagnosticsRecord) => {
    if (!diag || !diag.disk_total_bytes || diag.disk_total_bytes <= 0) return null;
    const totalGB = (diag.disk_total_bytes / (1024 * 1024 * 1024)).toFixed(0);
    const usedGB = (diag.disk_used_bytes / (1024 * 1024 * 1024)).toFixed(0);
    const percent = Math.min(100, Math.round((diag.disk_used_bytes / diag.disk_total_bytes) * 100));
    return { totalGB, usedGB, percent };
  };

  const getRamGB = (diag?: DiagnosticsRecord) => {
    if (!diag || !diag.ram_total_bytes || diag.ram_total_bytes <= 0) return null;
    return (diag.ram_total_bytes / (1024 * 1024 * 1024)).toFixed(1);
  };

  const formatRelativeTime = (isoString?: string) => {
    if (!isoString) return '';
    try {
      const diffMs = Date.now() - new Date(isoString).getTime();
      const diffSec = Math.floor(diffMs / 1000);
      if (diffSec < 60) return t('common.justNow') || 'только что';
      const diffMin = Math.floor(diffSec / 60);
      if (diffMin < 60) return `${diffMin} м`;
      const diffHours = Math.floor(diffMin / 60);
      if (diffHours < 24) return `${diffHours} ч`;
      return new Date(isoString).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
      return '';
    }
  };

  // Fleet aggregate metrics
  const totalRAMGB = stats ? (stats.total_ram_bytes / (1024 * 1024 * 1024)).toFixed(1) : '0';
  const totalDiskGB = stats ? (stats.total_disk_bytes / (1024 * 1024 * 1024)).toFixed(0) : '0';
  const usedDiskGB = stats ? (stats.total_disk_used / (1024 * 1024 * 1024)).toFixed(0) : '0';
  const diskPercent = stats && stats.total_disk_bytes > 0
    ? Math.round((stats.total_disk_used / stats.total_disk_bytes) * 100)
    : 0;

  // Active target server diagnostics
  const activeDiag = selectedServer
    ? diagnosticsMap[selectedServer.id] || stats?.server_diagnostics?.find((d) => d.server_id === selectedServer.id)
    : undefined;
  const activeDisk = getDiskInfo(activeDiag);
  const activeRamGB = getRamGB(activeDiag);

  // Filtered servers
  const filteredServers = useMemo(() => {
    return servers.filter((s) => {
      if (statusFilter !== 'all' && s.status !== statusFilter) return false;
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase();
      return (
        s.name.toLowerCase().includes(q) ||
        s.host.toLowerCase().includes(q) ||
        (s.group_name && s.group_name.toLowerCase().includes(q))
      );
    });
  }, [servers, statusFilter, searchQuery]);

  // Aggregated software stack across all nodes
  const softwareStack = useMemo(() => {
    const webservers = new Set<string>();
    const phpVersions = new Set<string>();
    const dbVersions = new Set<string>();
    let dockerNodesCount = 0;

    const allDiags = Object.values(diagnosticsMap);
    (allDiags.length > 0 ? allDiags : stats?.server_diagnostics || []).forEach((diag) => {
      (diag.installed_webservers || []).forEach((ws) => ws && webservers.add(ws.toLowerCase()));
      (diag.installed_php_versions || []).forEach((pv) => pv && phpVersions.add(pv));
      (diag.installed_db_versions || []).forEach((db) => db && dbVersions.add(db));
      if (diag.docker_version) dockerNodesCount++;
    });

    return {
      webservers: Array.from(webservers),
      phpVersions: Array.from(phpVersions).sort(),
      dbVersions: Array.from(dbVersions),
      dockerNodesCount,
    };
  }, [diagnosticsMap, stats]);

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={LayoutDashboard}
        description={t('empty.dashboardDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  const allHealthy = !stats || stats.offline_servers === 0;

  return (
    <div className="page space-y-6">
      {/* Top Header & Fast Actions */}
      <PageToolbar
        icon={<LayoutDashboard className="w-5 h-5 text-indigo-400" />}
        title={
          <div className="flex flex-wrap items-center gap-2.5">
            <span>{t('dashboard.overviewTitle') || 'Обзор инфраструктуры'}</span>
            {allHealthy ? (
              <Badge variant="success" dot pulse size="sm">
                {stats?.online_servers || servers.length} / {stats?.total_servers || servers.length} {t('dashboard.fleetHealthy') || 'Все серверы в сети'}
              </Badge>
            ) : (
              <Badge variant="danger" dot pulse size="sm">
                {t('dashboard.nodesOfflineAlert', { count: stats?.offline_servers || 0 })}
              </Badge>
            )}
          </div>
        }
        subtitle={t('dashboard.acrossAllNodes')}
        actions={
          <>
            {onRefresh && (
              <Button
                variant="secondary"
                size="md"
                onClick={onRefresh}
                disabled={isRefreshing}
                loading={isRefreshing}
                icon={<RefreshCw className="w-3.5 h-3.5" />}
                title={t('dashboard.refreshStats') || 'Обновить'}
              >
                <span className="hidden sm:inline">{t('dashboard.refreshStats') || 'Обновить'}</span>
              </Button>
            )}
            <Button
              variant="primary"
              size="md"
              onClick={onOpenAddServer}
              icon={<Plus className="w-4 h-4" />}
            >
              {t('servers.addServer')}
            </Button>
          </>
        }
      />

      {/* Cluster KPI Metrics Grid */}
      <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3.5">
        {/* Total Servers */}
        <div
          onClick={() => onSelectTab('servers')}
          className="bg-surface-850 border border-slate-800 hover:border-slate-700 p-4 rounded-2xl shadow-sm transition-all cursor-pointer group flex flex-col justify-between"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-400 group-hover:text-slate-300 transition-colors">
              {t('dashboard.nodes')}
            </span>
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-400 group-hover:bg-indigo-500/20 transition-colors">
              <Server className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-2.5">
            <div className="text-2xl font-bold text-white font-mono">{stats?.total_servers || servers.length}</div>
            <div className="mt-1 flex items-center gap-2 text-[11px]">
              <span className="text-emerald-400 flex items-center gap-0.5">
                <CheckCircle2 className="w-3 h-3" /> {stats?.online_servers || 0}
              </span>
              {(stats?.offline_servers || 0) > 0 && (
                <span className="text-rose-400 flex items-center gap-0.5">
                  <XCircle className="w-3 h-3" /> {stats?.offline_servers}
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Websites */}
        <div
          onClick={() => onSelectTab('sites')}
          className="bg-surface-850 border border-slate-800 hover:border-slate-700 p-4 rounded-2xl shadow-sm transition-all cursor-pointer group flex flex-col justify-between"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-400 group-hover:text-slate-300 transition-colors">
              {t('dashboard.websites') || 'Веб-сайты'}
            </span>
            <div className="p-1.5 rounded-lg bg-cyan-500/10 text-cyan-400 group-hover:bg-cyan-500/20 transition-colors">
              <Globe className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-2.5">
            <div className="text-2xl font-bold text-white font-mono">{stats?.total_sites ?? '—'}</div>
            <div className="mt-1 text-[11px] text-slate-400">
              {stats?.active_sites !== undefined ? (
                <span className="text-cyan-400">{stats.active_sites} {t('dashboard.activeWebsites') || 'активно'}</span>
              ) : (
                'Управление'
              )}
            </div>
          </div>
        </div>

        {/* SSL Certificates */}
        <div
          onClick={() => onSelectTab('ssl')}
          className="bg-surface-850 border border-slate-800 hover:border-slate-700 p-4 rounded-2xl shadow-sm transition-all cursor-pointer group flex flex-col justify-between"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-400 group-hover:text-slate-300 transition-colors">
              {t('dashboard.sslCertificates') || 'SSL-сертификаты'}
            </span>
            <div
              className={`p-1.5 rounded-lg transition-colors ${(stats?.expiring_ssl_certs || 0) > 0
                ? 'bg-amber-500/10 text-amber-400 group-hover:bg-amber-500/20'
                : 'bg-emerald-500/10 text-emerald-400 group-hover:bg-emerald-500/20'
                }`}
            >
              {(stats?.expiring_ssl_certs || 0) > 0 ? (
                <ShieldAlert className="w-4 h-4" />
              ) : (
                <ShieldCheck className="w-4 h-4" />
              )}
            </div>
          </div>
          <div className="mt-2.5">
            <div className="text-2xl font-bold text-white font-mono">{stats?.total_ssl_certs ?? '—'}</div>
            <div className="mt-1 text-[11px] truncate">
              {(stats?.expiring_ssl_certs || 0) > 0 ? (
                <span className="text-amber-400 font-medium">
                  {t('dashboard.certsExpiring', { count: stats?.expiring_ssl_certs || 0 })}
                </span>
              ) : (
                <span className="text-emerald-400">{t('dashboard.allCertsValid') || 'Все валидны'}</span>
              )}
            </div>
          </div>
        </div>

        {/* Compute Capacity */}
        <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-400">{t('dashboard.computeCapacity')}</span>
            <div className="p-1.5 rounded-lg bg-purple-500/10 text-purple-400">
              <Cpu className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-2.5">
            <div className="text-2xl font-bold text-white font-mono">{stats?.total_cores || 0}</div>
            <div className="mt-1 text-[11px] text-slate-400 truncate">{t('dashboard.vcpuCores')}</div>
          </div>
        </div>

        {/* Memory Allocation */}
        <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-400">{t('dashboard.totalMemory')}</span>
            <div className="p-1.5 rounded-lg bg-pink-500/10 text-pink-400">
              <Activity className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-2.5">
            <div className="text-2xl font-bold text-white font-mono">{totalRAMGB}</div>
            <div className="mt-1 text-[11px] text-slate-400 truncate">{t('dashboard.ramUnit')}</div>
          </div>
        </div>

        {/* Disk Storage */}
        <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-400">{t('dashboard.diskStorage')}</span>
            <div className="p-1.5 rounded-lg bg-amber-500/10 text-amber-400">
              <HardDrive className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-2.5">
            <div className="text-2xl font-bold text-white font-mono">
              {usedDiskGB} / {totalDiskGB} <span className="text-xs text-slate-400 font-sans">ГБ</span>
            </div>
            <div className="mt-1.5 w-full bg-slate-800 h-1 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all duration-500 ${diskPercent > 85 ? 'bg-rose-500' : diskPercent > 70 ? 'bg-amber-500' : 'bg-emerald-500'
                  }`}
                style={{ width: `${diskPercent}%` }}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Active Target Server Spotlight Card */}
      {selectedServer && (
        <div className="relative overflow-hidden bg-gradient-to-br from-surface-850 via-surface-850 to-indigo-950/20 border border-indigo-500/30 rounded-2xl p-5 shadow-xl space-y-4">
          <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
            <div className="flex items-start sm:items-center gap-3 min-w-0">
              <div className="p-2.5 rounded-xl bg-indigo-500/15 border border-indigo-500/30 text-indigo-400 shrink-0">
                <Server className="w-5 h-5" />
              </div>
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-xs font-semibold uppercase tracking-wider text-indigo-400">
                    {t('dashboard.activeServerSpotlight') || 'Целевой сервер'}
                  </span>
                  <span
                    className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold border ${selectedServer.status === 'online'
                      ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                      : 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                      }`}
                  >
                    <span
                      className={`w-1.5 h-1.5 rounded-full ${selectedServer.status === 'online' ? 'bg-emerald-400 animate-pulse' : 'bg-rose-400'
                        }`}
                    />
                    {selectedServer.status}
                  </span>
                  {selectedServer.group_name && (
                    <span className="px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 text-[10px] font-semibold border border-slate-700/60">
                      {selectedServer.group_name}
                    </span>
                  )}
                  {selectedServer.mode === 'audit' ? (
                    <span className="px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-300 text-[10px] font-semibold border border-amber-500/20 flex items-center gap-1">
                      <Eye className="w-3 h-3" /> {t('servers.modeAudit') || 'Аудит'}
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-300 text-[10px] font-semibold border border-emerald-500/20 flex items-center gap-1">
                      <Shield className="w-3 h-3" /> {t('servers.modeManagement') || 'Управление'}
                    </span>
                  )}
                </div>

                <div className="flex flex-wrap items-baseline gap-2.5">
                  <h2 className="text-lg font-bold text-white tracking-tight truncate">{maskServerName(selectedServer.name)}</h2>
                  <div className="flex items-center gap-1 font-mono text-xs text-slate-400 bg-slate-900/80 px-2 py-0.5 rounded border border-slate-800">
                    <span>{maskHost(selectedServer.host)}:{selectedServer.port}</span>
                    <button
                      onClick={(e) => handleCopy(`${selectedServer.host}:${selectedServer.port}`, 'spotlight-host', e)}
                      className="text-slate-500 hover:text-slate-300 transition-colors ml-0.5"
                      title={t('common.copy')}
                    >
                      {copiedId === 'spotlight-host' ? (
                        <Check className="w-3 h-3 text-emerald-400" />
                      ) : (
                        <Copy className="w-3 h-3" />
                      )}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            {/* Server Quick Switcher */}
            <div className="flex items-center gap-2 shrink-0">
              <span className="text-xs text-slate-400 hidden xl:inline">{t('dashboard.switchServer') || 'Сменить сервер'}:</span>
              <Select
                value={selectedServerId}
                onChange={(e) => setSelectedServerId(e.target.value)}
                sizeVariant="sm"
                className="max-w-[200px]"
              >
                {servers.map((s) => (
                  <option key={s.id} value={s.id}>
                    {maskServerName(s.name)} ({maskHost(s.host)})
                  </option>
                ))}
              </Select>
            </div>
          </div>

          {/* Quick Metrics for Active Server */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-2">
            {/* CPU & Model */}
            <div className="bg-slate-900/60 border border-slate-800/80 rounded-lg p-2.5 space-y-1">
              <div className="text-[11px] text-slate-400 flex items-center justify-between">
                <span>{t('dashboard.colCpu')}</span>
                <Cpu className="w-3.5 h-3.5 text-purple-400" />
              </div>
              <div className="text-sm font-bold text-white font-mono">
                {activeDiag?.cpu_cores ? `${activeDiag.cpu_cores} vCPU` : '—'}
              </div>
              <div className="text-[10px] text-slate-500 truncate" title={activeDiag?.cpu_model}>
                {activeDiag?.cpu_model || activeDiag?.arch || '—'}
              </div>
            </div>

            {/* RAM */}
            <div className="bg-slate-900/60 border border-slate-800/80 rounded-lg p-2.5 space-y-1">
              <div className="text-[11px] text-slate-400 flex items-center justify-between">
                <span>{t('dashboard.colRam')}</span>
                <Activity className="w-3.5 h-3.5 text-pink-400" />
              </div>
              <div className="text-sm font-bold text-white font-mono">
                {activeRamGB ? `${activeRamGB} ГБ` : '—'}
              </div>
              <div className="text-[10px] text-slate-500 truncate">
                {t('dashboard.availableRAM')}
              </div>
            </div>

            {/* Disk */}
            <div className="bg-slate-900/60 border border-slate-800/80 rounded-lg p-2.5 space-y-1">
              <div className="text-[11px] text-slate-400 flex items-center justify-between">
                <span>{t('dashboard.colDisk')}</span>
                <HardDrive className="w-3.5 h-3.5 text-amber-400" />
              </div>
              <div className="text-sm font-bold text-white font-mono">
                {activeDisk ? `${activeDisk.usedGB} / ${activeDisk.totalGB} ГБ` : '—'}
              </div>
              {activeDisk && (
                <div className="w-full bg-slate-800 h-1 rounded-full overflow-hidden mt-1">
                  <div
                    className={`h-full rounded-full ${activeDisk.percent > 85 ? 'bg-rose-500' : activeDisk.percent > 70 ? 'bg-amber-500' : 'bg-emerald-500'
                      }`}
                    style={{ width: `${activeDisk.percent}%` }}
                  />
                </div>
              )}
            </div>

            {/* Uptime & OS */}
            <div className="bg-slate-900/60 border border-slate-800/80 rounded-lg p-2.5 space-y-1">
              <div className="text-[11px] text-slate-400 flex items-center justify-between">
                <span>{t('dashboard.colUptime')}</span>
                <Clock className="w-3.5 h-3.5 text-cyan-400" />
              </div>
              <div className="text-sm font-bold text-white font-mono">
                {formatUptime(activeDiag?.uptime_seconds)}
              </div>
              <div className="text-[10px] text-slate-400 truncate" title={`${activeDiag?.os_name || ''} ${activeDiag?.os_version || ''}`}>
                {activeDiag?.os_name ? `${activeDiag.os_name} ${activeDiag.os_version || ''}` : 'Linux'}
              </div>
            </div>
          </div>

          {/* Software Stack on Active Server & Direct Quick Actions */}
          <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3 pt-2 border-t border-slate-800/80">
            {/* Installed Software Badges */}
            <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
              <span className="text-xs text-slate-400 mr-1">{t('dashboard.serverStack') || 'Стек'}:</span>
              {activeDiag?.installed_webservers && activeDiag.installed_webservers.length > 0 ? (
                activeDiag.installed_webservers.map((ws) => (
                  <span
                    key={ws}
                    className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 font-semibold uppercase text-[10px]"
                  >
                    {ws}
                  </span>
                ))
              ) : (
                <span className="text-slate-500 text-xs italic">Нет веб-сервера</span>
              )}

              {activeDiag?.installed_php_versions && activeDiag.installed_php_versions.length > 0 && (
                <span className="px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-300 border border-indigo-500/20 font-semibold text-[10px]">
                  PHP {activeDiag.installed_php_versions.join(', ')}
                </span>
              )}

              {activeDiag?.installed_db_versions && activeDiag.installed_db_versions.length > 0 && (
                <span className="px-2 py-0.5 rounded bg-amber-500/10 text-amber-300 border border-amber-500/20 font-semibold text-[10px]">
                  {activeDiag.installed_db_versions.join(', ')}
                </span>
              )}

              {activeDiag?.docker_version && (
                <span className="px-2 py-0.5 rounded bg-cyan-500/10 text-cyan-300 border border-cyan-500/20 font-semibold text-[10px]">
                  Docker
                </span>
              )}
            </div>

            {/* Direct Quick Action Buttons */}
            <div className="flex flex-wrap items-center gap-1.5 shrink-0">
              <Button
                variant="secondary"
                size="xs"
                onClick={() => onSelectTab('terminal')}
                icon={<Terminal className="w-3.5 h-3.5 text-emerald-400" />}
              >
                <span>{t('dashboard.openTerminal') || 'Терминал'}</span>
              </Button>

              <Button
                variant="secondary"
                size="xs"
                onClick={() => onSelectTab('filemanager')}
                icon={<Folder className="w-3.5 h-3.5 text-amber-400" />}
              >
                <span>{t('dashboard.openFiles') || 'Файлы'}</span>
              </Button>

              <Button
                variant="secondary"
                size="xs"
                onClick={() => onSelectTab('sites')}
                icon={<Globe className="w-3.5 h-3.5 text-cyan-400" />}
              >
                <span>{t('dashboard.openSites') || 'Сайты'}</span>
              </Button>

              <Button
                variant="secondary"
                size="xs"
                onClick={() => onSelectTab('databases')}
                icon={<Database className="w-3.5 h-3.5 text-purple-400" />}
              >
                <span>{t('dashboard.openDatabases') || 'Базы'}</span>
              </Button>

              <Button
                variant="secondary"
                size="xs"
                onClick={() => onSelectTab('security')}
                icon={<Shield className="w-3.5 h-3.5 text-rose-400" />}
              >
                <span>{t('dashboard.openSecurity') || 'Безопасность'}</span>
              </Button>

              <Button
                variant="secondary"
                size="xs"
                onClick={() => onSelectTab('cron')}
                icon={<Clock className="w-3.5 h-3.5 text-indigo-400" />}
              >
                <span>Cron</span>
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Main Grid: Connected Servers Table & Recent Tasks / Software Stack */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Left 2 Columns: Connected Servers Fleet Table */}
        <div className="xl:col-span-2 bg-surface-850 border border-slate-800 rounded-2xl p-5 shadow-lg flex flex-col justify-between space-y-4 min-w-0">
          <div>
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
              <div className="flex items-center gap-2">
                <Layers className="w-5 h-5 text-indigo-400" />
                <h3 className="font-bold text-white text-sm">{t('dashboard.connectedServers')}</h3>
                <span className="text-xs text-slate-400 font-mono">({servers.length})</span>
              </div>

              {/* Search & Status Filters */}
              <div className="flex items-center gap-2">
                <SearchInput
                  value={searchQuery}
                  onChangeValue={setSearchQuery}
                  placeholder={t('dashboard.searchServers') || 'Поиск...'}
                  sizeVariant="sm"
                  className="w-36 sm:w-48"
                />

                <Tabs
                  tabs={[
                    { id: 'all', label: t('dashboard.filterAll') || 'Все' },
                    { id: 'online', label: t('dashboard.filterOnline') || 'В сети' },
                    { id: 'offline', label: t('dashboard.filterOffline') || 'Офлайн' },
                  ]}
                  activeTab={statusFilter}
                  onChange={(id) => setStatusFilter(id as any)}
                  variant="pill"
                />

                <button
                  onClick={() => onSelectTab('servers')}
                  className="text-xs font-medium text-indigo-400 hover:text-indigo-300 flex items-center gap-0.5 ml-1 shrink-0"
                  title={t('dashboard.viewAllServers') || 'Все серверы'}
                >
                  <span className="hidden sm:inline">{t('dashboard.viewAllServers') || 'Все'}</span>
                  <ArrowUpRight className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="pl-2">{t('dashboard.colServer')}</TableHead>
                    <TableHead>{t('common.status')}</TableHead>
                    <TableHead>{t('dashboard.colIpAddress')}</TableHead>
                    <TableHead>{t('dashboard.colCpu')}</TableHead>
                    <TableHead>{t('dashboard.colRam')}</TableHead>
                    <TableHead>{t('dashboard.colDisk')}</TableHead>
                    <TableHead>{t('dashboard.colUptime')}</TableHead>
                    <TableHead className="text-right pr-2">{t('common.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredServers.length === 0 ? (
                    <TableEmpty
                      colSpan={8}
                      icon={<Search className="w-8 h-8 text-slate-600" />}
                      message={t('dashboard.noServersMatch') || 'Нет серверов, соответствующих фильтру'}
                    />
                  ) : (
                    filteredServers.map((s) => {
                      const isTarget = s.id === selectedServerId;
                      const diag = diagnosticsMap[s.id] || stats?.server_diagnostics?.find((d) => d.server_id === s.id);
                      const disk = getDiskInfo(diag);
                      const ramGB = getRamGB(diag);

                      return (
                        <TableRow
                          key={s.id}
                          onClick={() => setSelectedServerId(s.id)}
                          className={`cursor-pointer ${
                            isTarget ? 'bg-indigo-950/20 border-l-2 border-indigo-500' : ''
                          }`}
                        >
                          <TableCell className="pl-2 font-semibold text-white">
                            <div className="flex items-center gap-2">
                              <span
                                className={`w-2 h-2 rounded-full shrink-0 ${
                                  isTarget ? 'bg-indigo-400 ring-2 ring-indigo-500/40' : 'bg-slate-600'
                                }`}
                                title={isTarget ? t('dashboard.activeServerNotice') : ''}
                              />
                              <div className="min-w-0">
                                <div className="truncate font-sans max-w-[140px] text-slate-100 flex items-center gap-1.5">
                                  <span>{maskServerName(s.name)}</span>
                                  {isTarget && (
                                    <Badge variant="indigo" size="xs">
                                      Фокус
                                    </Badge>
                                  )}
                                </div>
                                {s.group_name && (
                                  <div className="text-[10px] text-slate-500 truncate font-normal">
                                    {s.group_name}
                                  </div>
                                )}
                              </div>
                            </div>
                          </TableCell>

                          <TableCell>
                            <Badge
                              variant={s.status === 'online' ? 'success' : s.status === 'offline' ? 'danger' : 'warning'}
                              size="xs"
                              dot
                            >
                              {s.status}
                            </Badge>
                          </TableCell>

                          <TableCell className="font-mono text-slate-300">
                            <div className="flex items-center gap-1">
                              <span>{maskHost(s.host)}:{s.port}</span>
                              <button
                                onClick={(e) => handleCopy(`${s.host}:${s.port}`, `table-${s.id}`, e)}
                                className="text-slate-500 hover:text-slate-300 transition-colors ml-0.5 p-0.5 rounded"
                                title={t('common.copy')}
                              >
                                {copiedId === `table-${s.id}` ? (
                                  <Check className="w-3 h-3 text-emerald-400" />
                                ) : (
                                  <Copy className="w-3 h-3" />
                                )}
                              </button>
                            </div>
                          </TableCell>

                          <TableCell className="font-mono text-slate-300">
                            {diag?.cpu_cores ? `${diag.cpu_cores} c` : '—'}
                          </TableCell>

                          <TableCell className="font-mono text-slate-300">
                            {ramGB ? `${ramGB} ГБ` : '—'}
                          </TableCell>

                          <TableCell className="font-mono text-slate-300">
                            {disk ? (
                              <div className="w-24 space-y-1">
                                <div className="text-[11px] flex justify-between">
                                  <span>{disk.usedGB}G</span>
                                  <span className="text-slate-400">{disk.percent}%</span>
                                </div>
                                <div className="w-full bg-slate-800 h-1 rounded-full overflow-hidden">
                                  <div
                                    className={`h-full rounded-full ${
                                      disk.percent > 85 ? 'bg-rose-500' : disk.percent > 70 ? 'bg-amber-500' : 'bg-emerald-500'
                                    }`}
                                    style={{ width: `${disk.percent}%` }}
                                  />
                                </div>
                              </div>
                            ) : (
                              '—'
                            )}
                          </TableCell>

                          <TableCell className="text-slate-400 font-mono text-[11px]">
                            {formatUptime(diag?.uptime_seconds)}
                          </TableCell>

                          <TableCell className="text-right pr-2">
                            <div className="flex items-center justify-end gap-1.5" onClick={(e) => e.stopPropagation()}>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                onClick={() => {
                                  setSelectedServerId(s.id);
                                  onSelectTab('terminal');
                                }}
                                title={t('servers.openTerminal')}
                                icon={<Terminal className="w-3.5 h-3.5 text-emerald-400" />}
                              />
                              <Button
                                variant="secondary"
                                size="xs"
                                onClick={() => {
                                  setSelectedServerId(s.id);
                                  onSelectTab('sites');
                                }}
                              >
                                Manage
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </div>
          </div>
        </div>

        {/* Right Column: Recent Tasks Activity & Fleet Software Stack */}
        <div className="space-y-6">
          {/* Recent Tasks Card */}
          <div className="bg-surface-850 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <ListTodo className="w-5 h-5 text-indigo-400" />
                <h3 className="font-bold text-white text-sm">{t('dashboard.recentTasks')}</h3>
              </div>
              <button
                onClick={() => onSelectTab('tasks')}
                className="text-xs font-medium text-indigo-400 hover:text-indigo-300 flex items-center gap-1"
              >
                <span>{t('dashboard.viewAllTasks')}</span>
                <ArrowUpRight className="w-3.5 h-3.5" />
              </button>
            </div>

            {(!stats?.recent_tasks || stats.recent_tasks.length === 0) ? (
              <div className="text-center py-8 text-xs text-slate-500">
                {t('dashboard.noRecentTasks')}
              </div>
            ) : (
              <div className="space-y-2.5">
                {stats.recent_tasks.slice(0, 5).map((task) => {
                  const targetServerName = servers.find((s) => s.id === task.server_id)?.name;

                  return (
                    <div
                      key={task.id}
                      onClick={() => onOpenTask(task)}
                      className="p-3 rounded-lg bg-slate-900 border border-slate-800/80 hover:border-slate-700 cursor-pointer transition-all space-y-1.5"
                    >
                      <div className="flex items-center justify-between text-xs gap-2">
                        <div className="flex items-center gap-1.5 min-w-0">
                          {targetServerName && (
                            <span className="px-1.5 py-0.2 rounded bg-slate-800 text-slate-400 text-[10px] font-mono shrink-0">
                              {targetServerName}
                            </span>
                          )}
                          <span className="font-semibold text-white truncate">{localizeTaskTitle(task, t)}</span>
                        </div>

                        <Badge
                          variant={
                            task.status === 'completed'
                              ? 'success'
                              : task.status === 'running'
                              ? 'indigo'
                              : task.status === 'failed'
                              ? 'danger'
                              : 'neutral'
                          }
                          size="xs"
                          dot={task.status === 'running'}
                          pulse={task.status === 'running'}
                        >
                          {task.status}
                        </Badge>
                      </div>

                      <div className="w-full bg-slate-800 h-1 rounded-full overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all duration-300 ${task.status === 'completed'
                            ? 'bg-emerald-500'
                            : task.status === 'failed'
                              ? 'bg-rose-500'
                              : 'bg-indigo-500'
                            }`}
                          style={{ width: `${task.progress}%` }}
                        />
                      </div>

                      <div className="flex justify-between text-[10px] text-slate-400 font-mono">
                        <span className="truncate max-w-[180px]">
                          {localizeTaskStep(task.current_step, t) || t('dashboard.taskInProgress')}
                        </span>
                        <span>{formatRelativeTime(task.created_at as unknown as string)}</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Cluster Software Ecosystem Card */}
          <div className="bg-surface-850 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-3.5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Box className="w-4 h-4 text-cyan-400" />
                <h4 className="font-bold text-white text-xs uppercase tracking-wider">
                  {t('dashboard.softwareEcosystem') || 'Стек сервисов кластера'}
                </h4>
              </div>
            </div>

            <div className="space-y-2 text-xs">
              {/* Web Servers */}
              <div className="flex items-center justify-between py-1.5 border-b border-slate-800/80">
                <span className="text-slate-400 flex items-center gap-1.5">
                  <Globe className="w-3.5 h-3.5 text-cyan-400" />
                  {t('dashboard.installedWebservers') || 'Веб-серверы'}
                </span>
                <div className="flex flex-wrap gap-1">
                  {softwareStack.webservers.length > 0 ? (
                    softwareStack.webservers.map((ws) => (
                      <span
                        key={ws}
                        onClick={() => onSelectTab('webservers')}
                        className="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-300 font-semibold text-[10px] uppercase cursor-pointer hover:bg-emerald-500/20 transition"
                      >
                        {ws}
                      </span>
                    ))
                  ) : (
                    <span className="text-slate-500 text-[11px]">—</span>
                  )}
                </div>
              </div>

              {/* PHP Versions */}
              <div className="flex items-center justify-between py-1.5 border-b border-slate-800/80">
                <span className="text-slate-400 flex items-center gap-1.5">
                  <Code className="w-3.5 h-3.5 text-indigo-400" />
                  {t('dashboard.installedPhp') || 'PHP'}
                </span>
                <div className="flex flex-wrap gap-1">
                  {softwareStack.phpVersions.length > 0 ? (
                    softwareStack.phpVersions.map((pv) => (
                      <span
                        key={pv}
                        onClick={() => onSelectTab('php')}
                        className="px-1.5 py-0.5 rounded bg-indigo-500/10 text-indigo-300 font-semibold text-[10px] cursor-pointer hover:bg-indigo-500/20 transition"
                      >
                        {pv}
                      </span>
                    ))
                  ) : (
                    <span className="text-slate-500 text-[11px]">—</span>
                  )}
                </div>
              </div>

              {/* Databases */}
              <div className="flex items-center justify-between py-1.5 border-b border-slate-800/80">
                <span className="text-slate-400 flex items-center gap-1.5">
                  <Database className="w-3.5 h-3.5 text-amber-400" />
                  {t('dashboard.installedDb') || 'Базы данных'}
                </span>
                <div className="flex flex-wrap gap-1">
                  {softwareStack.dbVersions.length > 0 ? (
                    softwareStack.dbVersions.map((db) => (
                      <span
                        key={db}
                        onClick={() => onSelectTab('databases')}
                        className="px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-300 font-semibold text-[10px] cursor-pointer hover:bg-amber-500/20 transition"
                      >
                        {db}
                      </span>
                    ))
                  ) : (
                    <span className="text-slate-500 text-[11px]">—</span>
                  )}
                </div>
              </div>

              {/* Docker */}
              <div className="flex items-center justify-between py-1.5">
                <span className="text-slate-400 flex items-center gap-1.5">
                  <Box className="w-3.5 h-3.5 text-cyan-400" />
                  Docker
                </span>
                <div>
                  {softwareStack.dockerNodesCount > 0 ? (
                    <span
                      onClick={() => onSelectTab('docker')}
                      className="px-1.5 py-0.5 rounded bg-cyan-500/10 text-cyan-300 font-semibold text-[10px] cursor-pointer hover:bg-cyan-500/20 transition"
                    >
                      {softwareStack.dockerNodesCount} {t('dashboard.nodes') || 'узлов'}
                    </span>
                  ) : (
                    <span className="text-slate-500 text-[11px]">—</span>
                  )}
                </div>
              </div>
            </div>

            {/* Vault & WAL Notice */}
            <div className="pt-2 border-t border-slate-800/80 flex items-center justify-between text-[11px] text-slate-400">
              <span className="flex items-center gap-1 text-slate-400">
                <Shield className="w-3.5 h-3.5 text-emerald-400" /> {t('dashboard.vaultNotice')}
              </span>
              <span className="font-mono text-slate-500">{t('dashboard.walNotice')}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
