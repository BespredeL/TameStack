import React, { Suspense, lazy, useEffect, useState } from 'react';
import { Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { AddServerModal } from './components/AddServerModal';
import { TaskModal } from './components/TaskModal';
import { DashboardView } from './views/DashboardView';
import { ServersView } from './views/ServersView';
const ProvisioningView = lazy(() => import('./views/ProvisioningView').then((m) => ({ default: m.ProvisioningView })));
const TasksView = lazy(() => import('./views/TasksView').then((m) => ({ default: m.TasksView })));
const PHPView = lazy(() => import('./views/PHPView').then((m) => ({ default: m.PHPView })));
const SitesView = lazy(() => import('./views/SitesView').then((m) => ({ default: m.SitesView })));
const WebServersView = lazy(() => import('./views/WebServersView').then((m) => ({ default: m.WebServersView })));
const PackagesView = lazy(() => import('./views/PackagesView').then((m) => ({ default: m.PackagesView })));
const ServicesView = lazy(() => import('./views/ServicesView').then((m) => ({ default: m.ServicesView })));
const SSLView = lazy(() => import('./views/SSLView').then((m) => ({ default: m.SSLView })));
const MailView = lazy(() => import('./views/MailView').then((m) => ({ default: m.MailView })));
const DNSView = lazy(() => import('./views/DNSView').then((m) => ({ default: m.DNSView })));
const DatabasesView = lazy(() => import('./views/DatabasesView').then((m) => ({ default: m.DatabasesView })));
const DockerView = lazy(() => import('./views/DockerView').then((m) => ({ default: m.DockerView })));
const FileManagerView = lazy(() => import('./views/FileManagerView').then((m) => ({ default: m.FileManagerView })));
const CronView = lazy(() => import('./views/CronView').then((m) => ({ default: m.CronView })));
const BackupsView = lazy(() => import('./views/BackupsView').then((m) => ({ default: m.BackupsView })));
const SecurityView = lazy(() => import('./views/SecurityView').then((m) => ({ default: m.SecurityView })));
const UsersView = lazy(() => import('./views/UsersView').then((m) => ({ default: m.UsersView })));
const MonitoringView = lazy(() => import('./views/MonitoringView').then((m) => ({ default: m.MonitoringView })));
const TerminalView = lazy(() => import('./views/TerminalView').then((m) => ({ default: m.TerminalView })));
const AIView = lazy(() => import('./views/AIView').then((m) => ({ default: m.AIView })));
const SettingsView = lazy(() => import('./views/SettingsView').then((m) => ({ default: m.SettingsView })));
import { ErrorBoundary } from './components/common/ErrorBoundary';
import { CommandPalette } from './components/common/CommandPalette';
import { GlobalAICopilot } from './components/ai/GlobalAICopilot';
import { UnlockVaultModal } from './components/UnlockVaultModal';
import { LockScreen } from './components/LockScreen';
import { DbUnlockScreen } from './components/DbUnlockScreen';
import { ActiveTasksBanner } from './components/ActiveTasksBanner';
import { NavigationTab, ServerRecord, TaskRecord } from './types';
import { ServerProvider, useServer } from './context/ServerContext';
import { PrivacyProvider } from './context/PrivacyContext';
import { useAppData } from './hooks/useAppData';
import { useVaultGate } from './hooks/useVaultGate';
import { Server, ShieldAlert } from 'lucide-react';
import { useTranslation } from './i18n/LanguageContext';

const TopProgressBar: React.FC<{ isRefreshing: boolean; runningTasksCount: number }> = ({
  isRefreshing,
  runningTasksCount,
}) => {
  const { isPageLoading } = useServer();
  const show = isPageLoading || isRefreshing || runningTasksCount > 0;
  if (!show) return <div className="h-0.5 w-full bg-transparent shrink-0" />;
  return (
    <div className="h-0.5 w-full bg-slate-900/60 overflow-hidden relative z-30 shrink-0">
      <div className="h-full w-full bg-gradient-to-r from-indigo-500 via-cyan-400 to-emerald-400 animate-progress-indeterminate shadow-[0_0_8px_rgba(99,102,241,0.8)]" />
    </div>
  );
};

export const App: React.FC = () => {
  const { t } = useTranslation();
  const [currentTab, setCurrentTab] = useState<NavigationTab>('dashboard');

  const {
    servers,
    diagnosticsMap,
    stats,
    tasks,
    isRefreshing,
    serversStatus,
    serversError,
    fetchData,
  } = useAppData();

  const {
    vaultStatus,
    isVaultChecking,
    isVaultUnlockOpen,
    setIsVaultUnlockOpen,
    vaultUnlockReason,
    setVaultUnlockReason,
    isDbGate,
    setIsDbGate,
    dbSealError,
    checkVaultStatus,
    handleLockVault,
  } = useVaultGate({ fetchData, t });

  // Modals
  const [isAddServerOpen, setIsAddServerOpen] = useState(false);
  const [serverToEdit, setServerToEdit] = useState<ServerRecord | null>(null);
  const [selectedTask, setSelectedTask] = useState<TaskRecord | null>(null);
  const [aiInitialContext, setAiInitialContext] = useState<{ serverId?: string; prompt?: string } | null>(null);
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState(false);
  const [isAICopilotOpen, setIsAICopilotOpen] = useState(false);
  const [navOpen, setNavOpen] = useState(false);

  // Global hotkeys listener (Ctrl+K, etc.)
  useEffect(() => {
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      // Disallow hotkeys if vault is locked or status is being verified
      if (vaultStatus?.has_master_password && vaultStatus?.is_locked) return;
      if (isVaultChecking) return;
      const isCmdOrCtrl = e.ctrlKey || e.metaKey;
      if (!isCmdOrCtrl) return;

      const target = e.target as HTMLElement | null;
      const isEditable =
        !!target &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.isContentEditable ||
          !!target.closest('.xterm'));

      const key = e.key.toLowerCase();
      if (key === 'k') {
        if (isEditable) return;
        e.preventDefault();
        setIsCommandPaletteOpen((prev) => !prev);
        return;
      }
      if (key === 'i') {
        if (isEditable) return;
        e.preventDefault();
        setIsAICopilotOpen((prev) => !prev);
        return;
      }

      if (isEditable) return;

      if (key === 't') {
        e.preventDefault();
        setCurrentTab('terminal');
      } else if (key === 'd') {
        e.preventDefault();
        setCurrentTab('dashboard');
      } else if (key === 's') {
        e.preventDefault();
        setCurrentTab('servers');
      }
    };

    window.addEventListener('keydown', handleGlobalKeyDown);
    return () => window.removeEventListener('keydown', handleGlobalKeyDown);
  }, []);

  const handleOpenAddServer = () => {
    setServerToEdit(null);
    setIsAddServerOpen(true);
  };

  const handleOpenEditServer = (server: ServerRecord) => {
    setServerToEdit(server);
    setIsAddServerOpen(true);
  };

  const handleCloseServerModal = () => {
    setIsAddServerOpen(false);
    setServerToEdit(null);
  };

  const handleNavigateToAI = (serverId?: string, prompt?: string) => {
    setAiInitialContext({ serverId, prompt });
    setCurrentTab('ai');
  };

  useEffect(() => {
    const mq = window.matchMedia('(min-width: 1024px)');
    const onChange = () => {
      if (mq.matches) setNavOpen(false);
    };
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, []);

  const runningTasksCount = tasks.filter((t) => t.status === 'running').length;
  const onlineCount = servers.filter((s) => s.status === 'online').length;

  const renderActiveView = () => {
    switch (currentTab) {
      case 'dashboard':
        return (
          <DashboardView
            stats={stats}
            servers={servers}
            diagnosticsMap={diagnosticsMap}
            isRefreshing={isRefreshing}
            onRefresh={fetchData}
            onSelectTab={setCurrentTab}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
            onEditServer={handleOpenEditServer}
          />
        );
      case 'servers':
        return (
          <ServersView
            servers={servers}
            diagnosticsMap={diagnosticsMap}
            onRefresh={fetchData}
            onOpenAddServer={handleOpenAddServer}
            onEditServer={handleOpenEditServer}
            onSelectServer={() => setCurrentTab('sites')}
          />
        );
      case 'provisioning':
        return (
          <ProvisioningView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'sites':
        return (
          <SitesView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
            onNavigateToAI={handleNavigateToAI}
          />
        );
      case 'php':
        return (
          <PHPView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'webservers':
        return (
          <WebServersView
            servers={servers}
            onOpenAddServer={handleOpenAddServer}
            onOpenTask={setSelectedTask}
          />
        );
      case 'packages':
        return <PackagesView onOpenAddServer={handleOpenAddServer} />;
      case 'services':
        return <ServicesView onOpenAddServer={handleOpenAddServer} />;
      case 'ssl':
        return (
          <SSLView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'databases':
        return (
          <DatabasesView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
            onNavigateToAI={handleNavigateToAI}
          />
        );
      case 'docker':
        return (
          <DockerView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'filemanager':
        return (
          <FileManagerView
            servers={servers}
            onOpenAddServer={handleOpenAddServer}
            onOpenTask={setSelectedTask}
          />
        );
      case 'cron':
        return (
          <CronView
            servers={servers}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'backups':
        return (
          <BackupsView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'security':
        return (
          <SecurityView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'users':
        return (
          <UsersView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'mail':
        return (
          <MailView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'dns':
        return (
          <DNSView
            servers={servers}
            onOpenTask={setSelectedTask}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'monitoring':
        return (
          <MonitoringView
            servers={servers}
            onOpenAddServer={handleOpenAddServer}
          />
        );
      case 'terminal':
        return (
          <TerminalView
            servers={servers}
            onOpenAddServer={handleOpenAddServer}
            onNavigateToAI={handleNavigateToAI}
          />
        );
      case 'ai':
        return (
          <AIView
            servers={servers}
            initialContext={aiInitialContext}
            onClearInitialContext={() => setAiInitialContext(null)}
          />
        );
      case 'tasks':
        return (
          <TasksView
            tasks={tasks}
            servers={servers}
            onOpenTask={setSelectedTask}
            onRefresh={fetchData}
            isRefreshing={isRefreshing}
          />
        );
      case 'settings':
        return <SettingsView />;
    }
  };

  // 1. Initial vault check splash screen
  if (isVaultChecking) {
    return (
      <div className="h-screen w-screen bg-slate-950 flex flex-col items-center justify-center text-slate-400 select-none">
        <div className="w-12 h-12 rounded-2xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400 mb-4 animate-pulse">
          <Server className="w-6 h-6" />
        </div>
        <div className="text-sm font-medium text-slate-300">TameStack</div>
      </div>
    );
  }

  // 2. Full Blocking Lock Screen when the encrypted database container is sealed
  if (isDbGate) {
    return (
      <DbUnlockScreen
        onUnlocked={async () => {
          setIsDbGate(false);
          await checkVaultStatus();
          fetchData();
        }}
      />
    );
  }

  // 3. Full Blocking Lock Screen when master password is set and vault is locked
  if (vaultStatus?.has_master_password && vaultStatus?.is_locked) {
    return (
      <LockScreen
        onUnlocked={async () => {
          await checkVaultStatus();
          fetchData();
        }}
        reason={vaultUnlockReason}
      />
    );
  }

  return (
    <ServerProvider servers={servers}>
      <PrivacyProvider>
        <div className="flex h-screen w-screen overflow-hidden bg-surface-900 text-slate-100 font-sans">
        {/* Left Sidebar Navigation */}
        <Sidebar
          currentTab={currentTab}
          onSelectTab={setCurrentTab}
          runningTasksCount={runningTasksCount}
          onlineCount={onlineCount}
          totalCount={servers.length}
          mobileOpen={navOpen}
          onCloseMobile={() => setNavOpen(false)}
          onOpenCommandPalette={() => setIsCommandPaletteOpen(true)}
        />

        {/* Main Content Area */}
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          <Header
            currentTab={currentTab}
            onOpenAddServer={handleOpenAddServer}
            onRefresh={fetchData}
            onOpenTasks={() => setCurrentTab('tasks')}
            runningTasksCount={runningTasksCount}
            isRefreshing={isRefreshing}
            onOpenCommandPalette={() => setIsCommandPaletteOpen(true)}
            onOpenAICopilot={() => setIsAICopilotOpen((prev) => !prev)}
            vaultStatus={vaultStatus}
            onOpenUnlockVault={() => {
              setVaultUnlockReason(null);
              setIsVaultUnlockOpen(true);
            }}
            onLockVault={handleLockVault}
            onOpenNav={() => setNavOpen(true)}
          />

          <TopProgressBar
            isRefreshing={isRefreshing}
            runningTasksCount={runningTasksCount}
          />

          {dbSealError && (
            <div className="px-4 py-2 bg-amber-500/10 border-b border-amber-500/30 text-amber-300 text-xs flex items-center gap-2 shrink-0">
              <ShieldAlert className="w-4 h-4 shrink-0" />
              <span>{t('app.dbSealFailed', { error: dbSealError })}</span>
            </div>
          )}

          <main
            className={`flex-1 min-h-0 scroll-pt-16 ${
              currentTab === 'ai' || currentTab === 'terminal'
                ? 'flex flex-col overflow-hidden'
                : 'overflow-y-auto'
            }`}
          >
            <ErrorBoundary key={currentTab} onReset={fetchData}>
              {serversStatus === 'loading' ? (
                <div className="px-4 py-6 sm:px-8 text-sm text-slate-400">{t('servers.loadingList')}</div>
              ) : serversStatus === 'error' ? (
                <div className="px-4 py-6 sm:px-8 space-y-3">
                  <p className="text-sm text-rose-300">{t('servers.loadError', { error: serversError || '' })}</p>
                  <button
                    onClick={fetchData}
                    className="px-3 py-1.5 rounded-lg bg-slate-800 text-xs text-slate-100"
                  >
                    {t('servers.retry')}
                  </button>
                </div>
              ) : (
                <Suspense fallback={<div className="px-4 py-6 sm:px-8 text-sm text-slate-400">{t('common.loading')}</div>}>
                  {renderActiveView()}
                </Suspense>
              )}
            </ErrorBoundary>
          </main>
        </div>

        {/* Global Modals */}
        <AddServerModal
          isOpen={isAddServerOpen}
          onClose={handleCloseServerModal}
          onServerAdded={fetchData}
          serverToEdit={serverToEdit}
        />

        <TaskModal
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
        />

        <CommandPalette
          isOpen={isCommandPaletteOpen}
          onClose={() => setIsCommandPaletteOpen(false)}
          onNavigate={(tab) => setCurrentTab(tab)}
          servers={servers}
          onOpenAddServer={handleOpenAddServer}
          onRefresh={fetchData}
          onOpenTasks={() => setCurrentTab('tasks')}
          onNavigateToAI={handleNavigateToAI}
        />

        <UnlockVaultModal
          isOpen={isVaultUnlockOpen}
          onClose={() => setIsVaultUnlockOpen(false)}
          onUnlocked={() => {
            checkVaultStatus();
            fetchData();
          }}
          reason={vaultUnlockReason}
          isClosable={!vaultStatus?.has_master_password || !vaultStatus?.is_locked}
        />

        {/* Global Active Tasks Banner */}
        <ActiveTasksBanner
          tasks={tasks.filter((t) => t.status === 'running')}
          onOpenTask={setSelectedTask}
        />

        {/* Global Context-Aware AI Copilot Drawer */}
        <GlobalAICopilot
          isOpen={isAICopilotOpen}
          onClose={() => setIsAICopilotOpen(false)}
          currentTab={currentTab}
          onOpenFullAI={() => setCurrentTab('ai')}
        />
        </div>
      </PrivacyProvider>
    </ServerProvider>
  );
};

export default App;