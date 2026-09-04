import { describe, expect, it } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { I18nProvider, useI18n } from './index';

function LocaleToggle() {
  const { locale, setLocale, t } = useI18n();
  return <button onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}>{t('language.switchToChinese')}</button>;
}

describe('I18nProvider', () => {
  it('updates the document language when the locale changes', () => {
    render(<I18nProvider><LocaleToggle /></I18nProvider>);
    expect(document.documentElement.lang).toBe('en');
    act(() => screen.getByRole('button').click());
    expect(document.documentElement.lang).toBe('zh-CN');
  });

  it('keeps known file-operation errors actionable', () => {
    function ErrorMessages() {
      const { t } = useI18n();
      return <>{['destination_same_directory', 'self_descendant', 'cross_device_move', 'not_directory', 'batch_limit_exceeded', 'io_error'].map((code) => <span key={code}>{t(`error.${code}`)}</span>)}</>;
    }
    document.documentElement.lang = 'en';
    render(<I18nProvider><ErrorMessages /></I18nProvider>);
    expect(screen.getByText('Choose a different destination directory.')).toBeInTheDocument();
    expect(screen.getByText('A folder cannot be moved or copied into itself.')).toBeInTheDocument();
    expect(screen.getByText('This item cannot be moved across file systems. Copy it instead.')).toBeInTheDocument();
    expect(screen.getByText('Choose a directory as the destination.')).toBeInTheDocument();
    expect(screen.getByText('This batch is too large. Select fewer items and try again.')).toBeInTheDocument();
    expect(screen.getByText('The server could not complete the file operation. Try again.')).toBeInTheDocument();
  });

  it('falls back to the localized generic error for unknown API codes', () => {
    function UnknownError() {
      const { t } = useI18n();
      return <span>{t('error.http_599')}</span>;
    }
    document.documentElement.lang = 'en';
    render(<I18nProvider><UnknownError /></I18nProvider>);
    expect(screen.getByText('Something went wrong. Please try again.')).toBeInTheDocument();
  });
});
