import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import { UploadQueueDrawer } from './UploadQueueDrawer';

const snapshot = vi.fn<() => unknown[]>(() => []);

vi.mock('../uploads/storage', () => ({
  uploadScope: () => 'scope',
  listStoredUploads: vi.fn().mockResolvedValue([])
}));

vi.mock('../uploads/queue', () => ({
  ReliableUploadQueue: class {
    subscribe(listener: () => void) { listener(); return () => {}; }
    snapshot() { return snapshot(); }
    restore() {}
    add() {}
  }
}));

const completed = { id: 'complete', phase: 'completed', progress: 1, receivedBytes: 12, name: 'report.txt', size: 12, createdAt: 1 };
const cancelled = { id: 'cancelled', phase: 'cancelled', progress: 0, receivedBytes: 0, name: 'draft.txt', size: 8, createdAt: 2 };

const renderDrawer = (onAllComplete = vi.fn()) => render(<I18nProvider><UploadQueueDrawer open onClose={() => {}} destination="" username="operator" onAllComplete={onAllComplete} /></I18nProvider>);

describe('UploadQueueDrawer', () => {
  it('exposes a keyboard-operable drop zone that opens the file picker', () => {
    snapshot.mockReturnValue([]);
    renderDrawer();
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    expect(input).toBeTruthy();
    const click = vi.spyOn(input, 'click');
    const dropZone = screen.getByRole('button', { name: 'Drop files here or choose files to upload' });
    fireEvent.keyDown(dropZone, { key: 'Enter' });
    expect(click).toHaveBeenCalledOnce();
  });

  it('does not announce cancellation as a successful completed queue', async () => {
    snapshot.mockReturnValue([cancelled]);
    const onAllComplete = vi.fn();
    await act(async () => { renderDrawer(onAllComplete); });
    expect(screen.queryByText('All uploads are complete.')).not.toBeInTheDocument();
    expect(onAllComplete).not.toHaveBeenCalled();
  });

  it('announces a terminal queue only when it contains a completed upload', async () => {
    snapshot.mockReturnValue([completed, cancelled]);
    const onAllComplete = vi.fn();
    await act(async () => { renderDrawer(onAllComplete); });
    await waitFor(() => expect(screen.getByText('All uploads are complete.')).toBeInTheDocument());
    expect(onAllComplete).toHaveBeenCalledOnce();
  });
});
