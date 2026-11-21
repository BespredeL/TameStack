import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  Globe,
  Settings,
  FileCode,
  Shield,
  Terminal,
  Save,
  RefreshCw,
  CheckCircle2,
  AlertTriangle,
  Loader2,
  Layers,
  ExternalLink,
  History,
  Eye,
  RotateCcw,
  GitCompare,
  Code2,
} from 'lucide-react';
import { db, site } from '../../wailsjs/go/models';
import { ModalShell } from './common/ModalShell';
import { Button, Tabs } from './ui';
import {
  GetSiteConfigFile,
  SaveSiteConfigFile,
  ListSiteConfigBackups,
  RollbackSiteConfigFile,
} from '../../wailsjs/go/main/App';
import { SiteAppTab } from './SiteAppTab';
import { SiteOverviewTab } from './SiteOverviewTab';
import { SiteSSLTab } from './SiteSSLTab';
import { SiteLogsTab } from './SiteLogsTab';
import { SiteRuntimeTab } from './SiteRuntimeTab';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';

interface ManageSiteModalProps {
  isOpen: boolean;
  site: db.SiteRecord;
  serverID: string;
  onClose: () => void;
  onSiteUpdated: () => void;
  onOpenTask?: (task: any) => void;
}

type TabType = 'overview' | 'app' | 'runtime' | 'vhost' | 'history' | 'ssl' | 'logs';

interface DiffLine {
  type: 'same' | 'add' | 'del';
  line: string;
}

function computeLineDiff(oldText: string, newText: string): DiffLine[] {
  const oldLines = oldText.split('\n');
  const newLines = newText.split('\n');
  const diff: DiffLine[] = [];
  let i = 0, j = 0;
  while (i < oldLines.length || j < newLines.length) {
    if (i < oldLines.length && j < newLines.length && oldLines[i] === newLines[j]) {
      diff.push({ type: 'same', line: oldLines[i] });
      i++;
      j++;
    } else {
      const oldInNew = newLines.indexOf(oldLines[i], j);
      const newInOld = oldLines.indexOf(newLines[j], i);
      if (j < newLines.length && (oldInNew === -1 || (newInOld !== -1 && newInOld >= i))) {
        diff.push({ type: 'add', line: newLines[j] });
        j++;
      } else if (i < oldLines.length) {
        diff.push({ type: 'del', line: oldLines[i] });
        i++;
      } else if (j < newLines.length) {
        diff.push({ type: 'add', line: newLines[j] });
        j++;
      }
    }
  }
  return diff;
}

