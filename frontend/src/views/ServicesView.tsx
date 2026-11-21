import React, { useState, useMemo } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  Cpu,
  RefreshCw,
  Play,
  Square,
  RotateCw,
  FileText,
  Shield,
  ShieldAlert,
  AlertTriangle,
  Globe,
  Radio,
  Activity,
  Network,
} from 'lucide-react';
import { useServer } from '../context/ServerContext';
import { useServerData } from '../hooks/useServerData';
import { PageToolbar, Button, Badge, Input, Select, SearchInput, Tabs } from '../components/ui';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { ModalShell } from '../components/common/ModalShell';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { services } from '../../wailsjs/go/models';
import {
  ListSystemUnits,
  ControlSystemUnit,
  GetSystemUnitLogs,
  ListSystemProcesses,
  KillSystemProcess,
  ListSystemListeningPorts,
  CheckDNS,
  CheckPortConnection,
  GetNetworkOverview,
} from '../../wailsjs/go/main/App';

interface ServicesViewProps {
  onOpenAddServer: () => void;
}

export const ServicesView: React.FC<ServicesViewProps> = ({ onOpenAddServer }) => {
  const {
    selectedServerId: activeServerId,
    selectedServer: activeServer,
    servers,
    setSelectedServerId: selectServer,
  } = useServer();

  const { t } = useTranslation();
  const confirm = useConfirm();
  const [activeTab, setActiveTab] = useState<'units' | 'processes' | 'network'>('units');
  const [error, setError] = useState<string | null>(null);

  // Units State
  const [unitTypeFilter, setUnitTypeFilter] = useState<string>('service');
  const [unitSearch, setUnitSearch] = useState<string>('');
  const [selectedLogUnit, setSelectedLogUnit] = useState<string | null>(null);
  const [unitLogs, setUnitLogs] = useState<string>('');
  const [loadingLogs, setLoadingLogs] = useState<boolean>(false);
  const [logLinesCount, setLogLinesCount] = useState<number>(100);

  // Processes State
  const [procSearch, setProcSearch] = useState<string>('');
  const [killModalTarget, setKillModalTarget] = useState<services.ProcessItem | null>(null);
  const [killSignal, setKillSignal] = useState<string>('SIGTERM');

  // Diagnostics State
  const [dnsDomain, setDnsDomain] = useState<string>('google.com');
  const [dnsType, setDnsType] = useState<string>('A');
  const [dnsResult, setDnsResult] = useState<services.DNSCheckResult | null>(null);
  const [checkingDns, setCheckingDns] = useState<boolean>(false);

  const [portHost, setPortHost] = useState<string>('1.1.1.1');
  const [portNum, setPortNum] = useState<number>(53);
  const [portResult, setPortResult] = useState<services.PortCheckResult | null>(null);
  const [checkingPort, setCheckingPort] = useState<boolean>(false);

  const { data: units, loading: loadingUnits, reload: loadUnits } = useServerData(
    (serverId) => ListSystemUnits(serverId, unitTypeFilter).then((r) => r || []),
    [activeTab, unitTypeFilter],
    {
      initial: [] as services.SystemUnit[],
      enabled: activeTab === 'units',
      loadingMessage: t('services.loadingUnits'),
      onStart: () => setError(null),
      onError: (msg) => setError(msg),
    },
  );

  const { data: processes, loading: loadingProcesses, reload: loadProcesses } = useServerData(
    (serverId) => ListSystemProcesses(serverId).then((r) => r || []),
    [activeTab],
    {
      initial: [] as services.ProcessItem[],
      enabled: activeTab === 'processes',
      loadingMessage: t('services.loadingProcesses'),
      onStart: () => setError(null),
      onError: (msg) => setError(msg),
    },
  );

  const { data: networkData, loading: loadingPorts, reload: loadNetwork } = useServerData(
    (serverId) =>
      Promise.all([
        ListSystemListeningPorts(serverId),
        GetNetworkOverview(serverId),
      ]).then(([portsRes, netRes]) => ({ ports: portsRes || [], netOverview: netRes })),
    [activeTab],
    {
      initial: {
        ports: [] as services.ListeningPortItem[],
        netOverview: null as services.NetworkOverview | null,
      },
      enabled: activeTab === 'network',
      loadingMessage: t('services.loadingNetwork'),
      onStart: () => setError(null),
      onError: (msg) => setError(msg),
    },
  );
  const ports = networkData.ports;
  const netOverview = networkData.netOverview;

  const handleControlUnit = async (unitName: string, action: string) => {
    if (!activeServerId) return;
    if (action === 'stop' && !(await confirm(t('services.stopUnitConfirm', { name: unitName }), { danger: true }))) {
      return;
    }
    if (action === 'restart' && !(await confirm(t('services.restartUnitConfirm', { name: unitName })))) {
      return;
    }
    if (action === 'disable' && !(await confirm(t('services.disableUnitConfirm', { name: unitName }), { danger: true }))) {
      return;
    }
    setError(null);
    try {
      await ControlSystemUnit(activeServerId, unitName, action);
      loadUnits();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleOpenLogs = async (unitName: string) => {
    if (!activeServerId) return;
    setSelectedLogUnit(unitName);
    setLoadingLogs(true);
    try {
      const logs = await GetSystemUnitLogs(activeServerId, unitName, logLinesCount);
      setUnitLogs(logs || t('services.logsEmpty'));
    } catch (err: unknown) {
      setUnitLogs(t('services.logsError', { error: getErrorMessage(err) }));
    } finally {
      setLoadingLogs(false);
    }
  };

  const handleKillConfirm = async () => {
    if (!activeServerId || !killModalTarget) return;
    setError(null);
    try {
      await KillSystemProcess(activeServerId, killModalTarget.pid, killSignal);
      setKillModalTarget(null);
      loadProcesses();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleRunDnsCheck = async () => {
    if (!activeServerId || !dnsDomain) return;
    setCheckingDns(true);
    try {
      const res = await CheckDNS(activeServerId, dnsDomain, dnsType);
      setDnsResult(res);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setCheckingDns(false);
    }
  };

  const handleRunPortCheck = async () => {
    if (!activeServerId || !portHost || !portNum) return;
    setCheckingPort(true);
    try {
      const res = await CheckPortConnection(activeServerId, portHost, portNum);
      setPortResult(res);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setCheckingPort(false);
    }
  };

  const isSshUnit = (name: string) => {
    const l = name.toLowerCase();
    return l.includes('ssh') || l.includes('network') || l.includes('resolved');
  };

  const activeUnitsCount = useMemo(() => {
    return units.filter((u) => u.sub === 'running' || u.active === 'active').length;
  }, [units]);

  const filteredUnits = units.filter((u) =>
    u.unit.toLowerCase().includes(unitSearch.toLowerCase()) ||
    u.description.toLowerCase().includes(unitSearch.toLowerCase())
  );

  const filteredProcesses = processes.filter((p) =>
    p.command.toLowerCase().includes(procSearch.toLowerCase()) ||
    p.user.toLowerCase().includes(procSearch.toLowerCase()) ||
    String(p.pid).includes(procSearch)
  );

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Cpu}
        description={t('empty.servicesDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Cpu className="w-5 h-5" />}
        servers={servers}
        selectedServerId={activeServerId}
        onSelectServer={selectServer}
        serverLabel={t('services.targetServer')}
        actions={
          <Button
            variant="secondary"
            size="md"
            onClick={() => {
              if (activeTab === 'units') loadUnits();
              else if (activeTab === 'processes') loadProcesses();
              else if (activeTab === 'network') loadNetwork();
            }}
            disabled={loadingUnits || loadingProcesses || loadingPorts}
            icon={<RefreshCw className={`w-3.5 h-3.5 ${loadingUnits || loadingProcesses || loadingPorts ? 'animate-spin text-indigo-400' : ''}`} />}
            title={t('services.refreshTooltip')}
          >
            {t('services.refreshBtn')}
          </Button>
        }
      />

      {/* Metrics Stat Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div
          onClick={() => setActiveTab('units')}
          className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between cursor-pointer hover:border-slate-700 transition"
        >
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('services.statUnits')}</span>
            <div className="flex items-baseline gap-1.5">
              <span className="text-2xl font-bold text-emerald-400 font-mono">{activeUnitsCount}</span>
              <span className="text-xs text-slate-500 font-mono">/ {units.length}</span>
            </div>
          </div>
          <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
            <Activity className="w-5 h-5" />
          </div>
        </div>

        <div
          onClick={() => setActiveTab('processes')}
          className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between cursor-pointer hover:border-slate-700 transition"
        >
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('services.statProcesses')}</span>
            <span className="text-2xl font-bold text-indigo-400 font-mono">{processes.length}</span>
          </div>
          <div className="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
            <Cpu className="w-5 h-5" />
          </div>
        </div>

        <div
          onClick={() => setActiveTab('network')}
          className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between cursor-pointer hover:border-slate-700 transition"
        >
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('services.statPorts')}</span>
            <span className="text-2xl font-bold text-sky-400 font-mono">{ports.length}</span>
          </div>
          <div className="w-10 h-10 rounded-xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-center text-sky-400">
            <Radio className="w-5 h-5" />
          </div>
        </div>

        <div
          onClick={() => setActiveTab('network')}
          className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center justify-between cursor-pointer hover:border-slate-700 transition"
        >
          <div>
            <span className="text-xs text-slate-400 block mb-1">{t('services.statNetwork')}</span>
            <span className="text-xs font-semibold text-amber-400 block mt-1">
              {netOverview ? t('services.statInterfacesActive') : t('services.statDiagnostics')}
            </span>
          </div>
          <div className="w-10 h-10 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400">
            <Network className="w-5 h-5" />
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
        tabs={[
          { id: 'units', label: t('services.tabUnits'), icon: <Activity className="w-4 h-4" /> },
          { id: 'processes', label: t('services.tabProcesses'), icon: <Cpu className="w-4 h-4" /> },
          { id: 'network', label: t('services.tabNetwork'), icon: <Network className="w-4 h-4" /> },
        ]}
        activeTab={activeTab}
        onChange={(tabId) => setActiveTab(tabId as any)}
        variant="line"
      />

      {/* TAB 1: SYSTEMD UNITS */}
      {activeTab === 'units' && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-surface-850 p-3 rounded-2xl border border-slate-800 shadow-sm">
            <div className="flex items-center gap-2">
              <span className="text-xs text-slate-400">{t('services.unitTypeLabel')}</span>
              <Select
                value={unitTypeFilter}
                onChange={(e) => setUnitTypeFilter(e.target.value)}
                sizeVariant="sm"
                className="w-auto cursor-pointer"
              >
                <option value="service">{t('services.unitTypeService')}</option>
                <option value="timer">{t('services.unitTypeTimer')}</option>
                <option value="socket">{t('services.unitTypeSocket')}</option>
                <option value="all">{t('services.unitTypeAll')}</option>
              </Select>
            </div>

            <div className="min-w-[260px] sm:max-w-xs flex-1">
              <SearchInput
                placeholder={t('services.searchUnitsPlaceholder')}
                value={unitSearch}
                onChange={(e) => setUnitSearch(e.target.value)}
                onClear={() => setUnitSearch('')}
                sizeVariant="sm"
              />
            </div>
          </div>

          <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-lg">
            <div className="p-3 border-b border-slate-800 flex justify-between items-center text-xs text-slate-400 bg-slate-950/40">
              <span>
                {t('services.unitsFound')} <span className="text-white font-semibold font-mono">{filteredUnits.length}</span>
              </span>
              {loadingUnits && (
                <span className="flex items-center gap-1.5 text-indigo-400">
                  <RefreshCw className="w-3.5 h-3.5 animate-spin" /> {t('services.loadingServices')}
                </span>
              )}
            </div>

            <div className="overflow-x-auto max-h-[600px] overflow-y-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-950/60 text-slate-400 uppercase font-mono tracking-wider sticky top-0 border-b border-slate-800">
                  <tr>
                    <th className="py-2.5 px-4">{t('services.colUnit')}</th>
                    <th className="py-2.5 px-4">{t('services.colUnitStatus')}</th>
                    <th className="py-2.5 px-4">{t('services.colUnitDescription')}</th>
                    <th className="py-2.5 px-4 text-right">{t('services.colUnitControl')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {filteredUnits.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="py-12 text-center text-slate-500">
                        {loadingUnits ? t('services.loadingUnits') : t('services.noUnitsFound')}
                      </td>
                    </tr>
                  ) : (
                    filteredUnits.map((u) => {
                      const isProtected = isSshUnit(u.unit);
                      const isRunning = u.sub === 'running' || u.active === 'active';

                      return (
                        <tr key={u.unit} className="hover:bg-slate-800/30 transition">
                          <td className="py-2.5 px-4 font-mono font-medium text-white flex items-center gap-1.5">
                            {isProtected && (
                              <span title={t('services.protectedServiceTooltip')}>
                                <Shield className="w-3.5 h-3.5 text-amber-400 shrink-0" />
                              </span>
                            )}
                            <span className="truncate max-w-[220px]">{u.unit}</span>
                          </td>
                          <td className="py-2.5 px-4">
                            <Badge
                              variant={isRunning ? 'success' : u.sub === 'failed' ? 'danger' : 'neutral'}
                              size="xs"
                            >
                              {isRunning ? t('services.statusRunning') : u.sub === 'failed' ? t('services.statusFailed') : u.sub || u.active}
                            </Badge>
                          </td>
                          <td className="py-2.5 px-4 text-slate-400 truncate max-w-sm" title={u.description}>
                            {u.description || '—'}
                          </td>
                          <td className="py-2.5 px-4 text-right">
                            <div className="flex items-center justify-end gap-1">
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                onClick={() => handleOpenLogs(u.unit)}
                                title={t('services.unitLogsTooltip')}
                              >
                                <FileText className="w-3.5 h-3.5" />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                onClick={() => handleControlUnit(u.unit, 'restart')}
                                title={t('services.restartServiceTooltip')}
                              >
                                <RotateCw className="w-3.5 h-3.5" />
                              </Button>
                              {isRunning ? (
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  onClick={() => handleControlUnit(u.unit, 'stop')}
                                  disabled={isProtected}
                                  className="text-slate-400 hover:text-rose-400 hover:bg-rose-500/10"
                                  title={isProtected ? t('services.stopProtectedTooltip') : t('services.stopServiceTooltip')}
                                >
                                  <Square className="w-3.5 h-3.5" />
                                </Button>
                              ) : (
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  onClick={() => handleControlUnit(u.unit, 'start')}
                                  className="text-slate-400 hover:text-emerald-400 hover:bg-emerald-500/10"
                                  title={t('services.startServiceTooltip')}
                                >
                                  <Play className="w-3.5 h-3.5" />
                                </Button>
                              )}
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* TAB 2: PROCESSES */}
      {activeTab === 'processes' && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-surface-850 p-3 rounded-xl border border-slate-800 shadow-sm">
            <div className="min-w-[260px] sm:max-w-md flex-1">
              <SearchInput
                placeholder={t('services.searchProcessesPlaceholder')}
                value={procSearch}
                onChange={(e) => setProcSearch(e.target.value)}
                onClear={() => setProcSearch('')}
                sizeVariant="sm"
              />
            </div>
            <div className="text-xs text-slate-400">
              {t('services.processesShown')} <span className="font-mono text-white font-semibold">{filteredProcesses.length}</span>
            </div>
          </div>

          <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-lg">
            <div className="overflow-x-auto max-h-[600px] overflow-y-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-950/60 text-slate-400 uppercase font-mono tracking-wider sticky top-0 border-b border-slate-800">
                  <tr>
                    <th className="py-2.5 px-4">PID</th>
                    <th className="py-2.5 px-4">{t('services.colProcessUser')}</th>
                    <th className="py-2.5 px-4">CPU %</th>
                    <th className="py-2.5 px-4">MEM %</th>
                    <th className="py-2.5 px-4">{t('services.colProcessTime')}</th>
                    <th className="py-2.5 px-4">{t('services.colProcessCommand')}</th>
                    <th className="py-2.5 px-4 text-right">{t('services.colProcessAction')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {filteredProcesses.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="py-12 text-center text-slate-500">
                        {loadingProcesses ? t('services.loadingProcesses') : t('services.noProcessesFound')}
                      </td>
                    </tr>
                  ) : (
                    filteredProcesses.map((p) => (
                      <tr key={p.pid} className="hover:bg-slate-800/30 transition">
                        <td className="py-2 px-4 font-mono font-semibold text-white">{p.pid}</td>
                        <td className="py-2 px-4 text-slate-400">{p.user}</td>
                        <td className="py-2 px-4 font-mono text-indigo-300">{p.cpu.toFixed(1)}%</td>
                        <td className="py-2 px-4 font-mono text-slate-300">{p.mem.toFixed(1)}%</td>
                        <td className="py-2 px-4 font-mono text-slate-400">{p.time}</td>
                        <td className="py-2 px-4 font-mono text-xs text-slate-300 truncate max-w-md" title={p.command}>
                          {p.command}
                        </td>
                        <td className="py-2 px-4 text-right">
                          <button
                            onClick={() => setKillModalTarget(p)}
                            disabled={p.pid <= 1}
                            className="px-2.5 py-1 rounded bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 text-[10px] font-semibold transition disabled:opacity-20 disabled:cursor-not-allowed"
                            title={p.pid <= 1 ? t('services.killPid1Forbidden') : t('services.killProcessTooltip')}
                          >
                            {t('services.killBtn')}
                          </button>
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

      {/* TAB 3: NETWORK & PORTS */}
      {activeTab === 'network' && (
        <div className="space-y-6">
          {/* Listening Ports Table */}
          <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-lg">
            <div className="p-3 border-b border-slate-800 flex justify-between items-center text-xs text-slate-400 bg-slate-950/40">
              <span className="font-semibold text-white flex items-center gap-2">
                <Radio className="w-4 h-4 text-indigo-400" />
                {t('services.listeningSockets')}
              </span>
              <span>
                {t('services.activePortsCount')} <span className="text-white font-mono font-semibold">{ports.length}</span>
              </span>
            </div>

            <div className="overflow-x-auto max-h-[350px] overflow-y-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-950/60 text-slate-400 uppercase font-mono tracking-wider sticky top-0 border-b border-slate-800">
                  <tr>
                    <th className="py-2 px-4">{t('services.colProtocol')}</th>
                    <th className="py-2 px-4">{t('services.colPort')}</th>
                    <th className="py-2 px-4">{t('services.colLocalAddr')}</th>
                    <th className="py-2 px-4">{t('services.colProcessService')}</th>
                    <th className="py-2 px-4">PID</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {ports.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="py-8 text-center text-slate-500">
                        {loadingPorts ? t('services.loadingNetwork') : t('services.noPortsFound')}
                      </td>
                    </tr>
                  ) : (
                    ports.map((pt, i) => (
                      <tr key={i} className="hover:bg-slate-800/30 transition">
                        <td className="py-2 px-4 font-mono uppercase text-indigo-400">{pt.proto}</td>
                        <td className="py-2 px-4 font-mono font-bold text-white">{pt.local_port}</td>
                        <td className="py-2 px-4 font-mono text-slate-400">{pt.local_addr}</td>
                        <td className="py-2 px-4 font-semibold text-slate-200">{pt.process || '—'}</td>
                        <td className="py-2 px-4 font-mono text-slate-400">{pt.pid || '—'}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Network Diagnostics Tools */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* DNS Test */}
            <div className="p-5 rounded-xl bg-surface-850 border border-slate-800 shadow-sm space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-white">
                <Globe className="w-4 h-4 text-indigo-400" />
                <span>{t('services.dnsResolverTitle')}</span>
              </div>
              <p className="text-xs text-slate-400 leading-relaxed">
                {t('services.dnsResolverDesc')}
              </p>
              <div className="flex gap-2">
                <Input
                  type="text"
                  placeholder={t('services.domainPlaceholder')}
                  value={dnsDomain}
                  onChange={(e) => setDnsDomain(e.target.value)}
                  sizeVariant="sm"
                  className="flex-1 font-mono"
                />
                <Select
                  value={dnsType}
                  onChange={(e) => setDnsType(e.target.value)}
                  sizeVariant="sm"
                  className="w-24 font-mono cursor-pointer"
                >
                  <option value="A">A</option>
                  <option value="AAAA">AAAA</option>
                  <option value="MX">MX</option>
                  <option value="TXT">TXT</option>
                </Select>
                <Button
                  onClick={handleRunDnsCheck}
                  disabled={checkingDns}
                  loading={checkingDns}
                  size="sm"
                  variant="primary"
                  className="shrink-0"
                >
                  {checkingDns ? t('services.checkingDns') : t('services.testBtn')}
                </Button>
              </div>

              {dnsResult && (
                <div className="p-3 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono">
                  {dnsResult.success ? (
                    <div className="text-emerald-400">
                      {t('services.dnsResponse', { records: dnsResult.records.join(', ') })}
                    </div>
                  ) : (
                    <div className="text-rose-400">{t('services.noDnsRecords')}</div>
                  )}
                </div>
              )}
            </div>

            {/* Port Connectivity Test */}
            <div className="p-5 rounded-xl bg-surface-850 border border-slate-800 shadow-sm space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-white">
                <Network className="w-4 h-4 text-indigo-400" />
                <span>{t('services.tcpCheckTitle')}</span>
              </div>
              <p className="text-xs text-slate-400 leading-relaxed">
                {t('services.tcpCheckDesc')}
              </p>
              <div className="flex gap-2">
                <Input
                  type="text"
                  placeholder={t('services.hostPlaceholder')}
                  value={portHost}
                  onChange={(e) => setPortHost(e.target.value)}
                  sizeVariant="sm"
                  className="flex-1 font-mono"
                />
                <Input
                  type="number"
                  placeholder={t('services.portPlaceholder')}
                  value={portNum}
                  onChange={(e) => setPortNum(Number(e.target.value))}
                  sizeVariant="sm"
                  className="w-24 font-mono"
                />
                <Button
                  onClick={handleRunPortCheck}
                  disabled={checkingPort}
                  loading={checkingPort}
                  size="sm"
                  variant="primary"
                  className="shrink-0"
                >
                  {checkingPort ? t('services.testingPort') : t('services.checkBtn')}
                </Button>
              </div>

              {portResult && (
                <div className="p-3 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono flex items-center justify-between">
                  <div className={portResult.open ? 'text-emerald-400' : 'text-rose-400'}>
                    {portResult.open ? t('services.portOpen') : t('services.portClosed')}
                  </div>
                  <div className="text-slate-400">{portResult.latency_ms} ms</div>
                </div>
              )}
            </div>
          </div>

          {/* Read-Only Notice */}
          {netOverview && (
            <div className="p-5 rounded-xl bg-surface-850 border border-slate-800 text-xs space-y-3 shadow-sm">
              <div className="flex items-center gap-2 text-amber-400 font-semibold">
                <ShieldAlert className="w-4 h-4" />
                <span>{t('services.netPolicyTitle')}</span>
              </div>
              <p className="text-slate-400 leading-relaxed">
                {t('services.netPolicyDesc') || netOverview.notice}
              </p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-3">
                <div className="p-3 bg-slate-950 rounded-lg border border-slate-800/80">
                  <div className="text-slate-400 font-mono text-[11px] mb-1.5 font-semibold">{t('services.netInterfaces')}</div>
                  <pre className="text-[10px] font-mono text-slate-300 overflow-x-auto whitespace-pre-wrap">{netOverview.interfaces}</pre>
                </div>
                <div className="p-3 bg-slate-950 rounded-lg border border-slate-800/80">
                  <div className="text-slate-400 font-mono text-[11px] mb-1.5 font-semibold">{t('services.netRouting')}</div>
                  <pre className="text-[10px] font-mono text-slate-300 overflow-x-auto whitespace-pre-wrap">{netOverview.routes}</pre>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* SERVICE LOGS MODAL */}
      {selectedLogUnit && (
        <ModalShell
          onClose={() => setSelectedLogUnit(null)}
          size="4xl"
          className="max-h-[85vh]"
          icon={<FileText className="w-5 h-5 text-indigo-400" />}
          iconClassName=""
          title={<>{t('services.serviceJournalTitle')} <span className="font-mono text-indigo-300">{selectedLogUnit}</span></>}
          headerActions={
            <>
              <Select
                value={logLinesCount}
                onChange={(e) => {
                  const l = Number(e.target.value);
                  setLogLinesCount(l);
                  handleOpenLogs(selectedLogUnit);
                }}
                sizeVariant="sm"
                className="w-auto"
              >
                <option value={50}>{t('services.linesCount', { count: 50 })}</option>
                <option value={100}>{t('services.linesCount', { count: 100 })}</option>
                <option value={250}>{t('services.linesCount', { count: 250 })}</option>
                <option value={500}>{t('services.linesCount', { count: 500 })}</option>
              </Select>
              <Button
                onClick={() => handleOpenLogs(selectedLogUnit)}
                disabled={loadingLogs}
                size="sm"
                variant="secondary"
                title={t('services.refreshJournalTooltip')}
              >
                <RefreshCw className={`w-3.5 h-3.5 ${loadingLogs ? 'animate-spin' : ''}`} />
              </Button>
            </>
          }
        >
          <div className="p-4 overflow-y-auto flex-1 bg-slate-950 font-mono text-xs text-slate-300 whitespace-pre-wrap select-text">
            {loadingLogs ? t('services.loadingJournal') : unitLogs}
          </div>

          <div className="p-3 border-t border-slate-800 flex justify-end bg-slate-950/60">
            <Button
              onClick={() => setSelectedLogUnit(null)}
              variant="secondary"
              size="sm"
            >
              {t('common.close')}
            </Button>
          </div>
        </ModalShell>
      )}

      {/* KILL PROCESS CONFIRMATION MODAL */}
      {killModalTarget && (
        <ModalShell
          onClose={() => setKillModalTarget(null)}
          size="md"
          icon={<AlertTriangle className="w-6 h-6 text-rose-400" />}
          iconClassName=""
          title={t('services.killModalTitle')}
        >
          <div className="p-5 space-y-4">
            <p className="text-xs text-slate-300">
              {t('services.killModalConfirm', { pid: killModalTarget.pid })}
            </p>
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-300 break-all">
              {killModalTarget.command}
            </div>

            <div className="space-y-1">
              <label className="text-xs text-slate-400 font-medium">{t('services.signalLabel')}</label>
              <Select
                value={killSignal}
                onChange={(e) => setKillSignal(e.target.value)}
                sizeVariant="md"
              >
                <option value="SIGTERM">{t('services.sigtermDesc')}</option>
                <option value="SIGKILL">{t('services.sigkillDesc')}</option>
                <option value="SIGHUP">{t('services.sighupDesc')}</option>
              </Select>
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <Button
                onClick={() => setKillModalTarget(null)}
                variant="secondary"
                size="sm"
              >
                {t('common.cancel')}
              </Button>
              <Button
                onClick={handleKillConfirm}
                variant="danger"
                size="sm"
              >
                {t('services.sendSignalBtn')}
              </Button>
            </div>
          </div>
        </ModalShell>
      )}
    </div>
  );
};
