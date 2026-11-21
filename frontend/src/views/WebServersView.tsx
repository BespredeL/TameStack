import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  HardDrive,
  Server,
  RefreshCw,
  CheckCircle2,
  Play,
  Square,
  RotateCw,
  FileCode,
  AlertCircle,
  Wrench,
  Download,
  Pencil,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import { InspectWebServers, TestWebserverConfig, ControlWebserver, RepairWebAccess } from '../../wailsjs/go/main/App';
import { webserver } from '../../wailsjs/go/models';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { PageToolbar, Button, Badge } from '../components/ui';
import { InstallWebServerModal } from '../components/InstallWebServerModal';
import { WebserverConfigModal } from '../components/WebserverConfigModal';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';
import { ModalShell } from '../components/common/ModalShell';
import { useServerData } from '../hooks/useServerData';

interface WebServersViewProps {
  servers: ServerRecord[];
  onOpenAddServer: () => void;
  onOpenTask: (task: TaskRecord) => void;
}

export const WebServersView: React.FC<WebServersViewProps> = ({
  servers,
  onOpenAddServer,
  onOpenTask,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
  } = useServer();
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [isRepairing, setIsRepairing] = useState(false);
  const [testResult, setTestResult] = useState<{ name: string; passed: boolean; output: string } | null>(null);
  const [installTarget, setInstallTarget] = useState<webserver.WebServerStatus | null>(null);
  const [configTarget, setConfigTarget] = useState<webserver.WebServerStatus | null>(null);

  const selectedServer = servers.find((s) => s.id === selectedServerID) || servers[0];

  const {
    data: webservers,
    loading: isLoading,
    reload: fetchWebservers,
    setData: setWebservers,
  } = useServerData(
    (serverId) => InspectWebServers(serverId).then((r) => r || []),
    [],
    {
      initial: [] as webserver.WebServerStatus[],
      loadingMessage: selectedServer ? t('webservers.scanning', { server: selectedServer.name }) : undefined,
      onError: (msg) => console.error(msg),
    },
  );

  useEffect(() => {
    const cleanup = EventsOn('task:finished', (data: TaskRecord) => {
      if (
        data &&
        (data.operation_type === 'webserver_install' || data.operation_type === 'webserver_control') &&
        data.server_id === selectedServerID &&
        data.status === 'completed'
      ) {
        fetchWebservers();
      }
    });
    return () => {
      if (typeof cleanup === 'function') cleanup();
    };
  }, [selectedServerID, fetchWebservers]);

  const handleAction = async (wsName: string, action: string) => {
    if (!selectedServer) return;
    if (action === 'stop' && !(await confirm(t('webservers.stopConfirm', { name: wsName.toUpperCase() }), { danger: true }))) {
      return;
    }
    if (action === 'restart' && !(await confirm(t('webservers.restartConfirm', { name: wsName.toUpperCase() })))) {
      return;
    }
    setActionLoading(`${wsName}-${action}`);
    try {
      await ControlWebserver(selectedServer.id, wsName, action);
      await fetchWebservers();
    } catch (err: unknown) {
      alert(`Action error: ${getErrorMessage(err)}`);
    } finally {
      setActionLoading(null);
    }
  };

  const handleTestConfig = async (wsName: string) => {
    if (!selectedServer) return;
    setActionLoading(`${wsName}-test`);
    try {
      const res = await TestWebserverConfig(selectedServer.id, wsName);
      const passed = Boolean(res.passed);
      const output = String(res.output || '');
      setTestResult({
        name: wsName,
        passed,
        output,
      });
      setWebservers((prev) =>
        prev.map((ws) =>
          ws.name === wsName ? { ...ws, test_passed: passed, test_output: output } : ws
        )
      );
    } catch (err: unknown) {
      alert(`Config test failed: ${getErrorMessage(err)}`);
    } finally {
      setActionLoading(null);
    }
  };

  const handleRepairWebAccess = async () => {
    if (!selectedServer) return;
    if (!(await confirm(t('webservers.repairAccessTooltip'), { danger: true }))) {
      return;
    }
    setIsRepairing(true);
    try {
      await RepairWebAccess(selectedServer.id);
      setTimeout(fetchWebservers, 2500);
    } catch (err: unknown) {
      alert(`Repair error: ${getErrorMessage(err)}`);
    } finally {
      setIsRepairing(false);
    }
  };


  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={HardDrive}
        description={t('empty.webserversDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        icon={<HardDrive className="w-5 h-5" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('webservers.targetServer')}
        actions={
          <>
            <Button
              variant="danger"
              size="md"
              onClick={handleRepairWebAccess}
              disabled={isRepairing}
              icon={<Wrench className={`w-3.5 h-3.5 ${isRepairing ? 'animate-spin' : ''}`} />}
              title={t('webservers.repairAccessTooltip')}
            >
              {isRepairing ? t('webservers.repairing') : t('webservers.repairAccess')}
            </Button>
            <Button
              variant="secondary"
              size="md"
              onClick={fetchWebservers}
              disabled={isLoading}
              icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />}
            >
              {t('webservers.scanWebservers')}
            </Button>
          </>
        }
      />

      {/* Web Servers Grid */}
      {isLoading ? (
        <LoadingState
          type="cards"
          title={t('webservers.loadingTitle')}
          subtitle={t('webservers.loadingSubtitle', { server: selectedServer?.name || '' })}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5 items-stretch">
          {webservers.map((ws) => {
            return (
              <div
                key={ws.name}
                className={`bg-surface-850 border rounded-2xl p-6 shadow-lg flex flex-col justify-between transition-all ${
                  ws.is_installed
                    ? 'border-slate-800 hover:border-slate-700/80'
                    : 'border-slate-800/60 bg-surface-900/40'
                }`}
              >
                <div>
                  {/* Header */}
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3 min-w-0">
                      <div
                        className={`w-12 h-12 rounded-xl flex items-center justify-center font-bold text-sm shrink-0 ${
                          ws.is_installed
                            ? 'bg-indigo-600/20 text-indigo-400 border border-indigo-500/30'
                            : 'bg-slate-800 text-slate-500 border border-slate-700'
                        }`}
                      >
                        <HardDrive className="w-6 h-6" />
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-bold text-white text-base truncate">{ws.display_name}</h3>
                        <div className="text-xs text-slate-400 font-mono mt-0.5 truncate">
                          {ws.is_installed ? t('webservers.version', { version: ws.version || t('webservers.versionDetected') }) : t('webservers.notInstalled')}
                        </div>
                      </div>
                    </div>

                    <Badge
                      variant={
                        ws.status === 'active'
                          ? 'success'
                          : ws.is_installed
                          ? ws.status === 'failed'
                            ? 'danger'
                            : 'warning'
                          : 'neutral'
                      }
                      dot={ws.status === 'active'}
                      pulse={ws.status === 'active'}
                    >
                      {ws.status === 'active'
                        ? t('webservers.serviceActive')
                        : ws.is_installed
                        ? ws.status === 'failed'
                          ? t('webservers.serviceFailed')
                          : t('webservers.serviceInactive')
                        : t('webservers.notInstalled')}
                    </Badge>
                  </div>

                  {ws.is_installed ? (
                    <div className="space-y-3">
                      {/* Main Config */}
                      <div className="p-3 bg-slate-900/90 rounded-xl border border-slate-800/80 flex items-center justify-between gap-3 text-xs">
                        <div className="min-w-0 flex-1">
                          <span className="text-slate-400 block text-[10px] font-semibold uppercase tracking-wider mb-0.5">
                            {t('webservers.vhostConfigLabel')}
                          </span>
                          <span className="font-mono text-slate-300 text-xs truncate block" title={ws.config_path}>
                            {ws.config_path || '—'}
                          </span>
                        </div>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setConfigTarget(ws)}
                          title={t('webservers.editConfig')}
                          icon={<Pencil className="w-3.5 h-3.5 text-indigo-400" />}
                        >
                          {t('webservers.editConfig')}
                        </Button>
                      </div>

                      {/* Syntax Status */}
                      <div className="p-3 bg-slate-900/90 rounded-xl border border-slate-800/80 flex items-center justify-between gap-2.5 text-xs">
                        <div className="flex items-center gap-2 min-w-0">
                          {ws.test_passed ? (
                            <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                          ) : (
                            <AlertCircle className="w-4 h-4 text-amber-300 shrink-0" />
                          )}
                          <span className="text-slate-400 text-xs">{t('webservers.testConfig')}:</span>
                          <span
                            className={`font-semibold px-2 py-0.5 rounded text-[11px] whitespace-nowrap ${
                              ws.test_passed
                                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                                : 'bg-amber-500/10 text-amber-300 border border-amber-500/20'
                            }`}
                          >
                            {ws.test_passed ? t('webservers.testConfigPassed') : t('webservers.testConfigFailed')}
                          </span>
                        </div>

                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => handleTestConfig(ws.name)}
                          disabled={actionLoading === `${ws.name}-test`}
                          loading={actionLoading === `${ws.name}-test`}
                          title={t('webservers.testConfig')}
                          icon={actionLoading !== `${ws.name}-test` ? <FileCode className="w-3.5 h-3.5 text-indigo-400" /> : undefined}
                        >
                          {t('webservers.testConfig')}
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <div className="p-5 rounded-xl bg-slate-900/40 border border-slate-800/60 text-xs text-slate-400 flex flex-col items-center justify-center text-center py-8 space-y-3">
                      <HardDrive className="w-8 h-8 text-slate-600 stroke-[1.5]" />
                      <p className="font-semibold text-slate-300">{t('webservers.notInstalled')}</p>
                      <Button
                        variant="primary"
                        size="md"
                        onClick={() => setInstallTarget(ws)}
                        icon={<Download className="w-3.5 h-3.5" />}
                      >
                        {t('webservers.install')}
                      </Button>
                    </div>
                  )}
                </div>

                {/* Action Controls */}
                {ws.is_installed && (
                  <div className="pt-4 mt-4 border-t border-slate-800/80 grid grid-cols-3 gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => handleAction(ws.name, 'restart')}
                      disabled={actionLoading !== null}
                      title="systemctl restart"
                      icon={<RotateCw className="w-3.5 h-3.5 text-indigo-400 shrink-0" />}
                    >
                      <span className="truncate">{t('webservers.restart')}</span>
                    </Button>

                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => handleAction(ws.name, 'reload')}
                      disabled={actionLoading !== null}
                      title="systemctl reload"
                      icon={<RefreshCw className="w-3.5 h-3.5 text-emerald-400 shrink-0" />}
                    >
                      <span className="truncate">{t('webservers.reload')}</span>
                    </Button>

                    {ws.status === 'active' ? (
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => handleAction(ws.name, 'stop')}
                        disabled={actionLoading !== null}
                        title="systemctl stop"
                        icon={<Square className="w-3.5 h-3.5 shrink-0" />}
                      >
                        <span className="truncate">{t('webservers.stop')}</span>
                      </Button>
                    ) : (
                      <Button
                        variant="success"
                        size="sm"
                        onClick={() => handleAction(ws.name, 'start')}
                        disabled={actionLoading !== null}
                        title="systemctl start"
                        icon={<Play className="w-3.5 h-3.5 shrink-0" />}
                      >
                        <span className="truncate">{t('webservers.start')}</span>
                      </Button>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Syntax Test Output Dialog */}
      {testResult && (
        <ModalShell
          onClose={() => setTestResult(null)}
          size="lg"
          icon={testResult.passed ? <CheckCircle2 className="w-5 h-5 text-emerald-400" /> : <AlertCircle className="w-5 h-5 text-rose-400" />}
          iconClassName=""
          title={`${t('webservers.testConfig')}: ${testResult.name.toUpperCase()}`}
        >
          <div className="p-6 space-y-4">
            <div className="p-3 bg-slate-950 rounded-lg border border-slate-800 font-mono text-xs text-slate-300 whitespace-pre-wrap leading-relaxed select-all">
              {testResult.output || '—'}
            </div>

            <div className="flex justify-end">
              <button
                onClick={() => setTestResult(null)}
                className="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium"
              >
                {t('common.close')}
              </button>
            </div>
          </div>
        </ModalShell>
      )}

      {/* Install Web Server Dialog */}
      {installTarget && selectedServer && (
        <InstallWebServerModal
          isOpen={true}
          serverID={selectedServer.id}
          serverName={selectedServer.name}
          wsType={installTarget.name}
          displayName={installTarget.display_name}
          onClose={() => setInstallTarget(null)}
          onTaskStarted={onOpenTask}
        />
      )}

      {/* Config Editor Dialog */}
      {configTarget && selectedServer && configTarget.config_path && (
        <WebserverConfigModal
          serverID={selectedServer.id}
          wsType={configTarget.name}
          filePath={configTarget.config_path}
          onClose={() => setConfigTarget(null)}
          onSaved={fetchWebservers}
        />
      )}
    </div>
  );
};
