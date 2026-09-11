import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, setCSRFToken } from './client';

afterEach(() => {
  vi.unstubAllGlobals();
  setCSRFToken('');
  document.head.innerHTML = '';
});

describe('API client', () => {
  it('adds only in-memory CSRF and form data to browser mutations', async () => {
    setCSRFToken('csrf-value');
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await api.mutate('do/newdir', { path: 'docs', dirname: 'notes' });
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.method).toBe('POST');
    expect((init.headers as Record<string, string>)['X-CSRF-Token']).toBe('csrf-value');
    expect(init.body).toBeInstanceOf(URLSearchParams);
  });

  it('normalizes server bootstrap and listing DTOs', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, session: { username: 'admin', csrf_token: 'csrf', expires_at: '2026-08-30T12:00:00Z' }, base_path: '/fileharbor', locale: 'zh-CN', capabilities: { browse: true, upload: true, mutate: false, editor_save: false } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, directory: { path: 'docs', parent_path: '', listing_token: 'token', entries: [{ name: 'notes.txt', path: 'docs/notes.txt', kind: 'file', size_bytes: 3, modified_at: '2026-08-30T12:00:00Z', mode: '-rw-r--r--', is_archive: false, previewable: true, editable: true, version: 'v' }], truncated: false } }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await expect(api.login('admin', 'password')).resolves.toMatchObject({ username: 'admin', language: 'zh', capabilities: { editorSave: false } });
    await expect(api.getListing('docs')).resolves.toMatchObject({ path: 'docs', parentPath: '', listingToken: 'token', entries: [{ sizeBytes: 3, previewable: true, editable: true }] });
  });

  it('normalizes recycle-bin entries and sends CSRF-protected permanent confirmations', async () => {
    setCSRFToken('csrf-value');
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, entries: [{ id: '0123456789abcdef0123456789abcdef', name: 'notes.txt', original_path: 'docs/notes.txt', kind: 'file', size_bytes: 3, deleted_at: '2026-09-11T12:00:00Z' }], next_cursor: 'fedcba9876543210fedcba9876543210' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, affected: 1 }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.getTrash()).resolves.toEqual({
      entries: [{ id: '0123456789abcdef0123456789abcdef', name: 'notes.txt', originalPath: 'docs/notes.txt', kind: 'file', sizeBytes: 3, deletedAt: '2026-09-11T12:00:00Z' }],
      nextCursor: 'fedcba9876543210fedcba9876543210'
    });
    await api.purgeTrash('0123456789abcdef0123456789abcdef', 'DELETE');
    await api.emptyTrash('DELETE');

    const [purgePath, purgeInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(purgePath).toContain('api/trash/0123456789abcdef0123456789abcdef/purge');
    expect((purgeInit.headers as Record<string, string>)['X-CSRF-Token']).toBe('csrf-value');
    expect(purgeInit.body).toBe(JSON.stringify({ confirmation: 'DELETE' }));
    const [emptyPath, emptyInit] = fetchMock.mock.calls[2] as [string, RequestInit];
    expect(emptyPath).toContain('api/trash/empty');
    expect((emptyInit.headers as Record<string, string>)['X-CSRF-Token']).toBe('csrf-value');
    expect(emptyInit.body).toBe(JSON.stringify({ confirmation: 'DELETE' }));
  });

  it('uses stable server error codes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: false, code: 'csrf_invalid' }), { status: 403 })));
    await expect(api.getProperties('secret')).rejects.toEqual(expect.objectContaining({ status: 403, code: 'csrf_invalid' }));
  });
});
