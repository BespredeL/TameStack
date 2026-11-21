import React, { createContext, useContext, useState, useEffect } from 'react';
import { Language, Translations } from './types';
import { ru } from './ru';
import { en } from './en';
import { AISetAppLanguage } from '../../wailsjs/go/main/App';

interface LanguageContextProps {
  language: Language;
  setLanguage: (lang: Language) => void;
  t: (path: string, params?: Record<string, string | number>) => string;
  translations: Translations;
}

const dictionaries: Record<Language, Translations> = {
  ru,
  en,
};

const pluralRules: Record<Language, Intl.PluralRules> = {
  ru: new Intl.PluralRules('ru-RU'),
  en: new Intl.PluralRules('en-US'),
};

const resolvePath = (dict: any, path: string): string | null => {
  const keys = path.split('.');
  let current = dict;
  for (const key of keys) {
    if (current === undefined || current === null) {
      return null;
    }
    current = current[key];
  }
  if (typeof current === 'string') return current;
  if (current && typeof current === 'object') {
    if (typeof current.defaultTitle === 'string') return current.defaultTitle;
    if (typeof current.title === 'string') return current.title;
  }
  return null;
};

const LanguageContext = createContext<LanguageContextProps>({
  language: 'ru',
  setLanguage: () => {},
  t: (path: string) => path,
  translations: ru,
});

export const LanguageProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [language, setLanguageState] = useState<Language>(() => {
    const saved = localStorage.getItem('tamestack_lang');
    if (saved === 'ru' || saved === 'en') {
      return saved;
    }
    return 'ru';
  });

  useEffect(() => {
    AISetAppLanguage(language).catch(() => {});
  }, [language]);

  const setLanguage = (lang: Language) => {
    setLanguageState(lang);
    localStorage.setItem('tamestack_lang', lang);
    AISetAppLanguage(lang).catch(() => {});
  };

  const currentDict = dictionaries[language] || ru;

  // Resolves nested keys like "nav.servers"; with a numeric `count` prefers the
  // "<key>_<plural-category>" variant, then "<key>_other", then the bare key.
  const lookup = (lang: Language, path: string, count?: number): string | null => {
    const dict = dictionaries[lang];
    if (count !== undefined) {
      const exact = resolvePath(dict, `${path}_${pluralRules[lang].select(count)}`);
      if (exact !== null) return exact;
      const other = resolvePath(dict, `${path}_other`);
      if (other !== null) return other;
    }
    return resolvePath(dict, path);
  };

  const t = (path: string, params?: Record<string, string | number>): string => {
    const count = typeof params?.count === 'number' ? params.count : undefined;
    let text = lookup(language, path, count);

    if (text === null && language !== 'en') {
      text = lookup('en', path, count);
    }

    if (text === null) {
      if (import.meta.env.DEV) {
        console.warn(`[i18n] missing key "${path}" for language "${language}"`);
      }
      return path;
    }

    if (params) {
      for (const [k, v] of Object.entries(params)) {
        text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v));
      }
    }

    return text;
  };

  return (
    <LanguageContext.Provider
      value={{
        language,
        setLanguage,
        t,
        translations: currentDict,
      }}
    >
      {children}
    </LanguageContext.Provider>
  );
};

export const useTranslation = () => useContext(LanguageContext);
