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
  Users,
  Mail,
  Globe2,
  Search,
  ChevronDown,
  ChevronRight,
  PanelLeftClose,
  PanelLeftOpen,
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
  onOpenCommandPalette?: () => void;
}

interface NavItem {
  id: NavigationTab;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  badge?: number | string;
}

interface NavGroup {
  id: string;
  label: string;
  items: NavItem[];
}

export const Sidebar: React.FC<SidebarProps> = ({
  currentTab,
  onSelectTab,
  runningTasksCount,
  totalCount,
  mobileOpen,
  onCloseMobile,
  onOpenCommandPalette,
}) => {
  const { t } = useTranslation();
  const { selectedServerId, setSelectedServerId, selectedServer, servers } = useServer();
  const [appVersion, setAppVersion] = useState<string | null>(null);
  const [isCompact, setIsCompact] = useState<boolean>(() => {
    try {
      return localStorage.getItem('tamestack_sidebar_compact') === 'true';
    } catch {
      return false;
    }
  });
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>(() => {
    try {
      const saved = localStorage.getItem('tamestack_sidebar_groups');
      if (saved) return JSON.parse(saved);
    } catch {
      // ignore storage errors
    }
    return {
      overview: true,
      web: true,
      data: true,
      system: false,
      security: true,
      tools: true,
    };
  });

  const toggleCompact = () => {
    setIsCompact((prev) => {
      const next = !prev;
      try {
        localStorage.setItem('tamestack_sidebar_compact', String(next));
      } catch {
        // ignore
      }
      return next;
    });
  };

  const toggleGroup = (groupId: string) => {
    setOpenGroups((prev) => {
      const next = { ...prev, [groupId]: !prev[groupId] };
      try {
        localStorage.setItem('tamestack_sidebar_groups', JSON.stringify(next));
      } catch {
        // ignore
      }
      return next;
    });
  };

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

  const navGroups: NavGroup[] = [
    {
      id: 'overview',
      label: t('nav.groups.overview'),
      items: [
        { id: 'dashboard', label: t('nav.dashboard'), icon: LayoutDashboard },
        { id: 'servers', label: t('nav.servers'), icon: Server, badge: totalCount },
        { id: 'monitoring', label: t('nav.monitoring'), icon: Activity },
        { id: 'tasks', label: t('nav.tasks'), icon: ListTodo, badge: runningTasksCount > 0 ? runningTasksCount : undefined },
      ],
    },
    {
      id: 'web',
      label: t('nav.groups.webApps'),
      items: [
        { id: 'sites', label: t('nav.sites'), icon: Globe },
        { id: 'webservers', label: t('nav.webservers'), icon: HardDrive },
        { id: 'php', label: t('nav.php'), icon: Code2 },
        { id: 'ssl', label: t('nav.ssl'), icon: ShieldCheck },
        { id: 'docker', label: t('nav.docker'), icon: Container },
      ],
    },
    {
      id: 'data',
      label: t('nav.groups.dataNetwork'),
      items: [
        { id: 'databases', label: t('nav.databases'), icon: Database },
        { id: 'mail', label: t('nav.mail'), icon: Mail },
        { id: 'dns', label: t('nav.dns'), icon: Globe2 },
      ],
    },
    {
      id: 'system',
      label: t('nav.groups.systemFiles'),
      items: [
        { id: 'filemanager', label: t('nav.filemanager'), icon: FolderTree },
        { id: 'cron', label: t('nav.cron'), icon: Clock },
        { id: 'backups', label: t('nav.backups'), icon: Archive },
        { id: 'services', label: t('nav.services'), icon: Cpu },
        { id: 'packages', label: t('nav.packages'), icon: Package },
        { id: 'provisioning', label: t('nav.provisioning'), icon: Wrench },
      ],
    },
    {
      id: 'security',
      label: t('nav.groups.securityAccess'),
      items: [
        { id: 'security', label: t('nav.security'), icon: Lock },
        { id: 'users', label: t('nav.users'), icon: Users },
      ],
    },
    {
      id: 'tools',
      label: t('nav.groups.tools'),
      items: [
        { id: 'terminal', label: t('nav.terminal'), icon: Terminal },
        { id: 'ai', label: t('nav.ai'), icon: Sparkles },
        { id: 'settings', label: t('nav.settings'), icon: Settings },
      ],
    },
  ];

  // Auto-expand the group containing the active tab.
  useEffect(() => {
    const activeGroup = navGroups.find((g) => g.items.some((item) => item.id === currentTab));
    if (activeGroup && !openGroups[activeGroup.id]) {
      setOpenGroups((prev) => ({ ...prev, [activeGroup.id]: true }));
    }
  }, [currentTab]);

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
        data-no-tooltip="true"
        className={`${isCompact ? 'w-64 lg:w-[72px]' : 'w-64'} bg-surface-950 border-r border-slate-800 flex flex-col h-screen select-none shrink-0 fixed inset-y-0 left-0 z-40 transition-all duration-200 ease-out lg:static lg:translate-x-0 ${mobileOpen ? 'translate-x-0' : '-translate-x-full'
          }`}
      >
        {/* Brand Header */}
        <div className={`h-16 ${isCompact ? 'px-5 lg:px-0 lg:justify-center' : 'px-5'} flex items-center justify-between border-b border-slate-800/80 bg-slate-950/40 gap-2`}>
          <div className={`flex items-center gap-3 min-w-0 ${isCompact ? 'lg:justify-center' : ''}`}>
            <img
              src={logoIcon}
              alt="TameStack"
              className="w-9 h-9 min-w-[36px] min-h-[36px] object-contain rounded-lg shadow-lg shadow-indigo-500/20 flex-none"
              title="TameStack"
            />
            <div className={`min-w-0 ${isCompact ? 'lg:hidden' : ''}`}>
              <div className="text-base font-bold text-white tracking-wide flex items-center gap-2">
                <span>TameStack</span>
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
            title={t('nav.closeMenu')}
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Target Server Selector directly under logo */}
        {servers.length > 0 && (
          <div className={`px-3.5 py-3 border-b border-slate-800/80 bg-slate-900/40 ${isCompact ? 'lg:hidden' : ''}`}>
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

        {/* Quick Search (Ctrl+K) */}
        {onOpenCommandPalette && (
          <div className={`px-3 pt-3 pb-1 ${isCompact ? 'lg:px-2' : ''}`}>
            <button
              type="button"
              onClick={onOpenCommandPalette}
              title={t('nav.quickJumpTooltip')}
              aria-label={t('nav.quickJumpTooltip')}
              className={`w-full flex items-center ${isCompact ? 'lg:justify-center lg:px-2 justify-between px-3' : 'justify-between px-3'} py-1.5 rounded-lg bg-slate-900/90 hover:bg-slate-800 border border-slate-800 text-xs text-slate-400 hover:text-slate-200 transition`}
            >
              <span className="flex items-center gap-2">
                <Search className="w-3.5 h-3.5 text-slate-400 shrink-0" />
                <span className={isCompact ? 'lg:hidden' : ''}>{t('nav.quickJumpPlaceholder')}</span>
              </span>
              <kbd className={`text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-400 ${isCompact ? 'lg:hidden' : ''}`}>
                Ctrl+K
              </kbd>
            </button>
          </div>
        )}

        {/* Navigation List (Grouped & Collapsible) */}
        <nav className={`flex-1 overflow-y-auto ${isCompact ? 'lg:px-2 px-3' : 'px-3'} py-3 space-y-3`}>
          {navGroups.map((group) => {
            const isOpen = !!openGroups[group.id];
            return (
              <div key={group.id} className="space-y-1">
                {!isCompact && (
                  <button
                    type="button"
                    onClick={() => toggleGroup(group.id)}
                    className="w-full flex items-center justify-between px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-slate-500 hover:text-slate-300 transition"
                    title={isOpen ? t('nav.collapseGroup') : t('nav.expandGroup')}
                  >
                    <span>{group.label}</span>
                    {isOpen ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                  </button>
                )}

                {(isOpen || isCompact) && (
                  <div className="space-y-0.5">
                    {group.items.map((item) => {
                      const Icon = item.icon;
                      const isActive = currentTab === item.id;
                      return (
                        <button
                          key={item.id}
                          onClick={() => {
                            onSelectTab(item.id);
                            onCloseMobile();
                          }}
                          title={item.label}
                          aria-label={item.label}
                          className={`relative w-full flex items-center ${isCompact ? 'lg:justify-center lg:px-2 justify-between px-3' : 'justify-between px-3'} py-2 rounded-lg text-sm font-medium transition-all ${isActive
                            ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/30 font-semibold'
                            : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60'
                            }`}
                        >
                          <div className="flex items-center gap-3 min-w-0">
                            <Icon className={`w-4 h-4 shrink-0 ${isActive ? 'text-white' : 'text-slate-400'}`} />
                            <span className={`truncate ${isCompact ? 'lg:hidden' : ''}`}>{item.label}</span>
                          </div>

                          {item.badge !== undefined && (
                            <span
                              className={`${isCompact ? 'lg:hidden' : ''} text-xs px-2 py-0.5 rounded-full font-bold ${isActive
                                ? 'bg-indigo-800 text-white'
                                : item.id === 'tasks' && runningTasksCount > 0
                                  ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30 animate-pulse'
                                  : 'bg-slate-800 text-slate-300'
                                }`}
                            >
                              {item.badge}
                            </span>
                          )}
                          {isCompact && item.id === 'tasks' && runningTasksCount > 0 && (
                            <span className="hidden lg:block absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-amber-400 animate-pulse" />
                          )}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </nav>

        {/* Copyright & Compact Toggle Footer */}
        <div className={`p-3 border-t border-slate-800/80 bg-slate-900/40 flex items-center ${isCompact ? 'lg:justify-center justify-between' : 'justify-between'} gap-2`}>
          <div className={`text-left min-w-0 ${isCompact ? 'lg:hidden' : ''}`}>
            <div className="text-xs font-medium text-slate-300 truncate">
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
          <button
            type="button"
            onClick={toggleCompact}
            className="hidden lg:inline-flex p-2 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800/80 border border-slate-800/80 transition shrink-0"
            title={isCompact ? t('nav.expandSidebar') : t('nav.compactSidebar')}
            aria-label={isCompact ? t('nav.expandSidebar') : t('nav.compactSidebar')}
          >
            {isCompact ? <PanelLeftOpen className="w-4 h-4" /> : <PanelLeftClose className="w-4 h-4" />}
          </button>
        </div>
      </aside>
    </>
  );
};

