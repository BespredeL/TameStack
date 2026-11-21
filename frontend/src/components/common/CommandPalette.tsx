import React, { useState, useEffect, useRef, useMemo } from 'react';
import {
  Search,
  Server,
  Globe,
  Database,
  Terminal,
  Shield,
  Activity,
  FolderTree,
  Clock,
  Archive,
  Layers,
  Sparkles,
  Settings,
  Plus,
  RefreshCw,
  ListTodo,
  LayoutDashboard,
  Cpu,
  ArrowRight,
  ShieldAlert,
} from 'lucide-react';
import { NavigationTab, ServerRecord } from '../../types';
import { useTranslation } from '../../i18n/LanguageContext';

export interface CommandPaletteProps {
  isOpen: boolean;
  onClose: () => void;
  onNavigate: (tab: NavigationTab) => void;
  servers: ServerRecord[];
  onOpenAddServer: () => void;
  onRefresh: () => void;
  onOpenTasks: () => void;
  onNavigateToAI: (serverId?: string, prompt?: string) => void;
}

interface CommandItem {
  id: string;
  category: 'nav' | 'server' | 'action';
  title: string;
  subtitle?: string;
  icon: React.ComponentType<{ className?: string }>;
  action: () => void;
}

export const CommandPalette: React.FC<CommandPaletteProps> = ({
  isOpen,
  onClose,
  onNavigate,
  servers,
  onOpenAddServer,
  onRefresh,
  onOpenTasks,
  onNavigateToAI,
}) => {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Build command items
  const items: CommandItem[] = useMemo(() => {
    const list: CommandItem[] = [];

    // 1. Navigation items
    const navTabs: { tab: NavigationTab; icon: any }[] = [
      { tab: 'dashboard', icon: LayoutDashboard },
      { tab: 'servers', icon: Server },
      { tab: 'sites', icon: Globe },
      { tab: 'webservers', icon: Cpu },
      { tab: 'php', icon: Layers },
      { tab: 'databases', icon: Database },
      { tab: 'docker', icon: Layers },
      { tab: 'ssl', icon: Shield },
      { tab: 'filemanager', icon: FolderTree },
      { tab: 'cron', icon: Clock },
      { tab: 'backups', icon: Archive },
      { tab: 'monitoring', icon: Activity },
      { tab: 'security', icon: ShieldAlert },
      { tab: 'terminal', icon: Terminal },
      { tab: 'ai', icon: Sparkles },
      { tab: 'tasks', icon: ListTodo },
      { tab: 'settings', icon: Settings },
    ];

    navTabs.forEach((n) => {
      list.push({
        id: `nav-${n.tab}`,
        category: 'nav',
        title: t(`nav.${n.tab}`),
        subtitle: t(`titles.${n.tab}.subtitle`),
        icon: n.icon,
        action: () => {
          onNavigate(n.tab);
          onClose();
        },
      });
    });

    // 2. Action items
    list.push({
      id: 'action-add-server',
      category: 'action',
      title: t('commandPalette.addServer'),
      subtitle: t('commandPalette.addServerDesc'),
      icon: Plus,
      action: () => {
        onClose();
        onOpenAddServer();
      },
    });

    list.push({
      id: 'action-refresh',
      category: 'action',
      title: t('commandPalette.refreshServers'),
      subtitle: t('commandPalette.refreshServersDesc'),
      icon: RefreshCw,
      action: () => {
        onClose();
        onRefresh();
      },
    });

    list.push({
      id: 'action-open-tasks',
      category: 'action',
      title: t('commandPalette.openTasks'),
      subtitle: t('commandPalette.openTasksDesc'),
      icon: ListTodo,
      action: () => {
        onClose();
        onOpenTasks();
      },
    });

    list.push({
      id: 'action-ai-audit',
      category: 'action',
      title: t('commandPalette.aiAudit'),
      subtitle: t('commandPalette.aiAuditDesc'),
      icon: Sparkles,
      action: () => {
        onClose();
        onNavigateToAI(undefined, t('commandPalette.aiAuditPrompt'));
      },
    });

    // 3. Servers list
    servers.forEach((s) => {
      list.push({
        id: `server-${s.id}`,
        category: 'server',
        title: s.name,
        subtitle: `${s.username}@${s.host}:${s.port} • ${s.group_name || t('commandPalette.defaultGroup')}`,
        icon: Server,
        action: () => {
          onNavigate('servers');
          onClose();
        },
      });
    });

    return list;
  }, [t, servers, onNavigate, onClose, onOpenAddServer, onRefresh, onOpenTasks, onNavigateToAI]);

  // Filter items based on query
  const filteredItems = useMemo(() => {
    if (!query.trim()) {
      return items;
    }
    const q = query.toLowerCase().trim();
    return items.filter(
      (item) =>
        item.title.toLowerCase().includes(q) ||
        (item.subtitle && item.subtitle.toLowerCase().includes(q))
    );
  }, [items, query]);

  // Reset selected index when query changes
  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  // Focus input on open
  useEffect(() => {
    if (isOpen) {
      setQuery('');
      setSelectedIndex(0);
      setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
    }
  }, [isOpen]);

  // Keyboard navigation
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedIndex((prev) => (prev + 1) % (filteredItems.length || 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedIndex((prev) => (prev - 1 + filteredItems.length) % (filteredItems.length || 1));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (filteredItems[selectedIndex]) {
          filteredItems[selectedIndex].action();
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, filteredItems, selectedIndex, onClose]);

  // Scroll active item into view
  useEffect(() => {
    if (listRef.current) {
      const activeElement = listRef.current.children[selectedIndex] as HTMLElement;
      if (activeElement) {
        activeElement.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [selectedIndex]);

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-start justify-center pt-20 p-4"
      onClick={onClose}
    >
      <div
        className="bg-surface-900 border border-slate-700/80 rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden flex flex-col max-h-[75vh] animate-in fade-in zoom-in-95 duration-150"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Search Input Bar */}
        <div className="flex items-center px-4 py-3.5 border-b border-slate-800 bg-surface-950/60 gap-3">
          <Search className="w-5 h-5 text-indigo-400 flex-shrink-0" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t('commandPalette.placeholder')}
            className="flex-1 bg-transparent text-slate-100 placeholder-slate-500 text-sm focus:outline-none"
          />
          <kbd className="hidden sm:inline-block px-2 py-0.5 text-[10px] font-mono font-semibold bg-slate-800 text-slate-400 border border-slate-700 rounded">
            ESC
          </kbd>
        </div>

        {/* Results List */}
        <div
          ref={listRef}
          className="flex-1 overflow-y-auto p-2 space-y-1 divide-y divide-slate-800/40"
        >
          {filteredItems.length === 0 ? (
            <div className="py-12 text-center text-slate-500 text-xs">
              {t('commandPalette.noResults')} "{query}"
            </div>
          ) : (
            filteredItems.map((item, index) => {
              const Icon = item.icon;
              const isSelected = index === selectedIndex;

              return (
                <div
                  key={item.id}
                  onClick={item.action}
                  onMouseEnter={() => setSelectedIndex(index)}
                  className={`flex items-center justify-between px-3.5 py-2.5 rounded-xl cursor-pointer transition text-xs ${
                    isSelected
                      ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-950/50'
                      : 'text-slate-300 hover:bg-slate-800/70 hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className={`p-2 rounded-lg flex-shrink-0 ${
                        isSelected
                          ? 'bg-white/10 text-white'
                          : 'bg-slate-800 text-slate-400 border border-slate-700/60'
                      }`}
                    >
                      <Icon className="w-4 h-4" />
                    </div>

                    <div className="min-w-0">
                      <div className="font-semibold truncate flex items-center gap-2">
                        <span>{item.title}</span>
                        {item.category === 'server' && (
                          <span
                            className={`px-1.5 py-0.2 rounded text-[10px] font-bold ${
                              isSelected
                                ? 'bg-white/20 text-white'
                                : 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                            }`}
                          >
                            {t('commandPalette.badgeServer')}
                          </span>
                        )}
                        {item.category === 'action' && (
                          <span
                            className={`px-1.5 py-0.2 rounded text-[10px] font-bold ${
                              isSelected
                                ? 'bg-white/20 text-white'
                                : 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                            }`}
                          >
                            {t('commandPalette.badgeAction')}
                          </span>
                        )}
                      </div>
                      {item.subtitle && (
                        <div
                          className={`text-[11px] truncate ${
                            isSelected ? 'text-indigo-100' : 'text-slate-500'
                          }`}
                        >
                          {item.subtitle}
                        </div>
                      )}
                    </div>
                  </div>

                  <ArrowRight
                    className={`w-3.5 h-3.5 flex-shrink-0 transition-opacity ${
                      isSelected ? 'opacity-100' : 'opacity-0'
                    }`}
                  />
                </div>
              );
            })
          )}
        </div>

        {/* Footer with Keyboard Hints */}
        <div className="px-4 py-2.5 bg-surface-950/80 border-t border-slate-800/80 flex items-center justify-between text-[11px] text-slate-400">
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1">
              <kbd className="px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700 text-[10px] font-mono">
                ↑↓
              </kbd>{' '}
              {t('commandPalette.navigate')}
            </span>
            <span className="flex items-center gap-1">
              <kbd className="px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700 text-[10px] font-mono">
                ↵
              </kbd>{' '}
              {t('commandPalette.select')}
            </span>
            <span className="flex items-center gap-1">
              <kbd className="px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700 text-[10px] font-mono">
                esc
              </kbd>{' '}
              {t('commandPalette.close')}
            </span>
          </div>
          <span className="text-slate-500 font-mono text-[10px]">{t('commandPalette.quickJumpLabel')}</span>
        </div>
      </div>
    </div>
  );
};
