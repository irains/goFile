import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { ThemePreferencesProvider } from '../appearance/ThemePreferencesProvider';
import { I18nProvider } from '../i18n';
import { SettingsPanel } from './SettingsPanel';

function renderPanel() {
  return render(
    <ThemePreferencesProvider>
      <I18nProvider>
        <SettingsPanel open onClose={() => {}} />
      </I18nProvider>
    </ThemePreferencesProvider>
  );
}

describe('SettingsPanel', () => {
  it('renders labelled mode, accent, and language groups', () => {
    renderPanel();
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Appearance' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Accent color' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Language' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Forest' })).toBeChecked();
    expect(screen.getAllByRole('radio')).toHaveLength(10);
  });

  it('persists a validated accent choice immediately', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    renderPanel();
    fireEvent.click(screen.getByRole('radio', { name: 'Harbor' }));
    expect(screen.getByRole('radio', { name: 'Harbor' })).toBeChecked();
    expect(document.documentElement).toHaveAttribute('data-fileharbor-accent', 'harbor');
    expect(setItem).toHaveBeenCalledWith('fileharbor-accent', 'harbor');
  });

  it('switches visible labels with the selected language', () => {
    renderPanel();
    fireEvent.click(screen.getByRole('radio', { name: '简体中文' }));
    expect(screen.getByRole('heading', { name: '设置' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: '主题色' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: '港湾' })).toBeInTheDocument();
  });
});
