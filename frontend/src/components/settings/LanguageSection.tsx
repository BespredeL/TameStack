import React from 'react';
import { Globe } from 'lucide-react';
import { useTranslation } from '../../i18n/LanguageContext';

export const LanguageSection: React.FC = () => {
  const { language, setLanguage, t } = useTranslation();
  const isRu = language === 'ru';

  return (
    <div id="settings-language" className="scroll-mt-14 bg-surface-850 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
            <Globe className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-sm font-bold text-white">{t('common.language')}</h3>
            <p className="text-xs text-slate-400">
              {t('settings.languageSubtitle')}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 bg-slate-900 p-1 rounded-xl border border-slate-800">
          <button
            onClick={() => setLanguage('ru')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition ${
              isRu
                ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/30'
                : 'text-slate-400 hover:text-white'
            }`}
          >
            🇷🇺 {t('common.russian')}
          </button>
          <button
            onClick={() => setLanguage('en')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition ${
              !isRu
                ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/30'
                : 'text-slate-400 hover:text-white'
            }`}
          >
            🇬🇧 {t('common.english')}
          </button>
        </div>
      </div>
    </div>
  );
};