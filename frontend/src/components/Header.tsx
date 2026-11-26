import React from 'react';
import { Plus, RefreshCw, Search, Lock, Unlock, Menu, Loader2, Sparkles } from 'lucide-react';
import { NavigationTab } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { useServer } from '../context/ServerContext';
import { Button } from './ui';
import { main } from '../../wailsjs/go/models';

interface HeaderProps {
  currentTab: NavigationTab;
  onOpenAddServer: () => void;
  onRefresh: () => void;
  onOpenTasks: () => void;
  runningTasksCount: number;
  isRefreshing: boolean;
  onOpenCommandPalette?: () => void;
  onOpenAICopilot?: () => void;
  vaultStatus?: main.VaultStatusInfo | null;
  onOpenUnlockVault?: () => void;
  onLockVault?: () => void;
  onOpenNav?: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  currentTab,
  onOpenAddServer,
  onRefresh,
  onOpenTasks,
  runningTasksCount,
  isRefreshing,
  onOpenCommandPalette,
  onOpenAICopilot,
  vaultStatus,
  onOpenUnlockVault,
  onLockVault,
  onOpenNav,
}) => {
  const { t, language, setLanguage } = useTranslation();
  const { isPageLoading, pageLoadingMessage } = useServer();

  const title = t(`titles.${currentTab}.title`);
  const subtitle = t(`titles.${currentTab}.subtitle`);

  return (
    <header className="min-h-16 px-3 sm:px-6 py-2 bg-surface-950/70 border-b border-slate-800 flex items-center gap-2 sm:gap-3 backdrop-blur-md sticky top-0 z-20">
      {onOpenNav && (
        <button
          type="button"
          onClick={onOpenNav}
          className="lg:hidden p-2 rounded-lg bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 shrink-0"
          aria-label={t('header.openMenu')}
        >
          <Menu className="w-4 h-4" />
        </button>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2.5">
          <h1 className="text-base sm:text-lg font-bold text-white tracking-tight truncate">
            {title}
          </h1>
          {(isPageLoading || isRefreshing) && (
            <div className="hidden sm:inline-flex items-center gap-1.5 text-xs text-indigo-300 bg-indigo-500/15 border border-indigo-500/30 px-2.5 py-0.5 rounded-full animate-pulse shrink-0">
              <RefreshCw className="w-3 h-3 animate-spin text-indigo-400" />
              <span className="truncate max-w-[240px] font-mono text-[11px]">
                {pageLoadingMessage || t('header.syncingSSH')}
              </span>
            </div>
          )}
        </div>
        <p className="text-xs text-slate-400 truncate hidden sm:block">{subtitle}</p>
      </div>

      <div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
        {/* Quick Search / Command Palette (Ctrl+K) */}
        {onOpenCommandPalette && (
          <button
            onClick={onOpenCommandPalette}
            className="h-8 flex items-center gap-2 px-3 rounded-lg bg-surface-900 border border-slate-700/80 text-slate-400 hover:text-slate-200 hover:border-slate-600 active:scale-[0.97] transition-all duration-100 ease-out text-xs shadow-xs cursor-pointer shrink-0"
            title={t('header.commandPaletteTooltip')}
            aria-label={t('header.commandPaletteTooltip')}
          >
            <Search className="w-3.5 h-3.5 text-indigo-400" />
            <span className="hidden lg:inline">{t('header.searchPlaceholder')}</span>
            <kbd className="hidden xl:inline px-1.5 py-0.5 text-[10px] font-mono font-semibold bg-slate-800 text-slate-400 border border-slate-700 rounded">
              Ctrl+K
            </kbd>
          </button>
        )}

        {/* Global AI Copilot (Ctrl+I) */}
        {onOpenAICopilot && (
          <button
            onClick={onOpenAICopilot}
            className="h-8 flex items-center gap-1.5 px-3 rounded-lg bg-indigo-500/15 border border-indigo-500/30 text-indigo-300 hover:text-white hover:bg-indigo-500/25 active:scale-[0.97] transition-all duration-100 ease-out text-xs font-medium shadow-xs cursor-pointer shrink-0"
            title={language === 'en' ? 'Open AI Copilot (Ctrl+I)' : 'Открыть AI-помощника (Ctrl+I)'}
            aria-label={language === 'en' ? 'Open AI Copilot (Ctrl+I)' : 'Открыть AI-помощника (Ctrl+I)'}
          >
            <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
            <span className="hidden sm:inline">AI</span>
            <kbd className="hidden xl:inline px-1.5 py-0.5 text-[10px] font-mono font-semibold bg-indigo-950/80 text-indigo-300 border border-indigo-500/30 rounded">
              Ctrl+I
            </kbd>
          </button>
        )}

        {/* Vault Lock / Unlock Status */}
        {vaultStatus?.has_master_password && (
          vaultStatus.is_locked ? (
            <button
              onClick={onOpenUnlockVault}
              className="h-8 flex items-center gap-1.5 px-3 rounded-lg text-xs font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40 hover:bg-amber-500/30 active:scale-[0.97] transition-all duration-100 ease-out shadow-xs animate-pulse cursor-pointer shrink-0"
              title={t('header.vaultUnlockTooltip')}
              aria-label={t('header.vaultUnlockTooltip')}
            >
              <Lock className="w-3.5 h-3.5 text-amber-400" />
              <span className="hidden lg:inline">{t('header.vaultLocked')}</span>
            </button>
          ) : (
            <button
              onClick={onLockVault}
              className="h-8 flex items-center gap-1.5 px-3 rounded-lg text-xs font-semibold bg-surface-900 text-slate-300 border border-slate-700/80 hover:text-white hover:border-slate-600 active:scale-[0.97] transition-all duration-100 ease-out shadow-xs cursor-pointer shrink-0"
              title={t('header.vaultLockTooltip')}
              aria-label={t('header.vaultLockTooltip')}
            >
              <Unlock className="w-3.5 h-3.5 text-emerald-400" />
              <span className="hidden lg:inline">{t('header.vaultOpen')}</span>
            </button>
          )
        )}

        {/* Language Switcher */}
        <div className="h-8 flex items-center bg-surface-900 border border-slate-700/80 rounded-lg p-0.5 text-xs font-semibold shrink-0">
          <button
            onClick={() => setLanguage('ru')}
            className={`h-full px-2.5 rounded-md flex items-center justify-center transition-all ${
              language === 'ru'
                ? 'bg-emerald-600 text-white shadow-xs'
                : 'text-slate-400 hover:text-slate-200'
            }`}
            title={t('header.ruTooltip')}
            aria-label={t('header.ruAria')}
          >
            RU
          </button>
          <button
            onClick={() => setLanguage('en')}
            className={`h-full px-2.5 rounded-md flex items-center justify-center transition-all ${
              language === 'en'
                ? 'bg-emerald-600 text-white shadow-xs'
                : 'text-slate-400 hover:text-slate-200'
            }`}
            title={t('header.enTooltip')}
            aria-label={t('header.enAria')}
          >
            EN
          </button>
        </div>

        {/* Running tasks indicator */}
        {runningTasksCount > 0 && (
          <button
            onClick={onOpenTasks}
            className="h-8 flex items-center gap-2 px-3 rounded-lg bg-amber-500/20 border border-amber-500/30 text-amber-300 text-xs font-medium hover:bg-amber-500/30 active:scale-[0.97] transition-all duration-100 ease-out shadow-xs shrink-0 cursor-pointer"
            aria-label={t('header.activeTasks')}
          >
            <Loader2 className="w-3.5 h-3.5 animate-spin text-amber-400" />
            <span className="tabular-nums font-bold">{runningTasksCount}</span>
            <span className="hidden lg:inline">{t('header.activeTasks')}</span>
          </button>
        )}

        {/* Refresh button */}
        <Button
          size="sm"
          variant="secondary"
          onClick={onRefresh}
          disabled={isRefreshing}
          className="h-8 w-8 !p-0"
          title={t('header.refresh')}
          aria-label={t('header.refresh')}
          icon={<RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin text-emerald-400' : ''}`} />}
        />

        {/* Primary Action Button */}
        <Button
          size="sm"
          variant="success"
          onClick={onOpenAddServer}
          title={t('header.addServer')}
          aria-label={t('header.addServer')}
          icon={<Plus className="w-4 h-4" />}
          className="h-8"
        >
          <span className="hidden md:inline">{t('header.addServer')}</span>
        </Button>
      </div>
    </header>
  );
};
