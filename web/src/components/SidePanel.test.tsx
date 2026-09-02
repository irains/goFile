import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import { SidePanel, PanelHeader } from './SidePanel';

describe('SidePanel', () => {
  it('renders a header with close affordance and content', () => {
    render(
      <I18nProvider>
        <SidePanel open onClose={() => {}} icon={<span>icon</span>} title="Properties">
          <p>Details</p>
        </SidePanel>
      </I18nProvider>
    );
    expect(screen.getByText('Properties')).toBeInTheDocument();
    expect(screen.getByText('Details')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument();
  });
});

describe('PanelHeader', () => {
  it('renders icon, title, and a close icon', () => {
    render(
      <I18nProvider>
        <PanelHeader icon={<span>icon</span>} title="title" onClose={() => {}} />
      </I18nProvider>
    );
    expect(screen.getByText('title')).toBeInTheDocument();
    expect(screen.getByText('icon')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument();
  });
});