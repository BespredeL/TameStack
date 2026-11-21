import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  Layers,
  Terminal,
  Play,
  RotateCw,
  CheckCircle2,
  AlertTriangle,
  FileCode,
  Clock,
  Cpu,
  RefreshCw,
  FolderSync,
  FileText,
  Save,
  Loader2,
  ExternalLink,
} from 'lucide-react';
import { db, apps } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { ModalShell } from './common/ModalShell';
import { Button, Input } from './ui';
import {
  InspectSiteApp,
  RunArtisan,
  FixAppPermissions,
  GetSiteEnv,
  SaveSiteEnv,
  ToggleLaravelScheduler,
  ConfigureQueueWorker,
  ControlQueueWorker,
  ReadLaravelLog,
} from '../../wailsjs/go/main/App';

interface SiteAppTabProps {
  site: db.SiteRecord;
}

export const SiteAppTab: React.FC<SiteAppTabProps> = ({ site }) => {
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  const [loading, setLoading] = useState(true);
  const [appInfo, setAppInfo] = useState<apps.AppInspectResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Artisan action state
  const [artisanLoading, setArtisanLoading] = useState(false);
  const [artisanOutput, setArtisanOutput] = useState<{ command: string; output: string; success: boolean; duration: number } | null>(null);

  // Env Editor state
  const [isEnvModalOpen, setIsEnvModalOpen] = useState(false);
  const [envContent, setEnvContent] = useState('');
  const [envLoading, setEnvLoading] = useState(false);
  const [envSaving, setEnvSaving] = useState(false);
  const [envMessage, setEnvMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Log viewer state
  const [isLogModalOpen, setIsLogModalOpen] = useState(false);
  const [logContent, setLogContent] = useState('');
  const [logLoading, setLogLoading] = useState(false);

  // Queue worker modal state
  const [isQueueModalOpen, setIsQueueModalOpen] = useState(false);
  const [queueConfig, setQueueConfig] = useState<apps.QueueWorkerConfig>(
    new apps.QueueWorkerConfig({
      site_id: site.id,
      connection: 'database',
      queue: 'default',
      tries: 3,
      timeout: 90,
      sleep: 3,
    })
  );
  const [queueActionLoading, setQueueActionLoading] = useState(false);

  const [actionInProgress, setActionInProgress] = useState<string | null>(null);

  useEffect(() => {
    loadAppInfo();
  }, [site.id]);

  const loadAppInfo = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await InspectSiteApp(site.id);
      setAppInfo(res);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const handleRunArtisan = async (cmd: string, isDangerous: boolean = false) => {
    if (isDangerous && !(await confirm(t('siteApp.dangerousConfirm', { cmd }), { danger: true }))) {
      return;
    }
    setArtisanLoading(true);
    try {
      const res = await RunArtisan(site.id, cmd, true);
      setArtisanOutput({
        command: cmd,
        output: res.output,
        success: res.success,
        duration: res.duration_ms,
      });
      loadAppInfo();
    } catch (err: unknown) {
      alert(t('siteApp.execError', { error: getErrorMessage(err) }));
    } finally {
      setArtisanLoading(false);
    }
  };

  const handleFixPermissions = async () => {
    setActionInProgress('permissions');
    try {
      await FixAppPermissions(site.id);
      await loadAppInfo();
    } catch (err: unknown) {
      alert(t('siteApp.fixPermsError', { error: getErrorMessage(err) }));
    } finally {
      setActionInProgress(null);
    }
  };

  const handleToggleScheduler = async (enable: boolean) => {
    setActionInProgress('scheduler');
    try {
      await ToggleLaravelScheduler(site.id, enable);
      await loadAppInfo();
    } catch (err: unknown) {
      alert(t('siteApp.setupSchedulerError', { error: getErrorMessage(err) }));
    } finally {
      setActionInProgress(null);
    }
  };

  const handleOpenEnv = async () => {
    setIsEnvModalOpen(true);
    setEnvLoading(true);
    setEnvMessage(null);
    try {
      const res = await GetSiteEnv(site.id);
      setEnvContent(res.content);
    } catch (err: unknown) {
      setEnvMessage({ type: 'error', text: getErrorMessage(err) });
    } finally {
      setEnvLoading(false);
    }
  };

  const handleSaveEnv = async () => {
    if (!(await confirm(t('siteApp.saveEnvConfirm'), { danger: true }))) {
      return;
    }
    setEnvSaving(true);
    setEnvMessage(null);
    try {
      await SaveSiteEnv(site.id, envContent);
      setEnvMessage({ type: 'success', text: t('siteApp.envSavedWithBackup') });
      loadAppInfo();
    } catch (err: unknown) {
      setEnvMessage({ type: 'error', text: getErrorMessage(err) });
    } finally {
      setEnvSaving(false);
    }
  };

  const handleOpenLogs = async () => {
    setIsLogModalOpen(true);
    setLogLoading(true);
    try {
      const text = await ReadLaravelLog(site.id, 200);
      setLogContent(text);
    } catch (err: unknown) {
      setLogContent(t('siteApp.loadLogError', { error: getErrorMessage(err) }));
    } finally {
      setLogLoading(false);
    }
  };

  const handleControlQueue = async (action: string) => {
    if (action === 'stop' && !(await confirm(t('siteApp.stopWorkerConfirm'), { danger: true }))) {
      return;
    }
    setQueueActionLoading(true);
    try {
      await ControlQueueWorker(site.id, action);
      await loadAppInfo();
    } catch (err: unknown) {
      alert(t('siteApp.workerControlError', { error: getErrorMessage(err) }));
    } finally {
      setQueueActionLoading(false);
    }
  };

  const handleInstallQueue = async () => {
    setQueueActionLoading(true);
    try {
      await ConfigureQueueWorker(site.id, queueConfig);
      setIsQueueModalOpen(false);
      await loadAppInfo();
    } catch (err: unknown) {
      alert(t('siteApp.workerSetupError', { error: getErrorMessage(err) }));
    } finally {
      setQueueActionLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-slate-400 space-y-3">
        <Loader2 className="w-8 h-8 animate-spin text-indigo-500" />
        <span className="text-xs">{t('siteApp.detectingFramework')}</span>
      </div>
    );
  }

  if (error || !appInfo) {
    return (
      <div className="p-6 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center justify-between">
        <span>{t('siteApp.appAnalysisError', { error: error || t('empty.noData') })}</span>
        <button
          onClick={loadAppInfo}
          className="px-3 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg transition"
        >{t('siteApp.retryBtn')}</button>
      </div>
    );
  }

  const isLaravel = appInfo.framework === 'laravel';
  const isWordPress = appInfo.framework === 'wordpress';
  const isSymfony = appInfo.framework === 'symfony';

  return (
    <div className="space-y-5">
      {/* Framework Summary Card */}
      <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div
            className={`w-12 h-12 rounded-xl flex items-center justify-center text-xl font-bold shadow-md ${
              isLaravel
                ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
                : isWordPress
                ? 'bg-sky-500/20 text-sky-400 border border-sky-500/30'
                : isSymfony
                ? 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30'
                : 'bg-slate-800 text-slate-400 border border-slate-700'
            }`}
          >
            {isLaravel ? 'L' : isWordPress ? 'W' : isSymfony ? 'S' : <Layers className="w-6 h-6" />}
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-bold text-white text-base capitalize">
                {appInfo.framework === 'unknown' ? t('siteApp.defaultPhpApp') : appInfo.framework}
              </h3>
              {appInfo.framework_version && (
                <span className="px-2 py-0.5 rounded text-[11px] font-mono bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">
                  {appInfo.framework_version}
                </span>
              )}
            </div>
            <p className="text-xs text-slate-400 mt-0.5">
              {t('siteApp.rootDirLabel')} <span className="font-mono text-slate-300">{site.document_root}</span>
              {appInfo.required_php && (
                <span className="ml-2 text-slate-500">{t('siteApp.requiresPhpLabel')} <span className="font-mono text-slate-400">{appInfo.required_php}</span></span>
              )}
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {appInfo.has_env_file && (
            <button
              onClick={handleOpenEnv}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold flex items-center gap-1.5 transition cursor-pointer"
            >
              <FileCode className="w-3.5 h-3.5 text-indigo-400" />
              <span>{t('siteApp.envEditorBtn')}</span>
            </button>
          )}

          {appInfo.available_logs && appInfo.available_logs.length > 0 && (
            <button
              onClick={handleOpenLogs}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold flex items-center gap-1.5 transition cursor-pointer"
            >
              <FileText className="w-3.5 h-3.5 text-emerald-400" />
              <span>{t('siteApp.appLogsBtn')}</span>
            </button>
          )}

          <button
            onClick={loadAppInfo}
            className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-slate-200 border border-slate-700 transition"
            title={t('siteApp.refreshStatusTooltip')}
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Issues / Health Alerts */}
      {appInfo.issues && appInfo.issues.length > 0 && (
        <div className="space-y-2">
          {appInfo.issues.map((iss: apps.AppIssue, i: number) => (
            <div
              key={i}
              className={`p-3.5 rounded-xl border flex items-center justify-between gap-3 text-xs ${
                iss.severity === 'error'
                  ? 'bg-rose-500/10 border-rose-500/25 text-rose-300'
                  : 'bg-amber-500/10 border-amber-500/25 text-amber-300'
              }`}
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                <span className="font-medium">{iss.message}</span>
              </div>
              {iss.fixable && (
                <button
                  onClick={() => {
                    if (iss.fix_action === 'fix_permissions') handleFixPermissions();
                    else if (iss.fix_action === 'key_generate') handleRunArtisan('key:generate', true);
                  }}
                  disabled={actionInProgress !== null}
                  className="px-2.5 py-1 rounded-lg bg-slate-900/90 hover:bg-slate-800 text-white font-semibold border border-slate-700 transition shrink-0 cursor-pointer disabled:opacity-50"
                >
                  {actionInProgress === 'permissions' ? t('siteApp.fixing') : t('siteApp.fix')}
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Laravel Artisan Whitelisted Commands */}
      {isLaravel && (
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Terminal className="w-4 h-4 text-indigo-400" />
              <h4 className="font-bold text-white text-xs tracking-wide uppercase">
                {t('siteApp.artisanCommandsHeading')}
              </h4>
            </div>
            {artisanLoading && (
              <span className="text-xs text-indigo-400 flex items-center gap-1.5 font-medium">
                <Loader2 className="w-3.5 h-3.5 animate-spin" /> {t('siteApp.executing')}
              </span>
            )}
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
            <button
              onClick={() => handleRunArtisan('cache:clear')}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <RotateCw className="w-3.5 h-3.5 text-indigo-400" />
              <span>cache:clear</span>
            </button>

            <button
              onClick={() => handleRunArtisan('config:cache')}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <FileCode className="w-3.5 h-3.5 text-emerald-400" />
              <span>config:cache</span>
            </button>

            <button
              onClick={() => handleRunArtisan('route:cache')}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <FolderSync className="w-3.5 h-3.5 text-sky-400" />
              <span>route:cache</span>
            </button>

            <button
              onClick={() => handleRunArtisan('view:cache')}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <Layers className="w-3.5 h-3.5 text-amber-400" />
              <span>view:cache</span>
            </button>

            <button
              onClick={() => handleRunArtisan('optimize')}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <Cpu className="w-3.5 h-3.5 text-emerald-400" />
              <span>optimize</span>
            </button>

            <button
              onClick={() => handleRunArtisan('storage:link')}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <ExternalLink className="w-3.5 h-3.5 text-indigo-400" />
              <span>storage:link</span>
            </button>

            <button
              onClick={() => handleRunArtisan('migrate:status')}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <CheckCircle2 className="w-3.5 h-3.5 text-sky-400" />
              <span>migrate:status</span>
            </button>

            <button
              onClick={() => handleRunArtisan('migrate', true)}
              disabled={artisanLoading}
              className="p-2 rounded-lg bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border border-amber-500/30 text-xs font-semibold transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
              title={t('siteApp.operatorApprovalRequired')}
            >
              <Play className="w-3.5 h-3.5 text-amber-400" />
              <span>migrate</span>
            </button>
          </div>

          {/* Artisan Output Console */}
          {artisanOutput && (
            <div className="mt-3 p-3 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono space-y-1">
              <div className="flex items-center justify-between text-[11px] text-slate-400 border-b border-slate-800/80 pb-1">
                <span>$ php artisan {artisanOutput.command}</span>
                <span className={artisanOutput.success ? 'text-emerald-400' : 'text-rose-400'}>
                  {artisanOutput.duration}ms • {artisanOutput.success ? 'OK' : 'FAIL'}
                </span>
              </div>
              <pre className="text-slate-300 whitespace-pre-wrap max-h-48 overflow-y-auto leading-relaxed pt-1">
                {artisanOutput.output || t('siteApp.commandNoOutput')}
              </pre>
            </div>
          )}
        </div>
      )}

      {/* Background Services: Scheduler & Queue Worker */}
      {isLaravel && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Laravel Scheduler */}
          <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col justify-between space-y-3">
            <div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Clock className="w-4 h-4 text-indigo-400" />
                  <h4 className="font-bold text-white text-xs uppercase tracking-wide">
                    {t('siteApp.schedulerHeading')}
                  </h4>
                </div>
                <span
                  className={`px-2 py-0.5 rounded-full text-[10px] font-semibold border ${
                    appInfo.scheduler_active
                      ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                      : 'bg-slate-800 text-slate-400 border-slate-700'
                  }`}
                >
                  {appInfo.scheduler_active ? t('siteApp.active') : t('siteApp.disabled')}
                </span>
              </div>
              <p className="text-[11px] text-slate-400 mt-2">
                {t('siteApp.schedulerCronDesc')}
              </p>
            </div>

            <div className="pt-2 border-t border-slate-800/60 flex items-center justify-end">
              <button
                onClick={() => handleToggleScheduler(!appInfo.scheduler_active)}
                disabled={actionInProgress !== null}
                className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 ${
                  appInfo.scheduler_active
                    ? 'bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30'
                    : 'bg-emerald-600 hover:bg-emerald-500 text-white'
                }`}
              >
                {appInfo.scheduler_active ? t('siteApp.disableScheduler') : t('siteApp.enableScheduler')}
              </button>
            </div>
          </div>

          {/* Queue Worker Service */}
          <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col justify-between space-y-3">
            <div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <RotateCw className="w-4 h-4 text-indigo-400" />
                  <h4 className="font-bold text-white text-xs uppercase tracking-wide">
                    {t('siteApp.queueWorkerHeading')}
                  </h4>
                </div>
                <span
                  className={`px-2 py-0.5 rounded-full text-[10px] font-semibold border ${
                    appInfo.queue_worker_status === 'running'
                      ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                      : appInfo.queue_worker_status === 'stopped'
                      ? 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                      : 'bg-slate-800 text-slate-400 border-slate-700'
                  }`}
                >
                  {appInfo.queue_worker_status === 'running'
                    ? t('siteApp.running')
                    : appInfo.queue_worker_status === 'stopped'
                    ? t('siteApp.stopped')
                    : t('siteApp.notInstalled')}
                </span>
              </div>
              <p className="text-[11px] text-slate-400 mt-2">
                {t('siteApp.queueWorkerDesc', { unit: appInfo.queue_worker_unit || 'sh-queue-...' })}
              </p>
            </div>

            <div className="pt-2 border-t border-slate-800/60 flex items-center justify-between gap-2">
              <button
                onClick={() => setIsQueueModalOpen(true)}
                className="px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold transition"
              >{t('siteApp.configureBtn')}</button>

              <div className="flex items-center gap-1.5">
                {appInfo.queue_worker_status === 'running' ? (
                  <>
                    <button
                      onClick={() => handleControlQueue('restart')}
                      disabled={queueActionLoading}
                      className="px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition cursor-pointer"
                    >{t('siteApp.restartBtn')}</button>
                    <button
                      onClick={() => handleControlQueue('stop')}
                      disabled={queueActionLoading}
                      className="px-2.5 py-1.5 rounded-lg bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30 text-xs font-semibold transition cursor-pointer"
                    >{t('siteApp.stopBtn')}</button>
                  </>
                ) : appInfo.queue_worker_status === 'stopped' ? (
                  <button
                    onClick={() => handleControlQueue('start')}
                    disabled={queueActionLoading}
                    className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold transition cursor-pointer"
                  >{t('siteApp.startBtn')}</button>
                ) : (
                  <button
                    onClick={() => setIsQueueModalOpen(true)}
                    className="px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold transition cursor-pointer"
                  >{t('siteApp.installUnitBtn')}</button>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Runtimes Information */}
      <div className="p-4 rounded-xl bg-slate-900/40 border border-slate-800/80 space-y-2">
        <h4 className="font-bold text-slate-400 text-[11px] uppercase tracking-wider">
          {t('siteApp.runtimesHeading')}
        </h4>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
          <div className="p-2.5 rounded-lg bg-slate-900 border border-slate-800">
            <span className="text-slate-500 block text-[10px]">Node.js</span>
            <span className="font-mono text-slate-200 font-semibold">
              {appInfo.runtimes.node_installed ? appInfo.runtimes.node_version : t('siteApp.notInstalled')}
            </span>
          </div>
          <div className="p-2.5 rounded-lg bg-slate-900 border border-slate-800">
            <span className="text-slate-500 block text-[10px]">NPM</span>
            <span className="font-mono text-slate-200 font-semibold">
              {appInfo.runtimes.npm_installed ? appInfo.runtimes.npm_version : t('siteApp.notInstalled')}
            </span>
          </div>
          <div className="p-2.5 rounded-lg bg-slate-900 border border-slate-800">
            <span className="text-slate-500 block text-[10px]">Python</span>
            <span className="font-mono text-slate-200 font-semibold">
              {appInfo.runtimes.python_installed ? appInfo.runtimes.python_version : t('siteApp.notInstalled')}
            </span>
          </div>
          <div className="p-2.5 rounded-lg bg-slate-900 border border-slate-800">
            <span className="text-slate-500 block text-[10px]">Go</span>
            <span className="font-mono text-slate-200 font-semibold">
              {appInfo.runtimes.go_installed ? appInfo.runtimes.go_version : t('siteApp.notInstalled')}
            </span>
          </div>
        </div>
      </div>

      {/* Env Editor Modal */}
      {isEnvModalOpen && (
        <ModalShell
          onClose={() => setIsEnvModalOpen(false)}
          size="4xl"
          className="h-[80vh]"
          icon={<FileCode className="w-5 h-5 text-indigo-400" />}
          iconClassName=""
          title={t('siteApp.envEditorHeading')}
        >

            {envMessage && (
              <div
                className={`mx-6 mt-4 p-3 rounded-xl border text-xs ${
                  envMessage.type === 'success'
                    ? 'bg-emerald-500/10 border-emerald-500/25 text-emerald-300'
                    : 'bg-rose-500/10 border-rose-500/25 text-rose-300'
                }`}
              >
                {envMessage.text}
              </div>
            )}

            <div className="p-6 flex-1 min-h-0 flex flex-col">
              {envLoading ? (
                <div className="flex-1 flex items-center justify-center text-slate-400">
                  <Loader2 className="w-6 h-6 animate-spin text-indigo-500" />
                </div>
              ) : (
                <textarea
                  value={envContent}
                  onChange={(e) => setEnvContent(e.target.value)}
                  className="w-full h-full p-4 font-mono text-xs text-slate-200 bg-slate-950 border border-slate-800 rounded-xl resize-none focus:outline-none focus:border-indigo-500 custom-scrollbar leading-relaxed"
                  spellCheck={false}
                />
              )}
            </div>

            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-between">
              <span className="text-[11px] text-slate-500">
                {t('siteApp.envBackupNotice')}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="md"
                  onClick={() => setIsEnvModalOpen(false)}
                >
                  {t('common.cancel')}
                </Button>
                <Button
                  variant="primary"
                  size="md"
                  onClick={handleSaveEnv}
                  disabled={envSaving || envLoading}
                  loading={envSaving}
                  icon={!envSaving ? <Save className="w-3.5 h-3.5" /> : undefined}
                >
                  {t('siteApp.saveEnvBtn')}
                </Button>
              </div>
            </div>
        </ModalShell>
      )}

      {/* Log Viewer Modal */}
      {isLogModalOpen && (
        <ModalShell
          onClose={() => setIsLogModalOpen(false)}
          size="4xl"
          className="h-[80vh]"
          icon={<FileText className="w-5 h-5 text-emerald-400" />}
          iconClassName=""
          title={t('siteApp.laravelLogHeading')}
        >

            <div className="p-6 flex-1 min-h-0">
              {logLoading ? (
                <div className="w-full h-full flex items-center justify-center text-slate-400">
                  <Loader2 className="w-6 h-6 animate-spin text-indigo-500" />
                </div>
              ) : (
                <pre className="w-full h-full p-4 font-mono text-[11px] text-slate-300 bg-slate-950 border border-slate-800 rounded-xl overflow-y-auto custom-scrollbar leading-relaxed whitespace-pre-wrap select-all">
                  {logContent}
                </pre>
              )}
            </div>

            <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end">
              <Button
                variant="ghost"
                size="md"
                onClick={() => setIsLogModalOpen(false)}
              >
                {t('common.close')}
              </Button>
            </div>
        </ModalShell>
      )}

      {/* Queue Worker Config Modal */}
      {isQueueModalOpen && (
        <ModalShell
          onClose={() => setIsQueueModalOpen(false)}
          size="md"
          title={t('siteApp.queueWorkerModalHeading')}
        >

            <div className="p-6 space-y-4 text-xs">
              <div>
                <label className="block text-slate-400 mb-1">{t('siteApp.connectionDriverLabel')}</label>
                <Input
                  type="text"
                  value={queueConfig.connection}
                  onChange={(e) => setQueueConfig({ ...queueConfig, connection: e.target.value })}
                  placeholder="database, redis, sqs"
                  className="font-mono"
                />
              </div>

              <div>
                <label className="block text-slate-400 mb-1">{t('siteApp.queueNameLabel')}</label>
                <Input
                  type="text"
                  value={queueConfig.queue}
                  onChange={(e) => setQueueConfig({ ...queueConfig, queue: e.target.value })}
                  placeholder="default"
                  className="font-mono"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-slate-400 mb-1">{t('siteApp.triesCountLabel')}</label>
                  <Input
                    type="number"
                    value={queueConfig.tries}
                    onChange={(e) => setQueueConfig({ ...queueConfig, tries: parseInt(e.target.value) || 3 })}
                    className="font-mono"
                  />
                </div>
                <div>
                  <label className="block text-slate-400 mb-1">{t('siteApp.timeoutSecLabel')}</label>
                  <Input
                    type="number"
                    value={queueConfig.timeout}
                    onChange={(e) => setQueueConfig({ ...queueConfig, timeout: parseInt(e.target.value) || 90 })}
                    className="font-mono"
                  />
                </div>
              </div>
            </div>

            <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button
                variant="ghost"
                size="md"
                onClick={() => setIsQueueModalOpen(false)}
              >
                {t('common.cancel')}
              </Button>
              <Button
                variant="primary"
                size="md"
                onClick={handleInstallQueue}
                disabled={queueActionLoading}
                loading={queueActionLoading}
              >
                {t('siteApp.applyServiceBtn')}
              </Button>
            </div>
        </ModalShell>
      )}
    </div>
  );
};
