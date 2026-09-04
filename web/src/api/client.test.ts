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

  it('uses stable server error codes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: false, code: 'csrf_invalid' }), { status: 403 })));
    await expect(api.getProperties('secret')).rejects.toEqual(expect.objectContaining({ status: 403, code: 'csrf_invalid' }));
  });
});
