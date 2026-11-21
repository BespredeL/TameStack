import React from 'react';
import { Cpu, ExternalLink } from 'lucide-react';
import { useTranslation } from '../../i18n/LanguageContext';
import { main } from '../../../wailsjs/go/models';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';

interface SystemInfoSectionProps {
  info: main.AppSettingsInfo | null;
}

export const SystemInfoSection: React.FC<SystemInfoSectionProps> = ({ info }) => {
  const { t } = useTranslation();

  return (
    <div id="settings-system" className="scroll-mt-14 bg-surface-850 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
      <div className="flex items-center gap-3">
        <div className="p-2.5 rounded-xl bg-blue-500/10 text-blue-400 border border-blue-500/20">
          <Cpu className="w-5 h-5" />
        </div>
        <div>
          <h3 className="text-sm font-bold text-white">
            {t('settings.systemRuntime')}
          </h3>
          <p className="text-xs text-slate-400">
            {t('settings.systemRuntimeSubtitle')}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs font-mono">
        <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
          <span className="text-slate-500 block text-[11px]">{t('settings.versionLabel')}</span>
          <span className="text-white font-semibold">{info?.version || '—'}</span>
        </div>

        <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
          <span className="text-slate-500 block text-[11px]">{t('settings.platformArchLabel')}</span>
          <span className="text-cyan-400 font-semibold">
            {info ? `${info.operating_system}/${info.architecture}` : t('common.unknown')}
          </span>
        </div>

        <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
          <span className="text-slate-500 block text-[11px]">{t('settings.goRuntimeLabel')}</span>
          <span className="text-indigo-300 font-semibold">{info?.go_version || '—'}</span>
        </div>

        <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
          <span className="text-slate-500 block text-[11px]">{t('settings.taskWorkersLabel')}</span>
          <span className="text-emerald-400 font-semibold">
            {t('settings.taskWorkersValue', { count: info?.active_workers ?? 4 })}
          </span>
        </div>
      </div>

      <div className="pt-3 border-t border-slate-800/80 flex flex-wrap items-center justify-between gap-3 text-xs">
        <div className="flex items-center gap-2 text-slate-400">
          <span className="font-semibold text-slate-300">TameStack</span>
          <span>•</span>
          <span>{t('settings.licenseLabel')}:</span>
          <span className="font-mono text-cyan-400 font-medium">GNU GPL v3.0</span>
        </div>
        <div className="flex items-center gap-2 text-slate-400">
          <span>© 2026 Aleksandr Kireev</span>
          <span>•</span>
          <button
            type="button"
            onClick={() => BrowserOpenURL('https://bespredel.name')}
            className="text-indigo-400 hover:text-indigo-300 transition hover:underline cursor-pointer flex items-center gap-1"
          >
            bespredel.name
            <ExternalLink className="w-3 h-3" />
          </button>
        </div>
      </div>
    </div>
  );
};