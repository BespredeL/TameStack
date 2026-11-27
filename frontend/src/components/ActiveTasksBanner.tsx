import React, { useState } from 'react';
import { Loader2, Terminal, ChevronDown, ChevronUp, Layers } from 'lucide-react';
import { TaskRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { usePrivacy } from '../context/PrivacyContext';
import { localizeTaskTitle, localizeTaskStep } from '../utils/taskI18n';

interface ActiveTasksBannerProps {
  tasks: TaskRecord[];
  onOpenTask: (task: TaskRecord) => void;
}

export const ActiveTasksBanner: React.FC<ActiveTasksBannerProps> = ({ tasks, onOpenTask }) => {
  const { t } = useTranslation();
  const { maskText } = usePrivacy();
  const [isMinimized, setIsMinimized] = useState(false);

  if (!tasks || tasks.length === 0) return null;

  const currentTask = tasks[0];
  const count = tasks.length;
  const localizedTitle = maskText(localizeTaskTitle(currentTask, t));
  const localizedStep = maskText(localizeTaskStep(currentTask.current_step, t));

  return (
    <div
      className="fixed bottom-4 right-4 z-40 max-w-sm sm:max-w-md w-full px-3 sm:px-0 transition-all duration-300"
      role="status"
      aria-live="polite"
    >
      <div className="bg-slate-900/95 backdrop-blur-md border border-indigo-500/40 rounded-2xl shadow-2xl p-3.5 space-y-2.5 text-slate-100 ring-1 ring-indigo-500/20">
        {/* Top Header */}
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            <div className="w-6 h-6 rounded-lg bg-indigo-500/20 border border-indigo-500/30 flex items-center justify-center shrink-0">
              <Loader2 className="w-3.5 h-3.5 text-indigo-400 animate-spin" />
            </div>
            <div className="min-w-0">
              <span className="text-xs font-bold text-white tracking-wide truncate block">
                {count > 1 ? t('tasks.banner.runningPlural', { count }) : t('tasks.banner.runningSingle')}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <span className="font-mono text-[11px] font-bold text-indigo-300 bg-indigo-500/20 border border-indigo-500/30 px-2 py-0.5 rounded-full tabular-nums">
              {currentTask.progress}%
            </span>
            <button
              onClick={() => setIsMinimized(!isMinimized)}
              className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
              title={isMinimized ? t('tasks.banner.expandTooltip') : t('tasks.banner.collapseTooltip')}
            >
              {isMinimized ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
            </button>
          </div>
        </div>

        {/* Content (collapsible) */}
        {!isMinimized && (
          <div className="space-y-2 pt-1 border-t border-slate-800/80">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="text-xs font-semibold text-white truncate" title={localizedTitle}>
                  {localizedTitle}
                </div>
                <div className="text-[11px] text-slate-400 font-mono truncate mt-0.5" title={localizedStep}>
                  {localizedStep || t('tasks.banner.executingDefault')}
                </div>
              </div>
            </div>

            {/* Progress Bar */}
            <div className="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-indigo-500 via-cyan-400 to-emerald-400 rounded-full transition-all duration-300 shadow-[0_0_8px_rgba(99,102,241,0.5)]"
                style={{ width: `${Math.max(5, Math.min(100, currentTask.progress))}%` }}
              />
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between text-xs pt-1">
              {count > 1 ? (
                <span className="text-[11px] text-slate-400 flex items-center gap-1">
                  <Layers className="w-3 h-3 text-indigo-400" />
                  <span>{t('tasks.banner.inQueue', { count: count - 1 })}</span>
                </span>
              ) : (
                <span className="text-[10px] text-slate-500 font-mono">{t('tasks.banner.engineTitle')}</span>
              )}

              <button
                onClick={() => onOpenTask(currentTask)}
                className="text-xs font-semibold text-indigo-400 hover:text-indigo-300 flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 border border-indigo-500/30 transition-colors cursor-pointer"
                title={t('tasks.banner.logsTooltip')}
              >
                <Terminal className="w-3 h-3 text-indigo-400" />
                <span>{t('tasks.banner.logsBtn')}</span>
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
