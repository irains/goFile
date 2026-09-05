import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { I18nProvider } from '../i18n';
import { ApiError, api } from '../api/client';
import { FolderDestinationPicker } from './FolderDestinationPicker';

const getDirectories = vi.spyOn(api, 'getDirectories');

function renderPicker(value = '') {
  const onChange = vi.fn();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><I18nProvider><FolderDestinationPicker value={value} onChange={onChange} /></I18nProvider></QueryClientProvider>);
  return onChange;
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('FolderDestinationPicker', () => {
  it('browses child folders, exposes parent navigation, and reports the selected path', async () => {
    getDirectories.mockImplementation(async (path = '') => ({
      ok: true,
      path,
      dirs: path ? [{ name: 'Reports', path: `${path}/Reports` }] : [{ name: 'Documents', path: 'Documents' }]
    }));
    const onChange = renderPicker();

    await screen.findByRole('button', { name: 'Documents' });
    fireEvent.click(screen.getByRole('button', { name: 'Documents' }));
    await waitFor(() => expect(onChange).toHaveBeenCalledWith('Documents'));
    expect(await screen.findByLabelText('Destination: Documents')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Up one level' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Up one level' }));
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith(''));
  });

  it('shows an explicit empty state', async () => {
    getDirectories.mockResolvedValue({ ok: true, path: '', dirs: [] });
    renderPicker();

    expect(await screen.findByText('No subfolders here.')).toBeInTheDocument();
    expect(screen.getByText('This folder can be used as the destination.')).toBeInTheDocument();
  });

  it('offers retry after a directory request fails', async () => {
    getDirectories.mockRejectedValueOnce(new ApiError(500, 'io_error')).mockResolvedValueOnce({ ok: true, path: '', dirs: [] });
    renderPicker();

    expect(await screen.findByText('The server could not complete the file operation. Try again.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByText('No subfolders here.')).toBeInTheDocument();
  });
});