export const ManageSiteModal: React.FC<ManageSiteModalProps> = ({
  isOpen,
  site: initialSite,
  serverID,
  onClose,
  onSiteUpdated,
  onOpenTask,
}) => {
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  const [currentSite, setCurrentSite] = useState<db.SiteRecord>(initialSite);
  const [activeTab, setActiveTab] = useState<TabType>('overview');

  // Vhost Config Tab State
  const [configContent, setConfigContent] = useState('');
  const [originalConfigContent, setOriginalConfigContent] = useState('');
  const [configPath, setConfigPath] = useState('');
  const [showDiffModal, setShowDiffModal] = useState(false);
  const [backups, setBackups] = useState<db.SiteConfigBackupRecord[]>([]);
  const [isLoadingBackups, setIsLoadingBackups] = useState(false);
  const [selectedBackup, setSelectedBackup] = useState<db.SiteConfigBackupRecord | null>(null);
  const [isRollingBack, setIsRollingBack] = useState(false);
  const siteWebServer = (initialSite.web_server || 'nginx').toLowerCase();
  const defaultEngine = siteWebServer.includes('apache') && !siteWebServer.includes('nginx') ? 'apache' : 'nginx';
  const [webserverType, setWebserverType] = useState(defaultEngine);
  const [isLoadingConfig, setIsLoadingConfig] = useState(false);
  const [isSavingConfig, setIsSavingConfig] = useState(false);
  const [configSuccess, setConfigSuccess] = useState<string | null>(null);
  const [configError, setConfigError] = useState<string | null>(null);

  useEffect(() => {
    setCurrentSite(initialSite);
    const ws = (initialSite.web_server || 'nginx').toLowerCase();
    const engine = ws.includes('apache') && !ws.includes('nginx') ? 'apache' : 'nginx';
    setWebserverType(engine);
    setConfigContent('');
    setConfigPath('');
  }, [initialSite, serverID]);

  // Load VirtualHost Config when switching to 'vhost' or 'history' tab
  useEffect(() => {
    if (activeTab === 'vhost' && !configContent) {
      loadConfigFile();
    }
    if (activeTab === 'history') {
      loadBackups();
    }
  }, [activeTab]);

  const loadConfigFile = async () => {
    setIsLoadingConfig(true);
    setConfigError(null);
    try {
      const res = await GetSiteConfigFile(currentSite.id);
      if (res) {
        setConfigContent(res.content);
        setOriginalConfigContent(res.content);
        setConfigPath(res.file_path);
        setWebserverType(res.webserver_type);
      }
    } catch (err: unknown) {
      setConfigError(getErrorMessage(err));
    } finally {
      setIsLoadingConfig(false);
    }
  };

  const loadBackups = async () => {
    setIsLoadingBackups(true);
    try {
      const list = await ListSiteConfigBackups(currentSite.id);
      setBackups(list || []);
    } catch (err) {
      console.error('Failed to load site config backups:', err);
    } finally {
      setIsLoadingBackups(false);
    }
  };

  const handleRollback = async (backupID: string) => {
    if (!(await confirm(t('manageSite.rollbackConfirm'), { danger: true }))) return;
    setIsRollingBack(true);
    try {
      await RollbackSiteConfigFile(backupID);
      await loadConfigFile();
      await loadBackups();
      setActiveTab('vhost');
      setConfigSuccess(t('manageSite.rollbackSuccess'));
      onSiteUpdated();
    } catch (err: unknown) {
      alert(`${t('common.error')}: ${getErrorMessage(err)}`);
    } finally {
      setIsRollingBack(false);
    }
  };

  const handleSaveConfig = async () => {
    // Diff check is required for discovered sites before saving
    if (currentSite.origin === 'discovered' && !showDiffModal) {
      setShowDiffModal(true);
      return;
    }
    await executeSaveConfig();
  };

  const executeSaveConfig = async () => {
    setIsSavingConfig(true);
    setConfigSuccess(null);
    setConfigError(null);
    setShowDiffModal(false);
    try {
      await SaveSiteConfigFile(currentSite.id, configContent);
      setOriginalConfigContent(configContent);
      setConfigSuccess(t('manageSite.configSuccess'));
      setTimeout(() => setConfigSuccess(null), 5000);
      onSiteUpdated();
    } catch (err: unknown) {
      setConfigError(getErrorMessage(err));
    } finally {
      setIsSavingConfig(false);
    }
  };

  if (!isOpen) return null;

  return (
    <>
    <ModalShell
      onClose={onClose}
      size="6xl"
      className="max-h-[90vh]"
      icon={<Globe className="w-5 h-5" />}
      title={currentSite.domain}
      headerExtra={
        <span
          className={`px-2 py-0.5 rounded-full text-[10px] font-semibold border ${
            currentSite.status === 'active'
              ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
              : 'bg-rose-500/15 text-rose-400 border-rose-500/30'
          }`}
        >
          {currentSite.status === 'active' ? t('manageSite.active') : t('manageSite.disabled')}
        </span>
      }
      subtitle={
        <>
          {t('manageSite.webServerLabel')}{' '}
          <span className="font-semibold text-slate-200 uppercase">
            {currentSite.web_server || t('manageSite.notSpecified')}
          </span>
          {currentSite.php_version && (
            <>
              {' '}
              {t('manageSite.phpVersionLabel')}{' '}
              <span className="text-purple-300 font-mono font-semibold">{currentSite.php_version}</span>
            </>
          )}
        </>
      }
      headerActions={
        <a
          href={`http://${currentSite.domain}`}
          target="_blank"
          rel="noreferrer"
          className="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition"
          title={t('manageSite.openSiteTooltip')}
        >
          <ExternalLink className="w-4 h-4" />
        </a>
      }
    >

        {/* Tab Navigation */}
        <Tabs
          variant="line"
          size="sm"
          activeTab={activeTab}
          onChange={(id) => setActiveTab(id as TabType)}
          className="px-6 bg-surface-900 shrink-0"
          tabs={[
            {
              id: 'overview',
              label: t('manageSite.tabs.overview'),
              title: t('manageSite.tabs.overview'),
              icon: <Settings className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'app',
              label: t('manageSite.tabs.app'),
              title: t('manageSite.tabs.app'),
              icon: <Layers className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'runtime',
              label: t('manageSite.tabs.runtime') || 'Runtime',
              title: 'PHP & Runtime Environment',
              icon: <Code2 className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'vhost',
              label: t('manageSite.tabs.vhost'),
              title: t('manageSite.tabs.vhost'),
              icon: <FileCode className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'history',
              label: t('manageSite.tabs.history'),
              title: t('manageSite.tabs.history'),
              icon: <History className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'ssl',
              label: t('manageSite.tabs.ssl'),
              title: t('manageSite.tabs.ssl'),
              icon: <Shield className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'logs',
              label: t('manageSite.tabs.logs'),
              title: t('manageSite.tabs.logs'),
              icon: <Terminal className="w-4 h-4 shrink-0" />,
            },
          ]}
        />

        {/* Tab Content Area */}
        <div className="p-6 overflow-y-auto flex-1 space-y-6">
          {/* TAB 1: OVERVIEW & GENERAL SETTINGS */}
          {activeTab === 'overview' && (
            <SiteOverviewTab
              site={currentSite}
              serverID={serverID}
              onSiteChange={setCurrentSite}
              onSiteUpdated={onSiteUpdated}
            />
          )}

          {/* TAB: APPLICATION FRAMEWORK (Laravel, WP, Symfony) */}
          {activeTab === 'app' && (
            <SiteAppTab site={currentSite} />
          )}

          {/* TAB: PHP & RUNTIME */}
          {activeTab === 'runtime' && (
            <SiteRuntimeTab
              site={currentSite}
              serverID={serverID}
              onSiteUpdated={onSiteUpdated}
            />
          )}

          {/* TAB 2: VIRTUALHOST CONFIG EDITOR */}
          {activeTab === 'vhost' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-[11px] text-slate-400 block font-mono">
                    {t('manageSite.pathLabel')} <span className="text-indigo-300 font-semibold">{configPath || t('manageSite.loading')}</span>
                  </span>
                  <p className="text-[11px] text-slate-500 mt-0.5">
                    {t('manageSite.configGuardianNotice', { testCmd: webserverType === 'apache' ? 'apachectl configtest' : 'nginx -t' })}
                  </p>
                </div>

                <div className="flex items-center gap-2">
                  <button
                    onClick={loadConfigFile}
                    disabled={isLoadingConfig}
                    className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
                    title={t('manageSite.refreshConfigTooltip')}
                  >
                    <RefreshCw className={`w-4 h-4 ${isLoadingConfig ? 'animate-spin' : ''}`} />
                  </button>

                  <button
                    onClick={() => setShowDiffModal(true)}
                    disabled={isLoadingConfig || !originalConfigContent}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-750 text-slate-200 rounded-lg text-xs font-semibold transition border border-slate-700 disabled:opacity-50"
                    title={t('manageSite.viewDiffTooltip')}
                  >
                    <GitCompare className="w-3.5 h-3.5 text-indigo-400" />
                    <span>Diff</span>
                  </button>

                  <button
                    onClick={handleSaveConfig}
                    disabled={isSavingConfig || isLoadingConfig}
                    className="flex items-center gap-2 px-4 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg text-xs font-bold shadow-md shadow-indigo-600/30 disabled:opacity-50"
                  >
                    {isSavingConfig ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
                    <span>{t('manageSite.saveAndTest')}</span>
                  </button>
                </div>
              </div>

              {currentSite.origin === 'discovered' && (
                <div className="p-2.5 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 shrink-0 text-amber-400" />
                  <span>
                    {t('manageSite.discoveredNotice')}
                  </span>
                </div>
              )}

              {configSuccess && (
                <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl text-xs text-emerald-300 flex items-center gap-2">
                  <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
                  <span>{configSuccess}</span>
                </div>
              )}

              {configError && (
                <div className="p-3 bg-rose-500/10 border border-rose-500/30 rounded-xl text-xs text-rose-300 flex items-start gap-2">
                  <AlertTriangle className="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
                  <pre className="font-mono whitespace-pre-wrap">{configError}</pre>
                </div>
              )}

              <div className="rounded-xl border border-slate-800 bg-slate-950 p-1">
                <textarea
                  value={configContent}
                  onChange={(e) => setConfigContent(e.target.value)}
                  rows={18}
                  disabled={isLoadingConfig}
                  className="w-full bg-slate-950 text-slate-100 font-mono text-xs p-3 focus:outline-none resize-none selection:bg-indigo-500/30"
                  spellCheck={false}
                />
              </div>
            </div>
          )}

          {/* TAB: CONFIG VERSION HISTORY */}
          {activeTab === 'history' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-white">{t('manageSite.historyTitle')}</h3>
                  <p className="text-[11px] text-slate-400 mt-0.5">
                    {t('manageSite.historyDesc')}
                  </p>
                </div>
                <button
                  onClick={loadBackups}
                  disabled={isLoadingBackups}
                  className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
                  title={t('common.refresh')}
                >
                  <RefreshCw className={`w-4 h-4 ${isLoadingBackups ? 'animate-spin' : ''}`} />
                </button>
              </div>

              {isLoadingBackups ? (
                <div className="py-12 text-center text-slate-400 text-xs flex items-center justify-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin text-indigo-400" />
                  <span>{t('common.loadingShort')}</span>
                </div>
              ) : backups.length === 0 ? (
                <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl text-slate-500 text-xs">
                  {t('common.emptyFilter')}
                </div>
              ) : (
                <div className="space-y-2.5">
                  {backups.map((b) => (
                    <div
                      key={b.id}
                      className="p-3.5 rounded-xl bg-slate-900 border border-slate-800 hover:border-slate-700 flex items-center justify-between gap-4 transition"
                    >
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <History className="w-4 h-4 text-indigo-400 shrink-0" />
                          <span className="font-mono text-xs font-bold text-white">
                            {new Date(b.created_at).toLocaleString()}
                          </span>
                        </div>
                        <p className="text-xs text-slate-400 mt-1 truncate">
                          {b.content_hash ? `Hash: ${b.content_hash.substring(0, 12)}...` : t('manageSite.vhostSnapshot')}
                        </p>
                        <span className="text-[10px] font-mono text-slate-500 block mt-0.5">{b.file_path}</span>
                      </div>

                      <div className="flex items-center gap-2 shrink-0">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setSelectedBackup(b)}
                          icon={<Eye className="w-3.5 h-3.5 text-slate-400" />}
                          title={t('fileScanner.viewBtn')}
                        >
                          {t('fileScanner.viewBtn')}
                        </Button>

                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => handleRollback(b.id)}
                          disabled={isRollingBack}
                          loading={isRollingBack}
                          icon={!isRollingBack ? <RotateCcw className="w-3.5 h-3.5 text-indigo-400" /> : undefined}
                          title={t('manageSite.rollbackBtn')}
                        >
                          {t('manageSite.rollbackBtn')}
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* TAB 3: SSL & HTTPS */}
          {activeTab === 'ssl' && (
            <SiteSSLTab site={currentSite} serverID={serverID} onClose={onClose} onOpenTask={onOpenTask} />
          )}

          {/* TAB 4: LOGS */}
          {activeTab === 'logs' && (
            <SiteLogsTab site={currentSite} />
          )}
        </div>
    </ModalShell>

      {/* Diff Preview Modal before saving vhost */}
      {showDiffModal && (
        <ModalShell
          onClose={() => setShowDiffModal(false)}
          zIndex="z-[60]"
          size="5xl"
          className="max-h-[85vh]"
          icon={<GitCompare className="w-5 h-5 text-indigo-400" />}
          iconClassName=""
          title={t('manageSite.diffPreviewTitle')}
          subtitle={<span className="font-mono">{configPath}</span>}
        >

            {currentSite.origin === 'discovered' && (
              <div className="px-6 py-3 bg-amber-500/10 border-b border-amber-500/20 text-amber-300 text-xs flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 shrink-0 text-amber-400" />
                <span>
                  {t('manageSite.diffDiscoveredWarning')}
                </span>
              </div>
            )}

            <div className="p-4 overflow-y-auto font-mono text-xs flex-1 bg-slate-950 max-h-[55vh] space-y-0.5">
              {computeLineDiff(originalConfigContent, configContent).map((d, idx) => (
                <div
                  key={idx}
                  className={`px-3 py-1 rounded flex items-start gap-3 ${
                    d.type === 'add'
                      ? 'bg-emerald-500/15 text-emerald-300'
                      : d.type === 'del'
                      ? 'bg-rose-500/15 text-rose-300'
                      : 'text-slate-400'
                  }`}
                >
                  <span className="w-4 select-none font-bold">
                    {d.type === 'add' ? '+' : d.type === 'del' ? '-' : ' '}
                  </span>
                  <span className="whitespace-pre-wrap break-all flex-1">{d.line || ' '}</span>
                </div>
              ))}
            </div>

            <div className="p-4 border-t border-slate-800 flex items-center justify-between bg-slate-900/60">
              <Button
                variant="ghost"
                size="md"
                onClick={() => setShowDiffModal(false)}
              >
                {t('manageSite.cancelBackToEdit')}
              </Button>

              <Button
                variant="primary"
                size="md"
                onClick={executeSaveConfig}
                disabled={isSavingConfig}
                loading={isSavingConfig}
                icon={!isSavingConfig ? <Save className="w-4 h-4" /> : undefined}
              >
                {t('manageSite.confirmAndApply')}
              </Button>
            </div>
        </ModalShell>
      )}

      {/* Backup Preview Modal */}
      {selectedBackup && (
        <ModalShell
          onClose={() => setSelectedBackup(null)}
          zIndex="z-[60]"
          size="4xl"
          className="max-h-[80vh]"
          icon={<History className="w-5 h-5 text-indigo-400" />}
          iconClassName=""
          title={t('manageSite.backupFromDate', { date: new Date(selectedBackup.created_at).toLocaleString() })}
          subtitle={selectedBackup.content_hash ? t('manageSite.checksumSha256', { hash: selectedBackup.content_hash }) : t('manageSite.configSnapshot')}
        >

            <div className="p-4 overflow-y-auto bg-slate-950 max-h-[50vh]">
              <pre className="font-mono text-xs text-slate-300 whitespace-pre-wrap">{selectedBackup.content}</pre>
            </div>

            <div className="p-4 border-t border-slate-800 flex items-center justify-between bg-slate-900/60">
              <Button
                variant="ghost"
                size="md"
                onClick={() => setSelectedBackup(null)}
              >
                {t('common.close')}
              </Button>

              <Button
                variant="primary"
                size="md"
                onClick={() => {
                  const id = selectedBackup.id;
                  setSelectedBackup(null);
                  handleRollback(id);
                }}
                icon={<RotateCcw className="w-4 h-4" />}
              >
                {t('manageSite.rollbackToVersion')}
              </Button>
            </div>
        </ModalShell>
      )}
    </>
  );
};
