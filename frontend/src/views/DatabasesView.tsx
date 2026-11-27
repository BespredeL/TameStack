import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { useServerRequest } from '../hooks/useServerRequest';
import {
  Database,
  Plus,
  RefreshCw,
  Trash2,
  Archive,
  RotateCcw,
  UserPlus,
  Users,
  Play,
  Square,
  Layers,
  Cpu,
  Sparkles,
  AlertCircle,
  Download,
  FileCode,
} from 'lucide-react';
import {
  InspectDatabases,
  ListDatabases,
  ListDatabaseUsers,
  DeleteDatabase,
  DeleteDatabaseUser,
  ControlDatabaseService,
} from '../../wailsjs/go/main/App';
import { database } from '../../wailsjs/go/models';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { ServerRecord, TaskRecord } from '../types';
import { PageToolbar, Button, SearchInput, Badge, Tabs } from '../components/ui';
import { CreateDatabaseModal } from '../components/CreateDatabaseModal';
import { CreateDatabaseUserModal } from '../components/CreateDatabaseUserModal';
import { BackupDatabaseModal } from '../components/BackupDatabaseModal';
import { RestoreDatabaseModal } from '../components/RestoreDatabaseModal';
import { InstallDatabaseModal } from '../components/InstallDatabaseModal';
import { DbConfigModal } from '../components/DbConfigModal';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';
import { usePrivacy } from '../context/PrivacyContext';

interface DatabasesViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
  onNavigateToAI?: (serverId?: string, prompt?: string) => void;
}

