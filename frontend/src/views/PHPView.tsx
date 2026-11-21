import React, { useState, useEffect, useMemo } from 'react';
import { getErrorMessage } from '../utils/errors';
import { useServerData } from '../hooks/useServerData';
import {
  Code2,
  RefreshCw,
  CheckCircle2,
  XCircle,
  RotateCw,
  Power,
  Pause,
  Layers,
  Settings2,
  FileCode,
  Download,
  AlertCircle,
  Loader2,
  Filter,
  Trash2,
  Copy,
  Edit,
  Search,
  CheckSquare,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import {
  InspectPHPVersions,
  RestartPHPFPM,
  TogglePHPFPM,
  UpdatePHPVersion,
  SetDefaultCLIPHP,
  DetectPHPHandlers,
  CheckPHPAppRequirements,
  TogglePHPExtension,
  RemovePHPExtension,
  ListPHPPools,
  TogglePHPPool,
  DeletePHPPool,
} from '../../wailsjs/go/main/App';
import { php } from '../../wailsjs/go/models';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { PageToolbar, Button, Badge, Tabs, SearchInput, Select } from '../components/ui';
import { InstallPHPModal } from '../components/InstallPHPModal';
import { InstallPHPExtensionsModal } from '../components/InstallPHPExtensionsModal';
import { EditPoolModal } from '../components/php/EditPoolModal';
import { ClonePoolModal } from '../components/php/ClonePoolModal';
import { CreatePoolModal } from '../components/php/CreatePoolModal';
import { PHPServiceLogsModal } from '../components/php/PHPServiceLogsModal';
import { PhpSettingsTab } from '../components/php/PhpSettingsTab';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { useTranslation } from '../i18n/LanguageContext';
import { useServer } from '../context/ServerContext';
import { useConfirm } from '../context/ConfirmContext';
import { useAlert } from '../context/AlertContext';

interface PHPViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

type MainTab = 'versions' | 'extensions' | 'pools' | 'config';

export const PHPView: React.FC<PHPViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
  } = useServer();
  const { t, language } = useTranslation();
  const confirm = useConfirm();
  const alert = useAlert();

  const [mainTab, setMainTab] = useState<MainTab>('versions');
  const [selectedVersionStr, setSelectedVersionStr] = useState<string>('8.3');

  const [restartingVer, setRestartingVer] = useState<string | null>(null);
  const [togglingVer, setTogglingVer] = useState<string | null>(null);
  const [updatingVer, setUpdatingVer] = useState<string | null>(null);
  const [settingDefaultCli, setSettingDefaultCli] = useState<string | null>(null);
  const [filter, setFilter] = useState<'all' | 'installed' | 'available' | 'unavailable'>('all');

  // Detected handlers
  const [handlers, setHandlers] = useState<string[]>([]);

  // Modals state
  const [installVersion, setInstallVersion] = useState<string | null>(null);
  const [extVersionInfo, setExtVersionInfo] = useState<php.PHPVersionInfo | null>(null);
  const [extInstallInfo, setExtInstallInfo] = useState<php.PHPVersionInfo | null>(null);
  const [iniVersion, setIniVersion] = useState<string | null>(null);
  const [logsModalVer, setLogsModalVer] = useState<string | null>(null);
  const [editingPool, setEditingPool] = useState<{ version: string; pool: php.FpmPoolConfig } | null>(null);
  const [cloningPool, setCloningPool] = useState<{ version: string; poolName: string } | null>(null);

  // Pools state
  const [pools, setPools] = useState<php.FpmPoolConfig[]>([]);
  const [loadingPools, setLoadingPools] = useState(false);
  const [togglingPool, setTogglingPool] = useState<string | null>(null);
  const [deletingPool, setDeletingPool] = useState<string | null>(null);

  // Pool Create modal
  const [poolVersion, setPoolVersion] = useState<string | null>(null);

  // Extensions Tab State
  const [extSearch, setExtSearch] = useState('');
  const [appReqResult, setAppReqResult] = useState<php.PHPAppRequirement | null>(null);
  const [checkingApp, setCheckingApp] = useState<string | null>(null);
  const [togglingExt, setTogglingExt] = useState<string | null>(null);
  const [removingExt, setRemovingExt] = useState<string | null>(null);

  const selectedServer = servers.find((s) => s.id === selectedServerID) || servers[0];

  const {
    data: versions,
    loading: isLoading,
    error,
    setError,
    reload: fetchPHPStatus,
  } = useServerData(
    (serverId) => InspectPHPVersions(serverId),
    [],
    {
      initial: [] as php.PHPVersionInfo[],
      reset: [] as php.PHPVersionInfo[],
      loadingMessage: selectedServer ? t('php.loadingVersions', { server: selectedServer.name }) : undefined,
    },
  );

  const installedVersions = useMemo(() => {
    return (versions || []).filter((v) => v.is_installed);
  }, [versions]);

  useEffect(() => {
    if (installedVersions.length > 0) {
      if (!installedVersions.some((v) => v.version === selectedVersionStr)) {
        setSelectedVersionStr(installedVersions[0].version);
      }
    } else if (versions && versions.length > 0) {
      setSelectedVersionStr(versions[0].version);
    }
  }, [versions, installedVersions]);

  // Load handlers
  useEffect(() => {
    if (selectedServer) {
      DetectPHPHandlers(selectedServer.id)
        .then((h) => setHandlers(h || []))
        .catch(() => setHandlers([]));
    }
  }, [selectedServer]);

  // Load pools
  const fetchPools = async () => {
    if (!selectedServer || !selectedVersionStr) return;
    setLoadingPools(true);
    try {
      const data = await ListPHPPools(selectedServer.id, selectedVersionStr);
      setPools(data || []);
    } catch (err: unknown) {
      console.error(err);
      setPools([]);
    } finally {
      setLoadingPools(false);
    }
  };

  useEffect(() => {
    if (mainTab === 'pools' && selectedServer && selectedVersionStr) {
      fetchPools();
    }
  }, [mainTab, selectedServer, selectedVersionStr]);

  const counts = useMemo(() => {
    let installed = 0;
    let available = 0;
    let unavailable = 0;
    (versions || []).forEach((ver) => {
      if (ver.is_installed) {
        installed++;
      } else if (ver.is_available) {
        available++;
      } else {
        unavailable++;
      }
    });
    return {
      all: (versions || []).length,
      installed,
      available,
      unavailable,
    };
  }, [versions]);

  const filteredVersions = useMemo(() => {
    return (versions || []).filter((ver) => {
      if (filter === 'installed') return ver.is_installed;
      if (filter === 'available') return !ver.is_installed && ver.is_available;
      if (filter === 'unavailable') return !ver.is_installed && !ver.is_available;
      return true;
    });
  }, [versions, filter]);

  const activeVersionObj = useMemo(() => {
    return (versions || []).find((v) => v.version === selectedVersionStr) || null;
  }, [versions, selectedVersionStr]);

  const handleRestartFPM = async (version: string) => {
    if (!selectedServer) return;
    setRestartingVer(version);
    try {
      await RestartPHPFPM(selectedServer.id, version);
      await fetchPHPStatus();
      if (mainTab === 'pools') await fetchPools();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
      setError(getErrorMessage(err));
    } finally {
      setRestartingVer(null);
    }
  };

  const handleToggleFPM = async (version: string, enable: boolean) => {
    if (!selectedServer) return;
    if (!enable && !(await confirm(t('php.stopConfirm', { version })))) {
      return;
    }
    setTogglingVer(version);
    try {
      await TogglePHPFPM(selectedServer.id, version, enable);
      await fetchPHPStatus();
      if (mainTab === 'pools') await fetchPools();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
      setError(getErrorMessage(err));
    } finally {
      setTogglingVer(null);
    }
  };

  const handleSetDefaultCLI = async (version: string) => {
    if (!selectedServer) return;
    setSettingDefaultCli(version);
    try {
      await SetDefaultCLIPHP(selectedServer.id, version);
      alert(t('php.defaultCliSetSuccess', { version }) || `PHP ${version} установлен версией по умолчанию для CLI`);
      await fetchPHPStatus();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setSettingDefaultCli(null);
    }
  };

  const handleUpdatePHP = async (version: string) => {
    if (!selectedServer) return;
    if (!(await confirm(t('php.updateConfirm', { version })))) {
      return;
    }
    setUpdatingVer(version);
    try {
      const task = await UpdatePHPVersion(selectedServer.id, version);
      if (task && onOpenTask) {
        onOpenTask(task as TaskRecord);
      }
    } catch (err: unknown) {
      alert(getErrorMessage(err));
      setError(getErrorMessage(err));
    } finally {
      setUpdatingVer(null);
    }
  };

  const handleCheckAppRequirements = async (appName: string) => {
    if (!selectedServer || !selectedVersionStr) return;
    setCheckingApp(appName);
    try {
      const result = await CheckPHPAppRequirements(selectedServer.id, selectedVersionStr, appName);
      setAppReqResult(result);
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setCheckingApp(null);
    }
  };

  const handleToggleExt = async (extName: string, enable: boolean) => {
    if (!selectedServer || !selectedVersionStr) return;
    setTogglingExt(extName);
    try {
      const task = await TogglePHPExtension(selectedServer.id, selectedVersionStr, extName, enable);
      if (task && onOpenTask) onOpenTask(task as TaskRecord);
      await fetchPHPStatus();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setTogglingExt(null);
    }
  };

  const handleRemoveExt = async (extName: string) => {
    if (!selectedServer || !selectedVersionStr) return;
    if (!(await confirm(t('php.confirmRemoveExt', { name: extName }) || `Удалить расширение ${extName}?`))) {
      return;
    }
    setRemovingExt(extName);
    try {
      const task = await RemovePHPExtension(selectedServer.id, selectedVersionStr, extName);
      if (task && onOpenTask) onOpenTask(task as TaskRecord);
      await fetchPHPStatus();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setRemovingExt(null);
    }
  };

  const handleTogglePool = async (pName: string, enable: boolean) => {
    if (!selectedServer || !selectedVersionStr) return;
    setTogglingPool(pName);
    try {
      await TogglePHPPool(selectedServer.id, selectedVersionStr, pName, enable);
      await fetchPools();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setTogglingPool(null);
    }
  };

  const handleDeletePool = async (pName: string) => {
    if (!selectedServer || !selectedVersionStr) return;
    if (!(await confirm(t('php.confirmDeletePool', { name: pName }) || `Удалить пул ${pName}?`))) {
      return;
    }
    setDeletingPool(pName);
    try {
      await DeletePHPPool(selectedServer.id, selectedVersionStr, pName);
      await fetchPools();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setDeletingPool(null);
    }
  };

  useEffect(() => {
    const unregister = EventsOn('task:finished', (data: TaskRecord) => {
      if (
        data &&
        data.server_id === selectedServerID &&
        data.status === 'completed' &&
        (data.operation_type === 'php_ext_install' ||
          data.operation_type === 'php_ext_remove' ||
          data.operation_type === 'php_ext_toggle' ||
          data.operation_type === 'php_fpm_toggle' ||
          data.operation_type === 'php_fpm_restart' ||
          data.operation_type === 'php_install' ||
          data.operation_type === 'php_update')
      ) {
        fetchPHPStatus();
        fetchPools();
      }
    });
    return () => {
      if (typeof unregister === 'function') unregister();
    };
  }, [selectedServerID, fetchPHPStatus]);

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Code2}
        description={t('empty.phpDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Code2 className="w-5 h-5" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('php.targetServer')}
        filters={
          handlers.length > 0 ? (
            <div className="hidden lg:flex items-center gap-1.5 bg-slate-900/80 px-3 py-1.5 rounded-xl border border-slate-800 text-xs text-slate-400">
              <span className="text-[11px] font-medium text-slate-500">{t('php.detectedHandlers') || 'Handlers'}:</span>
              {handlers.map((h) => (
                <Badge key={h} variant="indigo" mono size="xs">
                  {h}
                </Badge>
              ))}
            </div>
          ) : undefined
        }
        actions={
          <Button
            variant="secondary"
            size="md"
            onClick={() => {
              fetchPHPStatus();
              if (mainTab === 'pools') fetchPools();
            }}
            disabled={isLoading}
            icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />}
          >
            {t('php.scanServer')}
          </Button>
        }
      />

      {/* Main Tabs Navigation */}
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-3">
        <Tabs
          tabs={[
            {
              id: 'versions',
              label: t('php.tabVersions') || 'Версии PHP',
              icon: <Code2 className="w-4 h-4" />,
              badge: (
                <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] bg-slate-900/60 font-mono">
                  {counts.installed}
                </span>
              ),
            },
            {
              id: 'extensions',
              label: t('php.tabExtensions') || 'Расширения',
              icon: <Settings2 className="w-4 h-4" />,
            },
            {
              id: 'pools',
              label: t('php.tabPools') || 'FPM & Пулы',
              icon: <Layers className="w-4 h-4" />,
            },
            {
              id: 'config',
              label: t('php.tabConfig') || 'Конфигурация',
              icon: <FileCode className="w-4 h-4" />,
            },
          ]}
          activeTab={mainTab}
          onChange={(id) => setMainTab(id as any)}
          variant="pill"
        />

        {/* Selected Version Selector for sub-tabs */}
        {mainTab !== 'versions' && (
          <div className="flex items-center gap-2">
            <span className="text-xs text-slate-400">{t('php.activeVersionLabel') || 'Версия'}:</span>
            <Select
              value={selectedVersionStr}
              onChange={(e) => setSelectedVersionStr(e.target.value)}
              sizeVariant="sm"
              className="font-mono font-bold"
            >
              {installedVersions.map((v) => (
                <option key={v.version} value={v.version}>
                  PHP {v.version} {v.is_default_cli ? '★ CLI' : ''}
                </option>
              ))}
            </Select>
          </div>
        )}
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-3">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* TAB 1: VERSIONS */}
      {mainTab === 'versions' && (
        <div className="space-y-4">
          {/* Filters */}
          <div className="flex items-center justify-between">
            <Tabs
              tabs={[
                { id: 'all', label: t('php.filterAll') || 'Все', badge: <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-slate-800 text-slate-400">{counts.all}</span> },
                { id: 'installed', label: t('php.filterInstalled') || 'Установлено', badge: <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-slate-800 text-slate-400">{counts.installed}</span> },
                { id: 'available', label: t('php.filterAvailable') || 'Доступно', badge: <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-slate-800 text-slate-400">{counts.available}</span> },
                { id: 'unavailable', label: t('php.filterUnavailable') || 'Не доступно', badge: <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-slate-800 text-slate-400">{counts.unavailable}</span> },
              ]}
              activeTab={filter}
              onChange={(id) => setFilter(id as any)}
              variant="pill"
            />
          </div>

          {isLoading ? (
            <LoadingState
              type="cards"
              title={t('php.loadingTitle')}
              subtitle={t('php.loadingSubtitle', { server: selectedServer?.name || '' })}
            />
          ) : filteredVersions.length === 0 ? (
            <div className="bg-surface-850 border border-slate-800 rounded-2xl p-12 text-center flex flex-col items-center justify-center">
              <Filter className="w-8 h-8 text-slate-500 mb-2" />
              <h4 className="text-white font-medium text-sm mb-1">{t('php.noVersionsMatch')}</h4>
              <button
                onClick={() => setFilter('all')}
                className="mt-3 text-xs text-indigo-400 hover:text-indigo-300 font-medium underline"
              >
                {t('php.filterAll')}
              </button>
            </div>
          ) : (
            <div className="card-grid">
              {filteredVersions.map((ver) => {
                const activeExtCount = ver.extensions?.filter((e) => e.is_enabled).length || 0;
                const totalExtCount = ver.extensions?.length || 0;
                const isRestarting = restartingVer === ver.version;
                const isToggling = togglingVer === ver.version;
                const isInstalled = ver.is_installed;
                const isAvailable = ver.is_available;

                return (
                  <div
                    key={ver.version}
                    className="bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 shadow-lg flex flex-col justify-between transition-all"
                  >
                    <div>
                      {/* Header */}
                      <div className="flex items-start justify-between gap-2 mb-4">
                        <div className="flex items-center gap-3 min-w-0">
                          <div
                            className={`w-10 h-10 shrink-0 rounded-xl flex items-center justify-center font-mono font-bold text-sm ${isInstalled
                              ? ver.fpm_status === 'active'
                                ? 'bg-purple-600/20 text-purple-300 border border-purple-500/30'
                                : 'bg-slate-800/90 text-slate-400 border border-slate-700/60'
                              : isAvailable
                                ? 'bg-blue-600/15 text-blue-300 border border-blue-500/20'
                                : 'bg-slate-800/80 text-slate-500 border border-slate-700/50'
                              }`}
                          >
                            {ver.version}
                          </div>
                          <div className="min-w-0">
                            <div className="flex flex-wrap items-center gap-2">
                              <h3 className="font-bold text-white text-base">PHP {ver.version}</h3>
                              {ver.patch_version && (
                                <span className="text-xs text-slate-400 font-mono">
                                  ({ver.patch_version})
                                </span>
                              )}
                              {ver.is_default_cli && (
                                <span className="text-[10px] px-1.5 py-0.2 rounded bg-indigo-500/20 text-indigo-300 font-mono font-bold border border-indigo-500/30">
                                  {t('php.cliDefaultBadge')}
                                </span>
                              )}
                            </div>
                            <div className="text-xs text-slate-400 font-mono truncate">
                              {isInstalled
                                ? ver.fpm_status === 'active'
                                  ? t('php.installedAndReady')
                                  : t('php.installedAndStopped')
                                : isAvailable
                                  ? t('php.availableForInstall')
                                  : t('php.unavailableForInstall')}
                            </div>
                          </div>
                        </div>

                        <span
                          className={`shrink-0 px-2.5 py-0.5 rounded-full text-xs font-semibold inline-flex items-center gap-1.5 border whitespace-nowrap ${isInstalled
                            ? ver.fpm_status === 'active'
                              ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                              : 'bg-slate-800 text-slate-400 border-slate-700'
                            : isAvailable
                              ? 'bg-blue-500/15 text-blue-400 border-blue-500/30'
                              : 'bg-slate-800 text-slate-400 border-slate-700'
                            }`}
                        >
                          {isInstalled ? (
                            ver.fpm_status === 'active' ? (
                              <>
                                <CheckCircle2 className="w-3.5 h-3.5" /> {t('php.installed')}
                              </>
                            ) : (
                              <>
                                <Pause className="w-3.5 h-3.5" /> {t('php.stopped')}
                              </>
                            )
                          ) : isAvailable ? (
                            <>
                              <Download className="w-3.5 h-3.5" /> {t('php.available')}
                            </>
                          ) : (
                            <>
                              <XCircle className="w-3.5 h-3.5" /> {t('php.notAvailable')}
                            </>
                          )}
                        </span>
                      </div>

                      {/* Installed Details */}
                      {isInstalled ? (
                        <div className="space-y-3 mb-5">
                          {/* FPM Service status */}
                          <div className="p-3 rounded-lg bg-slate-900 border border-slate-800/80 flex items-center justify-between gap-2 text-xs">
                            <div className="min-w-0">
                              <span className="text-slate-400 block text-[11px]">{t('php.fpmService')}</span>
                              <div className="flex items-center gap-1.5 font-semibold text-white mt-0.5">
                                <span
                                  className={`w-2 h-2 rounded-full ${ver.fpm_status === 'active'
                                    ? 'bg-emerald-400 animate-pulse'
                                    : 'bg-slate-500'
                                    }`}
                                />
                                <span className="capitalize">
                                  {ver.fpm_status === 'active' ? t('php.fpmActive') : t('php.fpmInactive')}
                                </span>
                              </div>
                            </div>

                            <div className="flex items-center gap-1.5 shrink-0">
                              <button
                                onClick={() => handleToggleFPM(ver.version, ver.fpm_status !== 'active')}
                                disabled={isToggling || isRestarting}
                                className={`p-1.5 rounded-lg border text-xs transition cursor-pointer ${ver.fpm_status === 'active'
                                  ? 'text-emerald-400 hover:bg-emerald-500/10 border-emerald-500/20'
                                  : 'text-slate-400 hover:bg-slate-700/50 border-slate-700'
                                  }`}
                                title={
                                  ver.fpm_status === 'active'
                                    ? t('php.stopFpmTooltip', { version: ver.version })
                                    : t('php.startFpmTooltip', { version: ver.version })
                                }
                              >
                                <Power className={`w-3.5 h-3.5 ${isToggling ? 'animate-spin' : ''}`} />
                              </button>

                              {ver.fpm_status === 'active' && (
                                <button
                                  onClick={() => handleRestartFPM(ver.version)}
                                  disabled={isRestarting || isToggling}
                                  className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors shrink-0"
                                  title={t('php.restartFpmTooltip')}
                                >
                                  <RotateCw className={`w-3.5 h-3.5 ${isRestarting ? 'animate-spin text-indigo-400' : ''}`} />
                                </button>
                              )}

                              <button
                                onClick={() => setLogsModalVer(ver.version)}
                                className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors shrink-0"
                                title={t('php.viewLogsTooltip') || 'Просмотр логов службы'}
                              >
                                <FileCode className="w-3.5 h-3.5 text-cyan-400" />
                              </button>
                            </div>
                          </div>

                          {/* Socket path */}
                          <div className="text-[11px] font-mono text-slate-400 bg-slate-900/60 p-2.5 rounded-lg border border-slate-800/60 break-all select-all">
                            {t('php.socketLabel', { path: ver.socket_path })}
                          </div>

                          {/* Default CLI button if not default */}
                          {!ver.is_default_cli && (
                            <button
                              onClick={() => handleSetDefaultCLI(ver.version)}
                              disabled={settingDefaultCli === ver.version}
                              className="w-full py-1.5 px-3 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition-colors flex items-center justify-center gap-1.5 border border-slate-700"
                            >
                              {settingDefaultCli === ver.version ? (
                                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                              ) : (
                                <CheckSquare className="w-3.5 h-3.5 text-indigo-400" />
                              )}
                              <span>{t('php.setAsDefaultCLI') || 'Сделать версией CLI по умолчанию'}</span>
                            </button>
                          )}
                        </div>
                      ) : (
                        <div className="p-4 rounded-lg bg-slate-900/40 border border-slate-800/50 mb-5 text-xs text-slate-400 leading-relaxed">
                          {isAvailable
                            ? t('php.installDescription', { version: ver.version })
                            : (ver.unavailable_reason || t('php.notSupportedNotice'))}
                        </div>
                      )}
                    </div>

                    {/* Bottom action button */}
                    <div className="pt-3 border-t border-slate-800 flex items-center justify-between text-xs">
                      {isInstalled ? (
                        <div className="flex items-center gap-2 w-full">
                          <button
                            onClick={() => {
                              setSelectedVersionStr(ver.version);
                              setMainTab('extensions');
                            }}
                            className="flex-1 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-750 text-slate-200 font-medium flex items-center justify-center gap-1.5 transition-colors"
                          >
                            <Settings2 className="w-3.5 h-3.5 text-indigo-400" />
                            <span>{t('php.extensionsBtn')} ({activeExtCount})</span>
                          </button>

                          <button
                            onClick={() => {
                              setSelectedVersionStr(ver.version);
                              setMainTab('pools');
                            }}
                            className="flex-1 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-750 text-slate-200 font-medium flex items-center justify-center gap-1.5 transition-colors"
                          >
                            <Layers className="w-3.5 h-3.5 text-cyan-400" />
                            <span>{t('php.poolBtn')}</span>
                          </button>
                        </div>
                      ) : isAvailable ? (
                        <button
                          onClick={() => setInstallVersion(ver.version)}
                          className="w-full py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-bold flex items-center justify-center gap-2 shadow-md shadow-indigo-600/30 transition-all hover:scale-[1.01]"
                        >
                          <Download className="w-4 h-4" />
                          <span>{t('php.installPhpBtn', { version: ver.version })}</span>
                        </button>
                      ) : (
                        <div className="w-full py-2 rounded-lg bg-slate-800/40 border border-slate-800 text-slate-500 font-medium text-xs flex items-center justify-center gap-2 cursor-not-allowed select-none">
                          <XCircle className="w-4 h-4 text-slate-500" />
                          <span>{t('php.notAvailable')}</span>
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* TAB 2: EXTENSIONS */}
      {mainTab === 'extensions' && (
        <div className="space-y-6">
          {/* App Presets Checker */}
          <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h4 className="text-sm font-bold text-white flex items-center gap-2">
                  <span>{t('php.appPresetCheckerTitle') || 'Проверка совместимости приложений'}</span>
                </h4>
                <p className="text-xs text-slate-400 mt-0.5">
                  {t('php.appPresetCheckerSubtitle') || 'Проверьте наличие обязательных расширений для популярных CMS и фреймворков'}
                </p>
              </div>

              <div className="flex items-center gap-2">
                {(['laravel', 'wordpress', 'drupal'] as const).map((app) => (
                  <button
                    key={app}
                    onClick={() => handleCheckAppRequirements(app)}
                    disabled={checkingApp === app}
                    className="px-3 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-850 border border-slate-700 text-xs font-semibold text-slate-200 capitalize flex items-center gap-1.5 transition-colors disabled:opacity-50"
                  >
                    {checkingApp === app && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                    <span>{app}</span>
                  </button>
                ))}
              </div>
            </div>

            {/* Preset Result */}
            {appReqResult && (
              <div
                className={`p-4 rounded-xl border text-xs space-y-3 ${appReqResult.is_compatible
                  ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                  : 'bg-amber-500/10 border-amber-500/30 text-amber-300'
                  }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 font-bold uppercase tracking-wider text-xs">
                    {appReqResult.is_compatible ? (
                      <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                    ) : (
                      <AlertCircle className="w-4 h-4 text-amber-400" />
                    )}
                    <span>
                      {appReqResult.app_type}:{' '}
                      {appReqResult.is_compatible
                        ? (t('php.requirementsSatisfied') || 'Все необходимые расширения установлены')
                        : (t('php.missingRequirements') || 'Отсутствуют обязательные расширения')}
                    </span>
                  </div>

                  {!appReqResult.is_compatible && appReqResult.missing_extensions.length > 0 && (
                    <Button
                      variant="warning"
                      size="sm"
                      onClick={() => {
                        if (activeVersionObj) {
                          setExtInstallInfo(activeVersionObj);
                        }
                      }}
                    >
                      {t('php.installMissingExts') || 'Установить отсутствующие'}
                    </Button>
                  )}
                </div>

                {!appReqResult.is_compatible && appReqResult.missing_extensions.length > 0 && (
                  <div className="flex flex-wrap gap-1.5">
                    {appReqResult.missing_extensions.map((ext) => (
                      <span
                        key={ext}
                        className="px-2 py-0.5 rounded bg-rose-500/20 text-rose-300 border border-rose-500/30 font-mono text-[11px]"
                      >
                        {ext}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Extensions Search & Actions */}
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex-1 max-w-sm">
              <SearchInput
                value={extSearch}
                onChange={(e) => setExtSearch(e.target.value)}
                onClear={() => setExtSearch('')}
                placeholder={t('php.searchExtensions') || 'Поиск расширения...'}
              />
            </div>

            <Button
              variant="primary"
              size="md"
              onClick={() => {
                if (activeVersionObj) setExtInstallInfo(activeVersionObj);
              }}
              icon={<Download className="w-3.5 h-3.5" />}
            >
              {t('php.installNewExtensions') || 'Установить расширения'}
            </Button>
          </div>

          {/* Extensions Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {(activeVersionObj?.extensions || [])
              .filter((e) => e.name.toLowerCase().includes(extSearch.toLowerCase()) || (e.description && e.description.toLowerCase().includes(extSearch.toLowerCase())))
              .map((ext) => (
                <div
                  key={ext.name}
                  className="bg-surface-850 p-4 rounded-xl border border-slate-800 flex items-center justify-between gap-3"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-white text-xs font-mono">{ext.name}</span>
                      {ext.version && (
                        <span className="text-[10px] text-slate-500 font-mono">v{ext.version}</span>
                      )}
                    </div>
                    {ext.description && (
                      <p className="text-[11px] text-slate-400 truncate mt-0.5">{ext.description}</p>
                    )}
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      onClick={() => handleToggleExt(ext.name, !ext.is_enabled)}
                      disabled={togglingExt === ext.name}
                      className={`px-2 py-1 rounded text-[11px] font-semibold border transition-all ${ext.is_enabled
                        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300 hover:bg-emerald-500/20'
                        : 'bg-slate-800 border-slate-700 text-slate-400 hover:bg-slate-700'
                        }`}
                    >
                      {togglingExt === ext.name ? (
                        <Loader2 className="w-3 h-3 animate-spin" />
                      ) : ext.is_enabled ? (
                        'ON'
                      ) : (
                        'OFF'
                      )}
                    </button>

                    <button
                      onClick={() => handleRemoveExt(ext.name)}
                      disabled={removingExt === ext.name}
                      className="p-1 rounded text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition-colors"
                      title={t('common.delete') || 'Удалить'}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              ))}
          </div>
        </div>
      )}

      {/* TAB 3: FPM & POOLS */}
      {mainTab === 'pools' && (
        <div className="space-y-6">
          {/* FPM Status header */}
          <div className="flex flex-wrap items-center justify-between gap-4 bg-surface-850 p-4 rounded-xl border border-slate-800">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-xl bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
                <Layers className="w-5 h-5" />
              </div>
              <div>
                <h4 className="font-bold text-white text-sm">
                  PHP {selectedVersionStr}-FPM Service
                </h4>
                <div className="flex items-center gap-2 mt-0.5 text-xs text-slate-400">
                  <span
                    className={`w-2 h-2 rounded-full ${activeVersionObj?.fpm_status === 'active' ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'
                      }`}
                  />
                  <span className="capitalize">{activeVersionObj?.fpm_status || 'unknown'}</span>
                  <span>•</span>
                  <span>{pools.length} {t('php.poolsCount') || 'пулов'}</span>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <button
                onClick={() => setLogsModalVer(selectedVersionStr)}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium transition-colors"
              >
                <FileCode className="w-3.5 h-3.5 text-cyan-400" />
                <span>{t('php.viewLogsBtn') || 'Логи службы'}</span>
              </button>

              <button
                onClick={() => setPoolVersion(selectedVersionStr)}
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold transition-colors"
              >
                <Layers className="w-3.5 h-3.5" />
                <span>{t('php.createPoolBtn') || 'Создать пул'}</span>
              </button>
            </div>
          </div>

          {/* Pools Cards */}
          {loadingPools ? (
            <div className="flex items-center justify-center p-12 text-slate-400">
              <Loader2 className="w-8 h-8 animate-spin text-indigo-400 mr-2" />
              <span className="text-xs">{t('common.loadingShort')}</span>
            </div>
          ) : pools.length === 0 ? (
            <div className="text-center p-12 bg-surface-850 rounded-xl border border-slate-800">
              <p className="text-xs text-slate-400 mb-3">
                {t('php.noPoolsFound') || 'Пулы не найдены для этой версии PHP.'}
              </p>
              <button
                onClick={() => {
                  setPoolVersion(selectedVersionStr);
                }}
                className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold"
              >
                {t('php.createPoolBtn') || 'Создать пул'}
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {pools.map((p) => (
                <div
                  key={p.pool_name}
                  className="bg-surface-850 p-5 rounded-xl border border-slate-800 flex flex-col justify-between space-y-4 hover:border-slate-700 transition-all"
                >
                  <div className="space-y-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h4 className="font-bold text-white text-sm font-mono truncate">{p.pool_name}</h4>
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-semibold uppercase ${p.is_enabled !== false
                              ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/20'
                              : 'bg-slate-800 text-slate-500 border border-slate-700'
                              }`}
                          >
                            {p.is_enabled !== false ? 'Active' : 'Disabled'}
                          </span>
                        </div>
                        <span className="text-[11px] text-slate-400 font-mono block mt-0.5">
                          {p.user}:{p.group} • {p.pm_mode} (max {p.max_children})
                        </span>
                      </div>

                      <div className="flex items-center gap-1.5 shrink-0">
                        <button
                          onClick={() => handleTogglePool(p.pool_name, p.is_enabled === false)}
                          disabled={togglingPool === p.pool_name}
                          className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                          title={p.is_enabled !== false ? 'Disable pool' : 'Enable pool'}
                        >
                          <Power className={`w-3.5 h-3.5 ${togglingPool === p.pool_name ? 'animate-spin' : ''}`} />
                        </button>
                        <button
                          onClick={() => setEditingPool({ version: selectedVersionStr, pool: p })}
                          className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                          title="Edit pool settings"
                        >
                          <Edit className="w-3.5 h-3.5 text-indigo-400" />
                        </button>
                        <button
                          onClick={() => setCloningPool({ version: selectedVersionStr, poolName: p.pool_name })}
                          className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                          title="Clone pool"
                        >
                          <Copy className="w-3.5 h-3.5 text-cyan-400" />
                        </button>
                        {p.pool_name !== 'www' && (
                          <button
                            onClick={() => handleDeletePool(p.pool_name)}
                            disabled={deletingPool === p.pool_name}
                            className="p-1.5 rounded-lg bg-slate-800 hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 transition-colors"
                            title="Delete pool"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Listen Socket */}
                    <div className="bg-slate-900/60 p-2.5 rounded-lg border border-slate-800/80 font-mono text-[11px] text-slate-400 break-all select-all">
                      {p.listen || 'Socket not configured'}
                    </div>

                    {/* Quick Specs */}
                    <div className="grid grid-cols-3 gap-2 text-[11px] text-slate-400 font-mono bg-slate-900/40 p-2.5 rounded-lg">
                      <div>
                        <span className="text-slate-500 block text-[10px]">Memory:</span>
                        <span className="text-white font-semibold">{p.memory_limit || 'default'}</span>
                      </div>
                      <div>
                        <span className="text-slate-500 block text-[10px]">Upload:</span>
                        <span className="text-white font-semibold">{p.upload_max_filesize || 'default'}</span>
                      </div>
                      <div>
                        <span className="text-slate-500 block text-[10px]">Timeout:</span>
                        <span className="text-white font-semibold">{p.max_execution_time || 60}s</span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* TAB 4: CONFIGURATION */}
      {mainTab === 'config' && selectedServer && (
        <PhpSettingsTab serverId={selectedServer.id} version={selectedVersionStr} />
      )}

      {/* Modals */}
      {installVersion && selectedServer && (
        <InstallPHPModal
          isOpen={true}
          serverID={selectedServer.id}
          serverName={selectedServer.name}
          version={installVersion}
          onClose={() => setInstallVersion(null)}
          onTaskStarted={onOpenTask}
        />
      )}

      {extInstallInfo && selectedServer && (
        <InstallPHPExtensionsModal
          isOpen={true}
          serverID={selectedServer.id}
          serverName={selectedServer.name}
          version={extInstallInfo.version}
          installedExtensions={extInstallInfo.extensions}
          onClose={() => setExtInstallInfo(null)}
          onTaskStarted={onOpenTask}
        />
      )}

      {logsModalVer && selectedServer && (
        <PHPServiceLogsModal
          isOpen={true}
          serverId={selectedServer.id}
          version={logsModalVer}
          onClose={() => setLogsModalVer(null)}
        />
      )}

      {editingPool && selectedServer && (
        <EditPoolModal
          isOpen={true}
          serverId={selectedServer.id}
          version={editingPool.version}
          pool={editingPool.pool}
          onClose={() => setEditingPool(null)}
          onSuccess={fetchPools}
        />
      )}

      {cloningPool && selectedServer && (
        <ClonePoolModal
          isOpen={true}
          serverId={selectedServer.id}
          version={cloningPool.version}
          sourcePoolName={cloningPool.poolName}
          onClose={() => setCloningPool(null)}
          onSuccess={fetchPools}
        />
      )}

      {/* Create PHP-FPM Pool Modal */}
      {poolVersion && selectedServer && (
        <CreatePoolModal
          isOpen={true}
          serverId={selectedServer.id}
          version={poolVersion}
          onClose={() => setPoolVersion(null)}
          onSuccess={(name) => {
            alert(t('php.poolSuccess', { name }));
            fetchPools();
          }}
        />
      )}
    </div>
  );
};
