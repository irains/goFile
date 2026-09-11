import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import { I18nProvider } from '../i18n';
import { RecycleBinPanel } from './RecycleBinPanel';

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return { ...actual, api: { ...actual.api, getTrash: vi.fn(), restoreTrash: vi.fn(), purgeTrash: vi.fn(), emptyTrash: vi.fn() } };
});

const getTrash = vi.mocked(api.getTrash);
const restoreTrash = vi.mocked(api.restoreTrash);
const purgeTrash = vi.mocked(api.purgeTrash);
const emptyTrash = vi.mocked(api.emptyTrash);
const entry = {
  id: '0123456789abcdef0123456789abcdef',
  name: 'notes.txt',
  originalPath: 'reports/notes.txt',
  kind: 'file' as const,
  sizeBytes: 12,
  deletedAt: '2026-09-11T10:00:00Z'
};

function renderPanel(props: Partial<React.ComponentProps<typeof RecycleBinPanel>> = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const values = {
    open: true,
    onClose: vi.fn(),
    mutable: true,
    currentDirectory: 'reports',
    onRestored: vi.fn().mockResolvedValue(undefined),
    onNotice: vi.fn(),
    ...props
  };
  return {
    ...render(<QueryClientProvider client={queryClient}><I18nProvider><RecycleBinPanel {...values} /></I18nProvider></QueryClientProvider>),
    ...values
  };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('RecycleBinPanel', () => {
  it('requires the exact DELETE confirmation before permanently deleting an entry', async () => {
    getTrash.mockResolvedValue({ entries: [entry] });
    purgeTrash.mockResolvedValue({ ok: true });
    renderPanel();

    await screen.findByText('notes.txt');
    fireEvent.click(screen.getByRole('button', { name: 'Permanently delete notes.txt' }));
    const dialog = screen.getByRole('dialog');
    const confirm = screen.getByRole('button', { name: 'Permanently delete' });
    expect(confirm).toBeDisabled();

    fireEvent.change(screen.getByRole('textbox', { name: 'Type DELETE to confirm.' }), { target: { value: 'delete' } });
    expect(confirm).toBeDisabled();
    fireEvent.change(screen.getByRole('textbox', { name: 'Type DELETE to confirm.' }), { target: { value: 'DELETE' } });
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);

    await waitFor(() => expect(purgeTrash).toHaveBeenCalledWith(entry.id, 'DELETE'));
    expect(dialog).toBeInTheDocument();
  });

  it('restores an entry, refreshes its visible original directory, and announces success', async () => {
    getTrash.mockResolvedValue({ entries: [entry] });
    restoreTrash.mockResolvedValue({ ok: true, path: entry.originalPath });
    const { onRestored, onNotice } = renderPanel();

    await screen.findByText('notes.txt');
    fireEvent.click(screen.getByRole('button', { name: 'Restore' }));

    await waitFor(() => expect(restoreTrash).toHaveBeenCalledWith(entry.id));
    await waitFor(() => expect(onRestored).toHaveBeenCalledWith('reports'));
    expect(onNotice).toHaveBeenCalledWith('notes.txt restored.', 'success');
  });

  it('keeps the recycle bin browsable but hides mutable controls in read-only mode', async () => {
    getTrash.mockResolvedValue({ entries: [entry] });
    renderPanel({ mutable: false });

    await screen.findByText('notes.txt');
    expect(screen.getByText('Original location: reports/notes.txt')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Restore' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Empty recycle bin' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Permanently delete notes.txt' })).not.toBeInTheDocument();
  });

  it('submits the exact DELETE confirmation when emptying the bin', async () => {
    getTrash.mockResolvedValue({ entries: [entry] });
    emptyTrash.mockResolvedValue({ ok: true, affected: 1 });
    renderPanel();

    await screen.findByText('notes.txt');
    fireEvent.click(screen.getByRole('button', { name: 'Empty recycle bin' }));
    fireEvent.change(screen.getByRole('textbox', { name: 'Type DELETE to confirm.' }), { target: { value: 'DELETE' } });
    fireEvent.click(screen.getByRole('button', { name: 'Permanently delete' }));

    await waitFor(() => expect(emptyTrash).toHaveBeenCalledWith('DELETE'));
  });
});
