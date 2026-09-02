import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import { DialogShell } from './DialogShell';

describe('DialogShell', () => {
  it('renders title, content, and a cancel + confirm pair', () => {
    render(
      <I18nProvider>
        <DialogShell open title="Hello" onClose={() => {}} onConfirm={() => {}}>
          <p>Body</p>
        </DialogShell>
      </I18nProvider>
    );
    expect(screen.getByText('Hello')).toBeInTheDocument();
    expect(screen.getByText('Body')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument();
  });

  it('respects confirmTone="destructive"', () => {
    render(
      <I18nProvider>
        <DialogShell open title="x" confirmTone="destructive" confirmLabel="Delete" onClose={() => {}} onConfirm={() => {}}>
          <span />
        </DialogShell>
      </I18nProvider>
    );
    const confirm = screen.getByRole('button', { name: 'Delete' });
    expect(confirm.className).toContain('MuiButton-colorError');
  });
});