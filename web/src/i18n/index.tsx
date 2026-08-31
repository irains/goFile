import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
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

function resolve(messages: Translation, key: string): string {
  return key.split('.').reduce<unknown>((value, segment) => (value as Record<string, unknown>)?.[segment], messages) as string ?? key;
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(getRuntime().language);
  const value = useMemo<I18nContextValue>(() => ({
    locale, setLocale,
    t(key, values) {
      const result = resolve(locale === 'zh' ? zh : en, key);
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
