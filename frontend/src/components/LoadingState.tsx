import React from 'react';
import { useTranslation } from '../i18n/LanguageContext';
import { Loader2, Radio } from 'lucide-react';

interface LoadingStateProps {
  title?: string;
  subtitle?: string;
  type?: 'cards' | 'table' | 'stats' | 'explorer' | 'details';
  count?: number;
}

export const LoadingState: React.FC<LoadingStateProps> = ({
  title,
  subtitle,
  type = 'cards',
  count = 4,
}) => {
  const { t } = useTranslation();
  const resolvedTitle = title || t('common.loading.defaultTitle');
  const resolvedSubtitle = subtitle || t('common.loading.defaultSubtitle');
  const items = Array.from({ length: count });

  return (
    <div className="space-y-6 w-full animate-fadeIn" role="status" aria-busy="true">
      {/* Live Loading Header Banner */}
      <div className="p-4 rounded-2xl bg-surface-850/90 border border-indigo-500/30 flex items-center justify-between gap-4 shadow-lg backdrop-blur-sm relative overflow-hidden">
        {/* Glowing background accent */}
        <div className="absolute inset-0 bg-gradient-to-r from-indigo-500/10 via-cyan-500/5 to-transparent pointer-events-none" />

        <div className="flex items-center gap-3.5 min-w-0 relative z-10">
          <div className="relative flex items-center justify-center shrink-0">
            <div className="absolute w-8 h-8 rounded-full bg-indigo-500/20 animate-ping" />
            <div className="w-9 h-9 rounded-xl bg-indigo-600/20 border border-indigo-500/40 flex items-center justify-center text-indigo-400">
              <Loader2 className="w-5 h-5 animate-spin" />
            </div>
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-bold text-white tracking-tight truncate">{resolvedTitle}</h3>
              <span className="hidden sm:inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                <Radio className="w-2.5 h-2.5 text-indigo-400 animate-pulse" />
                {t('common.loading.sshActive')}
              </span>
            </div>
            <p className="text-xs text-slate-400 font-mono truncate mt-0.5">{resolvedSubtitle}</p>
          </div>
        </div>

        <div className="flex items-center gap-2 text-xs font-mono text-indigo-300 shrink-0 relative z-10">
          <span className="hidden md:inline text-[11px] text-slate-400">{t('common.loading.syncing')}</span>
          <div className="w-2 h-2 rounded-full bg-indigo-400 animate-pulse" />
        </div>
      </div>

      {/* SKELETON LAYOUTS */}
      {type === 'cards' && (
        <div className="card-grid">
          {items.map((_, i) => (
            <div
              key={i}
              className="bg-surface-850 border border-slate-800 rounded-2xl p-5 space-y-4 animate-shimmer shadow-md"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-center gap-3">
                  <div className="w-11 h-11 rounded-xl bg-slate-800/80 shrink-0" />
                  <div className="space-y-1.5">
                    <div className="h-4 w-32 bg-slate-800 rounded-md" />
                    <div className="h-3 w-20 bg-slate-800/60 rounded" />
                  </div>
                </div>
                <div className="h-5 w-16 bg-slate-800/80 rounded-full" />
              </div>

              <div className="space-y-2 pt-2 border-t border-slate-800/60">
                <div className="h-3.5 w-full bg-slate-800/60 rounded" />
                <div className="h-3.5 w-4/5 bg-slate-800/50 rounded" />
              </div>

              <div className="flex items-center justify-between pt-2">
                <div className="h-8 w-20 bg-slate-800 rounded-lg" />
                <div className="h-8 w-24 bg-slate-800/80 rounded-lg" />
              </div>
            </div>
          ))}
        </div>
      )}

      {type === 'table' && (
        <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm animate-shimmer">
          <div className="px-6 py-4 border-b border-slate-800 bg-slate-900/60 flex items-center justify-between">
            <div className="h-4 w-36 bg-slate-800 rounded" />
            <div className="h-8 w-28 bg-slate-800 rounded-lg" />
          </div>
          <div className="divide-y divide-slate-800/60">
            {items.map((_, i) => (
              <div key={i} className="px-6 py-4 flex items-center justify-between gap-4">
                <div className="flex items-center gap-3 flex-1">
                  <div className="w-8 h-8 rounded-lg bg-slate-800 shrink-0" />
                  <div className="space-y-1.5 flex-1 max-w-sm">
                    <div className="h-4 w-3/4 bg-slate-800 rounded" />
                    <div className="h-3 w-1/2 bg-slate-800/60 rounded" />
                  </div>
                </div>
                <div className="h-5 w-24 bg-slate-800/70 rounded-full hidden sm:block" />
                <div className="h-5 w-16 bg-slate-800/70 rounded hidden md:block" />
                <div className="h-8 w-20 bg-slate-800 rounded-lg shrink-0" />
              </div>
            ))}
          </div>
        </div>
      )}

      {type === 'stats' && (
        <div className="space-y-6">
          <div className="stat-grid">
            {items.map((_, i) => (
              <div
                key={i}
                className="bg-surface-850 border border-slate-800 p-5 rounded-2xl space-y-3 animate-shimmer shadow-md"
              >
                <div className="flex items-center justify-between">
                  <div className="h-3.5 w-24 bg-slate-800 rounded" />
                  <div className="w-8 h-8 rounded-lg bg-slate-800" />
                </div>
                <div className="h-7 w-20 bg-slate-800 rounded-lg" />
                <div className="h-2 w-full bg-slate-800/60 rounded-full" />
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
