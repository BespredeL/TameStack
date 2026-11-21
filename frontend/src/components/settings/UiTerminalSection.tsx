import React, { useState } from 'react';
import { Terminal as TerminalIcon } from 'lucide-react';
import { useTranslation } from '../../i18n/LanguageContext';

export const UiTerminalSection: React.FC = () => {
  const { t } = useTranslation();

  const [termFontSize, setTermFontSize] = useState<number>(() => {
    return parseInt(localStorage.getItem('tamestack_term_fontsize') || '13', 10);
  });
  const [termCursor, setTermCursor] = useState<string>(() => {
    return localStorage.getItem('tamestack_term_cursor') || 'block';
  });
  const [refreshInterval, setRefreshInterval] = useState<number>(() => {
    return parseInt(localStorage.getItem('tamestack_refresh_interval') || '10', 10);
  });

  const updateTermFontSize = (size: number) => {
    setTermFontSize(size);
    localStorage.setItem('tamestack_term_fontsize', size.toString());
  };

  const updateTermCursor = (cursor: string) => {
    setTermCursor(cursor);
    localStorage.setItem('tamestack_term_cursor', cursor);
  };

  const updateRefreshInterval = (sec: number) => {
    setRefreshInterval(sec);
    localStorage.setItem('tamestack_refresh_interval', sec.toString());
  };

  return (
    <div id="settings-ui" className="scroll-mt-14 bg-surface-850 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-5">
      <div className="flex items-center gap-3">
        <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-400 border border-amber-500/20">
          <TerminalIcon className="w-5 h-5" />
        </div>
        <div>
          <h3 className="text-sm font-bold text-white">
            {t('settings.uiAndTerminal')}
          </h3>
          <p className="text-xs text-slate-400">
            {t('settings.uiAndTerminalSubtitle')}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4 text-xs">
        {/* Terminal Font Size */}
        <div className="p-4 bg-slate-900 rounded-lg border border-slate-800 space-y-2">
          <span className="text-slate-400 font-semibold block">
            {t('settings.fontSize')}
          </span>
          <div className="flex items-center gap-1.5">
            {[12, 13, 14, 16, 18].map((size) => (
              <button
                key={size}
                onClick={() => updateTermFontSize(size)}
                className={`px-2.5 py-1 rounded text-xs font-mono font-bold transition ${
                  termFontSize === size
                    ? 'bg-indigo-600 text-white shadow'
                    : 'bg-slate-800 text-slate-400 hover:text-white hover:bg-slate-700'
                }`}
              >
                {size}px
              </button>
            ))}
          </div>
          <p className="text-[11px] text-slate-500">
            {t('settings.fontSizeDefault')}
          </p>
        </div>

        {/* Cursor Style */}
        <div className="p-4 bg-slate-900 rounded-lg border border-slate-800 space-y-2">
          <span className="text-slate-400 font-semibold block">
            {t('settings.cursorStyle')}
          </span>
          <div className="flex items-center gap-1.5">
            {[
              { id: 'block', label: t('settings.cursorBlock') },
              { id: 'underline', label: t('settings.cursorUnderline') },
              { id: 'bar', label: t('settings.cursorBar') },
            ].map((c) => (
              <button
                key={c.id}
                onClick={() => updateTermCursor(c.id)}
                className={`px-2.5 py-1 rounded text-xs font-semibold transition ${
                  termCursor === c.id
                    ? 'bg-indigo-600 text-white shadow'
                    : 'bg-slate-800 text-slate-400 hover:text-white hover:bg-slate-700'
                }`}
              >
                {c.label}
              </button>
            ))}
          </div>
          <p className="text-[11px] text-slate-500">
            {t('settings.cursorInteractive')}
          </p>
        </div>

        {/* Telemetry Refresh Rate */}
        <div className="p-4 bg-slate-900 rounded-lg border border-slate-800 space-y-2">
          <span className="text-slate-400 font-semibold block">
            {t('settings.telemetryRefresh')}
          </span>
          <div className="flex items-center gap-1.5">
            {[
              { val: 5, label: '5s' },
              { val: 10, label: '10s' },
              { val: 30, label: '30s' },
              { val: 0, label: t('settings.telemetryOff') },
            ].map((item) => (
              <button
                key={item.val}
                onClick={() => updateRefreshInterval(item.val)}
                className={`px-2.5 py-1 rounded text-xs font-semibold transition ${
                  refreshInterval === item.val
                    ? 'bg-indigo-600 text-white shadow'
                    : 'bg-slate-800 text-slate-400 hover:text-white hover:bg-slate-700'
                }`}
              >
                {item.label}
              </button>
            ))}
          </div>
          <p className="text-[11px] text-slate-500">
            {t('settings.telemetrySubtitle')}
          </p>
        </div>
      </div>
    </div>
  );
};