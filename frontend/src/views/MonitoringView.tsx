import React, { useState, useEffect, useRef } from 'react';
import {
  Activity,
  Cpu,
  Database,
  HardDrive,
  Clock,
  Globe,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  RefreshCw,
  Play,
  Pause,
  Zap,
} from 'lucide-react';
import { ServerRecord } from '../types';
import {
  GetRealtimeSystemMetrics,
  CheckEndpoint,
  ListSites,
} from '../../wailsjs/go/main/App';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { PageToolbar, Button, Input } from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';
import { useServer } from '../context/ServerContext';
import { usePrivacy } from '../context/PrivacyContext';

interface MonitoringViewProps {
  servers: ServerRecord[];
  onOpenAddServer: () => void;
}

export const MonitoringView: React.FC<MonitoringViewProps> = ({
  servers,
  onOpenAddServer,
}) => {
  const { selectedServerId, setSelectedServerId, setPageLoading } = useServer();
  const { t } = useTranslation();
  const { maskDomain, registerDomains } = usePrivacy();

  // Realtime system metrics
  const [metrics, setMetrics] = useState<any>(null);
  const [isLiveActive, setIsLiveActive] = useState(true);
  const [isMetricsLoading, setIsMetricsLoading] = useState(false);

  // Sites monitoring
  const [serverSites, setServerSites] = useState<any[]>([]);
  const [siteChecks, setSiteChecks] = useState<Record<string, any>>({});
  const [customUrl, setCustomUrl] = useState('');
  const [isCheckingUrl, setIsCheckingUrl] = useState(false);

  const [error, setError] = useState<string | null>(null);
  const liveIntervalRef = useRef<any>(null);
  const serverRef = useRef(selectedServerId);
  serverRef.current = selectedServerId;
  const hasMetricsRef = useRef(false);
  hasMetricsRef.current = metrics !== null;

  // Load sites for the selected server
  useEffect(() => {
    if (!selectedServerId) return;
    ListSites(selectedServerId)
      .then((sites) => {
        const list = sites || [];
        setServerSites(list);
        registerDomains(list.map((s: any) => s.domain).filter(Boolean));
      })
      .catch((err) => console.error('Failed to list sites:', err));
  }, [selectedServerId]);

  // Fetch real-time system metrics
  const fetchMetrics = async () => {
    if (!selectedServerId) return;
    const serverID = selectedServerId;
    try {
      setIsMetricsLoading(true);
      if (!hasMetricsRef.current) setPageLoading(true, t('monitoring.collectingMetrics'));
      const data = await GetRealtimeSystemMetrics(serverID);
      if (serverID !== serverRef.current) return;
      setMetrics(data);
    } catch (err: unknown) {
      if (serverID !== serverRef.current) return;
      setMetrics(null);
    } finally {
      if (serverID === serverRef.current) {
        setIsMetricsLoading(false);
        setPageLoading(false);
      }
    }
  };

  useEffect(() => {
    if (!selectedServerId) return;

    fetchMetrics();

    if (isLiveActive) {
      const seconds = parseInt(localStorage.getItem('tamestack_refresh_interval') || '10', 10);
      const intervalMs = Math.max(2, Number.isFinite(seconds) ? seconds : 10) * 1000;
      liveIntervalRef.current = setInterval(fetchMetrics, intervalMs);
    }

    return () => {
      if (liveIntervalRef.current) {
        clearInterval(liveIntervalRef.current);
      }
    };
  }, [selectedServerId, isLiveActive]);

  const handleCheckUrl = async (url: string) => {
    if (!url.trim()) return;
    try {
      setIsCheckingUrl(true);
      const res = await CheckEndpoint(url);
      setSiteChecks((prev) => ({ ...prev, [url]: res }));
    } catch (err: unknown) {
      setError(t('monitoring.checkError', { error: String(err) }));
    } finally {
      setIsCheckingUrl(false);
    }
  };

  const formatUptime = (seconds: number) => {
    const dSuffix = t('common.dayShort');
    const hSuffix = t('common.hourShort');
    const mSuffix = t('common.minShort');
    if (!seconds) return `0${mSuffix}`;
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    if (days > 0) return `${days}${dSuffix} ${hours}${hSuffix} ${mins}${mSuffix}`;
    if (hours > 0) return `${hours}${hSuffix} ${mins}${mSuffix}`;
    return `${mins}${mSuffix}`;
  };



  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Activity}
        description={t('empty.monitoringDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        title={t('monitoring.targetServer')}
        icon={<Activity className="w-5 h-5 text-emerald-400" />}
        servers={servers}
        selectedServerId={selectedServerId}
        onSelectServer={setSelectedServerId}
        actions={
          <>
            <Button
              variant={isLiveActive ? 'outline' : 'secondary'}
              size="md"
              onClick={() => setIsLiveActive(!isLiveActive)}
              icon={
                isLiveActive ? (
                  <>
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse mr-1" />
                    <Pause className="w-4 h-4" />
                  </>
                ) : (
                  <Play className="w-4 h-4" />
                )
              }
              title={t('monitoring.autoRefreshTooltip')}
              className={isLiveActive ? 'border-emerald-500/30 text-emerald-300 bg-emerald-500/10' : ''}
            >
              {isLiveActive ? t('monitoring.liveActive') : t('monitoring.livePaused')}
            </Button>
            <Button
              variant="secondary"
              size="md"
              onClick={fetchMetrics}
              disabled={isMetricsLoading}
              loading={isMetricsLoading}
              icon={<RefreshCw className="w-4 h-4" />}
              title={t('monitoring.pollNowTooltip')}
            >
              {t('monitoring.refreshBtn')}
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

      {/* METRICS CARDS GRID */}
      {isMetricsLoading && !metrics ? (
        <LoadingState
          type="stats"
          title={t('monitoring.loadingTitle')}
          subtitle={t('monitoring.loadingSubtitle')}
        />
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 min-[1400px]:grid-cols-4 gap-4 sm:gap-5">
        {/* Card 1: CPU Load */}
        <div className="bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 sm:p-6 space-y-3 shadow-sm transition-colors duration-150 ease-out">
          <div className="flex items-center justify-between gap-3 text-slate-400 text-xs">
            <span className="flex items-center gap-2 font-medium">
              <Cpu className="w-4 h-4 text-emerald-400 shrink-0" />
              <span>{t('monitoring.cpuLoad')}</span>
            </span>
            <span className="font-mono text-slate-400 shrink-0">{metrics?.timestamp || '--:--:--'}</span>
          </div>

          <div className="flex items-baseline gap-2">
            <span className="text-3xl font-extrabold text-slate-100 font-mono">
              {metrics ? metrics.cpu_percent.toFixed(1) : 0}%
            </span>
          </div>

          <div className="w-full bg-slate-900 rounded-full h-2 overflow-hidden border border-slate-800/50">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                (metrics?.cpu_percent || 0) > 85
                  ? 'bg-rose-500'
                  : (metrics?.cpu_percent || 0) > 60
                  ? 'bg-amber-500'
                  : 'bg-emerald-500'
              }`}
              style={{ width: `${Math.min(metrics?.cpu_percent || 0, 100)}%` }}
            />
          </div>

          <div className="pt-1 flex items-center justify-between text-[11px] text-slate-400 font-mono">
            <span>
              LA: {metrics ? `${metrics.load_1.toFixed(2)}, ${metrics.load_5.toFixed(2)}, ${metrics.load_15.toFixed(2)}` : '—'}
            </span>
          </div>
        </div>

        {/* Card 2: Memory (RAM) */}
        <div className="bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 sm:p-6 space-y-3 shadow-sm transition-colors duration-150 ease-out">
          <div className="flex items-center justify-between gap-3 text-slate-400 text-xs">
            <span className="flex items-center gap-2 font-medium">
              <Database className="w-4 h-4 text-blue-400 shrink-0" />
              <span>{t('monitoring.memoryUsage')}</span>
            </span>
            <span className="font-mono text-slate-300 whitespace-nowrap shrink-0">
              {metrics ? `${metrics.memory_used_mb} / ${metrics.memory_total_mb} MB` : '—'}
            </span>
          </div>

          <div className="flex items-baseline gap-2">
            <span className="text-3xl font-extrabold text-slate-100 font-mono">
              {metrics ? metrics.memory_used_percent.toFixed(1) : 0}%
            </span>
          </div>

          <div className="w-full bg-slate-900 rounded-full h-2 overflow-hidden border border-slate-800/50">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                (metrics?.memory_used_percent || 0) > 90
                  ? 'bg-rose-500'
                  : (metrics?.memory_used_percent || 0) > 75
                  ? 'bg-amber-500'
                  : 'bg-blue-500'
              }`}
              style={{ width: `${Math.min(metrics?.memory_used_percent || 0, 100)}%` }}
            />
          </div>

          <div className="pt-1 flex items-center justify-between text-[11px] text-slate-400 font-mono">
            <span>{metrics ? t('monitoring.memoryFree', { free: metrics.memory_free_mb }) : '—'}</span>
            <span>Swap: {metrics ? `${metrics.swap_used_mb} MB` : '—'}</span>
          </div>
        </div>

        {/* Card 3: Disk Root */}
        <div className="bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 sm:p-6 space-y-3 shadow-sm transition-colors duration-150 ease-out">
          <div className="flex items-center justify-between gap-3 text-slate-400 text-xs">
            <span className="flex items-center gap-2 font-medium">
              <HardDrive className="w-4 h-4 text-purple-400 shrink-0" />
              <span>{t('monitoring.diskUsage')}</span>
            </span>
            <span className="font-mono text-slate-300 whitespace-nowrap shrink-0">
              {metrics ? `${metrics.disk_used_gb.toFixed(1)} / ${metrics.disk_total_gb.toFixed(1)} GB` : '—'}
            </span>
          </div>

          <div className="flex items-baseline gap-2">
            <span className="text-3xl font-extrabold text-slate-100 font-mono">
              {metrics ? metrics.disk_used_percent.toFixed(1) : 0}%
            </span>
          </div>

          <div className="w-full bg-slate-900 rounded-full h-2 overflow-hidden border border-slate-800/50">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                (metrics?.disk_used_percent || 0) > 90
                  ? 'bg-rose-500'
                  : (metrics?.disk_used_percent || 0) > 75
                  ? 'bg-amber-500'
                  : 'bg-purple-500'
              }`}
              style={{ width: `${Math.min(metrics?.disk_used_percent || 0, 100)}%` }}
            />
          </div>

          <div className="pt-1 flex items-center justify-between text-[11px] text-slate-400 font-mono">
            <span>{metrics ? t('monitoring.diskFree', { free: metrics.disk_free_gb.toFixed(1) }) : '—'}</span>
          </div>
        </div>

        {/* Card 4: Uptime */}
        <div className="bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 sm:p-6 space-y-3 shadow-sm transition-colors duration-150 ease-out">
          <div className="flex items-center justify-between gap-3 text-slate-400 text-xs">
            <span className="flex items-center gap-2 font-medium">
              <Clock className="w-4 h-4 text-amber-400 shrink-0" />
              <span>{t('monitoring.uptime')}</span>
            </span>
            <span className={`shrink-0 ${metrics ? 'text-emerald-400 font-medium' : 'text-slate-500 font-medium'}`}>
              {metrics ? t('monitoring.online') : t('monitoring.noData')}
            </span>
          </div>

          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-100 font-mono">
              {metrics ? formatUptime(metrics.uptime_seconds) : '—'}
            </span>
          </div>

          <p className="text-xs text-slate-400 pt-3">
            {metrics ? t('monitoring.metricsCollected') : t('monitoring.noMetricsYet')}
          </p>
        </div>
      </div>
      )}

      {/* SITES AVAILABILITY & HTTP STATUS MONITOR */}
      <div className="bg-surface-850 border border-slate-800 rounded-2xl p-6 space-y-4 shadow-sm">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-800 pb-4">
          <div>
            <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
              <Globe className="w-5 h-5 text-emerald-400" />
              {t('monitoring.siteUptimeTitle')}
            </h3>
            <p className="text-xs text-slate-400 mt-0.5">
              {t('monitoring.siteUptimeDesc')}
            </p>
          </div>

          {/* Custom URL Tester */}
          <div className="flex items-center gap-2">
            <div className="w-56">
              <Input
                type="text"
                sizeVariant="sm"
                value={customUrl}
                onChange={(e) => setCustomUrl(e.target.value)}
                placeholder="https://example.com"
                className="font-mono"
              />
            </div>
            <Button
              variant="success"
              size="sm"
              onClick={() => handleCheckUrl(customUrl)}
              disabled={isCheckingUrl || !customUrl.trim()}
              loading={isCheckingUrl}
              icon={!isCheckingUrl ? <Zap className="w-3.5 h-3.5" /> : undefined}
            >
              {t('monitoring.checkBtn')}
            </Button>
          </div>
        </div>

        {serverSites.length === 0 && !customUrl ? (
          <div className="py-8 text-center text-slate-400 text-xs">
            {t('monitoring.noSitesNotice')}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3 pt-2">
            {serverSites.map((site) => {
              const url = `https://${site.domain}`;
              const check = siteChecks[url];

              return (
                <div
                  key={site.id}
                  className="bg-slate-900/60 border border-slate-800 hover:border-slate-700 rounded-xl p-4 flex items-center justify-between gap-3 transition"
                >
                  <div className="flex items-center gap-3">
                    <div>
                      {check ? (
                        check.is_up ? (
                          <CheckCircle2 className="w-5 h-5 text-emerald-400" />
                        ) : (
                          <XCircle className="w-5 h-5 text-rose-400" />
                        )
                      ) : (
                        <div className="w-5 h-5 rounded-full border border-slate-600 border-dashed" />
                      )}
                    </div>
                    <div>
                      <h4 className="text-sm font-semibold text-slate-200 font-mono">{maskDomain(site.domain)}</h4>
                      <p className="text-xs text-slate-400 mt-0.5">
                        {check ? (
                          <>
                            {t('monitoring.statusLabel')}{' '}
                            <strong className={check.is_up ? 'text-emerald-400' : 'text-rose-400'}>
                              {check.status_code || 'Err'}
                            </strong>{' '}
                            | {t('monitoring.latencyLabel', { ms: check.response_time_ms })}
                            {check.ssl_days_remaining >= 0 && (
                              <>
                                 | {t('monitoring.sslDaysLabel', { days: check.ssl_days_remaining })}
                              </>
                            )}
                          </>
                        ) : (
                          t('monitoring.notChecked')
                        )}
                      </p>
                    </div>
                  </div>

                  <button
                    onClick={() => handleCheckUrl(url)}
                    disabled={isCheckingUrl}
                    className="px-3 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/60 text-xs font-medium rounded-lg transition shrink-0"
                  >
                    {t('monitoring.pingBtn')}
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
