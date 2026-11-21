import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  Package,
  RefreshCw,
  Search,
  Download,
  Trash2,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  Layers,
  ShieldCheck,
  History,
} from 'lucide-react';
import { useServer } from '../context/ServerContext';
import { useServerData } from '../hooks/useServerData';
import { PageToolbar, Button, Badge, SearchInput, Tabs } from '../components/ui';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { ModalShell } from '../components/common/ModalShell';
import { packages } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import {
  ListInstalledPackages,
  SimulateInstallPackages,
  SimulateUpgradePackages,
  SimulateRemovePackages,
  InstallPackages,
  RemovePackages,
  UpgradePackages,
  ListPackageHistory,
} from '../../wailsjs/go/main/App';

type PackageCategory = 'all' | 'user' | 'system';

interface PackagesViewProps {
  onOpenAddServer: () => void;
}

export const PackagesView: React.FC<PackagesViewProps> = ({ onOpenAddServer }) => {
  const { t } = useTranslation();
  const {
    selectedServerId: activeServerId,
    selectedServer: activeServer,
    servers,
    setSelectedServerId: selectServer,
    setPageLoading,
  } = useServer();

  const [activeTab, setActiveTab] = useState<'installed' | 'updates' | 'install' | 'history'>('installed');
  const [searchQuery, setSearchQuery] = useState('');
  const [packageCategory, setPackageCategory] = useState<PackageCategory>('all');
  const [installedList, setInstalledList] = useState<packages.PackageItem[]>([]);
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 100;
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Simulation & action state
  const [simulating, setSimulating] = useState(false);
  const [simulation, setSimulation] = useState<packages.AptSimulationResult | null>(null);
  const [showSimModal, setShowSimModal] = useState(false);
  const [simActionType, setSimActionType] = useState<'install' | 'upgrade' | 'remove'>('upgrade');
  const [targetPackages, setTargetPackages] = useState<string[]>([]);
  const [newPkgInput, setNewPkgInput] = useState('');

  // Concurrency guards
  const isFetchingInstalledRef = useRef(false);

  const loadInstalled = useCallback(async (query: string = '') => {
    if (!activeServerId || isFetchingInstalledRef.current) return;
    isFetchingInstalledRef.current = true;
    setLoading(true);
    setError(null);
    setPageLoading(true, t('packages.loadingPackages'));
    try {
      const res = await ListInstalledPackages(activeServerId, query);
      setInstalledList(res || []);
      setCurrentPage(1);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      isFetchingInstalledRef.current = false;
      setLoading(false);
      setPageLoading(false);
    }
  }, [activeServerId, setPageLoading]);

  const { data: historyList, loading: loadingHistory, reload: loadHistory } = useServerData(
    (serverId) => ListPackageHistory(serverId).then((r) => r || []),
    [activeTab],
    {
      initial: [] as packages.PackageTaskHistoryItem[],
      enabled: activeTab === 'history',
      onError: (msg) => console.error('Failed to load history:', msg),
    },
  );

  useEffect(() => {
    if (activeServerId && activeTab === 'installed') {
      loadInstalled('');
    }
  }, [activeServerId, activeTab, loadInstalled]);

  // Classification counts
  const { userCount, systemCount } = useMemo(() => {
    let u = 0;
    let s = 0;
    for (const pkg of installedList) {
      const p = pkg as any;
      if (p.is_manual || (!p.is_system && (p.priority === 'optional' || p.priority === 'extra'))) {
        u++;
      } else {
        s++;
      }
    }
    return { userCount: u, systemCount: s };
  }, [installedList]);

  // Filtered by Category
  const categoryFilteredList = useMemo(() => {
    return installedList.filter((pkg) => {
      const p = pkg as any;
      const isUser = p.is_manual || (!p.is_system && (p.priority === 'optional' || p.priority === 'extra'));
      if (packageCategory === 'user') return isUser;
      if (packageCategory === 'system') return !isUser;
      return true;
    });
  }, [installedList, packageCategory]);

  const totalPages = Math.max(1, Math.ceil(categoryFilteredList.length / pageSize));
  const paginatedList = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return categoryFilteredList.slice(start, start + pageSize);
  }, [categoryFilteredList, currentPage, pageSize]);

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    loadInstalled(searchQuery);
  };

  const handleSimulateUpgrade = async () => {
    if (!activeServerId) return;
    setSimulating(true);
    setError(null);
    setPageLoading(true, t('packages.simulatingUpgrade'));
    try {
      const res = await SimulateUpgradePackages(activeServerId);
      setSimulation(res);
      setSimActionType('upgrade');
      setTargetPackages([]);
      setShowSimModal(true);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSimulating(false);
      setPageLoading(false);
    }
  };

  const handleSimulateInstall = async (pkgName: string) => {
    if (!activeServerId || !pkgName.trim()) return;
    const pkgs = pkgName.trim().split(/\s+/);
    setSimulating(true);
    setError(null);
    setPageLoading(true, t('packages.simulatingInstall', { packages: pkgs.join(', ') }));
    try {
      const res = await SimulateInstallPackages(activeServerId, pkgs);
      setSimulation(res);
      setSimActionType('install');
      setTargetPackages(pkgs);
      setShowSimModal(true);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSimulating(false);
      setPageLoading(false);
    }
  };

  const handleSimulateRemove = async (pkgName: string) => {
    if (!activeServerId) return;
    setSimulating(true);
    setError(null);
    setPageLoading(true, t('packages.simulatingRemove', { package: pkgName }));
    try {
      const res = await SimulateRemovePackages(activeServerId, [pkgName]);
      setSimulation(res);
      setSimActionType('remove');
      setTargetPackages([pkgName]);
      setShowSimModal(true);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSimulating(false);
      setPageLoading(false);
    }
  };

  const handleConfirmAction = async () => {
    if (!activeServerId) return;
    setError(null);
    try {
      if (simActionType === 'upgrade') {
        await UpgradePackages(activeServerId, targetPackages);
      } else if (simActionType === 'install') {
        await InstallPackages(activeServerId, targetPackages);
        setNewPkgInput('');
      } else if (simActionType === 'remove') {
        await RemovePackages(activeServerId, targetPackages);
      }
      setShowSimModal(false);
      setSimulation(null);
      setActiveTab('history');
      loadHistory();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Package}
        description={t('empty.packagesDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar / Server Selector */}
      <PageToolbar
        icon={<Package className="w-5 h-5" />}
        servers={servers}
        selectedServerId={activeServerId}
        onSelectServer={selectServer}
        serverLabel={t('packages.targetServer')}
        actions={
          <Button
            variant="secondary"
            size="md"
            onClick={() => {
              if (activeTab === 'installed') loadInstalled(searchQuery);
              else if (activeTab === 'history') loadHistory();
            }}
            disabled={loading}
            icon={<RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-indigo-400' : ''}`} />}
            title={t('packages.refreshTooltip')}
          >
            {t('packages.refreshBtn')}
          </Button>
        }
      />

      {/* Metrics Stat Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between">
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('packages.statTotal')}</span>
            <span className="text-2xl font-bold text-white font-mono">{installedList.length}</span>
          </div>
          <div className="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
            <Package className="w-5 h-5" />
          </div>
        </div>

        <div
          onClick={() => {
            setActiveTab('installed');
            setPackageCategory('user');
            setCurrentPage(1);
          }}
          className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between cursor-pointer hover:border-slate-700 transition"
        >
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('packages.statUser')}</span>
            <span className="text-2xl font-bold text-emerald-400 font-mono">{userCount}</span>
          </div>
          <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
            <Layers className="w-5 h-5" />
          </div>
        </div>

        <div
          onClick={() => {
            setActiveTab('installed');
            setPackageCategory('system');
            setCurrentPage(1);
          }}
          className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between cursor-pointer hover:border-slate-700 transition"
        >
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('packages.statSystem')}</span>
            <span className="text-2xl font-bold text-sky-400 font-mono">{systemCount}</span>
          </div>
          <div className="w-10 h-10 rounded-xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-center text-sky-400">
            <ShieldCheck className="w-5 h-5" />
          </div>
        </div>

        <div
          onClick={() => setActiveTab('updates')}
          className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between cursor-pointer hover:border-slate-700 transition"
        >
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('packages.statUpdates')}</span>
            <span className="text-xs font-semibold text-amber-400 block mt-1">{t('packages.statAptSimulation')}</span>
          </div>
          <div className="w-10 h-10 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400">
            <RefreshCw className="w-5 h-5" />
          </div>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs flex items-start gap-3">
          <AlertTriangle className="w-5 h-5 shrink-0 text-rose-400 mt-0.5" />
          <div className="flex-1 break-all">{error}</div>
        </div>
      )}

      {/* Tabs */}
      <Tabs
        variant="line"
        activeTab={activeTab}
        onChange={(id) => setActiveTab(id as any)}
        tabs={[
          {
            id: 'installed',
            label: t('packages.tabInstalledCount', { count: installedList.length }),
            icon: <Package className="w-4 h-4 shrink-0" />,
          },
          {
            id: 'updates',
            label: t('packages.tabUpgrade'),
            icon: <RefreshCw className="w-4 h-4 shrink-0" />,
          },
          {
            id: 'install',
            label: t('packages.tabInstall'),
            icon: <Download className="w-4 h-4 shrink-0" />,
          },
          {
            id: 'history',
            label: t('packages.tabHistory'),
            icon: <History className="w-4 h-4 shrink-0" />,
          },
        ]}
      />

      {/* TAB 1: Installed Packages */}
      {activeTab === 'installed' && (
        <div className="space-y-4">
          {/* Filter Bar & Search */}
          <div className="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3 bg-surface-850 p-3 rounded-2xl border border-slate-800 shadow-sm">
            {/* Category Pills */}
            <Tabs
              variant="pill"
              size="sm"
              activeTab={packageCategory}
              onChange={(id) => {
                setPackageCategory(id as PackageCategory);
                setCurrentPage(1);
              }}
              tabs={[
                { id: 'all', label: t('packages.filterAll', { count: installedList.length }) },
                {
                  id: 'user',
                  label: (
                    <span className="flex items-center gap-1.5">
                      <span>{t('packages.filterUser')}</span>
                      <span className="text-[10px] font-mono opacity-80">({userCount})</span>
                    </span>
                  ),
                },
                {
                  id: 'system',
                  label: (
                    <span className="flex items-center gap-1.5">
                      <span>{t('packages.filterSystem')}</span>
                      <span className="text-[10px] font-mono opacity-80">({systemCount})</span>
                    </span>
                  ),
                },
              ]}
            />

            {/* Search Input */}
            <form onSubmit={handleSearchSubmit} className="flex items-center gap-2 flex-1 md:max-w-md">
              <div className="flex-1">
                <SearchInput
                  value={searchQuery}
                  onChangeValue={setSearchQuery}
                  placeholder={t('packages.searchFilterPlaceholder')}
                  sizeVariant="sm"
                />
              </div>
              <Button
                type="submit"
                variant="primary"
                size="sm"
                disabled={loading}
              >
                {t('packages.searchBtn')}
              </Button>
            </form>
          </div>

          <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-lg">
            <div className="p-3 border-b border-slate-800 flex justify-between items-center text-xs text-slate-400 bg-slate-950/40">
              <span>
                {t('packages.filteredCount', { filtered: categoryFilteredList.length, total: installedList.length })}
              </span>
              {loading && (
                <span className="flex items-center gap-1.5 text-indigo-400">
                  <RefreshCw className="w-3.5 h-3.5 animate-spin" /> {t('packages.loading')}
                </span>
              )}
            </div>

            <div className="overflow-x-auto max-h-[600px] overflow-y-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-950/60 text-slate-400 uppercase font-mono tracking-wider sticky top-0 border-b border-slate-800">
                  <tr>
                    <th className="py-2.5 px-4">{t('packages.colPackage')}</th>
                    <th className="py-2.5 px-4">{t('packages.colType')}</th>
                    <th className="py-2.5 px-4">{t('packages.colVersion')}</th>
                    <th className="py-2.5 px-4">{t('packages.colArch')}</th>
                    <th className="py-2.5 px-4">{t('packages.colDescription')}</th>
                    <th className="py-2.5 px-4 text-right">{t('packages.colActions')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {paginatedList.length === 0 && !loading ? (
                    <tr>
                      <td colSpan={6} className="py-12 text-center text-slate-500">
                        {t('packages.noPackagesMatch')}
                      </td>
                    </tr>
                  ) : (
                    paginatedList.map((pkg) => {
                      const p = pkg as any;
                      const isUser = p.is_manual || (!p.is_system && (p.priority === 'optional' || p.priority === 'extra'));
                      return (
                        <tr key={pkg.name} className="hover:bg-slate-800/30 transition">
                          <td className="py-2.5 px-4 font-mono font-medium text-white">{pkg.name}</td>
                          <td className="py-2.5 px-4">
                            <Badge
                              variant={isUser ? 'success' : 'neutral'}
                              size="xs"
                            >
                              {isUser ? t('packages.typeUser') : t('packages.typeSystem')}
                            </Badge>
                          </td>
                          <td className="py-2.5 px-4 font-mono text-indigo-300">{pkg.version}</td>
                          <td className="py-2.5 px-4 text-slate-400 font-mono text-[11px]">{pkg.architecture}</td>
                          <td className="py-2.5 px-4 text-slate-400 truncate max-w-md" title={pkg.description}>
                            {pkg.description || '—'}
                          </td>
                          <td className="py-2.5 px-4 text-right">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => handleSimulateRemove(pkg.name)}
                              disabled={simulating}
                              className="text-slate-400 hover:text-rose-400 hover:bg-rose-500/10"
                              title={t('packages.removeTooltip')}
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </Button>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>

            {totalPages > 1 && (
              <div className="p-3 border-t border-slate-800 flex flex-wrap justify-between items-center gap-2 text-xs text-slate-400 bg-slate-950/40">
                <span>
                  {t('packages.paginationShown', { from: (currentPage - 1) * pageSize + 1, to: Math.min(currentPage * pageSize, categoryFilteredList.length), total: categoryFilteredList.length })}
                </span>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                    disabled={currentPage === 1}
                    className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-300 transition"
                  >
                    {t('packages.pagePrev')}
                  </button>
                  <span className="font-mono text-slate-300">
                    {currentPage} / {totalPages}
                  </span>
                  <button
                    onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                    disabled={currentPage === totalPages}
                    className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-300 transition"
                  >
                    {t('packages.pageNext')}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 2: System Upgrades */}
      {activeTab === 'updates' && (
        <div className="space-y-4">
          <div className="p-6 rounded-xl bg-surface-850 border border-slate-800 shadow-sm flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
            <div>
              <h4 className="text-sm font-semibold text-white mb-1 flex items-center gap-2">
                <span>{t('packages.upgradeCheckTitle')}</span>
              </h4>
              <p className="text-xs text-slate-400 max-w-2xl leading-relaxed">
                {t('packages.upgradeCheckDesc1')} {t('packages.upgradeCheckDesc2')}
              </p>
            </div>
            <button
              onClick={handleSimulateUpgrade}
              disabled={simulating}
              className="px-4 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold flex items-center gap-2 shadow-sm transition shrink-0"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${simulating ? 'animate-spin' : ''}`} />
              <span>{t('packages.runUpgradeSimulation')}</span>
            </button>
          </div>
        </div>
      )}

      {/* TAB 3: Install Package */}
      {activeTab === 'install' && (
        <div className="space-y-4">
          <div className="p-6 rounded-xl bg-surface-850 border border-slate-800 shadow-sm">
            <h4 className="text-sm font-semibold text-white mb-1">{t('packages.installTitle')}</h4>
            <p className="text-xs text-slate-400 mb-4">
              {t('packages.installDesc')}
            </p>

            <div className="flex gap-2 w-full max-w-3xl">
              <input
                type="text"
                placeholder={t('packages.installPlaceholder')}
                value={newPkgInput}
                onChange={(e) => setNewPkgInput(e.target.value)}
                className="flex-1 px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500 font-mono"
              />
              <button
                onClick={() => handleSimulateInstall(newPkgInput)}
                disabled={simulating || !newPkgInput.trim()}
                className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-semibold rounded-lg flex items-center gap-2 transition"
              >
                <Download className="w-3.5 h-3.5" />
                <span>{t('packages.simulateInstallBtn')}</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* TAB 4: Task History */}
      {activeTab === 'history' && (
        <div className="space-y-4">
          <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-lg">
            <div className="p-3 border-b border-slate-800 flex justify-between items-center text-xs text-slate-400 bg-slate-950/40">
              <span>{t('packages.historyTitle')}</span>
              <button
                onClick={loadHistory}
                disabled={loadingHistory}
                className="flex items-center gap-1.5 text-indigo-400 hover:text-indigo-300 transition"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${loadingHistory ? 'animate-spin' : ''}`} />
                <span>{t('packages.refreshHistoryBtn')}</span>
              </button>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-950/60 text-slate-400 uppercase font-mono tracking-wider border-b border-slate-800">
                  <tr>
                    <th className="py-2.5 px-4">{t('packages.colOperation')}</th>
                    <th className="py-2.5 px-4">{t('packages.colPackages')}</th>
                    <th className="py-2.5 px-4">{t('packages.colStatus')}</th>
                    <th className="py-2.5 px-4">{t('packages.colCreated')}</th>
                    <th className="py-2.5 px-4">{t('packages.colResult')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {historyList.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="py-12 text-center text-slate-500">
                        {t('packages.emptyHistory')}
                      </td>
                    </tr>
                  ) : (
                    historyList.map((item) => (
                      <tr key={item.id} className="hover:bg-slate-800/30 transition">
                        <td className="py-2.5 px-4 font-mono font-medium text-white">{item.operation_type}</td>
                        <td className="py-2.5 px-4 text-slate-300">{item.packages?.join(', ') || '—'}</td>
                        <td className="py-2.5 px-4">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-semibold uppercase ${item.status === 'completed'
                              ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                              : item.status === 'failed'
                                ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
                                : 'bg-amber-500/15 text-amber-400 border border-amber-500/30'
                              }`}
                          >
                            {item.status === 'completed' ? t('packages.statusCompleted') : item.status === 'failed' ? t('packages.statusFailed') : item.status}
                          </span>
                        </td>
                        <td className="py-2.5 px-4 text-slate-400 font-mono text-[11px]">
                          {item.created_at ? new Date(item.created_at).toLocaleString() : '—'}
                        </td>
                        <td className="py-2.5 px-4 text-slate-400 break-all max-w-xs truncate">
                          {item.error_message || t('packages.resultSuccess')}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* SIMULATION PREVIEW MODAL */}
      {showSimModal && simulation && (
        <ModalShell
          onClose={() => setShowSimModal(false)}
          size="2xl"
          className="max-h-[85vh]"
          icon={<Layers className="w-5 h-5 text-indigo-400" />}
          iconClassName=""
          title={
            simActionType === 'upgrade'
              ? t('packages.simUpgradeResult')
              : simActionType === 'install'
              ? t('packages.simInstallResult')
              : t('packages.simRemoveResult')
          }
        >

            {/* Modal Body */}
            <div className="p-5 overflow-y-auto space-y-4 text-xs">
              {/* Summary Stats */}
              <div className="grid grid-cols-4 gap-2 text-center">
                <div className="p-2.5 rounded-lg bg-indigo-500/10 border border-indigo-500/20">
                  <div className="text-lg font-bold text-indigo-300 font-mono">{simulation.upgraded_count}</div>
                  <div className="text-[10px] text-slate-400 uppercase font-medium">{t('packages.statWillUpgrade')}</div>
                </div>
                <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20">
                  <div className="text-lg font-bold text-emerald-300 font-mono">{simulation.installed_count}</div>
                  <div className="text-[10px] text-slate-400 uppercase font-medium">{t('packages.statWillInstall')}</div>
                </div>
                <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20">
                  <div className="text-lg font-bold text-rose-300 font-mono">{simulation.removed_count}</div>
                  <div className="text-[10px] text-slate-400 uppercase font-medium">{t('packages.statWillRemove')}</div>
                </div>
                <div className="p-2.5 rounded-lg bg-amber-500/10 border border-amber-500/20">
                  <div className="text-lg font-bold text-amber-300 font-mono">{simulation.held_count}</div>
                  <div className="text-[10px] text-slate-400 uppercase font-medium">{t('packages.statHeld')}</div>
                </div>
              </div>

              {/* Changes List */}
              <div className="space-y-1.5">
                <div className="font-semibold text-slate-300">{t('packages.plannedChangesList')}</div>
                <div className="border border-slate-800 rounded-lg max-h-56 overflow-y-auto divide-y divide-slate-800/60 bg-slate-900/40">
                  {simulation.changes && simulation.changes.length > 0 ? (
                    simulation.changes.map((c, i) => (
                      <div key={i} className="p-2 flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 min-w-0">
                          <span
                            className={`px-1.5 py-0.5 rounded text-[10px] font-mono uppercase font-bold shrink-0 ${c.action === 'upgrade'
                              ? 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30'
                              : c.action === 'install'
                                ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                                : 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
                              }`}
                          >
                            {c.action === 'upgrade' ? t('packages.actionUpgradeLabel') : c.action === 'install' ? t('packages.actionInstallLabel') : t('packages.actionRemoveLabel')}
                          </span>
                          <span className="font-mono text-white font-medium truncate">{c.name}</span>
                        </div>
                        <div className="flex items-center gap-1.5 text-slate-400 font-mono shrink-0">
                          {c.old_version && <span className="text-slate-500 line-through">{c.old_version}</span>}
                          {c.old_version && c.new_version && <ArrowRight className="w-3 h-3 text-slate-600" />}
                          {c.new_version && <span className="text-indigo-300">{c.new_version}</span>}
                          {c.arch && <span className="text-slate-600 text-[10px]">[{c.arch}]</span>}
                        </div>
                      </div>
                    ))
                  ) : (
                    <div className="p-4 text-center text-slate-500">
                      {t('packages.noChangesDetected')}
                    </div>
                  )}
                </div>
              </div>

              {/* Raw Simulation Output Accordion */}
              <details className="p-2 rounded bg-slate-900/60 border border-slate-800 text-[11px] text-slate-400">
                <summary className="cursor-pointer font-medium hover:text-slate-200">
                  {t('packages.showRawOutput')}
                </summary>
                <pre className="mt-2 p-2 bg-slate-950 rounded text-slate-400 overflow-x-auto text-[10px] font-mono max-h-40">
                  {simulation.raw_output}
                </pre>
              </details>
            </div>

            {/* Modal Footer */}
            <div className="p-4 border-t border-slate-800 flex items-center justify-between bg-slate-950/60">
              <button
                onClick={() => setShowSimModal(false)}
                className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition"
              >
                {t('common.cancel')}
              </button>

              <button
                onClick={handleConfirmAction}
                disabled={!simulation?.changes || simulation.changes.length === 0}
                className={`px-4 py-2 rounded-lg text-white text-xs font-semibold flex items-center gap-1.5 transition ${simActionType === 'remove'
                  ? 'bg-rose-600 hover:bg-rose-500'
                  : 'bg-emerald-600 hover:bg-emerald-500'
                  } disabled:opacity-40 disabled:cursor-not-allowed`}
              >
                <CheckCircle2 className="w-4 h-4" />
                <span>
                  {simActionType === 'upgrade'
                    ? t('packages.applyUpgradeBtn')
                    : simActionType === 'install'
                    ? t('packages.applyInstallBtn')
                    : t('packages.confirmRemoveBtn')}
                </span>
              </button>
            </div>
        </ModalShell>
      )}
    </div>
  );
};