import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { getRuntime } from '../runtime';
import { en, zh, type Translation } from './messages';

type Locale = 'en' | 'zh';
type TranslationKey = string;

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey, values?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

function resolve(messages: Translation, key: string): string | undefined {
  const value = key.split('.').reduce<unknown>((result, segment) => (result as Record<string, unknown>)?.[segment], messages);
  return typeof value === 'string' ? value : undefined;
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(getRuntime().language);
  useEffect(() => {
    document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en';
  }, [locale]);
  const value = useMemo<I18nContextValue>(() => ({
    locale, setLocale,
    t(key, values) {
      const messages = locale === 'zh' ? zh : en;
      const result = resolve(messages, key) ?? (key.startsWith('error.') ? messages.error.generic : key);
      return Object.entries(values ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), result);
    }
  }), [locale]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) throw new Error('useI18n must be used within I18nProvider');
  return value;
}
