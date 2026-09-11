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
  it('renders labelled appearance, palette, and language groups', () => {
    renderPanel();
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Appearance' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Color palette' })).toBeInTheDocument();
    expect(screen.getByText('Changes page surfaces and controls. Appearance chooses light or dark.')).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Language' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Forest' })).toBeChecked();
    expect(screen.getByRole('radio', { name: 'Graphite' })).toBeInTheDocument();
    expect(screen.getAllByRole('radio')).toHaveLength(17);
  });

  it('persists a validated full palette choice immediately', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    renderPanel();
    fireEvent.click(screen.getByRole('radio', { name: 'Graphite' }));
    expect(screen.getByRole('radio', { name: 'Graphite' })).toBeChecked();
    expect(document.documentElement).toHaveAttribute('data-fileharbor-accent', 'graphite');
    expect(setItem).toHaveBeenCalledWith('fileharbor-accent', 'graphite');
  });

  it('keeps palette previews out of the accessibility tree', () => {
    renderPanel();
    expect(document.querySelectorAll('[aria-hidden="true"]')).not.toHaveLength(0);
  });

  it('switches visible palette labels with the selected language', () => {
    renderPanel();
    fireEvent.click(screen.getByRole('radio', { name: '简体中文' }));
    expect(screen.getByRole('heading', { name: '设置' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: '配色方案' })).toBeInTheDocument();
    expect(screen.getByText('更改页面底色、面板和控件。外观决定浅色或深色。')).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: '港湾' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: '石墨' })).toBeInTheDocument();
  });
});
