import React, { useCallback, useEffect, useState } from 'react';
import { Download, ExternalLink, RefreshCw, Rocket, ShieldAlert } from 'lucide-react';
import { useTranslation } from '../../i18n/LanguageContext';
import { update } from '../../../wailsjs/go/models';
import { CheckForUpdates, InstallUpdate } from '../../../wailsjs/go/main/App';
import { BrowserOpenURL, EventsOn } from '../../../wailsjs/runtime/runtime';
import { useConfirm } from '../../context/ConfirmContext';
import { getErrorMessage } from '../../utils/errors';

interface UpdateProgress {
  phase: string;
  downloaded: number;
  total: number;
}

export const UpdateSection: React.FC = () => {
  const { t } = useTranslation();
  const confirm = useConfirm();

  const [checking, setChecking] = useState(false);
  const [result, setResult] = useState<update.CheckResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [installing, setInstalling] = useState(false);
  const [progress, setProgress] = useState<UpdateProgress | null>(null);
  const [ready, setReady] = useState(false);

  const runCheck = useCallback(async () => {
    setChecking(true);
    setError(null);
    try {
      const res = await CheckForUpdates();
      setResult(res);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setChecking(false);
    }
  }, []);

  useEffect(() => {
    const onProgress = (p: UpdateProgress) => setProgress(p);
    const onError = (p: { message?: string }) => {
      setInstalling(false);
      setProgress(null);
      setError(p?.message ?? t('settings.updateErrorUnknown'));
    };
    const onReady = () => {
      setInstalling(false);
      setProgress(null);
      setReady(true);
    };
    const unProgress = EventsOn('update:progress', onProgress);
    const unError = EventsOn('update:error', onError);
    const unReady = EventsOn('update:ready', onReady);
    return () => {
      if (typeof unProgress === 'function') unProgress();
      if (typeof unError === 'function') unError();
      if (typeof unReady === 'function') unReady();
    };
  }, [t]);

  // Silent check on first mount so the section reports the state without a click.
  useEffect(() => {
    runCheck();
  }, [runCheck]);

  const handleInstall = async () => {
    if (!result) return;
    const version = result.latest_version;
    const ok = await confirm(t('settings.updateInstallConfirm', { version }), {
      title: t('settings.updateInstallTitle'),
      confirmLabel: t('settings.updateInstallButton', { version }),
    });
    if (!ok) return;
    setInstalling(true);
    setError(null);
    try {
      await InstallUpdate();
      // Resolves just before the shutdown sequence starts; update:ready follows.
    } catch (err: unknown) {
      setInstalling(false);
      setProgress(null);
      setError(getErrorMessage(err));
    }
  };

  const phaseLabel = (phase: string) => {
    switch (phase) {
      case 'downloading':
        return t('settings.updatePhaseDownloading');
      case 'verifying':
        return t('settings.updatePhaseVerifying');
      case 'installing':
        return t('settings.updatePhaseInstalling');
      default:
        return phase;
    }
  };

  const percent =
    progress && progress.total > 0
      ? Math.min(100, Math.floor((progress.downloaded / progress.total) * 100))
      : null;

  return (
    <div
      id="settings-update"
      className="scroll-mt-14 bg-surface-850 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4"
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <Download className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-sm font-bold text-white">{t('settings.updateTitle')}</h3>
            <p className="text-xs text-slate-400">{t('settings.updateSubtitle')}</p>
          </div>
        </div>
        <button
          onClick={runCheck}
          disabled={checking || installing || ready}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold bg-slate-800 text-slate-200 hover:bg-slate-700 border border-slate-700/60 transition disabled:opacity-50"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${checking ? 'animate-spin' : ''}`} />
          {checking ? t('settings.updateChecking') : t('settings.updateCheckButton')}
        </button>
      </div>

      {error && (
        <div className="flex items-start justify-between gap-3 p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
          <div className="flex items-start gap-2 min-w-0">
            <ShieldAlert className="w-4 h-4 shrink-0 mt-0.5" />
            <span className="min-w-0 break-words">{t('settings.updateError', { error })}</span>
          </div>
          <button
            onClick={() => BrowserOpenURL('https://github.com/BespredeL/TameStack/releases')}
            className="flex items-center gap-1.5 px-2.5 py-1 shrink-0 rounded-md bg-rose-500/20 hover:bg-rose-500/30 text-rose-200 transition text-[11px]"
          >
            <ExternalLink className="w-3 h-3" />
            GitHub Releases
          </button>
        </div>
      )}

      {ready && (
        <div className="flex items-center gap-2 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs">
          <Rocket className="w-4 h-4 shrink-0" />
          <span>{t('settings.updateRestarting')}</span>
        </div>
      )}

      {installing && progress && (
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-slate-300">
            <span className="flex items-center gap-2">
              <RefreshCw className="w-3.5 h-3.5 animate-spin text-indigo-400" />
              {phaseLabel(progress.phase)}
            </span>
            {percent !== null && <span className="font-mono">{percent}%</span>}
          </div>
          <div className="h-1.5 w-full rounded-full bg-slate-900 overflow-hidden">
            {percent !== null ? (
              <div
                className="h-full bg-gradient-to-r from-indigo-500 to-emerald-400 transition-all"
                style={{ width: `${percent}%` }}
              />
            ) : (
              <div className="h-full w-full bg-gradient-to-r from-indigo-500/60 to-emerald-400/60 animate-progress-indeterminate" />
            )}
          </div>
        </div>
      )}

      {!installing && !ready && result && (
        <div className="text-xs space-y-3">
          <div className="flex flex-wrap gap-3">
            <div className="px-3 py-2 bg-slate-900 rounded-lg border border-slate-800">
              <span className="text-slate-500 block text-[11px]">{t('settings.updateCurrent')}</span>
              <span className="text-white font-mono font-semibold">{result.current_version}</span>
            </div>
            <div className="px-3 py-2 bg-slate-900 rounded-lg border border-slate-800">
              <span className="text-slate-500 block text-[11px]">{t('settings.updateLatest')}</span>
              <span
                className={`font-mono font-semibold ${result.available ? 'text-emerald-400' : 'text-slate-300'}`}
              >
                {result.latest_version}
              </span>
            </div>
          </div>

          {result.available ? (
            <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 space-y-3">
              <p className="text-xs text-emerald-300">
                {t('settings.updateAvailable', { version: result.latest_version })}
              </p>
              {!result.asset_found && (
                <p className="text-xs text-amber-300">{t('settings.updateNoAsset')}</p>
              )}
              <div className="flex flex-wrap items-center gap-2">
                {result.asset_found && (
                  <button
                    onClick={handleInstall}
                    disabled={checking}
                    className="flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium bg-emerald-600 hover:bg-emerald-500 text-white shadow-lg shadow-emerald-600/30 transition-all disabled:opacity-50"
                  >
                    <Rocket className="w-4 h-4" />
                    {t('settings.updateInstallButton', { version: result.latest_version })}
                  </button>
                )}
                {result.release?.html_url && (
                  <button
                    onClick={() => BrowserOpenURL(result.release!.html_url)}
                    className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-300 hover:text-white hover:bg-slate-800 border border-slate-700/60 transition"
                  >
                    <ExternalLink className="w-3.5 h-3.5" />
                    {t('settings.updateOpenRelease')}
                  </button>
                )}
              </div>
            </div>
          ) : (
            <p className="text-slate-400">
              {t('settings.updateUpToDate', { version: result.current_version })}
            </p>
          )}
        </div>
      )}
    </div>
  );
};
