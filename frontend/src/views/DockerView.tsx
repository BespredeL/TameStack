import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { useServerRequest } from '../hooks/useServerRequest';
import {
  Container,
  RefreshCw,
  Trash2,
  Play,
  Square,
  Terminal,
  Download,
  FileCode,
  HardDrive,
  Network,
  Layers,
  Sparkles,
  Activity,
} from 'lucide-react';
import {
  GetDockerStatus,
  ListDockerContainers,
  ListDockerImages,
  ListDockerVolumes,
  ListDockerNetworks,
  ListDockerComposeProjects,
  DockerContainerAction,
  DockerPrune,
} from '../../wailsjs/go/main/App';
import { docker } from '../../wailsjs/go/models';
import { PageToolbar, Button, SearchInput, Tabs } from '../components/ui';
import { ServerRecord, TaskRecord } from '../types';
import { ContainerLogsModal } from '../components/ContainerLogsModal';
import { PullImageModal } from '../components/PullImageModal';
import { ComposeEditorModal } from '../components/ComposeEditorModal';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';

interface DockerViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

export const DockerView: React.FC<DockerViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
    setPageLoading,
  } = useServer();
  const { t } = useTranslation();
  const confirm = useConfirm();
  const [status, setStatus] = useState<docker.DockerStatus | null>(null);
  const [activeTab, setActiveTab] = useState<'containers' | 'compose' | 'images' | 'volumes' | 'networks'>('containers');
  const [showAllContainers, setShowAllContainers] = useState<boolean>(true);

  const [containers, setContainers] = useState<docker.ContainerItem[]>([]);
  const [composeProjects, setComposeProjects] = useState<docker.ComposeProject[]>([]);
  const [images, setImages] = useState<docker.ImageItem[]>([]);
  const [volumes, setVolumes] = useState<docker.VolumeItem[]>([]);
  const [networks, setNetworks] = useState<docker.NetworkItem[]>([]);

  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState<string>('');
  const statusReq = useServerRequest();
  const tabReq = useServerRequest();

  // Modals state
  const [logsTarget, setLogsTarget] = useState<{ id: string; name: string } | null>(null);
  const [isPullOpen, setIsPullOpen] = useState<boolean>(false);
  const [composeTarget, setComposeTarget] = useState<{ dir: string; name: string } | null>(null);

  const fetchStatus = async () => {
    if (!selectedServerID) return;
    const serverID = selectedServerID;
    const gen = statusReq.next();
    setStatus(null);
    try {
      setIsLoading(true);
      setPageLoading(true, t('docker.loadingStatus'));
      const res = await GetDockerStatus(serverID);
      if (!statusReq.isCurrent(gen)) return;
      setStatus(res);
    } catch (err: unknown) {
      if (!statusReq.isCurrent(gen)) return;
      const msg = getErrorMessage(err);
      if (msg.includes('127') || msg.includes('not found')) {
        setStatus({ installed: false, version: '', compose_version: '', service_status: 'inactive', containers_count: 0, running_count: 0, images_count: 0, volumes_count: 0 });
        return;
      }
      setActionError(msg);
    } finally {
      if (statusReq.isCurrent(gen)) {
        setIsLoading(false);
        setPageLoading(false);
      }
    }
  };

  const fetchTabData = async () => {
    if (!selectedServerID) return;
    if (status && !status.installed) return;
    const serverID = selectedServerID;
    const gen = tabReq.next();
    setContainers([]);
    setComposeProjects([]);
    setImages([]);
    setVolumes([]);
    setNetworks([]);
    try {
      setIsLoading(true);
      setPageLoading(true, t('docker.loadingTab', { tab: activeTab }));
      switch (activeTab) {
        case 'containers': {
          const res = await ListDockerContainers(serverID, showAllContainers);
          if (!tabReq.isCurrent(gen)) return;
          setContainers(res || []);
          break;
        }
        case 'compose': {
          const res = await ListDockerComposeProjects(serverID);
          if (!tabReq.isCurrent(gen)) return;
          setComposeProjects(res || []);
          break;
        }
        case 'images': {
          const res = await ListDockerImages(serverID);
          if (!tabReq.isCurrent(gen)) return;
          setImages(res || []);
          break;
        }
        case 'volumes': {
          const res = await ListDockerVolumes(serverID);
          if (!tabReq.isCurrent(gen)) return;
          setVolumes(res || []);
          break;
        }
        case 'networks': {
          const res = await ListDockerNetworks(serverID);
          if (!tabReq.isCurrent(gen)) return;
          setNetworks(res || []);
          break;
        }
      }
    } catch (err: unknown) {
      if (!tabReq.isCurrent(gen)) return;
      const msg = getErrorMessage(err);
      if (msg.includes('127') || msg.includes('not found') || msg.includes('Process exited with status 127')) {
        // Docker not installed, suppress error
        return;
      }
      setActionError(msg);
    } finally {
      if (tabReq.isCurrent(gen)) {
        setIsLoading(false);
        setPageLoading(false);
      }
    }
  };

  useEffect(() => {
    fetchStatus();
  }, [selectedServerID]);

  useEffect(() => {
    fetchTabData();
  }, [selectedServerID, activeTab, showAllContainers]);

  const handleContainerAction = async (containerID: string, action: string, force: boolean = false) => {
    if (action === 'remove' && !(await confirm(t('docker.removeConfirm'), { danger: true, confirmLabel: t('common.delete') }))) {
      return;
    }
    if (action === 'stop' && !(await confirm(t('docker.stopConfirm'), { danger: true }))) {
      return;
    }
    try {
      const task = await DockerContainerAction({
        server_id: selectedServerID,
        container_id: containerID,
        action,
        force,
      });
      onOpenTask(task as any);
      setTimeout(fetchTabData, 2000);
      setTimeout(fetchStatus, 2000);
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    }
  };

  const handlePrune = async () => {
    if (!(await confirm(t('docker.pruneConfirm'), { danger: true }))) {
      return;
    }
    try {
      const task = await DockerPrune({
        server_id: selectedServerID,
        prune_type: 'all',
        prune_volumes: false,
      });
      onOpenTask(task as any);
      setTimeout(fetchTabData, 3000);
      setTimeout(fetchStatus, 3000);
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    }
  };

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Container}
        description={t('empty.dockerDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  const isRunning = status?.service_status === 'active';

  return (
    <div className="page space-y-6">
      {actionError && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
          {actionError}
        </div>
      )}
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Container className="w-5 h-5" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('docker.targetServer')}
        actions={
          <Button
            variant="secondary"
            size="md"
            onClick={() => {
              fetchStatus();
              fetchTabData();
            }}
            disabled={isLoading}
            icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />}
            title={t('docker.refresh')}
          >
            {t('docker.refresh')}
          </Button>
        }
      />

      {/* Docker Engine Status Card */}
      {isLoading && !status ? (
        <LoadingState
          type="stats"
          title={t('docker.loadingTitle')}
          subtitle={t('docker.loadingSubtitle')}
        />
      ) : status && status.installed ? (
        <>
          <div className="stat-grid">
            <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center gap-3.5 transition-colors">
              <div
                className={`w-11 h-11 rounded-xl flex items-center justify-center shrink-0 ${isRunning
                    ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                    : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                  }`}
              >
                <Activity className="w-5 h-5" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-1.5">
                  <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block truncate">
                    {t('docker.engineLabel')}
                  </span>
                  <span
                    className={`w-1.5 h-1.5 rounded-full shrink-0 ${
                      isRunning ? 'bg-emerald-400 animate-pulse' : 'bg-rose-400'
                    }`}
                  />
                </div>
                <div className="flex items-baseline gap-2 mt-0.5">
                  <span className={`text-base font-bold capitalize ${isRunning ? 'text-emerald-400' : 'text-rose-400'}`}>
                    {isRunning ? t('docker.statusActive') : t('docker.statusInactive')}
                  </span>
                  {status.version && (
                    <span className="text-[11px] text-slate-500 font-mono truncate" title={status.version}>
                      v{status.version.replace(/^Docker\s+version\s+/i, '').split(',')[0]}
                    </span>
                  )}
                </div>
              </div>
            </div>

            <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center gap-3.5 transition-colors">
              <div className="w-11 h-11 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0">
                <Container className="w-5 h-5" />
              </div>
              <div className="min-w-0 flex-1">
                <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block truncate">
                  {t('docker.containers')}
                </span>
                <div className="flex items-baseline gap-1.5 mt-0.5">
                  <span className="text-xl font-bold text-white font-mono">
                    {status.running_count}
                  </span>
                  <span className="text-xs text-slate-400 font-medium">
                    / {status.containers_count} {t('docker.containersActive')}
                  </span>
                </div>
              </div>
            </div>

            <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center gap-3.5 transition-colors">
              <div className="w-11 h-11 rounded-xl bg-purple-500/10 border border-purple-500/20 flex items-center justify-center text-purple-400 shrink-0">
                <Layers className="w-5 h-5" />
              </div>
              <div className="min-w-0 flex-1">
                <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block truncate">
                  {t('docker.imagesAndVolumes')}
                </span>
                <div className="flex items-baseline gap-1.5 mt-0.5">
                  <span className="text-xl font-bold text-white font-mono">
                    {status.images_count}
                  </span>
                  <span className="text-xs text-slate-400 font-medium">
                    {t('docker.imagesLabel')}
                  </span>
                  <span className="text-slate-600 font-bold mx-0.5">•</span>
                  <span className="text-xl font-bold text-white font-mono">
                    {status.volumes_count}
                  </span>
                  <span className="text-xs text-slate-400 font-medium">
                    {t('docker.volumesLabel')}
                  </span>
                </div>
              </div>
            </div>

            <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between gap-3 transition-colors">
              <div className="flex items-center gap-3.5 min-w-0">
                <div className="w-11 h-11 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400 shrink-0">
                  <Sparkles className="w-5 h-5" />
                </div>
                <div className="min-w-0">
                  <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block truncate">
                    {t('docker.cleanup')}
                  </span>
                  <span className="text-xs text-slate-500 truncate block mt-0.5">
                    {t('docker.cleanupDesc')}
                  </span>
                </div>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onClick={handlePrune}
                className="shrink-0"
              >
                {t('docker.cleanupBtn')}
              </Button>
            </div>
          </div>

          {/* Main Tabs Workspace */}
          <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
            {/* Workspace Toolbar */}
            <div className="px-6 py-4 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <Tabs
                  variant="pill"
                  activeTab={activeTab}
                  onChange={(id) => setActiveTab(id as any)}
                  tabs={[
                    {
                      id: 'containers',
                      label: t('docker.tabsContainers'),
                      icon: <Container className="w-3.5 h-3.5 shrink-0" />,
                    },
                    {
                      id: 'compose',
                      label: t('docker.tabsCompose'),
                      icon: <FileCode className="w-3.5 h-3.5 shrink-0" />,
                    },
                    {
                      id: 'images',
                      label: t('docker.tabsImages'),
                      icon: <Layers className="w-3.5 h-3.5 shrink-0" />,
                    },
                    {
                      id: 'volumes',
                      label: t('docker.tabsVolumes'),
                      icon: <HardDrive className="w-3.5 h-3.5 shrink-0" />,
                    },
                    {
                      id: 'networks',
                      label: t('docker.tabsNetworks'),
                      icon: <Network className="w-3.5 h-3.5 shrink-0" />,
                    },
                  ]}
                />

                <div className="w-48 sm:w-64">
                  <SearchInput
                    placeholder={t('docker.searchPlaceholder')}
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    onClear={() => setSearchQuery('')}
                  />
                </div>
              </div>

              <div className="flex items-center gap-2">
                {activeTab === 'containers' && (
                  <label className="flex items-center gap-2 text-xs text-slate-400 bg-slate-950 border border-slate-800 px-3 py-1.5 rounded-xl cursor-pointer">
                    <input
                      type="checkbox"
                      checked={showAllContainers}
                      onChange={(e) => setShowAllContainers(e.target.checked)}
                      className="rounded border-slate-700 text-indigo-600 focus:ring-0 bg-slate-900"
                    />
                    {t('docker.showStopped')}
                  </label>
                )}

                {activeTab === 'images' && (
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => setIsPullOpen(true)}
                    icon={<Download className="w-3.5 h-3.5" />}
                  >
                    {t('docker.pullImage')}
                  </Button>
                )}
              </div>
            </div>

            {/* Tab 1: Containers */}
            {activeTab === 'containers' && (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="border-b border-slate-800/80 bg-slate-950/40 text-slate-400 font-semibold uppercase tracking-wider">
                      <th className="px-6 py-3.5">{t('docker.colNameId')}</th>
                      <th className="px-6 py-3.5">{t('docker.colImage')}</th>
                      <th className="px-6 py-3.5">{t('docker.colStatus')}</th>
                      <th className="px-6 py-3.5">{t('docker.colResources')}</th>
                      <th className="px-6 py-3.5">{t('docker.colPorts')}</th>
                      <th className="px-6 py-3.5 text-right">{t('docker.colActions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50">
                    {containers.filter((c) =>
                      c.names.toLowerCase().includes(searchQuery.toLowerCase()) ||
                      c.image.toLowerCase().includes(searchQuery.toLowerCase())
                    ).length === 0 ? (
                      <tr>
                        <td colSpan={6} className="px-6 py-12 text-center text-slate-500">
                          {t('docker.noContainers')}
                        </td>
                      </tr>
                    ) : (
                      containers
                        .filter((c) =>
                          c.names.toLowerCase().includes(searchQuery.toLowerCase()) ||
                          c.image.toLowerCase().includes(searchQuery.toLowerCase())
                        )
                        .map((c) => {
                          const isUp = c.state === 'running';

                          return (
                            <tr key={c.id} className="hover:bg-slate-850/40 transition-colors">
                              <td className="px-6 py-4 font-medium text-white">
                                <div className="flex items-center gap-2">
                                  <span
                                    className={`w-2 h-2 rounded-full ${isUp ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600'
                                      }`}
                                  />
                                  <span className="font-semibold">{c.names}</span>
                                </div>
                                <span className="font-mono text-[11px] text-slate-500 pl-4 block">
                                  {c.id.slice(0, 12)}
                                </span>
                              </td>
                              <td className="px-6 py-4 font-mono text-indigo-400 text-[11px]">
                                {c.image}
                              </td>
                              <td className="px-6 py-4">
                                <span
                                  className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium border ${isUp
                                      ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                                      : 'bg-slate-800 text-slate-400 border-slate-700'
                                    }`}
                                >
                                  {c.status}
                                </span>
                              </td>
                              <td className="px-6 py-4 font-mono text-slate-300 text-[11px]">
                                {isUp ? (
                                  <div className="space-y-0.5">
                                    <div>CPU: <span className="text-white">{c.cpu_percent}</span></div>
                                    <div>RAM: <span className="text-slate-400">{c.mem_usage}</span></div>
                                  </div>
                                ) : (
                                  <span className="text-slate-600">-</span>
                                )}
                              </td>
                              <td className="px-6 py-4 font-mono text-slate-400 text-[11px] max-w-xs truncate">
                                {c.ports || '-'}
                              </td>
                              <td className="px-6 py-4 text-right">
                                <div className="flex items-center justify-end gap-1.5">
                                  {isUp ? (
                                    <>
                                      <button
                                        onClick={() => handleContainerAction(c.id, 'restart')}
                                        className="p-1.5 rounded-lg text-slate-400 hover:text-indigo-400 hover:bg-slate-800 transition-colors"
                                        title={t('docker.actionRestart')}
                                      >
                                        <RefreshCw className="w-3.5 h-3.5" />
                                      </button>
                                      <button
                                        onClick={() => handleContainerAction(c.id, 'stop')}
                                        className="p-1.5 rounded-lg text-slate-400 hover:text-amber-400 hover:bg-slate-800 transition-colors"
                                        title={t('docker.actionStop')}
                                      >
                                        <Square className="w-3.5 h-3.5" />
                                      </button>
                                    </>
                                  ) : (
                                    <button
                                      onClick={() => handleContainerAction(c.id, 'start')}
                                      className="p-1.5 rounded-lg text-slate-400 hover:text-emerald-400 hover:bg-slate-800 transition-colors"
                                      title={t('docker.actionStart')}
                                    >
                                      <Play className="w-3.5 h-3.5" />
                                    </button>
                                  )}
                                  <button
                                    onClick={() => setLogsTarget({ id: c.id, name: c.names })}
                                    className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                                    title={t('docker.actionLogs')}
                                  >
                                    <Terminal className="w-3.5 h-3.5" />
                                  </button>
                                  <button
                                    onClick={() => handleContainerAction(c.id, 'remove', true)}
                                    className="p-1.5 rounded-lg text-slate-400 hover:text-rose-500 hover:bg-slate-800 transition-colors"
                                    title={t('docker.actionDelete')}
                                  >
                                    <Trash2 className="w-3.5 h-3.5" />
                                  </button>
                                </div>
                              </td>
                            </tr>
                          );
                        })
                    )}
                  </tbody>
                </table>
              </div>
            )}

            {/* Tab 2: Compose Projects */}
            {activeTab === 'compose' && (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="border-b border-slate-800/80 bg-slate-950/40 text-slate-400 font-semibold uppercase tracking-wider">
                      <th className="px-6 py-3.5">{t('docker.colStackName')}</th>
                      <th className="px-6 py-3.5">{t('docker.colStatus')}</th>
                      <th className="px-6 py-3.5">{t('docker.colConfigPath')}</th>
                      <th className="px-6 py-3.5 text-right">{t('docker.colActions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50">
                    {composeProjects.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="px-6 py-12 text-center text-slate-500">
                          {t('docker.noCompose')}
                        </td>
                      </tr>
                    ) : (
                      composeProjects.map((proj) => (
                        <tr key={proj.name} className="hover:bg-slate-850/40 transition-colors">
                          <td className="px-6 py-4 font-mono font-medium text-white flex items-center gap-2">
                            <FileCode className="w-4 h-4 text-indigo-400" />
                            {proj.name}
                          </td>
                          <td className="px-6 py-4">
                            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                              {proj.status}
                            </span>
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-400">
                            {proj.config_file}
                          </td>
                          <td className="px-6 py-4 text-right">
                            <button
                              onClick={() => setComposeTarget({ dir: proj.working_dir, name: proj.name })}
                              className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium inline-flex items-center gap-1.5 transition-colors"
                            >
                              <FileCode className="w-3.5 h-3.5 text-indigo-400" />
                              {t('docker.viewEditCompose')}
                            </button>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            )}

            {/* Tab 3: Images */}
            {activeTab === 'images' && (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="border-b border-slate-800/80 bg-slate-950/40 text-slate-400 font-semibold uppercase tracking-wider">
                      <th className="px-6 py-3.5">{t('docker.colRepository')}</th>
                      <th className="px-6 py-3.5">{t('docker.colTag')}</th>
                      <th className="px-6 py-3.5">{t('docker.colImageId')}</th>
                      <th className="px-6 py-3.5">{t('docker.colSize')}</th>
                      <th className="px-6 py-3.5">{t('docker.colCreated')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50">
                    {images.length === 0 ? (
                      <tr>
                        <td colSpan={5} className="px-6 py-12 text-center text-slate-500">
                          {t('docker.noImages')}
                        </td>
                      </tr>
                    ) : (
                      images.map((img) => (
                        <tr key={img.id} className="hover:bg-slate-850/40 transition-colors">
                          <td className="px-6 py-4 font-mono font-medium text-white flex items-center gap-2">
                            <Layers className="w-4 h-4 text-purple-400" />
                            {img.repository}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-300">
                            {img.tag}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-500">
                            {img.id.slice(0, 12)}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-300">
                            {img.size}
                          </td>
                          <td className="px-6 py-4 text-slate-400">
                            {img.created_at}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            )}

            {/* Tab 4: Volumes */}
            {activeTab === 'volumes' && (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="border-b border-slate-800/80 bg-slate-950/40 text-slate-400 font-semibold uppercase tracking-wider">
                      <th className="px-6 py-3.5">{t('docker.colVolumeName')}</th>
                      <th className="px-6 py-3.5">{t('docker.colDriver')}</th>
                      <th className="px-6 py-3.5">{t('docker.colScope')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50">
                    {volumes.length === 0 ? (
                      <tr>
                        <td colSpan={3} className="px-6 py-12 text-center text-slate-500">
                          {t('docker.noVolumes')}
                        </td>
                      </tr>
                    ) : (
                      volumes.map((vol) => (
                        <tr key={vol.name} className="hover:bg-slate-850/40 transition-colors">
                          <td className="px-6 py-4 font-mono font-medium text-white flex items-center gap-2">
                            <HardDrive className="w-4 h-4 text-indigo-400" />
                            {vol.name}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-400">
                            {vol.driver}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-400">
                            {vol.scope}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            )}

            {/* Tab 5: Networks */}
            {activeTab === 'networks' && (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="border-b border-slate-800/80 bg-slate-950/40 text-slate-400 font-semibold uppercase tracking-wider">
                      <th className="px-6 py-3.5">{t('docker.colNetworkName')}</th>
                      <th className="px-6 py-3.5">{t('docker.colDriver')}</th>
                      <th className="px-6 py-3.5">{t('docker.colScope')}</th>
                      <th className="px-6 py-3.5">ID</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50">
                    {networks.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="px-6 py-12 text-center text-slate-500">
                          {t('docker.noNetworks')}
                        </td>
                      </tr>
                    ) : (
                      networks.map((net) => (
                        <tr key={net.id} className="hover:bg-slate-850/40 transition-colors">
                          <td className="px-6 py-4 font-mono font-medium text-white flex items-center gap-2">
                            <Network className="w-4 h-4 text-indigo-400" />
                            {net.name}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-400">
                            {net.driver}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-400">
                            {net.scope}
                          </td>
                          <td className="px-6 py-4 font-mono text-slate-500">
                            {net.id.slice(0, 12)}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      ) : (
        <div className="p-8 rounded-2xl bg-surface-900 border border-slate-800 text-center flex flex-col items-center justify-center space-y-3 py-12">
          <div className="w-12 h-12 rounded-xl bg-slate-800 text-slate-500 flex items-center justify-center border border-slate-700">
            <Container className="w-6 h-6" />
          </div>
          <h3 className="text-sm font-bold text-slate-200">
            {t('docker.notInstalledTitle')}
          </h3>
          <p className="text-xs text-slate-400 max-w-md leading-relaxed">
            {t('docker.notInstalledDesc')}
          </p>
        </div>
      )}

      {/* Modals */}
      {logsTarget && (
        <ContainerLogsModal
          serverID={selectedServerID}
          containerID={logsTarget.id}
          containerName={logsTarget.name}
          onClose={() => setLogsTarget(null)}
        />
      )}

      {isPullOpen && (
        <PullImageModal
          serverID={selectedServerID}
          onClose={() => setIsPullOpen(false)}
          onTaskCreated={(t) => {
            onOpenTask(t);
            setTimeout(fetchTabData, 2000);
          }}
        />
      )}

      {composeTarget && (
        <ComposeEditorModal
          serverID={selectedServerID}
          workingDir={composeTarget.dir}
          projectName={composeTarget.name}
          onClose={() => setComposeTarget(null)}
          onTaskCreated={(t) => {
            onOpenTask(t);
            setTimeout(fetchTabData, 2000);
          }}
        />
      )}
    </div>
  );
};

