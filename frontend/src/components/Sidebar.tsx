import React, { useEffect, useState } from 'react';
import {
  LayoutDashboard,
  Server,
  Globe,
  Code2,
  HardDrive,
  Package,
  Cpu,
  Database,
  Container,
  ShieldCheck,
  FolderTree,
  Clock,
  Archive,
  Activity,
  Lock,
  Terminal,
  Sparkles,
  ListTodo,
  Settings,
  X,
  Wrench,
} from 'lucide-react';
import { NavigationTab } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { useServer } from '../context/ServerContext';
import { Select } from './ui';
import { GetAppSettings } from '../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import logoIcon from '../assets/images/logo.png';

interface SidebarProps {
  currentTab: NavigationTab;
  onSelectTab: (tab: NavigationTab) => void;
  runningTasksCount: number;
  onlineCount: number;
  totalCount: number;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

interface NavItem {
  id: NavigationTab;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  badge?: number | string;
  isReady?: boolean;
}

export const Sidebar: React.FC<SidebarProps> = ({
  currentTab,
  onSelectTab,
  runningTasksCount,
  onlineCount,
  totalCount,
  mobileOpen,
  onCloseMobile,
}) => {
  const { t } = useTranslation();
  const { selectedServerId, setSelectedServerId, selectedServer, servers } = useServer();
  const [appVersion, setAppVersion] = useState<string | null>(null);

  useEffect(() => {
    GetAppSettings()
      .then((settings) => setAppVersion(settings.version))
      .catch(() => setAppVersion(null));
  }, []);

  useEffect(() => {
    if (!mobileOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCloseMobile();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [mobileOpen, onCloseMobile]);

  const navItems: NavItem[] = [
    { id: 'dashboard', label: t('nav.dashboard'), icon: LayoutDashboard, isReady: true },
    { id: 'servers', label: t('nav.servers'), icon: Server, badge: totalCount, isReady: true },
    { id: 'provisioning', label: t('nav.provisioning') || 'Server Setup', icon: Wrench, isReady: true },
    { id: 'sites', label: t('nav.sites'), icon: Globe, isReady: true },
    { id: 'webservers', label: t('nav.webservers'), icon: HardDrive, isReady: true },
    { id: 'php', label: t('nav.php'), icon: Code2, isReady: true },
    { id: 'packages', label: t('nav.packages'), icon: Package, isReady: true },
    { id: 'services', label: t('nav.services'), icon: Cpu, isReady: true },
    { id: 'databases', label: t('nav.databases'), icon: Database, isReady: true },
    { id: 'docker', label: t('nav.docker'), icon: Container, isReady: true },
    { id: 'ssl', label: t('nav.ssl'), icon: ShieldCheck, isReady: true },
    { id: 'filemanager', label: t('nav.filemanager'), icon: FolderTree, isReady: true },
    { id: 'cron', label: t('nav.cron'), icon: Clock, isReady: true },
    { id: 'backups', label: t('nav.backups'), icon: Archive, isReady: true },
    { id: 'monitoring', label: t('nav.monitoring'), icon: Activity, isReady: true },
    { id: 'security', label: t('nav.security'), icon: Lock, isReady: true },
    { id: 'terminal', label: t('nav.terminal'), icon: Terminal, isReady: true },
    { id: 'ai', label: t('nav.ai'), icon: Sparkles, isReady: true },
    { id: 'tasks', label: t('nav.tasks'), icon: ListTodo, badge: runningTasksCount > 0 ? runningTasksCount : undefined, isReady: true },
    { id: 'settings', label: t('nav.settings'), icon: Settings, isReady: true },
  ];

  return (
    <>
      {mobileOpen && (
        <button
          type="button"
          aria-label={t('nav.closeMenu')}
          className="fixed inset-0 z-30 bg-black/60 lg:hidden"
          onClick={onCloseMobile}
        />
      )}
      <aside
        className={`w-64 bg-surface-950 border-r border-slate-800 flex flex-col h-screen select-none shrink-0 fixed inset-y-0 left-0 z-40 transition-transform duration-200 ease-out lg:static lg:translate-x-0 ${mobileOpen ? 'translate-x-0' : '-translate-x-full'
          }`}
      >
        {/* Brand Header */}
        <div className="h-16 px-5 flex items-center justify-between border-b border-slate-800/80 bg-slate-950/40 gap-2">
          <div className="flex items-center gap-3 min-w-0">
            <img
              src={logoIcon}
              alt="TameStack"
              className="w-9 h-9 rounded-lg shadow-lg shadow-indigo-500/20 shrink-0"
            />
            <div className="min-w-0">
              <div className="text-base font-bold text-white tracking-wide flex items-center gap-2">
                <span className="truncate">TameStack</span>
                {appVersion && (
                  <span className="text-[10px] font-mono font-semibold leading-none bg-indigo-500/15 text-indigo-300 px-1.5 py-0.5 rounded border border-indigo-500/30 shrink-0">
                    v{appVersion}
                  </span>
                )}
              </div>
              <div className="text-[11px] text-slate-400 truncate">{t('nav.tagline')}</div>
            </div>
          </div>
          <button
            type="button"
            onClick={onCloseMobile}
            className="lg:hidden p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 shrink-0"
            aria-label={t('nav.closeMenu')}
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Target Server Selector directly under logo */}
        {servers.length > 0 && (
          <div className="px-3.5 py-3 border-b border-slate-800/80 bg-slate-900/40">
            <div className="flex items-center justify-between gap-1 mb-1.5 px-0.5">
              <span className="text-[10px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                <Server className="w-3 h-3 text-emerald-400" />
                {t('nav.targetServer')}
              </span>
              {selectedServer && (
                <span
                  className={`w-2 h-2 rounded-full shrink-0 ${selectedServer.status === 'online'
                    ? 'bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.7)]'
                    : selectedServer.status === 'degraded'
                      ? 'bg-amber-400 shadow-[0_0_6px_rgba(251,191,36,0.7)]'
                      : selectedServer.status === 'offline'
                        ? 'bg-rose-500'
                        : 'bg-slate-500'
                    }`}
                  title={selectedServer.status}
                />
              )}
            </div>

            <Select
              sizeVariant="sm"
              value={selectedServerId}
              onChange={(e) => setSelectedServerId(e.target.value)}
              title={selectedServer ? `${selectedServer.name} (${selectedServer.host})` : ''}
              className="font-medium"
            >
              {servers.map((s) => (
                <option key={s.id} value={s.id} className="bg-slate-900 text-slate-200">
                  {s.name} ({s.host})
                </option>
              ))}
            </Select>
          </div>
        )}

        {/* Navigation List */}
        <nav className="flex-1 overflow-y-auto px-3 py-4 space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = currentTab === item.id;
            return (
              <button
                key={item.id}
                onClick={() => {
                  onSelectTab(item.id);
                  onCloseMobile();
                }}
                className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium transition-all ${isActive
                  ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/30 font-semibold'
                  : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60'
                  }`}
              >
                <div className="flex items-center gap-3">
                  <Icon className={`w-4 h-4 ${isActive ? 'text-white' : 'text-slate-400'}`} />
                  <span>{item.label}</span>
                </div>

                <div className="flex items-center gap-1.5">
                  {!item.isReady && (
                    <span className="text-[9px] px-1 py-0.2 rounded bg-slate-800 text-slate-500 font-mono">
                      stage
                    </span>
                  )}
                  {item.badge !== undefined && (
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full font-bold ${isActive
                        ? 'bg-indigo-800 text-white'
                        : item.id === 'tasks' && runningTasksCount > 0
                          ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30 animate-pulse'
                          : 'bg-slate-800 text-slate-300'
                        }`}
                    >
                      {item.badge}
                    </span>
                  )}
                </div>
              </button>
            );
          })}
        </nav>

        {/* Copyright Widget */}
        <div className="p-3 border-t border-slate-800/80 bg-slate-900/40 text-center">
          <div className="text-xs font-medium text-slate-300">
            © 2026 Aleksandr Kireev
          </div>
          <button
            type="button"
            onClick={() => BrowserOpenURL('https://bespredel.name')}
            className="text-[11px] text-slate-500 hover:text-indigo-400 transition cursor-pointer mt-0.5 inline-block"
            title="https://bespredel.name"
          >
            https://bespredel.name
          </button>
        </div>
      </aside>
    </>
  );
};

