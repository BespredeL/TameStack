import React, { useState, useEffect, useRef } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Eye, EyeOff, AlertTriangle, Loader2, ShieldCheck } from 'lucide-react';
import { UnlockDatabase } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import logoIcon from '../assets/images/logo.png';

interface DbUnlockScreenProps {
  onUnlocked: () => void;
}

export const DbUnlockScreen: React.FC<DbUnlockScreenProps> = ({ onUnlocked }) => {
  const { t } = useTranslation();

  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const timer = setTimeout(() => inputRef.current?.focus(), 150);
    return () => clearTimeout(timer);
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!password || isLoading) return;

    setIsLoading(true);
    setError(null);

    try {
      await UnlockDatabase(password);
      setPassword('');
      onUnlocked();
    } catch (err: unknown) {
      const msg = getErrorMessage(err);
      if (msg.includes('invalid master password or corrupted')) {
        setError(t('dbLock.invalidPassword'));
      } else {
        setError(msg);
      }
      setTimeout(() => inputRef.current?.focus(), 100);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 bg-slate-950 flex flex-col items-center justify-center p-4 select-none overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_80%_80%_at_50%_-20%,rgba(99,102,241,0.15),rgba(255,255,255,0))] pointer-events-none" />
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_60%_60%_at_50%_120%,rgba(16,185,129,0.08),rgba(255,255,255,0))] pointer-events-none" />

      <div className="relative w-full max-w-md bg-slate-900/90 border border-slate-800 rounded-2xl p-8 shadow-2xl shadow-black/80 backdrop-blur-xl animate-in fade-in zoom-in-95 duration-200">
        <div className="flex flex-col items-center text-center space-y-4 mb-8">
          <div className="relative">
            <div className="w-16 h-16 rounded-2xl bg-gradient-to-tr from-indigo-500/20 to-purple-500/20 border border-indigo-500/30 flex items-center justify-center p-2.5 shadow-xl shadow-indigo-500/10">
              <img src={logoIcon} alt="TameStack" className="w-full h-full object-contain rounded-xl drop-shadow" />
            </div>
            <div className="absolute -bottom-1 -right-1 w-6 h-6 rounded-full bg-slate-950 border border-indigo-500/40 flex items-center justify-center text-indigo-400 shadow">
              <ShieldCheck className="w-3.5 h-3.5" />
            </div>
          </div>

          <div>
            <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 mb-2">
              <img src={logoIcon} alt="" className="w-3.5 h-3.5 rounded" />
              <span>{t('vault.brandLabel')}</span>
            </div>
            <h1 className="text-xl font-bold text-white tracking-tight">
              {t('dbLock.title')}
            </h1>
            <p className="text-xs text-slate-400 mt-1 max-w-xs mx-auto leading-relaxed">
              {t('dbLock.hint')}
            </p>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <label className="block text-xs font-medium text-slate-300">
              {t('vault.masterPassword')}
            </label>
            <div className="relative">
              <input
                ref={inputRef}
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••••••"
                required
                disabled={isLoading}
                autoFocus
                className="w-full pl-4 pr-11 py-3 bg-slate-950/80 border border-slate-700/80 rounded-xl text-sm text-white placeholder-slate-600 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 font-mono transition"
              />
              <button
                type="button"
                onClick={() => setShowPassword((prev) => !prev)}
                className="absolute right-3.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200 transition p-1"
                tabIndex={-1}
              >
                {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {error && (
            <div className="p-3 bg-rose-950/60 border border-rose-500/40 rounded-xl text-xs text-rose-300 flex items-start gap-2.5 animate-in fade-in">
              <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
              <span className="leading-relaxed">{error}</span>
            </div>
          )}

          <button
            type="submit"
            disabled={isLoading || !password}
            className="w-full mt-2 py-3 px-4 rounded-xl text-sm font-bold text-white bg-gradient-to-r from-indigo-600 via-indigo-500 to-emerald-500 hover:from-indigo-500 hover:to-emerald-400 shadow-lg shadow-indigo-600/25 transition flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
          >
            {isLoading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>{t('dbLock.decrypting')}</span>
              </>
            ) : (
              <>
                <ShieldCheck className="w-4 h-4" />
                <span>{t('dbLock.unlockButton')}</span>
              </>
            )}
          </button>
        </form>

        <div className="mt-6 pt-4 border-t border-slate-800/80 text-center">
          <p className="text-[11px] text-slate-400">
            {t('dbLock.footerNotice')}
          </p>
        </div>
      </div>
    </div>
  );
};