export const DatabasesView: React.FC<DatabasesViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
  onNavigateToAI,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
    setPageLoading,
  } = useServer();
  const [instances, setInstances] = useState<database.DatabaseInstanceInfo[]>([]);
  const [selectedEngine, setSelectedEngine] = useState<string>('mysql');
  const [activeTab, setActiveTab] = useState<'databases' | 'users'>('databases');

  const [databasesList, setDatabasesList] = useState<database.DatabaseItem[]>([]);
  const [usersList, setUsersList] = useState<database.DatabaseUser[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [engineError, setEngineError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const instancesReq = useServerRequest();
  const engineReq = useServerRequest();

  // Modals state
  const [isCreateDBOpen, setIsCreateDBOpen] = useState(false);
  const [isCreateUserOpen, setIsCreateUserOpen] = useState(false);
  const [backupTargetDB, setBackupTargetDB] = useState<string | null>(null);
  const [restoreTargetDB, setRestoreTargetDB] = useState<string | null>(null);
  const [installTarget, setInstallTarget] = useState<database.DatabaseInstanceInfo | null>(null);
  const [configTarget, setConfigTarget] = useState<database.DatabaseInstanceInfo | null>(null);
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();

  const fetchInstances = async () => {
    if (!selectedServerID) return;
    const serverID = selectedServerID;
    const gen = instancesReq.next();
    setInstances([]);
    try {
      setIsLoading(true);
      setPageLoading(true, t('databases.loadingInspection'));
      const res = await InspectDatabases(serverID);
      if (!instancesReq.isCurrent(gen)) return;
      setInstances(res || []);

      const currentInstalled = res.find((i) => i.engine === selectedEngine && i.installed);
      if (!currentInstalled) {
        const firstInstalled = res.find((i) => i.installed);
        if (firstInstalled) {
          setSelectedEngine(firstInstalled.engine);
        }
      }
    } catch (err: unknown) {
      if (!instancesReq.isCurrent(gen)) return;
      setActionError(getErrorMessage(err));
    } finally {
      if (instancesReq.isCurrent(gen)) {
        setIsLoading(false);
        setPageLoading(false);
      }
    }
  };

  const fetchEngineData = async () => {
    if (!selectedServerID || !selectedEngine) return;
    const serverID = selectedServerID;
    const engine = selectedEngine;
    const gen = engineReq.next();
    setDatabasesList([]);
    setUsersList([]);
    try {
      setIsLoading(true);
      setPageLoading(true, t('databases.loadingEngineData', { engine: engine.toUpperCase() }));
      setEngineError(null);
      let loadErr: string | null = null;
      const [dbs, users] = await Promise.all([
        ListDatabases(serverID, engine).catch((e: any) => {
          loadErr = String(e?.message || e);
          return [];
        }),
        ListDatabaseUsers(serverID, engine).catch((e: any) => {
          if (!loadErr) loadErr = String(e?.message || e);
          return [];
        }),
      ]);
      if (!engineReq.isCurrent(gen)) return;
      setDatabasesList(dbs || []);
      setUsersList(users || []);
      if (loadErr && (!dbs || dbs.length === 0) && (!users || users.length === 0)) {
        setEngineError(loadErr);
      }
    } catch (err: unknown) {
      if (!engineReq.isCurrent(gen)) return;
      setEngineError(getErrorMessage(err));
    } finally {
      if (engineReq.isCurrent(gen)) {
        setIsLoading(false);
        setPageLoading(false);
      }
    }
  };

  useEffect(() => {
    fetchInstances();
  }, [selectedServerID]);

  useEffect(() => {
    const currentInst = instances.find((i) => i.engine === selectedEngine);
    if (currentInst?.installed) {
      fetchEngineData();
    } else {
      setDatabasesList([]);
      setUsersList([]);
      setEngineError(null);
    }
  }, [selectedServerID, selectedEngine, instances]);

  useEffect(() => {
    const cleanup = EventsOn('task:finished', (data: TaskRecord) => {
      if (
        data &&
        (data.operation_type === 'db_install' ||
          data.operation_type === 'db_create' ||
          data.operation_type === 'db_delete' ||
          data.operation_type === 'db_user_create' ||
          data.operation_type === 'db_user_delete' ||
          data.operation_type === 'db_restore') &&
        data.server_id === selectedServerID &&
        data.status === 'completed'
      ) {
        fetchInstances();
        fetchEngineData();
      }
    });
    return () => {
      if (typeof cleanup === 'function') cleanup();
    };
  }, [selectedServerID]);

  const handleControlService = async (engine: string, action: string) => {
    if (action === 'stop' && !(await confirm(t('databases.stopConfirm', { engine: engine.toUpperCase() }), { danger: true }))) {
      return;
    }
    if (action === 'restart' && !(await confirm(t('databases.restartConfirm', { engine: engine.toUpperCase() })))) {
      return;
    }
    try {
      const task = await ControlDatabaseService(selectedServerID, engine, action);
      onOpenTask(task as any);
      setTimeout(fetchInstances, 2000);
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    }
  };

  const handleDeleteDB = async (dbName: string) => {
    if (!(await confirm(t('databases.deleteDbPrompt', { name: dbName }), { danger: true, confirmLabel: t('common.delete') }))) {
      return;
    }
    try {
      const task = await DeleteDatabase(selectedServerID, selectedEngine, dbName);
      onOpenTask(task as any);
      setTimeout(fetchEngineData, 2000);
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    }
  };

  const isSystemUser = (name: string) => {
    return ['root', 'debian-sys-maint', 'mysql.sys', 'mysql.session', 'mysql.infoschema', 'postgres'].includes(name.toLowerCase());
  };

  const handleDeleteUser = async (username: string, host: string) => {
    if (!(await confirm(t('databases.deleteUserPrompt', { username, host }), { danger: true, confirmLabel: t('common.delete') }))) {
      return;
    }
    try {
      await DeleteDatabaseUser(selectedServerID, selectedEngine, username, host);
      fetchEngineData();
    } catch (err: unknown) {
      alert(t('databases.deleteUserError', { error: getErrorMessage(err) }));
    }
  };

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };



  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Database}
        description={t('empty.databasesDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  const selectedServer = servers.find((s) => s.id === selectedServerID);
  const { maskDatabase, maskUser, maskHost } = usePrivacy();
  const selectedInstance = instances.find((i) => i.engine === selectedEngine);
  const filteredDatabases = databasesList.filter((d) =>
    d.name.toLowerCase().includes(searchQuery.toLowerCase())
  );
  const filteredUsers = usersList.filter((u) =>
    u.username.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="page space-y-6">
      {actionError && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
          {actionError}
        </div>
      )}
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Database className="w-5 h-5" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('databases.targetServer')}
        actions={
          <>
            <Button
              variant="secondary"
              size="md"
              onClick={() => onNavigateToAI?.(selectedServerID, t('databases.aiAuditPrompt', { engine: selectedEngine }))}
              icon={<Sparkles className="w-3.5 h-3.5 text-indigo-400" />}
              title={t('databases.aiAuditTooltip')}
            >
              {t('databases.aiAuditBtn')}
            </Button>
            <Button
              variant="secondary"
              size="md"
              onClick={() => {
                fetchInstances();
                fetchEngineData();
              }}
              disabled={isLoading}
              icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />}
              title={t('databases.refreshTooltip')}
            >
              {t('databases.refresh')}
            </Button>
          </>
        }
      />

      {engineError && (
        <div className="p-3.5 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start justify-between gap-3 text-xs text-amber-200">
          <div className="flex items-start gap-2.5">
            <AlertCircle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <div>
              <span className="font-semibold block mb-0.5">{t('databases.errorReading')}</span>
              <span className="font-mono text-[11px] opacity-90 break-all">{engineError}</span>
            </div>
          </div>
          <button onClick={() => setEngineError(null)} className="text-amber-400 hover:text-white text-xs px-1.5 py-0.5 rounded">
            ✕
          </button>
        </div>
      )}

      {/* DBMS Instances Grid */}
      {isLoading && instances.length === 0 ? (
        <LoadingState
          type="stats"
          title={t('databases.loadingTitle')}
          subtitle={t('databases.loadingSubtitle', { server: selectedServer?.name || '' })}
        />
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4 items-stretch">
          {instances.map((inst) => {
            const isSelected = selectedEngine === inst.engine;
            const isRunning = inst.service_status === 'active';

            return (
              <div
                key={inst.engine}
                onClick={() => inst.installed && setSelectedEngine(inst.engine)}
                className={`p-5 rounded-2xl border transition-all cursor-pointer relative flex flex-col justify-between ${isSelected
                  ? 'bg-slate-900/90 border-indigo-500 shadow-lg shadow-indigo-500/10 ring-1 ring-indigo-500/30'
                  : inst.installed
                    ? 'bg-surface-850 border-slate-800 hover:border-slate-700 shadow-sm'
                    : 'bg-surface-850/40 border-slate-800/40 cursor-default'
                  }`}
              >
                <div>
                  <div className="flex items-start justify-between gap-2 mb-3">
                    <div className="flex items-center gap-3 min-w-0">
                      <div
                        className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 font-bold ${inst.installed
                          ? isRunning
                            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                            : 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                          : 'bg-slate-800 text-slate-500 border border-slate-700'
                          }`}
                      >
                        <Layers className="w-5 h-5" />
                      </div>
                      <div className="min-w-0">
                        <h3 className="text-sm font-bold text-white truncate" title={inst.display_name}>
                          {inst.display_name}
                        </h3>
                        <p className="text-[11px] text-slate-400 font-mono">
                          {inst.installed ? t('databases.portLabel', { port: inst.port }) : t('databases.notInstalled')}
                        </p>
                      </div>
                    </div>

                    {inst.installed && (
                      <Badge
                        variant={isRunning ? 'success' : 'danger'}
                        dot
                        size="xs"
                      >
                        {isRunning ? t('databases.activeStatus') : inst.service_status}
                      </Badge>
                    )}
                  </div>

                  {inst.installed ? (
                    <div className="space-y-2 mt-4 pt-3 border-t border-slate-800/80">
                      <div className="flex items-center justify-between text-xs">
                        <span className="text-slate-400 flex items-center gap-1.5">
                          <Cpu className="w-3.5 h-3.5 text-slate-400 shrink-0" /> {t('databases.memoryRss')}
                        </span>
                        <span className="font-mono text-slate-200 font-medium tabular-nums">
                          {formatBytes(inst.memory_bytes)}
                        </span>
                      </div>
                      <div className="flex items-center justify-between text-xs">
                        <span className="text-slate-400 flex items-center gap-1.5">
                          <Database className="w-3.5 h-3.5 text-slate-400 shrink-0" /> {t('databases.databasesCount')}
                        </span>
                        <span className="font-mono text-slate-200 font-medium tabular-nums">{inst.db_count}</span>
                      </div>
                    </div>
                  ) : (
                    <div className="mt-4 pt-4 border-t border-slate-800/40 flex flex-col items-start gap-3">
                      <span className="text-xs text-slate-400 italic">{t('databases.notInstalledNotice')}</span>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setInstallTarget(inst);
                        }}
                        className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold flex items-center gap-1.5 shadow-md shadow-indigo-600/30 transition-colors cursor-pointer"
                      >
                        <Download className="w-3.5 h-3.5" />
                        <span>{t('databases.install')}</span>
                      </button>
                    </div>
                  )}
                </div>

                {/* Service Controls */}
                {inst.installed && (
                  <div className="grid grid-cols-2 gap-2 pt-3 mt-3 border-t border-slate-800/60">
                    {isRunning ? (
                      <>
                        <Button
                          variant="secondary"
                          size="xs"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleControlService(inst.engine, 'restart');
                          }}
                          className="w-full justify-center min-w-0"
                          title={t('databases.restartTooltip')}
                          icon={<RefreshCw className="w-3.5 h-3.5 text-indigo-400 shrink-0" />}
                        >
                          <span className="truncate">{t('databases.restartBtn')}</span>
                        </Button>
                        <Button
                          variant="danger"
                          size="xs"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleControlService(inst.engine, 'stop');
                          }}
                          className="w-full justify-center min-w-0"
                          title={t('databases.stopTooltip')}
                          icon={<Square className="w-3.5 h-3.5 shrink-0" />}
                        >
                          <span className="truncate">{t('databases.stopBtn')}</span>
                        </Button>
                      </>
                    ) : (
                      <Button
                        variant="success"
                        size="xs"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleControlService(inst.engine, 'start');
                        }}
                        className="col-span-2 w-full justify-center min-w-0"
                        icon={<Play className="w-3.5 h-3.5 shrink-0" />}
                      >
                        <span className="truncate">{t('databases.startBtn')}</span>
                      </Button>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Engine Management Workspace */}
      <div className="bg-surface-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        {!selectedInstance?.installed ? (
          <div className="p-12 text-center space-y-4">
            <div className="w-12 h-12 rounded-2xl bg-slate-800/80 border border-slate-700/60 flex items-center justify-center text-slate-400 mx-auto">
              <Database className="w-6 h-6" />
            </div>
            <div>
              <h4 className="text-sm font-bold text-white">
                {t('databases.notInstalledWorkspaceTitle', { engine: selectedInstance?.display_name || selectedEngine.toUpperCase() })}
              </h4>
              <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
                {t('databases.notInstalledWorkspaceDesc')}
              </p>
            </div>
            {selectedInstance && (
              <button
                onClick={() => setInstallTarget(selectedInstance)}
                className="px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold inline-flex items-center gap-2 shadow-lg shadow-indigo-600/30 transition-all cursor-pointer"
              >
                <Download className="w-3.5 h-3.5" />
                <span>{t('databases.install')} {selectedInstance.display_name}</span>
              </button>
            )}
          </div>
        ) : (
          <>
            {/* Workspace Sub-header */}
            <div className="px-6 py-4 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div className="flex flex-wrap items-center gap-3">
                <Tabs
                  variant="pill"
                  activeTab={activeTab}
                  onChange={(id) => setActiveTab(id as 'databases' | 'users')}
                  tabs={[
                    {
                      id: 'databases',
                      label: `${t('databases.tabDatabases')} (${databasesList.length})`,
                      icon: <Database className="w-3.5 h-3.5" />,
                    },
                    ...(selectedEngine !== 'redis'
                      ? [
                        {
                          id: 'users',
                          label: `${t('databases.tabUsers')} (${usersList.length})`,
                          icon: <Users className="w-3.5 h-3.5" />,
                        },
                      ]
                      : []),
                  ]}
                />

                <SearchInput
                  sizeVariant="sm"
                  placeholder={activeTab === 'databases' ? t('databases.searchDatabasesPlaceholder') : t('databases.searchUsersPlaceholder')}
                  value={searchQuery}
                  onChangeValue={setSearchQuery}
                  className="w-56"
                />
              </div>

              <div className="flex items-center gap-2">
                {selectedInstance?.installed && selectedInstance?.config_path && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setConfigTarget(selectedInstance)}
                    icon={<FileCode className="w-3.5 h-3.5 text-indigo-400" />}
                  >
                    {t('databases.editConfig')}
                  </Button>
                )}
                {selectedEngine !== 'redis' && (
                  <>
                    {activeTab === 'databases' ? (
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => setIsCreateDBOpen(true)}
                        icon={<Plus className="w-3.5 h-3.5" />}
                      >
                        <span>{t('databases.newDatabase')}</span>
                      </Button>
                    ) : (
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => setIsCreateUserOpen(true)}
                        icon={<UserPlus className="w-3.5 h-3.5" />}
                      >
                        <span>{t('databases.newUser')}</span>
                      </Button>
                    )}
                  </>
                )}
              </div>
            </div>

            {/* Tab Content */}
            {activeTab === 'databases' ? (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="border-b border-slate-800/80 bg-slate-950/40 text-slate-400 font-semibold uppercase tracking-wider">
                      <th className="px-6 py-3.5">{t('databases.colDbName')}</th>
                      <th className="px-6 py-3.5">{t('databases.colCharset')}</th>
                      <th className="px-6 py-3.5">{t('databases.colDbTables')}</th>
                      <th className="px-6 py-3.5">{t('databases.colSize')}</th>
                      <th className="px-6 py-3.5 text-right">{t('databases.colActions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50">
                    {isLoading ? (
                      <tr>
                        <td colSpan={5} className="px-6 py-12 text-center text-slate-400">
                          <div className="flex items-center justify-center gap-2">
                            <RefreshCw className="w-4 h-4 animate-spin text-indigo-400" />
                            <span>{t('databases.readingDatabases')}</span>
                          </div>
                        </td>
                      </tr>
                    ) : filteredDatabases.length === 0 ? (
                      <tr>
                        <td colSpan={5} className="px-6 py-12 text-center text-slate-500">
                          {t('databases.noDatabases')}
                        </td>
                      </tr>
                    ) : (
                      filteredDatabases.map((db) => {
                        const isSystemDB =
                          ['information_schema', 'mysql', 'performance_schema', 'sys', 'postgres'].includes(
                            db.name
                          );

                        return (
                          <tr key={db.name} className="hover:bg-slate-850/40 transition-colors">
                            <td className="px-6 py-4 font-mono font-medium text-white flex items-center gap-2">
                              <Database className="w-4 h-4 text-indigo-400" />
                              {isSystemDB ? db.name : maskDatabase(db.name)}
                              {isSystemDB && (
                                <span className="px-1.5 py-0.5 rounded text-[10px] bg-slate-800 text-slate-400 font-sans">
                                  {t('databases.isSystem')}
                                </span>
                              )}
                            </td>
                            <td className="px-6 py-4 text-slate-400 font-mono">
                              {db.charset}
                              {db.collation && db.collation !== 'none' ? ` (${db.collation})` : ''}
                            </td>
                            <td className="px-6 py-4 font-mono text-slate-300">
                              {db.tables_count > 0 ? db.tables_count : '-'}
                            </td>
                            <td className="px-6 py-4 font-mono text-slate-300">
                              {formatBytes(db.size_bytes)}
                            </td>
                            <td className="px-6 py-4 text-right">
                              <div className="flex items-center justify-end gap-1.5">
                                {selectedEngine !== 'redis' && (
                                  <>
                                    <button
                                      onClick={() => setBackupTargetDB(db.name)}
                                      className="p-1.5 rounded-lg text-slate-400 hover:text-amber-400 hover:bg-slate-800 transition-colors"
                                      title={t('databases.backupTooltip')}
                                    >
                                      <Archive className="w-3.5 h-3.5" />
                                    </button>
                                    <button
                                      onClick={() => setRestoreTargetDB(db.name)}
                                      className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition-colors"
                                      title={t('databases.restoreTooltip')}
                                    >
                                      <RotateCcw className="w-3.5 h-3.5" />
                                    </button>
                                    {!isSystemDB && (
                                      <button
                                        onClick={() => handleDeleteDB(db.name)}
                                        className="p-1.5 rounded-lg text-slate-400 hover:text-rose-500 hover:bg-slate-800 transition-colors"
                                        title={t('databases.deleteDbTooltip')}
                                      >
                                        <Trash2 className="w-3.5 h-3.5" />
                                      </button>
                                    )}
                                  </>
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
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="border-b border-slate-800/80 bg-slate-950/40 text-slate-400 font-semibold uppercase tracking-wider">
                      <th className="px-6 py-3.5">{t('databases.colAccount')}</th>
                      <th className="px-6 py-3.5">{t('databases.colAllowedHost')}</th>
                      <th className="px-6 py-3.5">{t('databases.colEngine')}</th>
                      <th className="px-6 py-3.5 text-right">{t('databases.colActions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50">
                    {isLoading ? (
                      <tr>
                        <td colSpan={4} className="px-6 py-12 text-center text-slate-400">
                          <div className="flex items-center justify-center gap-2">
                            <RefreshCw className="w-4 h-4 animate-spin text-indigo-400" />
                            <span>{t('databases.readingUsers')}</span>
                          </div>
                        </td>
                      </tr>
                    ) : filteredUsers.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="px-6 py-12 text-center text-slate-500">
                          {t('databases.noUsers')}
                        </td>
                      </tr>
                    ) : (
                      filteredUsers.map((user, idx) => (
                        <tr key={`${user.username}-${idx}`} className="hover:bg-slate-850/40 transition-colors">
                          <td className="px-6 py-4 font-mono font-medium text-white flex items-center gap-2">
                            <Users className="w-4 h-4 text-indigo-400" />
                            {isSystemUser(user.username) ? user.username : maskUser(user.username)}
                            {isSystemUser(user.username) && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 font-sans">
                                {t('databases.isSystemUser')}
                              </span>
                            )}
                          </td>
                          <td className="px-6 py-4 text-slate-400 font-mono">
                            {['localhost', '127.0.0.1', '%', '::1'].includes(user.host) ? user.host : maskHost(user.host)}
                          </td>
                          <td className="px-6 py-4 font-mono uppercase text-indigo-400">
                            {user.engine}
                          </td>
                          <td className="px-6 py-4 text-right">
                            {!isSystemUser(user.username) && (
                              <button
                                onClick={() => handleDeleteUser(user.username, user.host)}
                                className="p-1.5 rounded-lg text-slate-400 hover:text-rose-500 hover:bg-slate-800 transition-colors"
                                title={t('databases.deleteUserTooltip')}
                              >
                                <Trash2 className="w-3.5 h-3.5" />
                              </button>
                            )}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>

      {/* Modals */}
      {isCreateDBOpen && (
        <CreateDatabaseModal
          serverID={selectedServerID}
          engine={selectedEngine}
          onClose={() => setIsCreateDBOpen(false)}
          onTaskCreated={(t) => {
            onOpenTask(t);
            setTimeout(fetchEngineData, 2000);
          }}
        />
      )}

      {isCreateUserOpen && (
        <CreateDatabaseUserModal
          serverID={selectedServerID}
          engine={selectedEngine}
          availableDatabases={databasesList.map((d) => d.name)}
          onClose={() => setIsCreateUserOpen(false)}
          onTaskCreated={(t) => {
            onOpenTask(t);
            setTimeout(fetchEngineData, 2000);
          }}
        />
      )}

      {backupTargetDB && (
        <BackupDatabaseModal
          serverID={selectedServerID}
          engine={selectedEngine}
          databaseName={backupTargetDB}
          onClose={() => setBackupTargetDB(null)}
          onTaskCreated={onOpenTask}
        />
      )}

      {restoreTargetDB && (
        <RestoreDatabaseModal
          serverID={selectedServerID}
          engine={selectedEngine}
          databaseName={restoreTargetDB}
          onClose={() => setRestoreTargetDB(null)}
          onTaskCreated={onOpenTask}
        />
      )}

      {installTarget && selectedServer && (
        <InstallDatabaseModal
          isOpen={true}
          serverID={selectedServer.id}
          serverName={selectedServer.name}
          engine={installTarget.engine}
          displayName={installTarget.display_name}
          onClose={() => setInstallTarget(null)}
          onTaskStarted={onOpenTask}
        />
      )}

      {configTarget && selectedServer && configTarget.config_path && (
        <DbConfigModal
          serverID={selectedServer.id}
          engine={configTarget.engine}
          filePath={configTarget.config_path}
          onClose={() => setConfigTarget(null)}
          onSaved={() => {
            fetchInstances();
            fetchEngineData();
          }}
        />
      )}
    </div>
  );
};

